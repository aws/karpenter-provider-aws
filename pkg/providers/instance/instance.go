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

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/log"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/batcher"
	"github.com/aws/karpenter-provider-aws/pkg/cache"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

const (
	instanceTypeFlexibilityThreshold = 5 // falling back to on-demand without flexibility risks insufficient capacity errors
	maxInstanceTypes                 = 60
)

var (
	instanceStateFilter = ec2types.Filter{
		Name: aws.String("instance-state-name"),
		Values: []string{
			string(ec2types.InstanceStateNamePending),
			string(ec2types.InstanceStateNameRunning),
			string(ec2types.InstanceStateNameStopping),
			string(ec2types.InstanceStateNameStopped),
			string(ec2types.InstanceStateNameShuttingDown),
		},
	}
)

type Provider interface {
	Create(context.Context, *v1.EC2NodeClass, *karpv1.NodeClaim, map[string]string, []*cloudprovider.InstanceType) (*Instance, error)
	Get(context.Context, string) (*Instance, error)
	List(context.Context) ([]*Instance, error)
	Delete(context.Context, string) error
	CreateTags(context.Context, string, map[string]string) error
}

type DefaultProvider struct {
	region                      string
	ec2api                      sdk.EC2API
	unavailableOfferings        *cache.UnavailableOfferings
	subnetProvider              subnet.Provider
	launchTemplateProvider      launchtemplate.Provider
	ec2Batcher                  *batcher.EC2API
	capacityReservationProvider capacityreservation.Provider
}

func NewDefaultProvider(
	ctx context.Context,
	region string,
	ec2api sdk.EC2API,
	unavailableOfferings *cache.UnavailableOfferings,
	subnetProvider subnet.Provider,
	launchTemplateProvider launchtemplate.Provider,
	capacityReservationProvider capacityreservation.Provider,
) *DefaultProvider {
	return &DefaultProvider{
		region:                      region,
		ec2api:                      ec2api,
		unavailableOfferings:        unavailableOfferings,
		subnetProvider:              subnetProvider,
		launchTemplateProvider:      launchTemplateProvider,
		ec2Batcher:                  batcher.EC2(ctx, ec2api),
		capacityReservationProvider: capacityReservationProvider,
	}
}

func (p *DefaultProvider) Create(ctx context.Context, nodeClass *v1.EC2NodeClass, nodeClaim *karpv1.NodeClaim, tags map[string]string, instanceTypes []*cloudprovider.InstanceType) (*Instance, error) {
	schedulingRequirements := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...)
	// Only filter the instances if there are no minValues in the requirement.
	if !schedulingRequirements.HasMinValues() {
		instanceTypes = p.filterInstanceTypes(nodeClaim, instanceTypes)
	}
	// We filter out non-reserved instances regardless of the min-values settings, since if the launch is eligible for
	// reserved instances that's all we'll include in our fleet request.
	if reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...); reqs.Get(karpv1.CapacityTypeLabelKey).Has(karpv1.CapacityTypeReserved) {
		instanceTypes = p.filterReservedInstanceTypes(reqs, instanceTypes)
		if _, err := cloudprovider.InstanceTypes(instanceTypes).SatisfiesMinValues(schedulingRequirements); err != nil {
			return nil, cloudprovider.NewCreateError(fmt.Errorf("failed to construct CreateFleet request while respecting minValues requirements"), "CreateFleetRequestConstructionFailed", "Failed to construct CreateFleet request while respecting minValues")
		}
	}
	instanceTypes, err := cloudprovider.InstanceTypes(instanceTypes).Truncate(schedulingRequirements, maxInstanceTypes)
	if err != nil {
		return nil, cloudprovider.NewCreateError(fmt.Errorf("truncating instance types, %w", err), "InstanceTypeResolutionFailed", "Error truncating instance types based on the passed-in requirements")
	}
	capacityType := p.getCapacityType(nodeClaim, instanceTypes)
	fleetInstance, err := p.launchInstance(ctx, nodeClass, nodeClaim, capacityType, instanceTypes, tags)
	if awserrors.IsLaunchTemplateNotFound(err) {
		// retry once if launch template is not found. This allows karpenter to generate a new LT if the
		// cache was out-of-sync on the first try
		fleetInstance, err = p.launchInstance(ctx, nodeClass, nodeClaim, capacityType, instanceTypes, tags)
	}
	if err != nil {
		return nil, err
	}

	var capacityReservation string
	if capacityType == karpv1.CapacityTypeReserved {
		capacityReservation = p.getCapacityReservationIDForInstance(
			string(fleetInstance.InstanceType),
			*fleetInstance.LaunchTemplateAndOverrides.Overrides.AvailabilityZone,
			instanceTypes,
		)
	}
	return NewInstanceFromFleet(
		fleetInstance,
		tags,
		capacityType,
		capacityReservation,
		lo.Contains(lo.Keys(nodeClaim.Spec.Resources.Requests), v1.ResourceEFA),
	), nil
}

func (p *DefaultProvider) Get(ctx context.Context, id string) (*Instance, error) {
	out, err := p.ec2Batcher.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{id},
		Filters:     []ec2types.Filter{instanceStateFilter},
	})
	if awserrors.IsNotFound(err) {
		return nil, cloudprovider.NewNodeClaimNotFoundError(err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to describe ec2 instances, %w", err)
	}
	instances, err := instancesFromOutput(ctx, out)
	if err != nil {
		return nil, fmt.Errorf("getting instances from output, %w", err)
	}
	if len(instances) != 1 {
		return nil, fmt.Errorf("expected a single instance, %w", err)
	}
	return instances[0], nil
}

func (p *DefaultProvider) List(ctx context.Context) ([]*Instance, error) {
	var out = &ec2.DescribeInstancesOutput{}

	paginator := ec2.NewDescribeInstancesPaginator(p.ec2api, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: []string{v1.NodePoolTagKey},
			},
			{
				Name:   aws.String("tag-key"),
				Values: []string{v1.NodeClassTagKey},
			},
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", v1.EKSClusterNameTagKey)),
				Values: []string{options.FromContext(ctx).ClusterName},
			},
			instanceStateFilter,
		},
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing ec2 instances, %w", err)
		}
		out.Reservations = append(out.Reservations, page.Reservations...)
	}
	instances, err := instancesFromOutput(ctx, out)
	return instances, cloudprovider.IgnoreNodeClaimNotFoundError(err)
}

func (p *DefaultProvider) Delete(ctx context.Context, id string) error {
	out, err := p.Get(ctx, id)
	if err != nil {
		return err
	}
	// Check if the instance is already shutting-down to reduce the number of terminate-instance calls we make thereby
	// reducing our overall QPS. Due to EC2's eventual consistency model, the result of the terminate-instance or
	// describe-instance call may return a not found error even when the instance is not terminated -
	// https://docs.aws.amazon.com/ec2/latest/devguide/eventual-consistency.html. In this case, the instance will get
	// picked up by the garbage collection controller and will be cleaned up eventually.
	if out.State != ec2types.InstanceStateNameShuttingDown {
		if _, err := p.ec2Batcher.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: []string{id},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (p *DefaultProvider) CreateTags(ctx context.Context, id string, tags map[string]string) error {
	ec2Tags := lo.MapToSlice(tags, func(key, value string) ec2types.Tag {
		return ec2types.Tag{Key: aws.String(key), Value: aws.String(value)}
	})
	if _, err := p.ec2api.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{id},
		Tags:      ec2Tags,
	}); err != nil {
		if awserrors.IsNotFound(err) {
			return cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("tagging instance, %w", err))
		}
		return fmt.Errorf("tagging instance, %w", err)
	}
	return nil
}

func (p *DefaultProvider) launchInstance(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	nodeClaim *karpv1.NodeClaim,
	capacityType string,
	instanceTypes []*cloudprovider.InstanceType,
	tags map[string]string,
) (ec2types.CreateFleetInstance, error) {
	zonalSubnets, err := p.subnetProvider.ZonalSubnetsForLaunch(ctx, nodeClass, instanceTypes, capacityType)
	if err != nil {
		return ec2types.CreateFleetInstance{}, cloudprovider.NewCreateError(fmt.Errorf("getting subnets, %w", err), "SubnetResolutionFailed", "Error getting subnets")
	}

	// Get Launch Template Configs, which may differ due to GPU or Architecture requirements
	launchTemplateConfigs, err := p.getLaunchTemplateConfigs(ctx, nodeClass, nodeClaim, instanceTypes, zonalSubnets, capacityType, tags)
	if err != nil {
		reason, message := awserrors.ToReasonMessage(err)
		return ec2types.CreateFleetInstance{}, cloudprovider.NewCreateError(fmt.Errorf("getting launch template configs, %w", err), reason, fmt.Sprintf("Error getting launch template configs: %s", message))
	}
	if err := p.checkODFallback(nodeClaim, instanceTypes, launchTemplateConfigs); err != nil {
		log.FromContext(ctx).Error(err, "failed while checking on-demand fallback")
	}
	// Create fleet
	createFleetInput := GetCreateFleetInput(nodeClass, capacityType, tags, launchTemplateConfigs)
	if capacityType == karpv1.CapacityTypeSpot {
		createFleetInput.SpotOptions = &ec2types.SpotOptionsRequest{AllocationStrategy: ec2types.SpotAllocationStrategyPriceCapacityOptimized}
	} else {
		createFleetInput.OnDemandOptions = &ec2types.OnDemandOptionsRequest{AllocationStrategy: ec2types.FleetOnDemandAllocationStrategyLowestPrice}
	}

	createFleetOutput, err := p.ec2Batcher.CreateFleet(ctx, createFleetInput)
	p.subnetProvider.UpdateInflightIPs(createFleetInput, createFleetOutput, instanceTypes, lo.Values(zonalSubnets), capacityType)
	if err != nil {
		reason, message := awserrors.ToReasonMessage(err)
		if awserrors.IsLaunchTemplateNotFound(err) {
			for _, lt := range launchTemplateConfigs {
				p.launchTemplateProvider.InvalidateCache(ctx, aws.ToString(lt.LaunchTemplateSpecification.LaunchTemplateName), aws.ToString(lt.LaunchTemplateSpecification.LaunchTemplateId))
			}
			return ec2types.CreateFleetInstance{}, cloudprovider.NewCreateError(fmt.Errorf("launch templates not found when creating fleet request, %w", err), reason, fmt.Sprintf("Launch templates not found when creating fleet request: %s", message))
		}
		var reqErr *awshttp.ResponseError
		if errors.As(err, &reqErr) {
			return ec2types.CreateFleetInstance{}, cloudprovider.NewCreateError(fmt.Errorf("creating fleet request, %w (%v)", err, reqErr.ServiceRequestID()), reason, fmt.Sprintf("Error creating fleet request: %s", message))
		}
		return ec2types.CreateFleetInstance{}, cloudprovider.NewCreateError(fmt.Errorf("creating fleet request, %w", err), reason, fmt.Sprintf("Error creating fleet request: %s", message))
	}
	p.updateUnavailableOfferingsCache(ctx, createFleetOutput.Errors, capacityType, instanceTypes)
	if len(createFleetOutput.Instances) == 0 || len(createFleetOutput.Instances[0].InstanceIds) == 0 {
		return ec2types.CreateFleetInstance{}, combineFleetErrors(createFleetOutput.Errors)
	}
	return createFleetOutput.Instances[0], nil
}

func GetCreateFleetInput(nodeClass *v1.EC2NodeClass, capacityType string, tags map[string]string, launchTemplateConfigs []ec2types.FleetLaunchTemplateConfigRequest) *ec2.CreateFleetInput {
	return &ec2.CreateFleetInput{
		Type:                  ec2types.FleetTypeInstant,
		Context:               nodeClass.Spec.Context,
		LaunchTemplateConfigs: launchTemplateConfigs,
		TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: lo.Ternary(
				capacityType == karpv1.CapacityTypeReserved,
				ec2types.DefaultTargetCapacityType(karpv1.CapacityTypeOnDemand),
				ec2types.DefaultTargetCapacityType(capacityType),
			),
			TotalTargetCapacity: aws.Int32(1),
		},
		TagSpecifications: []ec2types.TagSpecification{
			{ResourceType: ec2types.ResourceTypeInstance, Tags: utils.MergeTags(tags)},
			{ResourceType: ec2types.ResourceTypeVolume, Tags: utils.MergeTags(tags)},
			{ResourceType: ec2types.ResourceTypeFleet, Tags: utils.MergeTags(tags)},
		},
	}
}

func (p *DefaultProvider) checkODFallback(nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType, launchTemplateConfigs []ec2types.FleetLaunchTemplateConfigRequest) error {
	// only evaluate for on-demand fallback if the capacity type for the request is OD and both OD and spot are allowed in requirements
	if p.getCapacityType(nodeClaim, instanceTypes) != karpv1.CapacityTypeOnDemand || !scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...).Get(karpv1.CapacityTypeLabelKey).Has(karpv1.CapacityTypeSpot) {
		return nil
	}

	// loop through the LT configs for currently considered instance types to get the flexibility count
	instanceTypeZones := map[string]struct{}{}
	for _, ltc := range launchTemplateConfigs {
		for _, override := range ltc.Overrides {
			instanceTypeZones[string(override.InstanceType)] = struct{}{}
		}
	}
	if len(instanceTypes) < instanceTypeFlexibilityThreshold {
		return fmt.Errorf("at least %d instance types are recommended when flexible to spot but requesting on-demand, "+
			"the current provisioning request only has %d instance type options", instanceTypeFlexibilityThreshold, len(instanceTypes))
	}
	return nil
}

func (p *DefaultProvider) getLaunchTemplateConfigs(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	nodeClaim *karpv1.NodeClaim,
	instanceTypes []*cloudprovider.InstanceType,
	zonalSubnets map[string]*subnet.Subnet,
	capacityType string,
	tags map[string]string,
) ([]ec2types.FleetLaunchTemplateConfigRequest, error) {
	var launchTemplateConfigs []ec2types.FleetLaunchTemplateConfigRequest
	launchTemplates, err := p.launchTemplateProvider.EnsureAll(ctx, nodeClass, nodeClaim, instanceTypes, capacityType, tags)
	if err != nil {
		return nil, fmt.Errorf("getting launch templates, %w", err)
	}
	requirements := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...)
	requirements[karpv1.CapacityTypeLabelKey] = scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityType)
	for _, launchTemplate := range launchTemplates {
		launchTemplateConfig := ec2types.FleetLaunchTemplateConfigRequest{
			Overrides: p.getOverrides(launchTemplate.InstanceTypes, zonalSubnets, requirements, launchTemplate.ImageID, launchTemplate.CapacityReservationID),
			LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
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
func (p *DefaultProvider) getOverrides(
	instanceTypes []*cloudprovider.InstanceType,
	zonalSubnets map[string]*subnet.Subnet,
	reqs scheduling.Requirements,
	image, capacityReservationID string,
) []ec2types.FleetLaunchTemplateOverridesRequest {
	// Unwrap all the offerings to a flat slice that includes a pointer
	// to the parent instance type name
	type offeringWithParentName struct {
		*cloudprovider.Offering
		parentInstanceTypeName ec2types.InstanceType
	}
	var filteredOfferings []offeringWithParentName
	for _, it := range instanceTypes {
		ofs := it.Offerings.Available().Compatible(reqs)
		// If we are generating a launch template for a specific capacity reservation, we only want to include the offering
		// for that capacity reservation when generating overrides.
		if capacityReservationID != "" {
			ofs = ofs.Compatible(scheduling.NewRequirements(scheduling.NewRequirement(
				cloudprovider.ReservationIDLabel,
				corev1.NodeSelectorOpIn,
				capacityReservationID,
			)))
		}
		for _, o := range ofs {
			filteredOfferings = append(filteredOfferings, offeringWithParentName{
				Offering:               o,
				parentInstanceTypeName: ec2types.InstanceType(it.Name),
			})
		}
	}
	var overrides []ec2types.FleetLaunchTemplateOverridesRequest
	for _, offering := range filteredOfferings {
		subnet, ok := zonalSubnets[offering.Zone()]
		if !ok {
			continue
		}
		overrides = append(overrides, ec2types.FleetLaunchTemplateOverridesRequest{
			InstanceType: offering.parentInstanceTypeName,
			SubnetId:     lo.ToPtr(subnet.ID),
			ImageId:      lo.ToPtr(image),
			// This is technically redundant, but is useful if we have to parse insufficient capacity errors from
			// CreateFleet so that we can figure out the zone rather than additional API calls to look up the subnet
			AvailabilityZone: lo.ToPtr(subnet.Zone),
		})
	}
	return overrides
}

func (p *DefaultProvider) updateUnavailableOfferingsCache(
	ctx context.Context,
	errs []ec2types.CreateFleetError,
	capacityType string,
	instanceTypes []*cloudprovider.InstanceType,
) {
	if capacityType != karpv1.CapacityTypeReserved {
		for _, err := range errs {
			if awserrors.IsUnfulfillableCapacity(err) {
				p.unavailableOfferings.MarkUnavailableForFleetErr(ctx, err, capacityType)
			}
		}
		return
	}

	reservationIDs := make([]string, 0, len(errs))
	for i := range errs {
		id := p.getCapacityReservationIDForInstance(
			string(errs[i].LaunchTemplateAndOverrides.Overrides.InstanceType),
			lo.FromPtr(errs[i].LaunchTemplateAndOverrides.Overrides.AvailabilityZone),
			instanceTypes,
		)
		reservationIDs = append(reservationIDs, id)
		log.FromContext(ctx).WithValues(
			"reason", lo.FromPtr(errs[i].ErrorCode),
			"instance-type", errs[i].LaunchTemplateAndOverrides.Overrides.InstanceType,
			"zone", lo.FromPtr(errs[i].LaunchTemplateAndOverrides.Overrides.AvailabilityZone),
			"capacity-reservation-id", id,
		).V(1).Info("marking capacity reservation unavailable")
	}
	p.capacityReservationProvider.MarkUnavailable(reservationIDs...)
}

func (p *DefaultProvider) getCapacityReservationIDForInstance(instance, zone string, instanceTypes []*cloudprovider.InstanceType) string {
	for _, it := range instanceTypes {
		if it.Name != instance {
			continue
		}
		for _, o := range it.Offerings {
			if o.CapacityType() != karpv1.CapacityTypeReserved || o.Zone() != zone {
				continue
			}
			return o.ReservationID()
		}
	}
	// note: this is an invariant that the caller must enforce, should not occur at runtime
	panic("reservation ID doesn't exist for reserved launch")
}

// getCapacityType selects the capacity type based on the flexibility of the NodeClaim and the available offerings.
// Prioritization is as follows: reserved, spot, on-demand.
func (p *DefaultProvider) getCapacityType(nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType) string {
	for _, capacityType := range []string{karpv1.CapacityTypeReserved, karpv1.CapacityTypeSpot} {
		requirements := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...)
		if !requirements.Get(karpv1.CapacityTypeLabelKey).Has(capacityType) {
			continue
		}
		requirements[karpv1.CapacityTypeLabelKey] = scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityType)
		for _, it := range instanceTypes {
			if len(it.Offerings.Available().Compatible(requirements)) != 0 {
				return capacityType
			}
		}
	}
	return karpv1.CapacityTypeOnDemand
}

// filterReservedInstanceTypes is used to filter the provided set of instance types to only include those with
// available reserved offerings if the nodeclaim is compatible. If there are no available reserved offerings, no
// filtering is applied.
func (*DefaultProvider) filterReservedInstanceTypes(nodeClaimRequirements scheduling.Requirements, instanceTypes []*cloudprovider.InstanceType) []*cloudprovider.InstanceType {
	nodeClaimRequirements[karpv1.CapacityTypeLabelKey] = scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeReserved)
	var reservedInstanceTypes []*cloudprovider.InstanceType
	for _, it := range instanceTypes {
		// We only want to include a single offering per pool (instance type / AZ combo). This is due to a limitation in the
		// CreateFleet API, which limits calls to specifying a single override per pool. We'll choose to launch into the pool
		// with the most capacity.
		zonalOfferings := map[string]*cloudprovider.Offering{}
		for _, o := range it.Offerings.Available().Compatible(nodeClaimRequirements) {
			if current, ok := zonalOfferings[o.Zone()]; !ok || o.ReservationCapacity > current.ReservationCapacity {
				zonalOfferings[o.Zone()] = o
			}
		}
		if len(zonalOfferings) == 0 {
			continue
		}
		// WARNING: It is only safe to mutate the slice containing the offerings, not the offerings themselves. The individual
		// offerings are cached, but not the slice storing them. This helps keep the launch path simple, but changes to the
		// caching strategy employed by the InstanceType provider could result in unexpected behavior.
		it.Offerings = lo.Values(zonalOfferings)
		reservedInstanceTypes = append(reservedInstanceTypes, it)
	}
	if len(reservedInstanceTypes) == 0 {
		return instanceTypes
	}
	return reservedInstanceTypes
}

// filterInstanceTypes is used to provide filtering on the list of potential instance types to further limit it to those
// that make the most sense given our specific AWS cloudprovider.
func (p *DefaultProvider) filterInstanceTypes(nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType) []*cloudprovider.InstanceType {
	instanceTypes = filterExoticInstanceTypes(instanceTypes)
	// If we could potentially launch either a spot or on-demand node, we want to filter out the spot instance types that
	// are more expensive than the cheapest on-demand type.
	if p.isMixedCapacityLaunch(nodeClaim, instanceTypes) {
		instanceTypes = filterUnwantedSpot(instanceTypes)
	}
	return instanceTypes
}

// isMixedCapacityLaunch returns true if nodepools and available offerings could potentially allow either a spot or
// and on-demand node to launch
func (p *DefaultProvider) isMixedCapacityLaunch(nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType) bool {
	requirements := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...)
	// requirements must allow both
	if !requirements.Get(karpv1.CapacityTypeLabelKey).Has(karpv1.CapacityTypeSpot) ||
		!requirements.Get(karpv1.CapacityTypeLabelKey).Has(karpv1.CapacityTypeOnDemand) {
		return false
	}
	hasSpotOfferings := false
	hasODOffering := false
	if requirements.Get(karpv1.CapacityTypeLabelKey).Has(karpv1.CapacityTypeSpot) {
		for _, instanceType := range instanceTypes {
			for _, offering := range instanceType.Offerings.Available() {
				if requirements.Compatible(offering.Requirements, scheduling.AllowUndefinedWellKnownLabels) != nil {
					continue
				}
				if offering.Requirements.Get(karpv1.CapacityTypeLabelKey).Any() == karpv1.CapacityTypeSpot {
					hasSpotOfferings = true
				} else {
					hasODOffering = true
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
			if o.Requirements.Get(karpv1.CapacityTypeLabelKey).Any() == karpv1.CapacityTypeOnDemand && o.Price < cheapestOnDemand {
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
func filterExoticInstanceTypes(instanceTypes []*cloudprovider.InstanceType) []*cloudprovider.InstanceType {
	var genericInstanceTypes []*cloudprovider.InstanceType
	for _, it := range instanceTypes {
		// deprioritize metal even if our opinionated filter isn't applied due to something like an instance family
		// requirement
		if _, ok := lo.Find(it.Requirements.Get(v1.LabelInstanceSize).Values(), func(size string) bool { return strings.Contains(size, "metal") }); ok {
			continue
		}
		if !resources.IsZero(it.Capacity[v1.ResourceAWSNeuron]) ||
			!resources.IsZero(it.Capacity[v1.ResourceAWSNeuronCore]) ||
			!resources.IsZero(it.Capacity[v1.ResourceAMDGPU]) ||
			!resources.IsZero(it.Capacity[v1.ResourceNVIDIAGPU]) ||
			!resources.IsZero(it.Capacity[v1.ResourceHabanaGaudi]) {
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

func instancesFromOutput(ctx context.Context, out *ec2.DescribeInstancesOutput) ([]*Instance, error) {
	if len(out.Reservations) == 0 {
		return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance not found"))
	}
	instances := lo.Flatten(lo.Map(out.Reservations, func(r ec2types.Reservation, _ int) []ec2types.Instance {
		return r.Instances
	}))
	if len(instances) == 0 {
		return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("instance not found"))
	}
	// Get a consistent ordering for instances
	sort.Slice(instances, func(i, j int) bool {
		return aws.ToString(instances[i].InstanceId) < aws.ToString(instances[j].InstanceId)
	})
	return lo.Map(instances, func(i ec2types.Instance, _ int) *Instance { return NewInstance(ctx, i) }), nil
}

func combineFleetErrors(fleetErrs []ec2types.CreateFleetError) (errs error) {
	unique := sets.NewString()
	for _, err := range fleetErrs {
		unique.Insert(fmt.Sprintf("%s: %s", aws.ToString(err.ErrorCode), aws.ToString(err.ErrorMessage)))
	}
	for errorCode := range unique {
		errs = multierr.Append(errs, errors.New(errorCode))
	}
	// If all the Fleet errors are ICE errors then we should wrap the combined error in the generic ICE error
	iceErrorCount := lo.CountBy(fleetErrs, func(err ec2types.CreateFleetError) bool { return awserrors.IsUnfulfillableCapacity(err) })
	if iceErrorCount == len(fleetErrs) {
		return cloudprovider.NewInsufficientCapacityError(fmt.Errorf("with fleet error(s), %w", errs))
	}
	reason, message := awserrors.ToReasonMessage(errs)
	return cloudprovider.NewCreateError(errs, reason, message)
}
