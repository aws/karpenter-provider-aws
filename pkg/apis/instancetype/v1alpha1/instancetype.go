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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstanceTypeSpec is the instance type setting override specification
// for specifying custom values on a per-instance type basis for scheduling and
// launching of nodes
type InstanceTypeSpec struct {
	// Resources contains a map of allocatable resources for the instance type
	// used by the scheduler. This resource list can contain known resources (cpu, memory, etc.)
	// or it may also contain unknown custom device resources for custom device plugins
	// +optional
	Resources v1.ResourceList `json:"resources,omitempty"`
}

// InstanceType is the Schema for the InstanceType API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=instancetypes,scope=Cluster,categories=karpenter
// +kubebuilder:subresource:status
type InstanceType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InstanceTypeSpec `json:"spec,omitempty"`
}

// InstanceTypeList contains a list of InstanceType
// +kubebuilder:object:root=true
type InstanceTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceType `json:"items"`
}
