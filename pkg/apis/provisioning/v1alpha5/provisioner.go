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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	// InstanceTypeFilter allows filtering the instance types provided by the cloud provider.
	InstanceTypeFilter *InstanceTypeFilter `json:"instanceTypeFilter,omitempty"`
}

// InstanceTypeFilter is the schema for the instance type filtering
type InstanceTypeFilter struct {
	// MinResources specifies the minimum amount of resources required for an instance type to be considered.  If an
	// instance type has less than the resources specified here, it will be filtered out.
	MinResources v1.ResourceList `json:"minResources,omitempty"`
	// MaxResources specifies the maximum amount of resources required for an instance type to be considered.  If an
	// instance type has more of any resources specified here, it will be filtered out.  If an instance type doesn't have
	// a resource specified here, the quantity is considered to be zero and the instance type will not be filtered out
	// by this parameter.
	MaxResources v1.ResourceList `json:"maxResources,omitempty"`
	// MemoryPerCPU allows specifying the minimum and maximum amounts of memory per CPU that are required.  This allows
	// filtering out instance types that don't have a desired memory to CPU ratio.
	MemoryPerCPU *MinMax `json:"memoryPerCPU,omitempty"`
	// NameIncludeExpressions are regular expressions to match against instance types names, that if matched the instance types
	// are included.  If no expressions are supplied, then no instance types will be excluded by the NameIncludeExpressions.
	// If multiple expressions are supplied, the NameIncludeExpressions has an OR semantic. If a name is matched by both
	// NameIncludeExpressions and NameExcludeExpressions, it will be excluded.
	NameIncludeExpressions []string `json:"nameIncludeExpressions,omitempty"`
	// NameExcludeExpressions are regular expressions to match against instance types names, that if matched the instance types
	// are excluded.  If no expressions are supplied, then no instance types will be excluded by the NameExcludeExpressions.
	// If multiple expressions are supplied, the NameExcludeExpressions has an OR semantic. If a name is matched by both
	// NameIncludeExpressions and NameExcludeExpressions, it will be excluded.
	NameExcludeExpressions []string `json:"nameExcludeExpressions,omitempty"`
}

// MinMax is the schema for a min/max range.  Both Min and Max are optional allowing configuring just a Min or Max value.
type MinMax struct {
	// Min is the minimum value for the filter.
	Min *resource.Quantity `json:"min,omitempty"`
	// Max is the maximum value for the filter
	Max *resource.Quantity `json:"max,omitempty"`
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
