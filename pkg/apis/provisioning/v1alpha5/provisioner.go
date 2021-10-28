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
	"sort"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ProvisionerSpec is the top level provisioner specification. Provisioners
// launch nodes in response to pods where status.conditions[type=unschedulable,
// status=true]. Node configuration is driven by through a combination of
// provisioner specification (defaults) and pod scheduling constraints
// (overrides). A single provisioner is capable of managing highly diverse
// capacity within a single cluster and in most cases, only one should be
// necessary. It's possible to define multiple provisioners. These provisioners
// may have different defaults and can be specifically targeted by pods using
// pod.spec.nodeSelector["karpenter.sh/provisioner-name"]=$PROVISIONER_NAME.
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
}

// Constraints are applied to all nodes created by the provisioner.
type Constraints struct {
	// Labels are layered with Requirements and applied to every node.
	//+optional
	Labels map[string]string `json:"labels,omitempty"`
	// Taints will be applied to every node launched by the Provisioner. If
	// specified, the provisioner will not provision nodes for pods that do not
	// have matching tolerations. Additional taints will be created that match
	// pod tolerations on a per-node basis.
	// +optional
	Taints []v1.Taint `json:"taints,omitempty"`
	// Requirements are layered with Labels and applied to every node.
	Requirements Requirements `json:"requirements,omitempty"`
	// Provider contains fields specific to your cloudprovider.
	// +kubebuilder:pruning:PreserveUnknownFields
	Provider *runtime.RawExtension `json:"provider,omitempty"`
}

// Requirements is a decorated alias type for []v1.NodeSelectorRequirements
type Requirements []v1.NodeSelectorRequirement

// Provisioner is the Schema for the Provisioners API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=provisioners,scope=Cluster
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

// Zones for the constraints
func (r Requirements) Zones() sets.String {
	return r.Requirement(v1.LabelTopologyZone)
}

// InstanceTypes for the constraints
func (r Requirements) InstanceTypes() sets.String {
	return r.Requirement(v1.LabelInstanceTypeStable)
}

// Architectures for the constraints
func (r Requirements) Architectures() sets.String {
	return r.Requirement(v1.LabelArchStable)
}

// OperatingSystems for the constraints
func (r Requirements) OperatingSystems() sets.String {
	return r.Requirement(v1.LabelOSStable)
}

func (r Requirements) WithProvisioner(provisioner Provisioner) Requirements {
	return r.
		With(provisioner.Spec.Requirements).
		WithLabels(provisioner.Spec.Labels).
		WithLabels(map[string]string{ProvisionerNameLabelKey: provisioner.Name})
}

func (r Requirements) With(requirements Requirements) Requirements {
	return append(r, requirements...)
}

func (r Requirements) WithLabels(labels map[string]string) Requirements {
	for key, value := range labels {
		r = append(r, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	return r
}

func (r Requirements) WithPod(pod *v1.Pod) Requirements {
	for key, value := range pod.Spec.NodeSelector {
		r = append(r, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	if pod.Spec.Affinity == nil || pod.Spec.Affinity.NodeAffinity == nil {
		return r
	}
	// Select heaviest preference and treat as a requirement. An outer loop will iteratively unconstrain them if unsatisfiable.
	if preferred := pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution; len(preferred) > 0 {
		sort.Slice(preferred, func(i int, j int) bool { return preferred[i].Weight > preferred[j].Weight })
		r = append(r, preferred[0].Preference.MatchExpressions...)
	}
	// Select first requirement. An outer loop will iteratively remove OR requirements if unsatisfiable
	if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
		len(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) > 0 {
		r = append(r, pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions...)
	}
	return r
}

func (r Requirements) Consolidate() (requirements Requirements) {
	for _, key := range r.Keys() {
		requirements = append(requirements, v1.NodeSelectorRequirement{
			Key:      key,
			Operator: v1.NodeSelectorOpIn,
			Values:   r.Requirement(key).List(),
		})
	}
	return requirements
}

func (r Requirements) CustomLabels() map[string]string {
	labels := map[string]string{}
	for _, key := range r.Keys() {
		if !WellKnownLabels.Has(key) {
			if requirement := r.Requirement(key); len(requirement) > 0 {
				labels[key] = requirement.List()[0]
			}
		}
	}
	return labels
}

func (r Requirements) WellKnown() (requirements Requirements) {
	for _, requirement := range r {
		if WellKnownLabels.Has(requirement.Key) {
			requirements = append(requirements, requirement)
		}
	}
	return requirements
}

// GetLabels returns unique set of the label keys from the requirements
func (r Requirements) Keys() []string {
	keys := sets.NewString()
	for _, requirement := range r {
		keys.Insert(requirement.Key)
	}
	return keys.List()
}

// Values for the provided key constrained by the requirements
func (r Requirements) Requirement(key string) sets.String {
	var result sets.String
	// OpIn
	for _, requirement := range r {
		if requirement.Key == key && requirement.Operator == v1.NodeSelectorOpIn {
			if result == nil {
				result = sets.NewString(requirement.Values...)
			} else {
				result = result.Intersection(sets.NewString(requirement.Values...))
			}
		}
	}
	// OpNotIn
	for _, requirement := range r {
		if requirement.Key == key && requirement.Operator == v1.NodeSelectorOpNotIn {
			result = result.Difference(sets.NewString(requirement.Values...))
		}
	}
	// Unconstrained
	if result == nil {
		return nil
	}
	return result
}
