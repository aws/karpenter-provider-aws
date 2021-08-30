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
		Labels:          overrideLabels(constraints, pod),
		Taints:          overrideTaints(constraints, pod),
		Zones:           overrideZones(constraints, pod),
		InstanceTypes:   overrideInstanceTypes(constraints, pod),
		Architecture:    overrideArchitecture(constraints, pod),
		OperatingSystem: overrideOperatingSystem(constraints, pod),
	}
}

func overrideLabels(c *v1alpha3.Constraints, pod *v1.Pod) map[string]string {
	return functional.UnionStringMaps(c.Labels, pod.Spec.NodeSelector)
}

func overrideTaints(c *v1alpha3.Constraints, pod *v1.Pod) []v1.Taint {
	taints := []v1.Taint{}
	// Generate taints from pod tolerations
	for _, toleration := range pod.Spec.Tolerations {
		// Only OpEqual is supported
		if toleration.Operator != v1.TolerationOpEqual {
			continue
		}
		// Use effect if defined, otherwise taint all effects
		if toleration.Effect != "" {
			taints = append(taints, v1.Taint{Key: toleration.Key, Value: toleration.Value, Effect: toleration.Effect})
		} else {
			taints = append(taints,
				v1.Taint{Key: toleration.Key, Value: toleration.Value, Effect: v1.TaintEffectNoSchedule},
				v1.Taint{Key: toleration.Key, Value: toleration.Value, Effect: v1.TaintEffectNoExecute},
			)
		}
	}
	// Add default taints if not overriden by pod above
	for _, taint := range c.Taints {
		if !HasTaint(taints, taint.Key) {
			taints = append(taints, taint)
		}
	}
	return taints
}

func overrideZones(c *v1alpha3.Constraints, pod *v1.Pod) []string {
	// Pod may override zone
	if zone, ok := pod.Spec.NodeSelector[v1.LabelTopologyZone]; ok {
		return []string{zone}
	}
	// Default to provisioner constraints
	if len(c.Zones) != 0 {
		return c.Zones
	}
	// Otherwise unconstrained
	return nil
}

func overrideInstanceTypes(c *v1alpha3.Constraints, pod *v1.Pod) []string {
	// Pod may override instance type
	if instanceType, ok := pod.Spec.NodeSelector[v1.LabelInstanceTypeStable]; ok {
		return []string{instanceType}
	}
	// Default to provisioner constraints
	if len(c.InstanceTypes) != 0 {
		return c.InstanceTypes
	}
	// Otherwise unconstrained
	return nil
}

func overrideArchitecture(c *v1alpha3.Constraints, pod *v1.Pod) *string {
	// Pod may override arch
	if architecture, ok := pod.Spec.NodeSelector[v1.LabelArchStable]; ok {
		return &architecture
	}
	// Use constraints if defined
	if c.Architecture != nil {
		return c.Architecture
	}
	// Default to amd64
	return &v1alpha3.ArchitectureAmd64
}

func overrideOperatingSystem(c *v1alpha3.Constraints, pod *v1.Pod) *string {
	// Pod may override os
	if operatingSystem, ok := pod.Spec.NodeSelector[v1.LabelOSStable]; ok {
		return &operatingSystem
	}
	// Use constraints if defined
	if c.OperatingSystem != nil {
		return c.OperatingSystem
	}
	// Default to linux
	return &v1alpha3.OperatingSystemLinux
}
