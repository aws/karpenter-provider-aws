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
	"k8s.io/apimachinery/pkg/util/sets"
)

// Requirements is a decorated alias type for []v1.NodeSelectorRequirements
type Requirements []v1.NodeSelectorRequirement

func (r Requirements) Zones() sets.String {
	return r.Requirement(v1.LabelTopologyZone)
}

func (r Requirements) InstanceTypes() sets.String {
	return r.Requirement(v1.LabelInstanceTypeStable)
}

func (r Requirements) Architectures() sets.String {
	return r.Requirement(v1.LabelArchStable)
}

func (r Requirements) OperatingSystems() sets.String {
	return r.Requirement(v1.LabelOSStable)
}

func (r Requirements) CapacityTypes() sets.String {
	return r.Requirement(LabelCapacityType)
}

func (r Requirements) With(requirements Requirements) Requirements {
	return append(r, requirements...)
}

func LabelRequirements(labels map[string]string) (r Requirements) {
	for key, value := range labels {
		r = append(r, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	return r
}

func PodRequirements(pod *v1.Pod) (r Requirements) {
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

// Consolidate combines In and NotIn requirements for each unique key, producing
// an equivalent minimal representation of the requirements. This is useful as
// requirements may be appended from a variety of sources and then consolidated.
// Caution: If a key has contains a `NotIn` operator without a corresponding
// `In` operator, the requirement will permanently be [] after consolidation. To
// avoid this, include the broadest `In` requirements before consolidating.
func (r Requirements) Consolidate() (requirements Requirements) {
	for _, key := range r.Keys() {
		requirements = append(requirements, v1.NodeSelectorRequirement{
			Key:      key,
			Operator: v1.NodeSelectorOpIn,
			Values:   r.Requirement(key).UnsortedList(),
		})
	}
	return requirements
}

func (r Requirements) WellKnown() (requirements Requirements) {
	for _, requirement := range r {
		if WellKnownLabels.Has(requirement.Key) {
			requirements = append(requirements, requirement)
		}
	}
	return requirements
}

// Keys returns unique set of the label keys from the requirements
func (r Requirements) Keys() []string {
	keys := sets.NewString()
	for _, requirement := range r {
		keys.Insert(requirement.Key)
	}
	return keys.UnsortedList()
}

// Requirements for the provided key, nil if unconstrained
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
	return result
}
