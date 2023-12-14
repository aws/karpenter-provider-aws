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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
)

// Instance is an internal data representation of either an ec2.Instance or an ec2.FleetInstance
// It contains all the common data that is needed to inject into the Machine from either of these responses
type Instance struct {
	LaunchTime       time.Time
	State            string
	ID               string
	ImageID          string
	Type             string
	Zone             string
	CapacityType     string
	SecurityGroupIDs []string
	SubnetID         string
	Tags             map[string]string
	EFAEnabled       bool
}

func NewInstance(out *ec2.Instance) *Instance {
	return &Instance{
		LaunchTime:   aws.TimeValue(out.LaunchTime),
		State:        aws.StringValue(out.State.Name),
		ID:           aws.StringValue(out.InstanceId),
		ImageID:      aws.StringValue(out.ImageId),
		Type:         aws.StringValue(out.InstanceType),
		Zone:         aws.StringValue(out.Placement.AvailabilityZone),
		CapacityType: lo.Ternary(out.SpotInstanceRequestId != nil, corev1beta1.CapacityTypeSpot, corev1beta1.CapacityTypeOnDemand),
		SecurityGroupIDs: lo.Map(out.SecurityGroups, func(securitygroup *ec2.GroupIdentifier, _ int) string {
			return aws.StringValue(securitygroup.GroupId)
		}),
		SubnetID: aws.StringValue(out.SubnetId),
		Tags:     lo.SliceToMap(out.Tags, func(t *ec2.Tag) (string, string) { return aws.StringValue(t.Key), aws.StringValue(t.Value) }),
		EFAEnabled: lo.ContainsBy(out.NetworkInterfaces, func(ni *ec2.InstanceNetworkInterface) bool {
			return ni != nil && lo.FromPtr(ni.InterfaceType) == ec2.NetworkInterfaceTypeEfa
		}),
	}

}

func NewInstanceFromFleet(out *ec2.CreateFleetInstance, tags map[string]string, efaEnabled bool) *Instance {
	return &Instance{
		LaunchTime:   time.Now(), // estimate the launch time since we just launched
		State:        ec2.StatePending,
		ID:           aws.StringValue(out.InstanceIds[0]),
		ImageID:      aws.StringValue(out.LaunchTemplateAndOverrides.Overrides.ImageId),
		Type:         aws.StringValue(out.InstanceType),
		Zone:         aws.StringValue(out.LaunchTemplateAndOverrides.Overrides.AvailabilityZone),
		CapacityType: aws.StringValue(out.Lifecycle),
		SubnetID:     aws.StringValue(out.LaunchTemplateAndOverrides.Overrides.SubnetId),
		Tags:         tags,
		EFAEnabled:   efaEnabled,
	}
}
