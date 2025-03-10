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
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator/options"
)

// Instance is an internal data representation of either an ec2.Instance or an ec2.FleetInstance
// It contains all the common data that is needed to inject into the Machine from either of these responses
type Instance struct {
	LaunchTime            time.Time
	State                 ec2types.InstanceStateName
	ID                    string
	ImageID               string
	Type                  ec2types.InstanceType
	Zone                  string
	CapacityType          string
	CapacityReservationID string
	SecurityGroupIDs      []string
	SubnetID              string
	Tags                  map[string]string
	EFAEnabled            bool
}

func NewInstance(ctx context.Context, out ec2types.Instance) *Instance {
	return &Instance{
		LaunchTime: lo.FromPtr(out.LaunchTime),
		State:      out.State.Name,
		ID:         lo.FromPtr(out.InstanceId),
		ImageID:    lo.FromPtr(out.ImageId),
		Type:       out.InstanceType,
		Zone:       lo.FromPtr(out.Placement.AvailabilityZone),
		// NOTE: Only set the capacity type to reserved and assign a reservation ID if the feature gate is enabled. It's
		// possible for these to be set if the instance launched into an open ODCR, but treating it as reserved would induce
		// drift.
		CapacityType: lo.If(out.SpotInstanceRequestId != nil, karpv1.CapacityTypeSpot).
			ElseIf(out.CapacityReservationId != nil && options.FromContext(ctx).FeatureGates.ReservedCapacity, karpv1.CapacityTypeReserved).
			Else(karpv1.CapacityTypeOnDemand),
		CapacityReservationID: lo.Ternary(
			options.FromContext(ctx).FeatureGates.ReservedCapacity,
			lo.FromPtr(out.CapacityReservationId),
			"",
		),
		SecurityGroupIDs: lo.Map(out.SecurityGroups, func(securitygroup ec2types.GroupIdentifier, _ int) string {
			return lo.FromPtr(securitygroup.GroupId)
		}),
		SubnetID: lo.FromPtr(out.SubnetId),
		Tags:     lo.SliceToMap(out.Tags, func(t ec2types.Tag) (string, string) { return lo.FromPtr(t.Key), lo.FromPtr(t.Value) }),
		EFAEnabled: lo.ContainsBy(out.NetworkInterfaces, func(item ec2types.InstanceNetworkInterface) bool {
			return item.InterfaceType != nil && *item.InterfaceType == string(ec2types.NetworkInterfaceTypeEfa)
		}),
	}

}

func NewInstanceFromFleet(
	out ec2types.CreateFleetInstance,
	tags map[string]string,
	capacityType string,
	capacityReservationID string,
	efaEnabled bool,
) *Instance {
	return &Instance{
		LaunchTime:            time.Now(), // estimate the launch time since we just launched
		State:                 ec2types.InstanceStateNamePending,
		ID:                    out.InstanceIds[0],
		ImageID:               lo.FromPtr(out.LaunchTemplateAndOverrides.Overrides.ImageId),
		Type:                  out.InstanceType,
		Zone:                  lo.FromPtr(out.LaunchTemplateAndOverrides.Overrides.AvailabilityZone),
		CapacityType:          capacityType,
		CapacityReservationID: capacityReservationID,
		SubnetID:              lo.FromPtr(out.LaunchTemplateAndOverrides.Overrides.SubnetId),
		Tags:                  tags,
		EFAEnabled:            efaEnabled,
	}
}
