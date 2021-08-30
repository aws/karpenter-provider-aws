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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	v1 "k8s.io/api/core/v1"
)

func NewConstraintsWithOverrides(constraints *v1alpha3.Constraints, pod *v1.Pod) *v1alpha3.Constraints {
	return &v1alpha3.Constraints{
		Provider:         constraints.Provider,
		Labels:           functional.UnionStringMaps(constraints.Labels, pod.Spec.NodeSelector),
		Taints:           overrideTaints(constraints.Taints, pod),
		Zones:            GetOrDefault(v1.LabelTopologyZone, pod.Spec.NodeSelector, constraints.Zones),
		InstanceTypes:    GetOrDefault(v1.LabelInstanceTypeStable, pod.Spec.NodeSelector, constraints.InstanceTypes),
		Architectures:    GetOrDefault(v1.LabelArchStable, pod.Spec.NodeSelector, constraints.Architectures),
		OperatingSystems: GetOrDefault(v1.LabelOSStable, pod.Spec.NodeSelector, constraints.OperatingSystems),
	}
}

// GetOrDefault uses a nodeSelector's value if exists, otherwise defaults
func GetOrDefault(key string, nodeSelector map[string]string, defaults []string) []string {
	// Use override if set
	if nodeSelector != nil && len(nodeSelector[key]) > 0 {
		return []string{nodeSelector[key]}
	}
	// Otherwise use defaults
	return defaults
}

func overrideTaints(taints []v1.Taint, pod *v1.Pod) []v1.Taint {
	overrides := []v1.Taint{}
	// Generate taints from pod tolerations
	for _, toleration := range pod.Spec.Tolerations {
		// Only OpEqual is supported
		if toleration.Operator != v1.TolerationOpEqual {
			continue
		}
		// Use effect if defined, otherwise taint all effects
		if toleration.Effect != "" {
			overrides = append(overrides, v1.Taint{Key: toleration.Key, Value: toleration.Value, Effect: toleration.Effect})
		} else {
			overrides = append(overrides,
				v1.Taint{Key: toleration.Key, Value: toleration.Value, Effect: v1.TaintEffectNoSchedule},
				v1.Taint{Key: toleration.Key, Value: toleration.Value, Effect: v1.TaintEffectNoExecute},
			)
		}
	}
	// Add default taints if not overriden by pod above
	for _, taint := range taints {
		if !HasTaint(overrides, taint.Key) {
			overrides = append(overrides, taint)
		}
	}
	return overrides
}
