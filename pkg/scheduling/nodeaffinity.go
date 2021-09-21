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

package scheduling

import (
	"github.com/awslabs/karpenter/pkg/utils/functional"
	v1 "k8s.io/api/core/v1"
)

type NodeAffinity []v1.NodeSelectorRequirement

// NodeAffinityFor constructs a set of requirements for the pods
func NodeAffinityFor(pods ...*v1.Pod) (nodeAffinity NodeAffinity) {
	for _, pod := range pods {
		// Convert node selectors to requirements
		for key, value := range pod.Spec.NodeSelector {
			nodeAffinity = append(nodeAffinity, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
		}
		if pod.Spec.Affinity == nil || pod.Spec.Affinity.NodeAffinity == nil {
			continue
		}
		// Preferences are treated as requirements. An outer loop will iteratively unconstrain them if unsatisfiable
		for _, term := range pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			nodeAffinity = append(nodeAffinity, term.Preference.MatchExpressions...)
		}
		if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				nodeAffinity = append(nodeAffinity, term.MatchExpressions...)
			}
		}
	}
	return nodeAffinity
}

// GetLabels returns the label keys specified by the scheduling rules
func (n NodeAffinity) GetLabels() []string {
	keys := map[string]bool{}
	for _, requirement := range n {
		keys[requirement.Key] = true
	}
	result := []string{}
	for key := range keys {
		result = append(result, key)
	}
	return result
}

// GetLabelValues for the provided key. Default values are used to substract options for NotIn.
func (n NodeAffinity) GetLabelValues(label string, constraints ...[]string) []string {
	// Intersect external constraints
	result := functional.IntersectStringSlice(constraints...)
	// OpIn
	for _, requirement := range n {
		if requirement.Key == label && requirement.Operator == v1.NodeSelectorOpIn {
			result = functional.IntersectStringSlice(result, requirement.Values)
		}
	}
	// OpNotIn
	for _, requirement := range n {
		if requirement.Key == label && requirement.Operator == v1.NodeSelectorOpNotIn {
			result = functional.StringSliceWithout(result, requirement.Values...)
		}
	}
	return result
}
