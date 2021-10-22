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
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
)

// NewConstraints overrides the constraints with pod scheduling constraints
func NewConstraints(ctx context.Context, constraints *v1alpha5.Constraints, pod *v1.Pod) (*v1alpha5.Constraints, error) {
	// Validate that the pod is viable
	if err := multierr.Combine(
		validateAffinity(pod),
		validateTopology(pod),
		Taints(constraints.Taints).Tolerates(pod),
	); err != nil {
		return nil, err
	}
	requirements := constraints.Requirements.With(pod)
	if err := requirements.Validate(); err != nil {
		return nil, err
	}
	return &v1alpha5.Constraints{
		Requirements: requirements,
		Labels:       generateLabels(requirements),
		Taints:       generateTaints(constraints.Taints, pod.Spec.Tolerations),
		Provider:     constraints.Provider,
	}, nil
}

func generateTaints(taints []v1.Taint, tolerations []v1.Toleration) []v1.Taint {
	for _, toleration := range tolerations {
		// Only OpEqual is supported. OpExists does not make sense for
		// provisioning -- in theory we could create a taint on the node with a
		// random string, but it's unclear use case this would accomplish.
		if toleration.Operator != v1.TolerationOpEqual {
			continue
		}
		var generated []v1.Taint
		// Use effect if defined, otherwise taint all effects
		if toleration.Effect != "" {
			generated = []v1.Taint{{Key: toleration.Key, Value: toleration.Value, Effect: toleration.Effect}}
		} else {
			generated = []v1.Taint{
				{Key: toleration.Key, Value: toleration.Value, Effect: v1.TaintEffectNoSchedule},
				{Key: toleration.Key, Value: toleration.Value, Effect: v1.TaintEffectNoExecute},
			}
		}
		// Only add taints that do not already exist on constraints
		for _, taint := range generated {
			if !Taints(taints).Has(taint) {
				taints = append(taints, taint)
			}
		}
	}
	return taints
}

func generateLabels(requirements v1alpha5.Requirements) map[string]string {
	labels := map[string]string{}
	for _, label := range requirements.GetLabels() {
		// Only include labels that aren't well known. Well known labels will be populated by the kubelet
		if _, ok := v1alpha5.WellKnownLabels[label]; !ok {
			labels[label] = requirements.GetLabelValues(label)[0]
		}
	}
	return labels
}

func validateTopology(pod *v1.Pod) (errs error) {
	for _, constraint := range pod.Spec.TopologySpreadConstraints {
		if supported := []string{v1.LabelHostname, v1.LabelTopologyZone}; !functional.ContainsString(supported, constraint.TopologyKey) {
			errs = multierr.Append(errs, fmt.Errorf("unsupported topology key, %s not in %s", constraint.TopologyKey, supported))
		}
	}
	return errs
}

func validateAffinity(pod *v1.Pod) (errs error) {
	if pod.Spec.Affinity == nil {
		return nil
	}
	if pod.Spec.Affinity.PodAffinity != nil {
		errs = multierr.Append(errs, fmt.Errorf("pod affinity is not supported"))
	}
	if pod.Spec.Affinity.PodAntiAffinity != nil {
		errs = multierr.Append(errs, fmt.Errorf("pod anti-affinity is not supported"))
	}
	if pod.Spec.Affinity.NodeAffinity != nil {
		for _, term := range pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			errs = multierr.Append(errs, validateNodeSelectorTerm(term.Preference))
		}
		if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				errs = multierr.Append(errs, validateNodeSelectorTerm(term))
			}
		}
	}
	return errs
}

func validateNodeSelectorTerm(term v1.NodeSelectorTerm) (errs error) {
	if term.MatchFields != nil {
		errs = multierr.Append(errs, fmt.Errorf("matchFields is not supported"))
	}
	if term.MatchExpressions != nil {
		for _, requirement := range term.MatchExpressions {
			if !functional.ContainsString([]string{string(v1.NodeSelectorOpIn), string(v1.NodeSelectorOpNotIn)}, string(requirement.Operator)) {
				errs = multierr.Append(errs, fmt.Errorf("unsupported operator, %s", requirement.Operator))
			}
		}
	}
	return errs
}
