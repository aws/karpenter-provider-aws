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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
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

func NewInstance(out ec2types.Instance) *Instance {
	return &Instance{
		LaunchTime: aws.ToTime(out.LaunchTime),
		State:      out.State.Name,
		ID:         aws.ToString(out.InstanceId),
		ImageID:    aws.ToString(out.ImageId),
		Type:       out.InstanceType,
		Zone:       aws.ToString(out.Placement.AvailabilityZone),
		CapacityType: func() string {
			switch {
			case out.SpotInstanceRequestId != nil:
				return karpv1.CapacityTypeSpot
			case out.CapacityReservationId != nil:
				return karpv1.CapacityTypeReserved
			default:
				return karpv1.CapacityTypeOnDemand
			}
		}(),
		CapacityReservationID: lo.FromPtr(out.CapacityReservationId),
		SecurityGroupIDs: lo.Map(out.SecurityGroups, func(securitygroup ec2types.GroupIdentifier, _ int) string {
			return aws.ToString(securitygroup.GroupId)
		}),
		SubnetID: aws.ToString(out.SubnetId),
		Tags:     lo.SliceToMap(out.Tags, func(t ec2types.Tag) (string, string) { return aws.ToString(t.Key), aws.ToString(t.Value) }),
		EFAEnabled: lo.ContainsBy(out.NetworkInterfaces, func(item ec2types.InstanceNetworkInterface) bool {
			return item.InterfaceType != nil && *item.InterfaceType == string(ec2types.NetworkInterfaceTypeEfa)
		}),
	}

}

func NewInstanceFromFleet(out ec2types.CreateFleetInstance, tags map[string]string, efaEnabled bool) *Instance {
	return &Instance{
		LaunchTime:   time.Now(), // estimate the launch time since we just launched
		State:        ec2types.InstanceStateNamePending,
		ID:           out.InstanceIds[0],
		ImageID:      aws.ToString(out.LaunchTemplateAndOverrides.Overrides.ImageId),
		Type:         out.InstanceType,
		Zone:         aws.ToString(out.LaunchTemplateAndOverrides.Overrides.AvailabilityZone),
		CapacityType: string(out.Lifecycle),
		SubnetID:     aws.ToString(out.LaunchTemplateAndOverrides.Overrides.SubnetId),
		Tags:         tags,
		EFAEnabled:   efaEnabled,
	}
}
