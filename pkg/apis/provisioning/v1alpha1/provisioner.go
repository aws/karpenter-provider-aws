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

type ProvisionerSpec struct {
	// +optional
	Cluster *ClusterSpec `json:"cluster,omitempty"`
	// Taints will be applied to every node launched by the Provisioner. If
	// specified, the provisioner will not provision nodes for pods that do not
	// have matching tolerations.
	// +optional
	Taints []v1.Taint `json:"taints,omitempty"`
	// Labels will be applied to every node launched by the Provisioner unless
	// overriden by pod node selectors. Well known labels control provisioning
	// behavior. Additional labels may be supported by your cloudprovider.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Zones constrains where nodes will be launched by the Provisioner. If
	// unspecified, defaults to all zones in the region. Cannot be specified if
	// label "topology.kubernetes.io/zone" is specified.
	// +optional
	Zones []string `json:"zones,omitempty"`
	// InstanceTypes constraints which instances types will be used for nodes
	// launched by the Provisioner. If unspecified, supports all types. Cannot
	// be specified if label "node.kubernetes.io/instance-type" is specified.
	InstanceTypes []string `json:"instanceTypes,omitempty"`
	// TTLSeconds determines how long to wait before attempting to terminate a node.
	// +optional
	TTLSeconds *int32 `json:"ttlSeconds,omitempty"`
}

var (
	// Well known, supported labels
	ArchitectureLabelKey    = "kubernetes.io/arch"
	OperatingSystemLabelKey = "kubernetes.io/os"

	// Reserved labels
	ProvisionerNameLabelKey      = SchemeGroupVersion.Group + "/name"
	ProvisionerNamespaceLabelKey = SchemeGroupVersion.Group + "/namespace"
	ProvisionerUnderutilizedKey  = SchemeGroupVersion.Group + "/underutilized"
	ProvisionerTTLKey            = SchemeGroupVersion.Group + "/ttl"

	// Use ProvisionerSpec instead
	ZoneLabelKey         = "topology.kubernetes.io/zone"
	InstanceTypeLabelKey = "node.kubernetes.io/instance-type"
)

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
