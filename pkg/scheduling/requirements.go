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
	"fmt"
	"sort"
	"strings"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	stringsets "k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/sets"
)

// Requirements are an efficient set representation under the hood. Since its underlying
// types are slices and maps, this type should not be used as a pointer.
type Requirements map[string]sets.Set

func NewRequirements(requirements ...Requirements) Requirements {
	r := Requirements{}
	for _, requirement := range requirements {
		r.Add(requirement)
	}
	return r
}

// NewRequirements constructs requirements from NodeSelectorRequirements
func NewNodeSelectorRequirements(requirements ...v1.NodeSelectorRequirement) Requirements {
	r := NewRequirements()
	for _, requirement := range requirements {
		if normalized, ok := v1alpha5.NormalizedLabels[requirement.Key]; ok {
			requirement.Key = normalized
		}
		if v1alpha5.IgnoredLabels.Has(requirement.Key) {
			continue
		}
		var values sets.Set
		switch requirement.Operator {
		case v1.NodeSelectorOpIn:
			values = sets.NewSet(requirement.Values...)
		case v1.NodeSelectorOpNotIn:
			values = sets.NewComplementSet(requirement.Values...)
		case v1.NodeSelectorOpExists:
			values = sets.NewComplementSet()
		case v1.NodeSelectorOpDoesNotExist:
			values = sets.NewSet()
		}
		r.Add(map[string]sets.Set{requirement.Key: values})
	}
	return r
}

// NewLabelRequirements constructs requirements from labels
func NewLabelRequirements(labels map[string]string) Requirements {
	requirements := NewRequirements()
	for key, value := range labels {
		requirements.Add(Requirements{key: sets.NewSet(value)})
	}
	return requirements
}

// NewPodRequirements constructs requirements from a pod
func NewPodRequirements(pod *v1.Pod) Requirements {
	var requirements []v1.NodeSelectorRequirement
	for key, value := range pod.Spec.NodeSelector {
		requirements = append(requirements, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	if pod.Spec.Affinity == nil || pod.Spec.Affinity.NodeAffinity == nil {
		return NewNodeSelectorRequirements(requirements...)
	}
	// The legal operators for pod affinity and anti-affinity are In, NotIn, Exists, DoesNotExist.
	// Select heaviest preference and treat as a requirement. An outer loop will iteratively unconstrain them if unsatisfiable.
	if preferred := pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution; len(preferred) > 0 {
		sort.Slice(preferred, func(i int, j int) bool { return preferred[i].Weight > preferred[j].Weight })
		requirements = append(requirements, preferred[0].Preference.MatchExpressions...)
	}
	// Select first requirement. An outer loop will iteratively remove OR requirements if unsatisfiable
	if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
		len(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) > 0 {
		requirements = append(requirements, pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions...)
	}
	return NewNodeSelectorRequirements(requirements...)
}

// Add requirements to provided requirements. Mutates existing requirements
func (r Requirements) Add(requirements Requirements) {
	for key, values := range requirements {
		if existing, ok := r[key]; ok {
			values = values.Intersection(existing)
		}
		r[key] = values
	}
}

// Keys returns unique set of the label keys from the requirements
func (r Requirements) Keys() stringsets.String {
	keys := stringsets.NewString()
	for key := range r {
		keys.Insert(key)
	}
	return keys
}

func (r Requirements) Has(key string) bool {
	_, ok := r[key]
	return ok
}

func (r Requirements) Get(key string) sets.Set {
	if _, ok := r[key]; ok {
		return r[key]
	}
	return sets.NewComplementSet()
}

// Compatible ensures the provided requirements can be met.
func (r Requirements) Compatible(requirements Requirements) (errs error) {
	// Custom Labels must intersect, but if not defined are denied.
	for key := range requirements.Keys().Difference(v1alpha5.WellKnownLabels) {
		if operator := requirements.Get(key).Type(); r.Has(key) || operator == v1.NodeSelectorOpNotIn || operator == v1.NodeSelectorOpDoesNotExist {
			continue
		}
		errs = multierr.Append(errs, fmt.Errorf("%s not in %s, key %s", requirements.Get(key), r.Get(key), key))
	}
	// Well Known Labels must intersect, but if not defined, are allowed.
	return multierr.Append(errs, r.Intersects(requirements))
}

// Intersects returns errors if the requirements don't have overlapping values, undefined keys are allowed
func (r Requirements) Intersects(requirements Requirements) (errs error) {
	for key := range r.Keys().Intersection(requirements.Keys()) {
		existing := r.Get(key)
		incoming := requirements.Get(key)
		// There must be some value, except
		if existing.Intersection(incoming).Len() == 0 {
			// where the incoming requirement has operator { NotIn, DoesNotExist }
			if operator := incoming.Type(); operator == v1.NodeSelectorOpNotIn || operator == v1.NodeSelectorOpDoesNotExist {
				// and the existing requirement has operator { NotIn, DoesNotExist }
				if operator := existing.Type(); operator == v1.NodeSelectorOpNotIn || operator == v1.NodeSelectorOpDoesNotExist {
					continue
				}
			}
			errs = multierr.Append(errs, fmt.Errorf("%s not in %s, key %s", incoming, existing, key))
		}
	}
	return errs
}

func (r Requirements) Labels() map[string]string {
	labels := map[string]string{}
	for key, values := range r {
		if !v1alpha5.IsRestrictedNodeLabel(key) {
			switch values.Type() {
			case v1.NodeSelectorOpIn, v1.NodeSelectorOpExists:
				labels[key] = r.Get(key).Any()
			}
		}
	}
	return labels
}

func (r Requirements) String() string {
	var sb strings.Builder
	for key, req := range r {
		if v1alpha5.RestrictedLabels.Has(key) {
			continue
		}
		values := req.Values().List()
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		if len(values) > 5 {
			values[5] = fmt.Sprintf("and %d others", len(values)-5)
			values = values[0:6]
		}
		fmt.Fprintf(&sb, "%s %s %v", key, req.Type(), values)
	}
	return sb.String()
}
