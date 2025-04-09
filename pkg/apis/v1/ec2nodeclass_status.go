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

var (
	CapacityReservationsEnabled = false
)

const (
	ConditionTypeSubnetsReady              = "SubnetsReady"
	ConditionTypeSecurityGroupsReady       = "SecurityGroupsReady"
	ConditionTypeAMIsReady                 = "AMIsReady"
	ConditionTypeInstanceProfileReady      = "InstanceProfileReady"
	ConditionTypeCapacityReservationsReady = "CapacityReservationsReady"
	ConditionTypeClusterCIDRReady          = "ClusterCIDRReady"
	ConditionTypeValidationSucceeded       = "ValidationSucceeded"
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
	// Deprecation status of the AMI
	// +optional
	Deprecated bool `json:"deprecated,omitempty"`
	// Name of the AMI
	// +optional
	Name string `json:"name,omitempty"`
	// Requirements of the AMI to be utilized on an instance type
	// +required
	Requirements []corev1.NodeSelectorRequirement `json:"requirements"`
}

type CapacityReservation struct {
	// The availability zone the capacity reservation is available in.
	// +required
	AvailabilityZone string `json:"availabilityZone"`
	// The time at which the capacity reservation expires. Once expired, the reserved capacity is released and Karpenter
	// will no longer be able to launch instances into that reservation.
	// +optional
	EndTime *metav1.Time `json:"endTime,omitempty" hash:"ignore"`
	// The id for the capacity reservation.
	// +kubebuilder:validation:Pattern:="^cr-[0-9a-z]+$"
	// +required
	ID string `json:"id"`
	// Indicates the type of instance launches the capacity reservation accepts.
	// +kubebuilder:validation:Enum:={open,targeted}
	// +required
	InstanceMatchCriteria string `json:"instanceMatchCriteria"`
	// The instance type for the capacity reservation.
	// +required
	InstanceType string `json:"instanceType"`
	// The ID of the AWS account that owns the capacity reservation.
	// +kubebuilder:validation:Pattern:="^[0-9]{12}$"
	// +required
	OwnerID string `json:"ownerID"`
}

// EC2NodeClassStatus contains the resolved state of the EC2NodeClass
type EC2NodeClassStatus struct {
	// Subnets contains the current subnet values that are available to the
	// cluster under the subnet selectors.
	// +optional
	Subnets []Subnet `json:"subnets,omitempty"`
	// SecurityGroups contains the current security group values that are available to the
	// cluster under the SecurityGroups selectors.
	// +optional
	SecurityGroups []SecurityGroup `json:"securityGroups,omitempty"`
	// CapacityReservations contains the current capacity reservation values that are available to this NodeClass under the
	// CapacityReservation selectors.
	// +optional
	CapacityReservations []CapacityReservation `json:"capacityReservations,omitempty"`
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
	conds := []string{
		ConditionTypeAMIsReady,
		ConditionTypeSubnetsReady,
		ConditionTypeSecurityGroupsReady,
		ConditionTypeInstanceProfileReady,
		ConditionTypeClusterCIDRReady,
		ConditionTypeValidationSucceeded,
	}
	if CapacityReservationsEnabled {
		conds = append(conds, ConditionTypeCapacityReservationsReady)
	}
	return status.NewReadyConditions(conds...).For(in)
}

func (in *EC2NodeClass) GetConditions() []status.Condition {
	return in.Status.Conditions
}

func (in *EC2NodeClass) SetConditions(conditions []status.Condition) {
	in.Status.Conditions = conditions
}
