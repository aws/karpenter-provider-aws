// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make" to regenerate code after modifying this file

// SecurityGroupPolicySpec defines the desired state of SecurityGroupPolicy
type SecurityGroupPolicySpec struct {
	PodSelector            *metav1.LabelSelector `json:"podSelector,omitempty"`
	ServiceAccountSelector *metav1.LabelSelector `json:"serviceAccountSelector,omitempty"`
	SecurityGroups         GroupIds              `json:"securityGroups,omitempty"`
}

// GroupIds contains the list of security groups that will be applied to the network interface of the pod matching the criteria.
type GroupIds struct {
	// Groups is the list of EC2 Security Groups Ids that need to be applied to the ENI of a Pod.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=5
	Groups []string `json:"groupIds,omitempty"`
}

// ServiceAccountSelector contains the selection criteria for matching pod with service account that matches the label selector
// requirement and the exact name of the service account.
type ServiceAccountSelector struct {
	*metav1.LabelSelector `json:",omitempty"`
	// matchNames is the list of service account names. The requirements are ANDed
	// +kubebuilder:validation:MinItems=1
	MatchNames []string `json:"matchNames,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Security-Group-Ids",type=string,JSONPath=`.spec.securityGroups.groupIds`,description="The security group IDs to apply to the elastic network interface of pods that match this policy"
// +kubebuilder:resource:shortName=sgp

// Custom Resource Definition for applying security groups to pods
type SecurityGroupPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SecurityGroupPolicySpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// SecurityGroupPolicyList contains a list of SecurityGroupPolicy
type SecurityGroupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityGroupPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecurityGroupPolicy{}, &SecurityGroupPolicyList{})
}
