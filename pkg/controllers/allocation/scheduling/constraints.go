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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/scheduling"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
)

// NewConstraints overrides the constraints with pod scheduling constraints
func NewConstraints(ctx context.Context, constraints *v1alpha4.Constraints, pod *v1.Pod) (*v1alpha4.Constraints, error) {
	// Validate that the pod is viable
	if err := multierr.Combine(
		validateAffinity(pod),
		validateTopology(pod),
		scheduling.Taints(constraints.Taints).Tolerates(pod),
	); err != nil {
		return nil, err
	}

	// Copy constraints and apply pod scheduling constraints
	constraints = constraints.DeepCopy()
	if err := constraints.Constrain(ctx, pod); err != nil {
		return nil, err
	}
	if err := generateLabels(constraints, pod); err != nil {
		return nil, err
	}
	if err := generateTaints(constraints, pod); err != nil {
		return nil, err
	}
	return constraints, nil
}

func generateTaints(constraints *v1alpha4.Constraints, pod *v1.Pod) error {
	taints := scheduling.Taints(constraints.Taints)
	for _, toleration := range pod.Spec.Tolerations {
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
			if !taints.Has(taint) {
				taints = append(taints, taint)
			}
		}
	}
	constraints.Taints = taints
	return nil
}

func generateLabels(constraints *v1alpha4.Constraints, pod *v1.Pod) error {
	labels := map[string]string{}
	// Default to constraint labels
	for key, value := range constraints.Labels {
		labels[key] = value
	}
	// Override with pod labels
	nodeAffinity := scheduling.NodeAffinityFor(pod)
	for _, key := range nodeAffinity.GetLabels() {
		if _, ok := v1alpha4.WellKnownLabels[key]; !ok {
			var labelConstraints []string
			if value, ok := constraints.Labels[key]; ok {
				labelConstraints = append(labelConstraints, value)
			}
			values := nodeAffinity.GetLabelValues(key, labelConstraints)
			if len(values) == 0 {
				return fmt.Errorf("label %s is too constrained", key)
			}
			labels[key] = values[0]
		}
	}
	constraints.Labels = labels
	return nil
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
