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

package v1alpha5

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionerSpec is the top level provisioner specification. Provisioners
// launch nodes in response to pods that are unschedulable. A single provisioner
// is capable of managing a diverse set of nodes. Node properties are determined
// from a combination of provisioner and pod scheduling constraints.
type ProvisionerSpec struct {
	// Constraints are applied to all nodes launched by this provisioner.
	Constraints `json:",inline"`
	// TTLSecondsAfterEmpty is the number of seconds the controller will wait
	// before attempting to delete a node, measured from when the node is
	// detected to be empty. A Node is considered to be empty when it does not
	// have pods scheduled to it, excluding daemonsets.
	//
	// Termination due to underutilization is disabled if this field is not set.
	// +optional
	TTLSecondsAfterEmpty *int64 `json:"ttlSecondsAfterEmpty,omitempty"`
	// TTLSecondsUntilExpired is the number of seconds the controller will wait
	// before terminating a node, measured from when the node is created. This
	// is useful to implement features like eventually consistent node upgrade,
	// memory leak protection, and disruption testing.
	//
	// Termination due to expiration is disabled if this field is not set.
	// +optional
	TTLSecondsUntilExpired *int64 `json:"ttlSecondsUntilExpired,omitempty"`
	// Limits define a set of bounds for provisioning capacity.
	Limits *Limits `json:"limits,omitempty"`
}

// Provisioner is the Schema for the Provisioners API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=provisioners,scope=Cluster,categories=karpenter
// +kubebuilder:subresource:status
type Provisioner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProvisionerSpec   `json:"spec,omitempty"`
	Status ProvisionerStatus `json:"status,omitempty"`
}

// ProvisionerList contains a list of Provisioner
// +kubebuilder:object:root=true
type ProvisionerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provisioner `json:"items"`
}
