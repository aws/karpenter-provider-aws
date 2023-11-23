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

package instance

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/utils/resources"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/batcher"
	"github.com/aws/karpenter/pkg/cache"
	awserrors "github.com/aws/karpenter/pkg/errors"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/providers/instancetype"
	"github.com/aws/karpenter/pkg/providers/launchtemplate"
	"github.com/aws/karpenter/pkg/providers/subnet"
	"github.com/aws/karpenter/pkg/utils"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
)

var (
	// MaxInstanceTypes defines the number of instance type options to pass to CreateFleet
	MaxInstanceTypes                 = 60
	instanceTypeFlexibilityThreshold = 5 // falling back to on-demand without flexibility risks insufficient capacity errors

	instanceStateFilter = &ec2.Filter{
		Name:   aws.String("instance-state-name"),
		Values: aws.StringSlice([]string{ec2.InstanceStateNamePending, ec2.InstanceStateNameRunning, ec2.InstanceStateNameStopping, ec2.InstanceStateNameStopped, ec2.InstanceStateNameShuttingDown}),
	}
)

type Provider struct {
	region                 string
	ec2api                 ec2iface.EC2API
	unavailableOfferings   *cache.UnavailableOfferings
	instanceTypeProvider   *instancetype.Provider
	subnetProvider         *subnet.Provider
	launchTemplateProvider *launchtemplate.Provider
	ec2Batcher             *batcher.EC2API
}

func NewProvider(ctx context.Context, region string, ec2api ec2iface.EC2API, unavailableOfferings *cache.UnavailableOfferings,
	instanceTypeProvider *instancetype.Provider, subnetProvider *subnet.Provider, launchTemplateProvider *launchtemplate.Provider) *Provider {
	return &Provider{
		region:                 region,
		ec2api:                 ec2api,
		unavailableOfferings:   unavailableOfferings,
		instanceTypeProvider:   instanceTypeProvider,
		subnetProvider:         subnetProvider,
		launchTemplateProvider: launchTemplateProvider,
		ec2Batcher:             batcher.EC2(ctx, ec2api),
	}
}

func (p *Provider) Create(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, nodeClaim *corev1beta1.NodeClaim, instanceTypes []*cloudprovider.InstanceType) (*Instance, error) {
	instanceTypes = p.filterInstanceTypes(nodeClaim, instanceTypes)
	instanceTypes = orderInstanceTypesByPrice(instanceTypes, scheduling.NewNodeSelectorRequirements(nodeClaim.Spec.Requirements...))
	if len(instanceTypes) > MaxInstanceTypes {
		instanceTypes = instanceTypes[0:MaxInstanceTypes]
	}
	tags := getTags(ctx, nodeClass, nodeClaim)
	fleetInstance, err := p.launchInstance(ctx, nodeClass, nodeClaim, instanceTypes, tags)
	if awserrors.IsLaunchTemplateNotFound(err) {
		// retry once if launch template is not found. This allows karpenter to generate a new LT if the
		// cache was out-of-sync on the first try
		fleetInstance, err = p.launchInstance(ctx, nodeClass, nodeClaim, instanceTypes, tags)
	}
	if err != nil {
		return nil, err
	}
	return NewInstanceFromFleet(fleetInstance, tags), nil
}

func (p *Provider) Link(ctx context.Context, id, provisionerName string) error {
	if err := p.CreateTags(ctx, id, map[string]string{
		v1alpha5.MachineManagedByAnnotationKey: options.FromContext(ctx).ClusterName,
		v1alpha5.ProvisionerNameLabelKey:       provisionerName,
	}); err != nil {
		return fmt.Errorf("linking tags, %w", err)
	}
	return nil
}

func (p *Provider) Get(ctx context.Context, id string) (*Instance, error) {
	out, err := p.ec2Batcher.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{id}),
		Filters:     []*ec2.Filter{instanceStateFilter},
	})
	if awserrors.IsNotFound(err) {
		return nil, cloudprovider.NewNodeClaimNotFoundError(err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to describe ec2 instances, %w", err)
	}
	instances, err := instancesFromOutput(out)
	if err != nil {
		return nil, fmt.Errorf("getting instances from output, %w", err)
	}
	if len(instances) != 1 {
		return nil, fmt.Errorf("expected a single instance, %w", err)
	}
	return instances[0], nil
}

func (p *Provider) List(ctx context.Context) ([]*Instance, error) {
	var out = &ec2.DescribeInstancesOutput{}
	err := p.ec2api.DescribeInstancesPagesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: aws.StringSlice([]string{v1alpha5.ProvisionerNameLabelKey, corev1beta1.NodePoolLabelKey}),
			},
			{
				Name:   aws.String("tag-key"),
				Values: aws.StringSlice([]string{fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName)}),
			},
			instanceStateFilter,
		},
	}, func(page *ec2.DescribeInstancesOutput, _ bool) bool {
		out.Reservations = append(out.Reservations, page.Reservations...)
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("describing ec2 instances, %w", err)
	}
	instances, err := instancesFromOutput(out)
	return instances, cloudprovider.IgnoreNodeClaimNotFoundError(err)
}

func (p *Provider) Delete(ctx context.Context, id string) error {
	if _, err := p.ec2Batcher.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(id)},
	}); err != nil {
		if awserrors.IsNotFound(err) {
			return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance already terminated"))
		}
		if _, e := p.Get(ctx, id); err != nil {
			if cloudprovider.IsNodeClaimNotFoundError(e) {
				return e
			}
			err = multierr.Append(err, e)
		}
		return fmt.Errorf("terminating instance, %w", err)
	}
	return nil
}

func (p *Provider) CreateTags(ctx context.Context, id string, tags map[string]string) error {
	ec2Tags := lo.MapToSlice(tags, func(key, value string) *ec2.Tag {
		return &ec2.Tag{Key: aws.String(key), Value: aws.String(value)}
	})
	if _, err := p.ec2api.CreateTagsWithContext(ctx, &ec2.CreateTagsInput{
		Resources: aws.StringSlice([]string{id}),
		Tags:      ec2Tags,
	}); err != nil {
		if awserrors.IsNotFound(err) {
			return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("tagging instance, %w", err))
		}
		return fmt.Errorf("tagging instance, %w", err)
	}
	return nil
}

func (p *Provider) launchInstance(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, nodeClaim *corev1beta1.NodeClaim, instanceTypes []*cloudprovider.InstanceType, tags map[string]string) (*ec2.CreateFleetInstance, error) {
	capacityType := p.getCapacityType(nodeClaim, instanceTypes)
	zonalSubnets, err := p.subnetProvider.ZonalSubnetsForLaunch(ctx, nodeClass, instanceTypes, capacityType)
	if err != nil {
		return nil, fmt.Errorf("getting subnets, %w", err)
	}

	// Get Launch Template Configs, which may differ due to GPU or Architecture requirements
	launchTemplateConfigs, err := p.getLaunchTemplateConfigs(ctx, nodeClass, nodeClaim, instanceTypes, zonalSubnets, capacityType, tags)
	if err != nil {
		return nil, fmt.Errorf("getting launch template configs, %w", err)
	}
	if err := p.checkODFallback(nodeClaim, instanceTypes, launchTemplateConfigs); err != nil {
		logging.FromContext(ctx).Warn(err.Error())
	}
	// Create fleet
	createFleetInput := &ec2.CreateFleetInput{
		Type:                  aws.String(ec2.FleetTypeInstant),
		Context:               nodeClass.Spec.Context,
		LaunchTemplateConfigs: launchTemplateConfigs,
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(capacityType),
			TotalTargetCapacity:       aws.Int64(1),
		},
		TagSpecifications: []*ec2.TagSpecification{
			{ResourceType: aws.String(ec2.ResourceTypeInstance), Tags: utils.MergeTags(tags)},
			{ResourceType: aws.String(ec2.ResourceTypeVolume), Tags: utils.MergeTags(tags)},
			{ResourceType: aws.String(ec2.ResourceTypeFleet), Tags: utils.MergeTags(tags)},
		},
	}
	if capacityType == corev1beta1.CapacityTypeSpot {
		createFleetInput.SpotOptions = &ec2.SpotOptionsRequest{AllocationStrategy: aws.String(ec2.SpotAllocationStrategyPriceCapacityOptimized)}
	} else {
		createFleetInput.OnDemandOptions = &ec2.OnDemandOptionsRequest{AllocationStrategy: aws.String(ec2.FleetOnDemandAllocationStrategyLowestPrice)}
	}

	createFleetOutput, err := p.ec2Batcher.CreateFleet(ctx, createFleetInput)
	p.subnetProvider.UpdateInflightIPs(createFleetInput, createFleetOutput, instanceTypes, lo.Values(zonalSubnets), capacityType)
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
	return createFleetOutput.Instances[0], nil
}

func getTags(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, nodeClaim *corev1beta1.NodeClaim) map[string]string {
	var overridableTags, staticTags map[string]string
	if nodeClaim.IsMachine {
		overridableTags = map[string]string{
			"Name": fmt.Sprintf("%s/%s", v1alpha5.ProvisionerNameLabelKey, nodeClaim.Labels[v1alpha5.ProvisionerNameLabelKey]),
		}
		staticTags = map[string]string{
			fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName): "owned",
			v1alpha5.ProvisionerNameLabelKey:                                              nodeClaim.Labels[v1alpha5.ProvisionerNameLabelKey],
			v1alpha5.MachineManagedByAnnotationKey:                                        options.FromContext(ctx).ClusterName,
		}
	} else {
		staticTags = map[string]string{
			fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName): "owned",
			corev1beta1.NodePoolLabelKey:       nodeClaim.Labels[corev1beta1.NodePoolLabelKey],
			corev1beta1.ManagedByAnnotationKey: options.FromContext(ctx).ClusterName,
			v1beta1.LabelNodeClass:             nodeClass.Name,
		}
	}
	return lo.Assign(overridableTags, settings.FromContext(ctx).Tags, nodeClass.Spec.Tags, staticTags)
}

func (p *Provider) checkODFallback(nodeClaim *corev1beta1.NodeClaim, instanceTypes []*cloudprovider.InstanceType, launchTemplateConfigs []*ec2.FleetLaunchTemplateConfigRequest) error {
	// only evaluate for on-demand fallback if the capacity type for the request is OD and both OD and spot are allowed in requirements
	if p.getCapacityType(nodeClaim, instanceTypes) != corev1beta1.CapacityTypeOnDemand || !scheduling.NewNodeSelectorRequirements(nodeClaim.Spec.Requirements...).Get(corev1beta1.CapacityTypeLabelKey).Has(corev1beta1.CapacityTypeSpot) {
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

func (p *Provider) getLaunchTemplateConfigs(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, nodeClaim *corev1beta1.NodeClaim,
	instanceTypes []*cloudprovider.InstanceType, zonalSubnets map[string]*ec2.Subnet, capacityType string, tags map[string]string) ([]*ec2.FleetLaunchTemplateConfigRequest, error) {
	var launchTemplateConfigs []*ec2.FleetLaunchTemplateConfigRequest
	launchTemplates, err := p.launchTemplateProvider.EnsureAll(ctx, nodeClass, nodeClaim, instanceTypes, capacityType, tags)
	if err != nil {
		return nil, fmt.Errorf("getting launch templates, %w", err)
	}
	for _, launchTemplate := range launchTemplates {
		launchTemplateConfig := &ec2.FleetLaunchTemplateConfigRequest{
			Overrides: p.getOverrides(launchTemplate.InstanceTypes, zonalSubnets, scheduling.NewNodeSelectorRequirements(nodeClaim.Spec.Requirements...).Get(v1.LabelTopologyZone), capacityType, launchTemplate.ImageID),
			LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: aws.String(launchTemplate.Name),
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
func (p *Provider) getOverrides(instanceTypes []*cloudprovider.InstanceType, zonalSubnets map[string]*ec2.Subnet, zones *scheduling.Requirement, capacityType string, image string) []*ec2.FleetLaunchTemplateOverridesRequest {
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

	var overrides []*ec2.FleetLaunchTemplateOverridesRequest
	for _, offering := range unwrappedOfferings {
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
		overrides = append(overrides, &ec2.FleetLaunchTemplateOverridesRequest{
			InstanceType: aws.String(offering.parentInstanceTypeName),
			SubnetId:     subnet.SubnetId,
			ImageId:      aws.String(image),
			// This is technically redundant, but is useful if we have to parse insufficient capacity errors from
			// CreateFleet so that we can figure out the zone rather than additional API calls to look up the subnet
			AvailabilityZone: subnet.AvailabilityZone,
		})
	}
	return overrides
}

func (p *Provider) updateUnavailableOfferingsCache(ctx context.Context, errors []*ec2.CreateFleetError, capacityType string) {
	for _, err := range errors {
		if awserrors.IsUnfulfillableCapacity(err) {
			p.unavailableOfferings.MarkUnavailableForFleetErr(ctx, err, capacityType)
		}
	}
}

// getCapacityType selects spot if both constraints are flexible and there is an
// available offering. The AWS Cloud Provider defaults to [ on-demand ], so spot
// must be explicitly included in capacity type requirements.
func (p *Provider) getCapacityType(nodeClaim *corev1beta1.NodeClaim, instanceTypes []*cloudprovider.InstanceType) string {
	requirements := scheduling.NewNodeSelectorRequirements(nodeClaim.
		Spec.Requirements...)
	if requirements.Get(corev1beta1.CapacityTypeLabelKey).Has(corev1beta1.CapacityTypeSpot) {
		for _, instanceType := range instanceTypes {
			for _, offering := range instanceType.Offerings.Available() {
				if requirements.Get(v1.LabelTopologyZone).Has(offering.Zone) && offering.CapacityType == corev1beta1.CapacityTypeSpot {
					return corev1beta1.CapacityTypeSpot
				}
			}
		}
	}
	return corev1beta1.CapacityTypeOnDemand
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

// filterInstanceTypes is used to provide filtering on the list of potential instance types to further limit it to those
// that make the most sense given our specific AWS cloudprovider.
func (p *Provider) filterInstanceTypes(nodeClaim *corev1beta1.NodeClaim, instanceTypes []*cloudprovider.InstanceType) []*cloudprovider.InstanceType {
	instanceTypes = filterExoticInstanceTypes(instanceTypes, nodeClaim.IsMachine)
	// If we could potentially launch either a spot or on-demand node, we want to filter out the spot instance types that
	// are more expensive than the cheapest on-demand type.
	if p.isMixedCapacityLaunch(nodeClaim, instanceTypes) {
		instanceTypes = filterUnwantedSpot(instanceTypes)
	}
	return instanceTypes
}

// isMixedCapacityLaunch returns true if provisioners and available offerings could potentially allow either a spot or
// and on-demand node to launch
func (p *Provider) isMixedCapacityLaunch(nodeClaim *corev1beta1.NodeClaim, instanceTypes []*cloudprovider.InstanceType) bool {
	requirements := scheduling.NewNodeSelectorRequirements(nodeClaim.Spec.Requirements...)
	// requirements must allow both
	if !requirements.Get(corev1beta1.CapacityTypeLabelKey).Has(corev1beta1.CapacityTypeSpot) ||
		!requirements.Get(corev1beta1.CapacityTypeLabelKey).Has(corev1beta1.CapacityTypeOnDemand) {
		return false
	}
	hasSpotOfferings := false
	hasODOffering := false
	if requirements.Get(corev1beta1.CapacityTypeLabelKey).Has(corev1beta1.CapacityTypeSpot) {
		for _, instanceType := range instanceTypes {
			for _, offering := range instanceType.Offerings.Available() {
				if requirements.Get(v1.LabelTopologyZone).Has(offering.Zone) {
					if offering.CapacityType == corev1beta1.CapacityTypeSpot {
						hasSpotOfferings = true
					} else {
						hasODOffering = true
					}
				}
			}
		}
	}
	return hasSpotOfferings && hasODOffering
}

// filterUnwantedSpot is used to filter out spot types that are more expensive than the cheapest on-demand type that we
// could launch during mixed capacity-type launches
func filterUnwantedSpot(instanceTypes []*cloudprovider.InstanceType) []*cloudprovider.InstanceType {
	cheapestOnDemand := math.MaxFloat64
	// first, find the price of our cheapest available on-demand instance type that could support this node
	for _, it := range instanceTypes {
		for _, o := range it.Offerings.Available() {
			if o.CapacityType == corev1beta1.CapacityTypeOnDemand && o.Price < cheapestOnDemand {
				cheapestOnDemand = o.Price
			}
		}
	}

	// Filter out any types where the cheapest offering, which should be spot, is more expensive than the cheapest
	// on-demand instance type that would have worked. This prevents us from getting a larger more-expensive spot
	// instance type compared to the cheapest sufficiently large on-demand instance type
	instanceTypes = lo.Filter(instanceTypes, func(item *cloudprovider.InstanceType, index int) bool {
		available := item.Offerings.Available()
		if len(available) == 0 {
			return false
		}
		return available.Cheapest().Price <= cheapestOnDemand
	})
	return instanceTypes
}

// filterExoticInstanceTypes is used to eliminate less desirable instance types (like GPUs) from the list of possible instance types when
// a set of more appropriate instance types would work. If a set of more desirable instance types is not found, then the original slice
// of instance types are returned.
func filterExoticInstanceTypes(instanceTypes []*cloudprovider.InstanceType, isMachine bool) []*cloudprovider.InstanceType {
	var genericInstanceTypes []*cloudprovider.InstanceType
	for _, it := range instanceTypes {
		// deprioritize metal even if our opinionated filter isn't applied due to something like an instance family
		// requirement
		if _, ok := lo.Find(it.Requirements.Get(lo.Ternary(isMachine, v1alpha1.LabelInstanceSize, v1beta1.LabelInstanceSize)).Values(), func(size string) bool { return strings.Contains(size, "metal") }); ok {
			continue
		}
		if !resources.IsZero(it.Capacity[lo.Ternary(isMachine, v1alpha1.ResourceAWSNeuron, v1beta1.ResourceAWSNeuron)]) ||
			!resources.IsZero(it.Capacity[lo.Ternary(isMachine, v1alpha1.ResourceAMDGPU, v1beta1.ResourceAMDGPU)]) ||
			!resources.IsZero(it.Capacity[lo.Ternary(isMachine, v1alpha1.ResourceNVIDIAGPU, v1beta1.ResourceNVIDIAGPU)]) ||
			!resources.IsZero(it.Capacity[lo.Ternary(isMachine, v1alpha1.ResourceHabanaGaudi, v1beta1.ResourceHabanaGaudi)]) {
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

func instancesFromOutput(out *ec2.DescribeInstancesOutput) ([]*Instance, error) {
	if len(out.Reservations) == 0 {
		return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance not found"))
	}
	instances := lo.Flatten(lo.Map(out.Reservations, func(r *ec2.Reservation, _ int) []*ec2.Instance {
		return r.Instances
	}))
	if len(instances) == 0 {
		return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance not found"))
	}
	// Get a consistent ordering for instances
	sort.Slice(instances, func(i, j int) bool {
		return aws.StringValue(instances[i].InstanceId) < aws.StringValue(instances[j].InstanceId)
	})
	return lo.Map(instances, func(i *ec2.Instance, _ int) *Instance { return NewInstance(i) }), nil
}

func combineFleetErrors(errors []*ec2.CreateFleetError) (errs error) {
	unique := sets.NewString()
	for _, err := range errors {
		unique.Insert(fmt.Sprintf("%s: %s", aws.StringValue(err.ErrorCode), aws.StringValue(err.ErrorMessage)))
	}
	for errorCode := range unique {
		errs = multierr.Append(errs, fmt.Errorf(errorCode))
	}
	// If all the Fleet errors are ICE errors then we should wrap the combined error in the generic ICE error
	iceErrorCount := lo.CountBy(errors, func(err *ec2.CreateFleetError) bool { return awserrors.IsUnfulfillableCapacity(err) })
	if iceErrorCount == len(errors) {
		return cloudprovider.NewInsufficientCapacityError(fmt.Errorf("with fleet error(s), %w", errs))
	}
	return fmt.Errorf("with fleet error(s), %w", errs)
}
