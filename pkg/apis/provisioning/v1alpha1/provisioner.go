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
	"github.com/awslabs/karpenter/pkg/utils/functional"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionerSpec is the top level provisioner specification. Provisioners
// launch nodes in response to pods where status.conditions[type=unschedulable,
// status=true]. Node configuration is driven by through a combination of
// provisioner specification (defaults) and pod scheduling constraints
// (overrides). A single provisioner is capable of managing highly diverse
// capacity within a single cluster and in most cases, only one should be
// necessary. For advanced use cases like workload separation and sharding, it's
// possible to define multiple provisioners. These provisioners may have
// different defaults and can be specifically targeted by pods using
// pod.spec.nodeSelector["provisioning.karpenter.sh/name"]=$PROVISIONER_NAME.
type ProvisionerSpec struct {
	// +optional
	Cluster *ClusterSpec `json:"cluster,omitempty"`
	// Constraints applied to nodes created by the provisioner
	Constraints `json:",inline"`
	// TTLSeconds determines how long to wait before attempting to terminate a node.
	// +optional
	TTLSeconds *int32 `json:"ttlSeconds,omitempty"`
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

// Constraints are applied to all nodes created by the provisioner. They can be
// overriden by NodeSelectors at the pod level.
type Constraints struct {
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
	// InstanceTypes constrains which instances types will be used for nodes
	// launched by the Provisioner. If unspecified, it will support all types.
	// Cannot be specified if label "node.kubernetes.io/instance-type" is specified.
	// +optional
	InstanceTypes []string `json:"instanceTypes,omitempty"`
	// Architecture constrains the underlying node architecture
	// +optional
	Architecture *string `json:"architecture,omitempty"`
	// OperatingSystem constrains the underlying node operating system
	// +optional
	OperatingSystem *string `json:"operatingSystem,omitempty"`
}

var (
	ArchitectureAmd64 = "amd64"
	ArchitectureArm64 = "arm64"
)

var (
	OperatingSystemLinux = "linux"
)

var (
	// Well known, supported labels
	ArchitectureLabelKey    = "kubernetes.io/arch"
	OperatingSystemLabelKey = "kubernetes.io/os"

	// Reserved labels
	ProvisionerNameLabelKey          = SchemeGroupVersion.Group + "/name"
	ProvisionerNamespaceLabelKey     = SchemeGroupVersion.Group + "/namespace"
	ProvisionerUnderutilizedLabelKey = SchemeGroupVersion.Group + "/underutilized"

	// Reserved annotations
	ProvisionerTTLKey = SchemeGroupVersion.Group + "/ttl"

	// Use ProvisionerSpec instead
	ZoneLabelKey         = "topology.kubernetes.io/zone"
	InstanceTypeLabelKey = "node.kubernetes.io/instance-type"
)

const (
	KarpenterFinalizer = "karpenter.sh/termination"
)

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

func (c *Constraints) WithLabel(key string, value string) *Constraints {
	c.Labels = functional.UnionStringMaps(c.Labels, map[string]string{key: value})
	return c
}

func (c *Constraints) WithOverrides(pod *v1.Pod) *Constraints {
	return &Constraints{
		Taints:          c.Taints,
		Labels:          functional.UnionStringMaps(c.Labels, pod.Spec.NodeSelector),
		Zones:           c.getZones(pod),
		InstanceTypes:   c.getInstanceTypes(pod),
		Architecture:    c.getArchitecture(pod),
		OperatingSystem: c.getOperatingSystem(pod),
	}
}

func (c *Constraints) getZones(pod *v1.Pod) []string {
	// Pod may override zone
	if zone, ok := pod.Spec.NodeSelector[ZoneLabelKey]; ok {
		return []string{zone}
	}
	// Default to provisioner constraints
	if len(c.Zones) != 0 {
		return c.Zones
	}
	// Otherwise unconstrained
	return nil
}

func (c *Constraints) getInstanceTypes(pod *v1.Pod) []string {
	// Pod may override instance type
	if instanceType, ok := pod.Spec.NodeSelector[InstanceTypeLabelKey]; ok {
		return []string{instanceType}
	}
	// Default to provisioner constraints
	if len(c.InstanceTypes) != 0 {
		return c.InstanceTypes
	}
	// Otherwise unconstrained
	return nil
}

func (c *Constraints) getArchitecture(pod *v1.Pod) *string {
	// Pod may override arch
	if architecture, ok := pod.Spec.NodeSelector[ArchitectureLabelKey]; ok {
		return &architecture
	}
	// Use constraints if defined
	if c.Architecture != nil {
		return c.Architecture
	}
	// Default to amd64
	return &ArchitectureAmd64
}

func (c *Constraints) getOperatingSystem(pod *v1.Pod) *string {
	// Pod may override os
	if operatingSystem, ok := pod.Spec.NodeSelector[OperatingSystemLabelKey]; ok {
		return &operatingSystem
	}
	// Use constraints if defined
	if c.OperatingSystem != nil {
		return c.OperatingSystem
	}
	// Default to linux
	return &OperatingSystemLinux
}
