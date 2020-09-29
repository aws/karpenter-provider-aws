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
	// Replicas displays the current size of the ScalableNodeGroup
	Replicas int32 `json:"replicas,omitempty"`
	// Conditions is the set of conditions required for the scalable node group
	// to successfully enforce the replica count of the underlying group
	Conditions apis.Conditions `json:"conditions,omitempty"`
}

var ScalableNodeGroupConditions = apis.NewLivingConditionSet(
	ScalingActive,
	AbleToScale,
	ScalingUnbounded,
)

func (s *ScalableNodeGroup) IsHappy() bool {
	return ScalableNodeGroupConditions.Manage(s).IsHappy()
}

func (s *ScalableNodeGroup) GetConditions() apis.Conditions {
	return s.Status.Conditions
}

func (s *ScalableNodeGroup) SetConditions(conditions apis.Conditions) {
	s.Status.Conditions = conditions
}
