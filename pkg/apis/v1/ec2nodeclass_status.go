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
	"fmt"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/serrors"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
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
	// RootDeviceName is the device name of the root volume (/dev/xvda)
	// +optional
	RootDeviceName string `json:"rootDeviceName,omitempty"`
	// RootDeviceSnapshotID is the snapshot ID of the AMI's root device
	// +optional
	RootDeviceSnapshotID string `json:"rootDeviceSnapshotID,omitempty"`
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
	// The type of capacity reservation.
	// +kubebuilder:validation:Enum:={default,capacity-block}
	// +kubebuilder:default=default
	// +optional
	ReservationType CapacityReservationType `json:"reservationType"`
	// The state of the capacity reservation. A capacity reservation is considered to be expiring if it is within the EC2
	// reclaimation window. Only capacity-block reservations may be in this state.
	// +kubebuilder:validation:Enum:={active,expiring}
	// +kubebuilder:default=active
	// +optional
	State CapacityReservationState `json:"state"`
}

type CapacityReservationType string

const (
	CapacityReservationTypeDefault       CapacityReservationType = "default"
	CapacityReservationTypeCapacityBlock CapacityReservationType = "capacity-block"
)

func (CapacityReservationType) Values() []CapacityReservationType {
	return []CapacityReservationType{
		CapacityReservationTypeDefault,
		CapacityReservationTypeCapacityBlock,
	}
}

type CapacityReservationState string

const (
	CapacityReservationStateActive   CapacityReservationState = "active"
	CapacityReservationStateExpiring CapacityReservationState = "expiring"
)

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

func (in *EC2NodeClass) AMIs() []AMI {
	return in.Status.AMIs
}

func (in *EC2NodeClass) CapacityReservations() []CapacityReservation {
	return in.Status.CapacityReservations
}

type ZoneInfo struct {
	Zone   string
	ZoneID string
}

func (in *EC2NodeClass) ZoneInfo() []ZoneInfo {
	return lo.Map(in.Status.Subnets, func(_ Subnet, i int) ZoneInfo {
		return ZoneInfo{
			Zone:   in.Status.Subnets[i].Zone,
			ZoneID: in.Status.Subnets[i].ZoneID,
		}
	})
}

func CapacityReservationTypeFromEC2(capacityReservationType ec2types.CapacityReservationType) (CapacityReservationType, error) {
	if capacityReservationType == "" {
		return CapacityReservationTypeDefault, nil
	}
	resolvedType, ok := lo.Find(CapacityReservationType("").Values(), func(crt CapacityReservationType) bool {
		return string(crt) == string(capacityReservationType)
	})
	if !ok {
		return "", serrors.Wrap(
			fmt.Errorf("received capacity reservation with unsupported reservation type from ec2"),
			"reservation-type", string(capacityReservationType),
		)
	}
	return resolvedType, nil
}

func CapacityReservationFromEC2(clk clock.Clock, cr *ec2types.CapacityReservation) (CapacityReservation, error) {
	const capacityReservationExpirationPeriod = time.Minute * 40
	// Guard against new instance match criteria added in the future. See https://github.com/kubernetes-sigs/karpenter/issues/806
	// for a similar issue.
	if !lo.Contains([]ec2types.InstanceMatchCriteria{
		ec2types.InstanceMatchCriteriaOpen,
		ec2types.InstanceMatchCriteriaTargeted,
	}, cr.InstanceMatchCriteria) {
		return CapacityReservation{}, serrors.Wrap(
			fmt.Errorf("received capacity reservation with unsupported instance match criteria from ec2"),
			"capacity-reservation", *cr.CapacityReservationId,
			"instance-match-criteria", cr.InstanceMatchCriteria,
		)
	}
	reservationType, err := CapacityReservationTypeFromEC2(cr.ReservationType)
	if err != nil {
		return CapacityReservation{}, serrors.Wrap(err, "capacity-reservation", *cr.CapacityReservationId)
	}
	var endTime *metav1.Time
	if cr.EndDate != nil {
		endTime = lo.ToPtr(metav1.NewTime(*cr.EndDate))
	}
	var state CapacityReservationState
	if reservationType != CapacityReservationTypeCapacityBlock || endTime == nil || clk.Now().Before(endTime.Add(-capacityReservationExpirationPeriod)) {
		state = CapacityReservationStateActive
	} else {
		state = CapacityReservationStateExpiring
	}
	return CapacityReservation{
		AvailabilityZone:      *cr.AvailabilityZone,
		EndTime:               endTime,
		ID:                    *cr.CapacityReservationId,
		InstanceMatchCriteria: string(cr.InstanceMatchCriteria),
		InstanceType:          *cr.InstanceType,
		OwnerID:               *cr.OwnerId,
		ReservationType:       reservationType,
		State:                 state,
	}, nil
}
