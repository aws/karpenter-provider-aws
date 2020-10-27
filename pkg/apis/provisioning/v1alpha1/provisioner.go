/*
Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Provisioner defines a set of underlying nodes. Nodes may be different shapes,
// but fulfill a specific contract according to the provisioner's specification.
//
// 1. Equally Sized: Replicas * Provisioner.Capacity = Σ(Nodes, Node.Capacity)
//
// 2. Equally Packable: Node.Capacity % Provisioner.Capacity = 0
//
// 3. Equally Scheduable: Provisioner.Labels,Taints = Node.Labels,Taints
//
// These properties allow for capacity to be specified in terms of commonly
// used workload sharding patterns without explicitly specifying the underlying
// machine types to meet these goals. This enables the controller to optimize
// costs, avoid insufficient capacity errors, and migrate to new machine types,
// all without user intervention.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
type Provisioner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProvisionerSpec   `json:"spec,omitempty"`
	Status ProvisionerStatus `json:"status,omitempty"`
}

// ProvisionerList contains a list of Provisioner +kubebuilder:object:root=true
type ProvisionerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provisioner `json:"items"`
}

// ProvisionerSpec defines the Provisioner
type ProvisionerSpec struct {
	// CloudProvider specifies where the capacity should be hosted
	Type CloudProvider `json:"type"`

	// Replicas controls number of copies of capacity managed by the
	// provisioner. This field implements the scale subresoure for autoscaling.
	Replicas int32 `json:"replicas"`

	// Capacity specifies a resource shape that will be made available by the
	// provisioner
	Capacity v1.ResourceList `json:"capacity,omitempty"`

	// Taints, similar to labels, will be applied to all underlying nodes
	// created by the provisioner.
	Taints []v1.Taint `json:"taints,omitempty"`

	// Locality specifies the physical spacial distribution of the nodes.
	Locality Locality `json:"locality,omitempty"`

	// Preemptable controls if nodes can be reclaimed by the provider.
	Preemptable bool `json:"preemptable"`
}

// CloudProvider type, see enum below.
type CloudProvider string

const (
	// AWS CloudProvider launches nodes using AWS EC2
	AWS CloudProvider = "AWS"
)

// Locality type, see enum below.
type Locality string

const (
	// Zonal Locality is commonly used for high performance computing use cases.
	Zonal Locality = "Zonal"
	// Regional Locality is commonly used for high availability use cases.
	Regional Locality = "Regional"
)
