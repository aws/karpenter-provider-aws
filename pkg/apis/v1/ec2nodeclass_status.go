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

package v1

import (
	"github.com/awslabs/operatorpkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionTypeSubnetsReady         = "SubnetsReady"
	ConditionTypeSecurityGroupsReady  = "SecurityGroupsReady"
	ConditionTypeAMIsReady            = "AMIsReady"
	ConditionTypeInstanceProfileReady = "InstanceProfileReady"
)

// Subnet contains resolved Subnet selector values utilized for node launch
type Subnet struct {
	// ID of the subnet
	// +required
	ID string `json:"id"`
	// The associated availability zone
	// +required
	Zone string `json:"zone"`
	// The associated availability zone ID
	// +optional
	ZoneID string `json:"zoneID,omitempty"`
}

// SecurityGroup contains resolved SecurityGroup selector values utilized for node launch
type SecurityGroup struct {
	// ID of the security group
	// +required
	ID string `json:"id"`
	// Name of the security group
	// +optional
	Name string `json:"name,omitempty"`
}

// AMI contains resolved AMI selector values utilized for node launch
type AMI struct {
	// ID of the AMI
	// +required
	ID string `json:"id"`
	// Name of the AMI
	// +optional
	Name string `json:"name,omitempty"`
	// Requirements of the AMI to be utilized on an instance type
	// +required
	Requirements []corev1.NodeSelectorRequirement `json:"requirements"`
}

// CapacityReservation contains resolved Capacity Reservation selector values utilized for node launch
type CapacityReservation struct {
	// ID of the Capacity Reservation
	// +required
	ID string `json:"id"`
	// AvailabilityZone of the Capacity Reservation
	// +required
	AvailabilityZone string `json:"availabilityZone"`
	// Available Instance Count of the Capacity Reservation
	// +required
	AvailableInstanceCount int `json:"availableInstanceCount"`
	// The date and time at which the Capacity Reservation expires. When a Capacity
	// Reservation expires, the reserved capacity is released and you can no longer
	// launch instances into it. The Capacity Reservation's state changes to expired
	// when it reaches its end date and time.
	// +optional
	EndTime *metav1.Time `json:"endTime,omitempty"`
	// Indicates the type of instance launches that the Capacity Reservation accepts. The options include:
	//   - open:
	//       The Capacity Reservation accepts all instances that have
	//       matching attributes (instance type, platform, and Availability
	//       Zone). Instances that have matching attributes launch into the
	//       Capacity Reservation automatically without specifying any
	//       additional parameters.
	//   - targeted:
	//       The Capacity Reservation only accepts instances that
	//       have matching attributes (instance type, platform, and
	//       Availability Zone), and explicitly target the Capacity
	//       Reservation. This ensures that only permitted instances can use
	//       the reserved capacity.
	// +required
	InstanceMatchCriteria string `json:"instanceMatchCriteria"`
	// Instance Type of the Capacity Reservation
	// +required
	InstanceType string `json:"instanceType"`
	// The id of the AWS account that owns the Capacity Reservation
	// +required
	OwnerID string `json:"ownerId"`
	// Total Instance Count of the Capacity Reservation
	// +required
	TotalInstanceCount int `json:"totalInstanceCount"`
}

// EC2NodeClassStatus contains the resolved state of the EC2NodeClass
type EC2NodeClassStatus struct {
	// CapacityReservations contains the current Capacity Reservations values that are available to the
	// cluster under the CapacityReservations selectors.
	// +optional
	CapacityReservations []CapacityReservation `json:"capacityReservations,omitempty"`
	// Subnets contains the current Subnet values that are available to the
	// cluster under the subnet selectors.
	// +optional
	Subnets []Subnet `json:"subnets,omitempty"`
	// SecurityGroups contains the current Security Groups values that are available to the
	// cluster under the SecurityGroups selectors.
	// +optional
	SecurityGroups []SecurityGroup `json:"securityGroups,omitempty"`
	// AMI contains the current AMI values that are available to the
	// cluster under the AMI selectors.
	// +optional
	AMIs []AMI `json:"amis,omitempty"`
	// InstanceProfile contains the resolved instance profile for the role
	// +optional
	InstanceProfile string `json:"instanceProfile,omitempty"`
	// Conditions contains signals for health and readiness
	// +optional
	Conditions []status.Condition `json:"conditions,omitempty"`
}

func (in *EC2NodeClass) StatusConditions() status.ConditionSet {
	return status.NewReadyConditions(
		ConditionTypeAMIsReady,
		ConditionTypeSubnetsReady,
		ConditionTypeSecurityGroupsReady,
		ConditionTypeInstanceProfileReady,
	).For(in)
}

func (in *EC2NodeClass) GetConditions() []status.Condition {
	return in.Status.Conditions
}

func (in *EC2NodeClass) SetConditions(conditions []status.Condition) {
	in.Status.Conditions = conditions
}
