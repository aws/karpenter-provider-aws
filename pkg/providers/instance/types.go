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
	karpopts "sigs.k8s.io/karpenter/pkg/operator/options"
)

// Instance is an internal data representation of either an ec2.Instance or an ec2.FleetInstance
// It contains all the common data that is needed to inject into the Machine from either of these responses
type Instance struct {
	LaunchTime              time.Time
	State                   ec2types.InstanceStateName
	ID                      string
	ImageID                 string
	Type                    ec2types.InstanceType
	Zone                    string
	CapacityType            string
	SecurityGroupIDs        []string
	SubnetID                string
	Tags                    map[string]string
	EFAEnabled              bool
	CapacityReservationID   *string
	CapacityReservationType *v1.CapacityReservationType
	Tenancy                 string
}

func NewInstance(ctx context.Context, instance ec2types.Instance) *Instance {
	capacityType := capacityTypeFromInstance(ctx, instance)
	var capacityReservationID *string
	if capacityType == karpv1.CapacityTypeReserved {
		capacityReservationID = lo.ToPtr(*instance.CapacityReservationId)
	}
	return &Instance{
		LaunchTime: lo.FromPtr(instance.LaunchTime),
		State:      instance.State.Name,
		ID:         lo.FromPtr(instance.InstanceId),
		ImageID:    lo.FromPtr(instance.ImageId),
		Type:       instance.InstanceType,
		Zone:       lo.FromPtr(instance.Placement.AvailabilityZone),
		// NOTE: Only set the capacity type to reserved and assign a reservation ID if the feature gate is enabled. It's
		// possible for these to be set if the instance launched into an open ODCR, but treating it as reserved would induce
		// drift.
		CapacityType: capacityType,
		SecurityGroupIDs: lo.Map(instance.SecurityGroups, func(securitygroup ec2types.GroupIdentifier, _ int) string {
			return lo.FromPtr(securitygroup.GroupId)
		}),
		SubnetID: lo.FromPtr(instance.SubnetId),
		Tags:     lo.SliceToMap(instance.Tags, func(t ec2types.Tag) (string, string) { return lo.FromPtr(t.Key), lo.FromPtr(t.Value) }),
		EFAEnabled: lo.ContainsBy(instance.NetworkInterfaces, func(item ec2types.InstanceNetworkInterface) bool {
			return item.InterfaceType != nil && *item.InterfaceType == string(ec2types.NetworkInterfaceTypeEfa)
		}),
		CapacityReservationID: capacityReservationID,
		CapacityReservationType: lo.If[*v1.CapacityReservationType](capacityType != karpv1.CapacityTypeReserved, nil).
			ElseIf(instance.InstanceLifecycle == ec2types.InstanceLifecycleTypeCapacityBlock, lo.ToPtr(v1.CapacityReservationTypeCapacityBlock)).
			Else(lo.ToPtr(v1.CapacityReservationTypeDefault)),
		Tenancy: tenancyFromInstance(instance),
	}
}

func tenancyFromInstance(instance ec2types.Instance) string {
	var tenancy = instance.Placement.Tenancy
	if tenancy == "" {
		tenancy = ec2types.TenancyDefault
	}
	return string(tenancy)
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

func WithCapacityReservationDetails(id string, crt v1.CapacityReservationType) NewInstanceFromFleetOpts {
	return func(i *Instance) {
		i.CapacityReservationID = lo.ToPtr(id)
		i.CapacityReservationType = lo.ToPtr(crt)
	}
}

func WithEFAEnabled() NewInstanceFromFleetOpts {
	return func(i *Instance) { i.EFAEnabled = true }
}

func WithTenancy(tenancy string) NewInstanceFromFleetOpts {
	return func(i *Instance) { i.Tenancy = tenancy }
}

func NewInstanceFromFleet(
	out ec2types.CreateFleetInstance,
	capacityType string,
	tags map[string]string,
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
	}))
	return resolved
}

type CreateFleetInputBuilder struct {
	capacityType          string
	tagSpecifications     []ec2types.TagSpecification
	launchTemplateConfigs []ec2types.FleetLaunchTemplateConfigRequest

	contextID               *string
	capacityReservationType v1.CapacityReservationType
	overlay                 bool
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
		launchTemplateConfigs: launchTemplateConfigs,
		overlay:               false,
	}
}

func (b *CreateFleetInputBuilder) WithContextID(contextID string) *CreateFleetInputBuilder {
	b.contextID = &contextID
	return b
}

func (b *CreateFleetInputBuilder) WithOverlay() *CreateFleetInputBuilder {
	b.overlay = true
	return b
}

func (b *CreateFleetInputBuilder) WithCapacityReservationType(crt v1.CapacityReservationType) *CreateFleetInputBuilder {
	if b.capacityType != karpv1.CapacityTypeReserved {
		panic("can not specify capacity reservation type when capacity type is not reserved")
	}
	b.capacityReservationType = crt
	return b
}

func (b *CreateFleetInputBuilder) defaultTargetCapacityType() ec2types.DefaultTargetCapacityType {
	switch b.capacityType {
	case karpv1.CapacityTypeReserved:
		if b.capacityReservationType == v1.CapacityReservationTypeCapacityBlock {
			return ec2types.DefaultTargetCapacityTypeCapacityBlock
		} else {
			return ec2types.DefaultTargetCapacityTypeOnDemand
		}
	case karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeSpot:
		return ec2types.DefaultTargetCapacityType(b.capacityType)
	}
	panic(fmt.Sprintf("invalid capacity type %q provided to create fleet input", b.capacityType))
}

func (b *CreateFleetInputBuilder) Build() *ec2.CreateFleetInput {
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
			AllocationStrategy: lo.Ternary(b.overlay, ec2types.SpotAllocationStrategyCapacityOptimizedPrioritized, ec2types.SpotAllocationStrategyPriceCapacityOptimized),
		}
	} else if b.capacityReservationType != v1.CapacityReservationTypeCapacityBlock {
		input.OnDemandOptions = &ec2types.OnDemandOptionsRequest{
			AllocationStrategy: lo.Ternary(b.overlay, ec2types.FleetOnDemandAllocationStrategyPrioritized, ec2types.FleetOnDemandAllocationStrategyLowestPrice),
		}
	}
	return input
}
