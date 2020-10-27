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

import "knative.dev/pkg/apis"

// ScalableNodeGroupStatus holds status information for the ScalableNodeGroup
// +kubebuilder:subresource:status
type ScalableNodeGroupStatus struct {
	// Replicas displays the actual size of the ScalableNodeGroup
	// at the time of the last reconciliation
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
	// Conditions is the set of conditions required for the scalable node group
	// to successfully enforce the replica count of the underlying group
	// +optional
	Conditions apis.Conditions `json:"conditions,omitempty"`
}

const (
	// Active indicates that the controller is able to take actions: it's
	// correctly configured, can make necessary API calls, and isn't disabled.
	Active apis.ConditionType = "Active"
)

// We use knative's libraries for ConditionSets to manage status conditions.
// Conditions are all of "true-happy" polarity. If any condition is false, the resource's "happiness" is false.
// https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-conditions
// https://github.com/knative/serving/blob/f1582404be275d6eaaf89ccd908fb44aef9e48b5/vendor/knative.dev/pkg/apis/condition_set.go
func (s *ScalableNodeGroup) StatusConditions() apis.ConditionManager {
	return apis.NewLivingConditionSet(
		Active,
		AbleToScale,
	).Manage(s)
}

func (s *ScalableNodeGroup) MarkActive() {
	s.StatusConditions().MarkTrue(Active)
}

func (s *ScalableNodeGroup) MarkNotActive(message string) {
	s.StatusConditions().MarkFalse(Active, "", message)
}

func (s *ScalableNodeGroup) GetConditions() apis.Conditions {
	return s.Status.Conditions
}

func (s *ScalableNodeGroup) SetConditions(conditions apis.Conditions) {
	s.Status.Conditions = conditions
}
