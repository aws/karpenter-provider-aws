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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProvisionerSpec struct {
	// +optional
	Cluster     *ClusterSpec    `json:"cluster,omitempty"`
	Allocator   AllocatorSpec   `json:"allocator,omitempty"`
	Reallocator ReallocatorSpec `json:"reallocator,omitempty"`
}

// ClusterSpec configures the cluster that the provisioner operates against. If
// not specified, it will default to using the controller's kube-config.
type ClusterSpec struct {
	// Name is required to detect implementing cloud provider resources.
	// +required
	Name string `json:"name"`
	// CABundle is required for nodes to verify API Server certificates.
	// +required
	CABundle string `json:"caBundle"`
	// Endpoint is required for nodes to connect to the API Server.
	// +required
	Endpoint string `json:"endpoint"`
}

// AllocatorSpec configures node allocation policy
type AllocatorSpec struct {
	InstanceTypes []string `json:"instanceTypes,omitempty"`
}

// ReallocatorSpec configures node reallocation policy
type ReallocatorSpec struct {
}

// Provisioner is the Schema for the Provisioners API
// +kubebuilder:object:root=true
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
