/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudprovider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"

	corev1alpha1 "github.com/aws/karpenter-core/pkg/apis/v1alpha1"
	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awserrors "github.com/aws/karpenter/pkg/errors"
	"github.com/aws/karpenter/pkg/utils"

	"github.com/aws/karpenter-core/pkg/utils/resources"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
)

var (
	instanceTypeFlexibilityThreshold = 5 // falling back to on-demand without flexibility risks insufficient capacity errors
)

type InstanceProvider struct {
	ec2api                 ec2iface.EC2API
	instanceTypeProvider   *InstanceTypeProvider
	subnetProvider         *SubnetProvider
	launchTemplateProvider *LaunchTemplateProvider
	createFleetBatcher     *CreateFleetBatcher
}

func NewInstanceProvider(ctx context.Context, ec2api ec2iface.EC2API, instanceTypeProvider *InstanceTypeProvider, subnetProvider *SubnetProvider, launchTemplateProvider *LaunchTemplateProvider) *InstanceProvider {
	return &InstanceProvider{
		ec2api:                 ec2api,
		instanceTypeProvider:   instanceTypeProvider,
		subnetProvider:         subnetProvider,
		launchTemplateProvider: launchTemplateProvider,
		createFleetBatcher:     NewCreateFleetBatcher(ctx, ec2api),
	}
}

// Create an instance given the constraints.
// instanceTypes should be sorted by priority for spot capacity type.
// If spot is not used, the instanceTypes are not required to be sorted
// because we are using ec2 fleet's lowest-price OD allocation strategy
func (p *InstanceProvider) Create(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, machine *corev1alpha1.Machine, instanceTypes []*cloudprovider.InstanceType) (*v1.Node, error) {
	instanceTypes = filterInstanceTypes(instanceTypes)
	instanceTypes = orderInstanceTypesByPrice(instanceTypes, scheduling.NewNodeSelectorRequirements(machine.Spec.Requirements...))
	if len(instanceTypes) > MaxInstanceTypes {
		instanceTypes = instanceTypes[0:MaxInstanceTypes]
	}

	id, err := p.launchInstance(ctx, nodeTemplate, machine, instanceTypes)
	if awserrors.IsLaunchTemplateNotFound(err) {
		// retry once if launch template is not found. This allows karpenter to generate a new LT if the
		// cache was out-of-sync on the first try
		id, err = p.launchInstance(ctx, nodeTemplate, machine, instanceTypes)
	}
	if err != nil {
		return nil, err
	}
	// Get Instance with backoff retry since EC2 is eventually consistent
	instance := &ec2.Instance{}
	if err := retry.Do(
		func() (err error) { instance, err = p.getInstance(ctx, aws.StringValue(id)); return err },
		retry.Delay(1*time.Second),
		retry.Attempts(6),
		retry.LastErrorOnly(true),
	); err != nil {
		return nil, fmt.Errorf("retrieving node name for instance %s, %w", aws.StringValue(instance.InstanceId), err)
	}
	logging.FromContext(ctx).With(
		"launched-instance", aws.StringValue(instance.InstanceId),
		"hostname", aws.StringValue(instance.PrivateDnsName),
		"type", aws.StringValue(instance.InstanceType),
		"zone", aws.StringValue(instance.Placement.AvailabilityZone),
		"capacity-type", getCapacityType(instance)).Infof("launched new instance")

	// Convert Instance to Node
	return p.instanceToNode(instance, instanceTypes), nil
}

func (p *InstanceProvider) Terminate(ctx context.Context, node *v1.Node) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("node", node.Name))
	id, err := utils.ParseInstanceID(node)
	if err != nil {
		return fmt.Errorf("getting instance ID for node %s, %w", node.Name, err)
	}
	if _, err = p.ec2api.TerminateInstancesWithContext(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []*string{id},
	}); err != nil {
		if awserrors.IsNotFound(err) {
			return nil
		}
		if _, errMsg := p.getInstance(ctx, aws.StringValue(id)); err != nil {
			if awserrors.IsInstanceTerminated(errMsg) || awserrors.IsNotFound(errMsg) {
				logging.FromContext(ctx).Debugf("instance already terminated")
				return nil
			}
			err = multierr.Append(err, errMsg)
		}

		return fmt.Errorf("terminating instance %s, %w", node.Name, err)
	}
	return nil
}

// can remove cyclo ignore after China launch price-capacity-optimized
// nolint: gocyclo
func (p *InstanceProvider) launchInstance(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, machine *corev1alpha1.Machine, instanceTypes []*cloudprovider.InstanceType) (*string, error) {
	capacityType := p.getCapacityType(machine, instanceTypes)
	// Get Launch Template Configs, which may differ due to GPU or Architecture requirements
	launchTemplateConfigs, err := p.getLaunchTemplateConfigs(ctx, nodeTemplate, machine, instanceTypes, capacityType)
	if err != nil {
		return nil, fmt.Errorf("getting launch template configs, %w", err)
	}
	if err := p.checkODFallback(machine, instanceTypes, launchTemplateConfigs); err != nil {
		logging.FromContext(ctx).Warn(err.Error())
	}
	// Create fleet
	tags := v1alpha1.MergeTags(ctx, awssettings.FromContext(ctx).Tags, nodeTemplate.Spec.Tags, map[string]string{fmt.Sprintf("kubernetes.io/cluster/%s", awssettings.FromContext(ctx).ClusterName): "owned"})
	createFleetInput := &ec2.CreateFleetInput{
		Type:                  aws.String(ec2.FleetTypeInstant),
		Context:               nodeTemplate.Spec.Context,
		LaunchTemplateConfigs: launchTemplateConfigs,
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(capacityType),
			TotalTargetCapacity:       aws.Int64(1),
		},
		TagSpecifications: []*ec2.TagSpecification{
			{ResourceType: aws.String(ec2.ResourceTypeInstance), Tags: tags},
			{ResourceType: aws.String(ec2.ResourceTypeVolume), Tags: tags},
			{ResourceType: aws.String(ec2.ResourceTypeFleet), Tags: tags},
		},
	}
	if capacityType == v1alpha5.CapacityTypeSpot && strings.HasPrefix(p.instanceTypeProvider.region, "cn-") {
		createFleetInput.SpotOptions = &ec2.SpotOptionsRequest{AllocationStrategy: aws.String(ec2.AllocationStrategyCapacityOptimizedPrioritized)}
	} else if capacityType == v1alpha5.CapacityTypeSpot {
		createFleetInput.SpotOptions = &ec2.SpotOptionsRequest{AllocationStrategy: aws.String(ec2.AllocationStrategyPriceCapacityOptimized)}
	} else {
		createFleetInput.OnDemandOptions = &ec2.OnDemandOptionsRequest{AllocationStrategy: aws.String(ec2.FleetOnDemandAllocationStrategyLowestPrice)}
	}

	createFleetOutput, err := p.createFleetBatcher.CreateFleet(ctx, createFleetInput)
	if err != nil {
		if awserrors.IsLaunchTemplateNotFound(err) {
			for _, lt := range launchTemplateConfigs {
				p.launchTemplateProvider.Invalidate(ctx, aws.StringValue(lt.LaunchTemplateSpecification.LaunchTemplateName), aws.StringValue(lt.LaunchTemplateSpecification.LaunchTemplateId))
			}
			return nil, fmt.Errorf("creating fleet %w", err)
		}
		var reqFailure awserr.RequestFailure
		if errors.As(err, &reqFailure) {
			return nil, fmt.Errorf("creating fleet %w (%s)", err, reqFailure.RequestID())
		}
		return nil, fmt.Errorf("creating fleet %w", err)
	}
	p.updateUnavailableOfferingsCache(ctx, createFleetOutput.Errors, capacityType)
	if len(createFleetOutput.Instances) == 0 || len(createFleetOutput.Instances[0].InstanceIds) == 0 {
		return nil, combineFleetErrors(createFleetOutput.Errors)
	}
	return createFleetOutput.Instances[0].InstanceIds[0], nil
}

func (p *InstanceProvider) checkODFallback(machine *corev1alpha1.Machine, instanceTypes []*cloudprovider.InstanceType, launchTemplateConfigs []*ec2.FleetLaunchTemplateConfigRequest) error {
	// only evaluate for on-demand fallback if the capacity type for the request is OD and both OD and spot are allowed in requirements
	if p.getCapacityType(machine, instanceTypes) != v1alpha5.CapacityTypeOnDemand || !scheduling.NewNodeSelectorRequirements(machine.Spec.Requirements...).Get(v1alpha5.LabelCapacityType).Has(v1alpha5.CapacityTypeSpot) {
		return nil
	}

	// loop through the LT configs for currently considered instance types to get the flexibility count
	instanceTypeZones := map[string]struct{}{}
	for _, ltc := range launchTemplateConfigs {
		for _, override := range ltc.Overrides {
			if override.InstanceType != nil {
				instanceTypeZones[*override.InstanceType] = struct{}{}
			}
		}
	}
	if len(instanceTypes) < instanceTypeFlexibilityThreshold {
		return fmt.Errorf("at least %d instance types are recommended when flexible to spot but requesting on-demand, "+
			"the current provisioning request only has %d instance type options", instanceTypeFlexibilityThreshold, len(instanceTypes))
	}
	return nil
}

func (p *InstanceProvider) getLaunchTemplateConfigs(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, machine *corev1alpha1.Machine,
	instanceTypes []*cloudprovider.InstanceType, capacityType string) ([]*ec2.FleetLaunchTemplateConfigRequest, error) {

	// Get subnets given the constraints
	subnets, err := p.subnetProvider.Get(ctx, nodeTemplate)
	if err != nil {
		return nil, fmt.Errorf("getting subnets, %w", err)
	}
	var launchTemplateConfigs []*ec2.FleetLaunchTemplateConfigRequest
	launchTemplates, err := p.launchTemplateProvider.Get(ctx, nodeTemplate, machine, instanceTypes, map[string]string{v1alpha5.LabelCapacityType: capacityType})
	if err != nil {
		return nil, fmt.Errorf("getting launch templates, %w", err)
	}
	for launchTemplateName, instanceTypes := range launchTemplates {
		launchTemplateConfig := &ec2.FleetLaunchTemplateConfigRequest{
			Overrides: p.getOverrides(instanceTypes, subnets, scheduling.NewNodeSelectorRequirements(machine.Spec.Requirements...).Get(v1.LabelTopologyZone), capacityType),
			LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: aws.String(launchTemplateName),
				Version:            aws.String("$Latest"),
			},
		}
		if len(launchTemplateConfig.Overrides) > 0 {
			launchTemplateConfigs = append(launchTemplateConfigs, launchTemplateConfig)
		}
	}
	if len(launchTemplateConfigs) == 0 {
		return nil, fmt.Errorf("no capacity offerings are currently available given the constraints")
	}
	return launchTemplateConfigs, nil
}

// getOverrides creates and returns launch template overrides for the cross product of InstanceTypes and subnets (with subnets being constrained by
// zones and the offerings in InstanceTypes)
func (p *InstanceProvider) getOverrides(instanceTypes []*cloudprovider.InstanceType, subnets []*ec2.Subnet, zones *scheduling.Requirement, capacityType string) []*ec2.FleetLaunchTemplateOverridesRequest {
	// sort subnets in ascending order of available IP addresses and populate map with most available subnet per AZ
	zonalSubnets := map[string]*ec2.Subnet{}
	sort.Slice(subnets, func(i, j int) bool {
		return aws.Int64Value(subnets[i].AvailableIpAddressCount) < aws.Int64Value(subnets[j].AvailableIpAddressCount)
	})
	for _, subnet := range subnets {
		zonalSubnets[*subnet.AvailabilityZone] = subnet
	}

	// Unwrap all the offerings to a flat slice that includes a pointer
	// to the parent instance type name
	type offeringWithParentName struct {
		cloudprovider.Offering
		parentInstanceTypeName string
	}
	var unwrappedOfferings []offeringWithParentName
	for _, it := range instanceTypes {
		ofs := lo.Map(it.Offerings.Available(), func(of cloudprovider.Offering, _ int) offeringWithParentName {
			return offeringWithParentName{
				Offering:               of,
				parentInstanceTypeName: it.Name,
			}
		})
		unwrappedOfferings = append(unwrappedOfferings, ofs...)
	}

	// Sort all the potential offerings by each individual offering price
	sort.Slice(unwrappedOfferings, func(i, j int) bool {
		return unwrappedOfferings[i].Price < unwrappedOfferings[j].Price
	})

	var overrides []*ec2.FleetLaunchTemplateOverridesRequest
	for i, offering := range unwrappedOfferings {
		if capacityType != offering.CapacityType {
			continue
		}
		if !zones.Has(offering.Zone) {
			continue
		}
		subnet, ok := zonalSubnets[offering.Zone]
		if !ok {
			continue
		}
		override := &ec2.FleetLaunchTemplateOverridesRequest{
			InstanceType: aws.String(offering.parentInstanceTypeName),
			SubnetId:     subnet.SubnetId,
			// This is technically redundant, but is useful if we have to parse insufficient capacity errors from
			// CreateFleet so that we can figure out the zone rather than additional API calls to look up the subnet
			AvailabilityZone: subnet.AvailabilityZone,
		}

		// For China regions (until price-capacity-optimized is released)
		// Add a priority for spot requests since we are using the capacity-optimized-prioritized spot allocation strategy
		// to reduce the likelihood of getting an excessively large instance type.
		// InstanceTypes are sorted by vcpus and memory so this prioritizes smaller instance types.
		if capacityType == v1alpha5.CapacityTypeSpot && strings.HasPrefix(p.instanceTypeProvider.region, "cn-") {
			override.Priority = aws.Float64(float64(i))
		}

		overrides = append(overrides, override)
	}
	return overrides
}

func (p *InstanceProvider) getInstance(ctx context.Context, id string) (*ec2.Instance, error) {
	describeInstancesOutput, err := p.ec2api.DescribeInstancesWithContext(ctx, &ec2.DescribeInstancesInput{InstanceIds: aws.StringSlice([]string{id})})
	if awserrors.IsNotFound(err) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("failed to describe ec2 instances, %w", err)
	}
	if len(describeInstancesOutput.Reservations) != 1 || len(describeInstancesOutput.Reservations[0].Instances) != 1 {
		return nil, awserrors.InstanceTerminatedError{Err: fmt.Errorf("expected instance but got 0")}
	}
	instance := describeInstancesOutput.Reservations[0].Instances[0]
	if *instance.State.Name == ec2.InstanceStateNameTerminated {
		return nil, awserrors.InstanceTerminatedError{Err: fmt.Errorf("instance is in terminated state")}
	}
	if len(aws.StringValue(instance.PrivateDnsName)) == 0 {
		return nil, multierr.Append(err, fmt.Errorf("got instance %s but PrivateDnsName was not set", aws.StringValue(instance.InstanceId)))
	}
	return instance, nil
}

func (p *InstanceProvider) instanceToNode(instance *ec2.Instance, instanceTypes []*cloudprovider.InstanceType) *v1.Node {
	for _, instanceType := range instanceTypes {
		if instanceType.Name == aws.StringValue(instance.InstanceType) {
			nodeName := strings.ToLower(aws.StringValue(instance.PrivateDnsName))
			labels := map[string]string{}
			for key, req := range instanceType.Requirements {
				if req.Len() == 1 {
					labels[key] = req.Values()[0]
				}
			}
			labels[v1alpha1.LabelInstanceAMIID] = aws.StringValue(instance.ImageId)
			labels[v1.LabelTopologyZone] = aws.StringValue(instance.Placement.AvailabilityZone)
			labels[v1alpha5.LabelCapacityType] = getCapacityType(instance)

			return &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   nodeName,
					Labels: labels,
				},
				Spec: v1.NodeSpec{
					ProviderID: fmt.Sprintf("aws:///%s/%s", aws.StringValue(instance.Placement.AvailabilityZone), aws.StringValue(instance.InstanceId)),
				},
			}
		}
	}
	panic(fmt.Sprintf("unrecognized instance type %s", aws.StringValue(instance.InstanceType)))
}

func (p *InstanceProvider) updateUnavailableOfferingsCache(ctx context.Context, errors []*ec2.CreateFleetError, capacityType string) {
	for _, err := range errors {
		if awserrors.IsUnfulfillableCapacity(err) {
			p.instanceTypeProvider.unavailableOfferings.MarkUnavailableForFleetErr(ctx, err, capacityType)
		}
	}
}

// getCapacityType selects spot if both constraints are flexible and there is an
// available offering. The AWS Cloud Provider defaults to [ on-demand ], so spot
// must be explicitly included in capacity type requirements.
func (p *InstanceProvider) getCapacityType(machine *corev1alpha1.Machine, instanceTypes []*cloudprovider.InstanceType) string {
	requirements := scheduling.NewNodeSelectorRequirements(machine.
		Spec.Requirements...)
	if requirements.Get(v1alpha5.LabelCapacityType).Has(v1alpha5.CapacityTypeSpot) {
		for _, instanceType := range instanceTypes {
			for _, offering := range instanceType.Offerings.Available() {
				if requirements.Get(v1.LabelTopologyZone).Has(offering.Zone) && offering.CapacityType == v1alpha5.CapacityTypeSpot {
					return v1alpha5.CapacityTypeSpot
				}
			}
		}
	}
	return v1alpha5.CapacityTypeOnDemand
}

func orderInstanceTypesByPrice(instanceTypes []*cloudprovider.InstanceType, requirements scheduling.Requirements) []*cloudprovider.InstanceType {
	// Order instance types so that we get the cheapest instance types of the available offerings
	sort.Slice(instanceTypes, func(i, j int) bool {
		iPrice := math.MaxFloat64
		jPrice := math.MaxFloat64
		if len(instanceTypes[i].Offerings.Available().Requirements(requirements)) > 0 {
			iPrice = instanceTypes[i].Offerings.Available().Requirements(requirements).Cheapest().Price
		}
		if len(instanceTypes[j].Offerings.Available().Requirements(requirements)) > 0 {
			jPrice = instanceTypes[j].Offerings.Available().Requirements(requirements).Cheapest().Price
		}
		if iPrice == jPrice {
			return instanceTypes[i].Name < instanceTypes[j].Name
		}
		return iPrice < jPrice
	})
	return instanceTypes
}

// filterInstanceTypes is used to eliminate less desirable instance types (like GPUs) from the list of possible instance types when
// a set of more appropriate instance types would work. If a set of more desirable instance types is not found, then the original slice
// of instance types are returned.
func filterInstanceTypes(instanceTypes []*cloudprovider.InstanceType) []*cloudprovider.InstanceType {
	var genericInstanceTypes []*cloudprovider.InstanceType
	for _, it := range instanceTypes {
		// deprioritize metal even if our opinionated filter isn't applied due to something like an instance family
		// requirement
		if it.Requirements.Get(v1alpha1.LabelInstanceSize).Has("metal") {
			continue
		}
		if !resources.IsZero(it.Capacity[v1alpha1.ResourceAWSNeuron]) ||
			!resources.IsZero(it.Capacity[v1alpha1.ResourceAMDGPU]) ||
			!resources.IsZero(it.Capacity[v1alpha1.ResourceNVIDIAGPU]) ||
			!resources.IsZero(it.Capacity[v1alpha1.ResourceHabanaGaudi]) {
			continue
		}
		genericInstanceTypes = append(genericInstanceTypes, it)
	}
	// if we got some subset of instance types, then prefer to use those
	if len(genericInstanceTypes) != 0 {
		return genericInstanceTypes
	}
	return instanceTypes
}

func combineFleetErrors(errors []*ec2.CreateFleetError) (errs error) {
	unique := sets.NewString()
	for _, err := range errors {
		unique.Insert(fmt.Sprintf("%s: %s", aws.StringValue(err.ErrorCode), aws.StringValue(err.ErrorMessage)))
	}
	for errorCode := range unique {
		errs = multierr.Append(errs, fmt.Errorf(errorCode))
	}
	return fmt.Errorf("with fleet error(s), %w", errs)
}

func getCapacityType(instance *ec2.Instance) string {
	if instance.SpotInstanceRequestId != nil {
		return v1alpha5.CapacityTypeSpot
	}
	return v1alpha5.CapacityTypeOnDemand
}
