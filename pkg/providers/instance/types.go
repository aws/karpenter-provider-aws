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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/option"
	"github.com/imdario/mergo"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	karpopts "sigs.k8s.io/karpenter/pkg/operator/options"
)

// Instance is an internal data representation of either an ec2.Instance or an ec2.FleetInstance
// It contains all the common data that is needed to inject into the Machine from either of these responses
type Instance struct {
	LaunchTime                 time.Time
	State                      ec2types.InstanceStateName
	ID                         string
	ImageID                    string
	Type                       ec2types.InstanceType
	Zone                       string
	ZoneID                     string
	CapacityType               string
	SecurityGroupIDs           []string
	SubnetID                   string
	Tags                       map[string]string
	EFACount                   int
	CapacityReservationDetails *CapacityReservationDetails
	Tenancy                    string
	PartitionNumber            *int32
}

type CapacityReservationDetails struct {
	ID            string
	Type          v1.CapacityReservationType
	Interruptible bool
}

func NewInstance(ctx context.Context, instance ec2types.Instance) *Instance {
	capacityType := capacityTypeFromInstance(ctx, instance)
	var capacityReservationID string
	if capacityType == karpv1.CapacityTypeReserved {
		capacityReservationID = *instance.CapacityReservationId
	}
	capacityReservationDetails := lo.Ternary(capacityType != karpv1.CapacityTypeReserved, nil, &CapacityReservationDetails{
		ID: capacityReservationID,
		Type: lo.Ternary(instance.InstanceLifecycle == ec2types.InstanceLifecycleTypeCapacityBlock,
			v1.CapacityReservationTypeCapacityBlock, v1.CapacityReservationTypeDefault),
		Interruptible: instance.InstanceLifecycle == ec2types.InstanceLifecycleTypeInterruptibleCapacityReservation,
	})
	return &Instance{
		LaunchTime: lo.FromPtr(instance.LaunchTime),
		State:      instance.State.Name,
		ID:         lo.FromPtr(instance.InstanceId),
		ImageID:    lo.FromPtr(instance.ImageId),
		Type:       instance.InstanceType,
		Zone:       lo.FromPtr(instance.Placement.AvailabilityZone),
		ZoneID:     lo.FromPtr(instance.Placement.AvailabilityZoneId),
		// NOTE: Only set the capacity type to reserved and assign a reservation ID if the feature gate is enabled. It's
		// possible for these to be set if the instance launched into an open ODCR, but treating it as reserved would induce
		// drift.
		CapacityType: capacityType,
		SecurityGroupIDs: lo.Map(instance.SecurityGroups, func(securitygroup ec2types.GroupIdentifier, _ int) string {
			return lo.FromPtr(securitygroup.GroupId)
		}),
		SubnetID: lo.FromPtr(instance.SubnetId),
		Tags:     lo.SliceToMap(instance.Tags, func(t ec2types.Tag) (string, string) { return lo.FromPtr(t.Key), lo.FromPtr(t.Value) }),
		EFACount: lo.CountBy(instance.NetworkInterfaces, func(item ec2types.InstanceNetworkInterface) bool {
			return item.InterfaceType != nil && (*item.InterfaceType == string(ec2types.NetworkInterfaceTypeEfa) ||
				*item.InterfaceType == string(ec2types.NetworkInterfaceTypeEfaOnly))
		}),
		CapacityReservationDetails: capacityReservationDetails,
		Tenancy:                    tenancyFromInstance(instance),
		PartitionNumber:            partitionNumberFromInstance(instance),
	}
}

func partitionNumberFromInstance(instance ec2types.Instance) *int32 {
	if instance.Placement != nil && instance.Placement.PartitionNumber != nil && *instance.Placement.PartitionNumber != 0 {
		return instance.Placement.PartitionNumber
	}
	return nil
}

func tenancyFromInstance(instance ec2types.Instance) string {
	tenancy := instance.Placement.Tenancy
	return string(lo.Ternary(tenancy == "", ec2types.TenancyDefault, tenancy))
}

func capacityTypeFromInstance(ctx context.Context, instance ec2types.Instance) string {
	if instance.SpotInstanceRequestId != nil {
		return karpv1.CapacityTypeSpot
	}
	if karpopts.FromContext(ctx).FeatureGates.ReservedCapacity &&
		instance.CapacityReservationId != nil &&
		instance.CapacityReservationSpecification.CapacityReservationPreference == ec2types.CapacityReservationPreferenceCapacityReservationsOnly {
		return karpv1.CapacityTypeReserved
	}
	return karpv1.CapacityTypeOnDemand
}

type NewInstanceFromFleetOpts = option.Function[Instance]

func WithCapacityReservationDetails(capacityReservationDetails *CapacityReservationDetails) NewInstanceFromFleetOpts {
	return func(i *Instance) {
		i.CapacityReservationDetails = capacityReservationDetails
	}
}

func WithEFACount(efaCount int) NewInstanceFromFleetOpts {
	return func(i *Instance) { i.EFACount = efaCount }
}

func NewInstanceFromFleet(
	out ec2types.CreateFleetInstance,
	capacityType string,
	tags map[string]string,
	tenancyType string,
	opts ...NewInstanceFromFleetOpts,
) *Instance {
	resolved := option.Resolve(opts...)
	lo.Must0(mergo.Merge(resolved, &Instance{
		LaunchTime:   time.Now(), // estimate the launch time since we just launched
		State:        ec2types.InstanceStateNamePending,
		ID:           out.InstanceIds[0],
		ImageID:      lo.FromPtr(out.LaunchTemplateAndOverrides.Overrides.ImageId),
		Type:         out.InstanceType,
		Zone:         lo.FromPtr(out.LaunchTemplateAndOverrides.Overrides.AvailabilityZone),
		CapacityType: capacityType,
		SubnetID:     lo.FromPtr(out.LaunchTemplateAndOverrides.Overrides.SubnetId),
		Tags:         tags,
		Tenancy:      tenancyType,
	}))
	return resolved
}

// AllocationStrategyProvider chooses the CreateFleet allocation strategies for a launch.
type AllocationStrategyProvider interface {
	// OnDemandAllocationStrategy returns the allocation strategy for an on-demand class launch. nodeClaim may be nil.
	OnDemandAllocationStrategy(ctx context.Context, nodeClaim *karpv1.NodeClaim, capacityType string, overlay bool) ec2types.FleetOnDemandAllocationStrategy
	// SpotAllocationStrategy returns the allocation strategy for a spot launch. nodeClaim may be nil.
	SpotAllocationStrategy(ctx context.Context, nodeClaim *karpv1.NodeClaim, overlay bool) ec2types.SpotAllocationStrategy
}

// DefaultAllocationStrategyProvider uses lowest-price for on-demand and price-capacity-optimized for spot, or their
// prioritized variants when NodeOverlay prices are in use.
type DefaultAllocationStrategyProvider struct{}

func (DefaultAllocationStrategyProvider) OnDemandAllocationStrategy(_ context.Context, _ *karpv1.NodeClaim, _ string, overlay bool) ec2types.FleetOnDemandAllocationStrategy {
	return lo.Ternary(overlay, ec2types.FleetOnDemandAllocationStrategyPrioritized, ec2types.FleetOnDemandAllocationStrategyLowestPrice)
}

func (DefaultAllocationStrategyProvider) SpotAllocationStrategy(_ context.Context, _ *karpv1.NodeClaim, overlay bool) ec2types.SpotAllocationStrategy {
	return lo.Ternary(overlay, ec2types.SpotAllocationStrategyCapacityOptimizedPrioritized, ec2types.SpotAllocationStrategyPriceCapacityOptimized)
}

type CreateFleetInputBuilder struct {
	capacityType          string
	tagSpecifications     []ec2types.TagSpecification
	launchTemplateConfigs []ec2types.FleetLaunchTemplateConfigRequest

	contextID                        *string
	capacityReservationType          v1.CapacityReservationType
	capacityReservationInterruptible bool
	overlay                          bool
	allocationStrategyProvider       AllocationStrategyProvider
}

func NewCreateFleetInputBuilder(capacityType string, tags map[string]string, launchTemplateConfigs []ec2types.FleetLaunchTemplateConfigRequest) *CreateFleetInputBuilder {
	var taggedResources = []ec2types.ResourceType{
		ec2types.ResourceTypeInstance,
		ec2types.ResourceTypeVolume,
		ec2types.ResourceTypeFleet,
	}
	return &CreateFleetInputBuilder{
		capacityType: capacityType,
		tagSpecifications: lo.Map(taggedResources, func(resource ec2types.ResourceType, _ int) ec2types.TagSpecification {
			return ec2types.TagSpecification{ResourceType: resource, Tags: utils.EC2MergeTags(tags)}
		}),
		launchTemplateConfigs:      launchTemplateConfigs,
		overlay:                    false,
		allocationStrategyProvider: DefaultAllocationStrategyProvider{},
	}
}

// WithAllocationStrategyProvider chooses the allocation strategies with p instead of DefaultAllocationStrategyProvider.
func (b *CreateFleetInputBuilder) WithAllocationStrategyProvider(p AllocationStrategyProvider) *CreateFleetInputBuilder {
	if p != nil {
		b.allocationStrategyProvider = p
	}
	return b
}

func (b *CreateFleetInputBuilder) WithContextID(contextID string) *CreateFleetInputBuilder {
	b.contextID = &contextID
	return b
}

func (b *CreateFleetInputBuilder) WithOverlay() *CreateFleetInputBuilder {
	b.overlay = true
	return b
}

func (b *CreateFleetInputBuilder) WithCapacityReservationType(crt v1.CapacityReservationType, interruptible bool) *CreateFleetInputBuilder {
	if b.capacityType != karpv1.CapacityTypeReserved {
		panic("can not specify capacity reservation type when capacity type is not reserved")
	}
	b.capacityReservationType = crt
	b.capacityReservationInterruptible = interruptible
	return b
}

func (b *CreateFleetInputBuilder) defaultTargetCapacityType() ec2types.DefaultTargetCapacityType {
	switch b.capacityType {
	case karpv1.CapacityTypeReserved:
		if b.capacityReservationType == v1.CapacityReservationTypeCapacityBlock {
			return ec2types.DefaultTargetCapacityTypeCapacityBlock
		} else if b.capacityReservationInterruptible {
			return ec2types.DefaultTargetCapacityTypeReservedCapacity
		} else {
			return ec2types.DefaultTargetCapacityTypeOnDemand
		}
	case karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeSpot:
		return ec2types.DefaultTargetCapacityType(b.capacityType)
	}
	if class, ok := cloudprovider.CapacityTypeClassOf(b.capacityType); ok && class == cloudprovider.CapacityTypeClassOnDemand {
		return ec2types.DefaultTargetCapacityTypeOnDemand
	}
	panic(fmt.Sprintf("invalid capacity type %q provided to create fleet input", b.capacityType))
}

// Build builds the CreateFleetInput for a NodeClaim's launch. nodeClaim may be nil.
func (b *CreateFleetInputBuilder) Build(ctx context.Context, nodeClaim *karpv1.NodeClaim) *ec2.CreateFleetInput {
	input := &ec2.CreateFleetInput{
		Type:                  ec2types.FleetTypeInstant,
		Context:               b.contextID,
		LaunchTemplateConfigs: b.launchTemplateConfigs,
		TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: b.defaultTargetCapacityType(),
			TotalTargetCapacity:       lo.ToPtr[int32](1),
		},
		TagSpecifications: b.tagSpecifications,
	}
	if b.capacityType == karpv1.CapacityTypeSpot {
		input.SpotOptions = &ec2types.SpotOptionsRequest{
			AllocationStrategy: b.allocationStrategyProvider.SpotAllocationStrategy(ctx, nodeClaim, b.overlay),
		}
	} else if b.capacityReservationInterruptible {
		input.ReservedCapacityOptions = &ec2types.ReservedCapacityOptionsRequest{
			ReservationTypes: []ec2types.FleetReservationType{
				ec2types.FleetReservationTypeInterruptibleCapacityReservation,
			},
		}
	} else if b.capacityReservationType != v1.CapacityReservationTypeCapacityBlock {
		input.OnDemandOptions = &ec2types.OnDemandOptionsRequest{
			AllocationStrategy: b.allocationStrategyProvider.OnDemandAllocationStrategy(ctx, nodeClaim, b.capacityType, b.overlay),
		}
	}
	return input
}
