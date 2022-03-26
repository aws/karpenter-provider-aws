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

	"github.com/aws/karpenter/pkg/utils/resources"

	"go.uber.org/multierr"
	stringsets "k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/sets"

	v1 "k8s.io/api/core/v1"
)

// +k8s:deepcopy-gen=true
type Requirements struct {
	requirements map[string]sets.Set
}

// NewRequirements constructs requirements from NodeSelectorRequirements
func NewRequirements(requirements ...v1.NodeSelectorRequirement) Requirements {
	r := Requirements{requirements: map[string]sets.Set{}}
	r.AddNodeSelectors(requirements...)
	return r
}

// NewPodRequirements constructs requirements from a pod
func NewPodRequirements(pod *v1.Pod) Requirements {
	var requirements []v1.NodeSelectorRequirement
	for key, value := range pod.Spec.NodeSelector {
		requirements = append(requirements, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	if pod.Spec.Affinity == nil || pod.Spec.Affinity.NodeAffinity == nil {
		return NewRequirements(requirements...)
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
	return NewRequirements(requirements...)
}

func InstanceTypeRequirements(instanceTypes []cloudprovider.InstanceType) Requirements {
	supported := map[string]sets.Set{
		v1.LabelInstanceTypeStable: sets.NewSet(),
		v1.LabelTopologyZone:       sets.NewSet(),
		v1.LabelArchStable:         sets.NewSet(),
		v1.LabelOSStable:           sets.NewSet(),
		v1alpha5.LabelCapacityType: sets.NewSet(),
	}
	for _, instanceType := range instanceTypes {
		for _, offering := range instanceType.Offerings() {
			supported[v1.LabelTopologyZone].Insert(offering.Zone)
			supported[v1alpha5.LabelCapacityType].Insert(offering.CapacityType)
		}
		supported[v1.LabelInstanceTypeStable].Insert(instanceType.Name())
		supported[v1.LabelArchStable].Insert(instanceType.Architecture())
		supported[v1.LabelOSStable].Insert(instanceType.OperatingSystems().List()...)
	}
	requirements := NewRequirements()
	for key, values := range supported {
		requirements.AddNodeSelectors(v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: values.Values().UnsortedList()})
	}
	return requirements
}

func Compatible(it cloudprovider.InstanceType, requirements Requirements) bool {
	if !requirements.Get(v1.LabelInstanceTypeStable).Has(it.Name()) {
		return false
	}
	if !requirements.Get(v1.LabelArchStable).Has(it.Architecture()) {
		return false
	}
	if !requirements.Get(v1.LabelOSStable).HasAny(it.OperatingSystems().List()...) {
		return false
	}
	// acceptable if we have any offering that is valid
	for _, offering := range it.Offerings() {
		if requirements.Get(v1.LabelTopologyZone).Has(offering.Zone) && requirements.Get(v1alpha5.LabelCapacityType).Has(offering.CapacityType) {
			return true
		}
	}
	return false
}

func FilterInstanceTypes(instanceTypes []cloudprovider.InstanceType, requirements Requirements, requests v1.ResourceList) []cloudprovider.InstanceType {
	var result []cloudprovider.InstanceType
	for _, instanceType := range instanceTypes {
		if !Compatible(instanceType, requirements) {
			continue
		}
		if !resources.Fits(resources.Merge(requests, instanceType.Overhead()), instanceType.Resources()) {
			continue
		}
		result = append(result, instanceType)
	}
	return result
}

// NewLabelRequirements constructs requirements from labels
func NewLabelRequirements(labels map[string]string) Requirements {
	requirements := []v1.NodeSelectorRequirement{}
	for key, value := range labels {
		requirements = append(requirements, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	return NewRequirements(requirements...)
}

func (r *Requirements) Add(requirements Requirements) {
	for key, requirement := range requirements.requirements {
		existing, ok := r.requirements[key]
		if !ok {
			r.requirements[key] = requirement
		} else {
			r.requirements[key] = existing.Intersection(requirement)
		}
	}
}

//gocyclo:ignore
func (r *Requirements) AddNodeSelectors(requirements ...v1.NodeSelectorRequirement) {
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
		if existing, ok := r.requirements[requirement.Key]; ok {
			newValuesLen := values.Len()
			values = values.Intersection(existing)
			if values.Len() == 0 && (existing.Len() > 0 || newValuesLen > 0) {
				values.MarkInvalid()
			}
		}
		r.requirements[requirement.Key] = values
	}
}

// Keys returns unique set of the label keys from the requirements
func (r Requirements) Keys() stringsets.String {
	keys := stringsets.NewString()
	for k := range r.requirements {
		keys.Insert(k)
	}
	return keys
}

func (r Requirements) Has(key string) bool {
	_, ok := r.requirements[key]
	return ok
}

func (r Requirements) Get(key string) sets.Set {
	return r.requirements[key]
}

func (r Requirements) Zones() stringsets.String {
	return r.Get(v1.LabelTopologyZone).Values()
}

func (r Requirements) InstanceTypes() stringsets.String {
	return r.Get(v1.LabelInstanceTypeStable).Values()
}

func (r Requirements) Architectures() stringsets.String {
	return r.Get(v1.LabelArchStable).Values()
}

func (r Requirements) OperatingSystems() stringsets.String {
	return r.Get(v1.LabelOSStable).Values()
}

func (r Requirements) CapacityTypes() stringsets.String {
	return r.Get(v1alpha5.LabelCapacityType).Values()
}

// Compatible ensures the provided requirements can be met.
func (r Requirements) Compatible(requirements Requirements) (errs error) {
	for key, requirement := range requirements.requirements {
		intersection := requirement.Intersection(r.Get(key))
		// There must be some value, except in these cases
		if intersection.Len() == 0 {
			// Where incoming requirement has operator { NotIn, DoesNotExist }
			if requirement.Type() == v1.NodeSelectorOpNotIn || requirement.Type() == v1.NodeSelectorOpDoesNotExist {
				// And existing requirement has operator { NotIn, DoesNotExist }
				if r.Get(key).Type() == v1.NodeSelectorOpNotIn || r.Get(key).Type() == v1.NodeSelectorOpDoesNotExist {
					continue
				}
			}
			errs = multierr.Append(errs, fmt.Errorf("%s not in %s, key %s", requirement, r.Get(key), key))
		}
	}
	return errs
}

func (r Requirements) ToNodeSelector() []v1.NodeSelectorRequirement {
	var nodeSelectors []v1.NodeSelectorRequirement
	for k, v := range r.requirements {
		req := v1.NodeSelectorRequirement{
			Key:      k,
			Operator: v.Type(),
		}
		if v.IsComplement() {
			for s := range v.ComplementValues() {
				req.Values = append(req.Values, s)
			}
		} else {
			for s := range v.Values() {
				req.Values = append(req.Values, s)
			}
		}

		nodeSelectors = append(nodeSelectors, req)
	}
	return nodeSelectors
}

func (r Requirements) Validate() error {
	for k, v := range r.requirements {
		if v.IsInvalid() {
			return fmt.Errorf("unsatisfiable %s", k)
		}
	}
	return nil
}
