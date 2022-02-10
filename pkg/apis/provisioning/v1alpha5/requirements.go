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
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/karpenter/pkg/utils/sets"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	stringsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

var (
	ArchitectureAmd64    = "amd64"
	ArchitectureArm64    = "arm64"
	OperatingSystemLinux = "linux"

	// RestrictedLabels are injected by Cloud Providers
	RestrictedLabels = stringsets.NewString(
		// Used internally by provisioning logic
		EmptinessTimestampAnnotationKey,
		v1.LabelHostname,
	)

	// AllowedLabelDomains are domains that may be restricted, but that is allowed because
	// they are not used in a context where they may be passed as argument to kubelet.
	// AllowedLabelDomains are evaluated before RestrictedLabelDomains
	AllowedLabelDomains = stringsets.NewString(
		"kops.k8s.io",
	)

	// These are either prohibited by the kubelet or reserved by karpenter
	// They are evaluated after AllowedLabelDomains
	KarpenterLabelDomain   = "karpenter.sh"
	RestrictedLabelDomains = stringsets.NewString(
		"kubernetes.io",
		"k8s.io",
		KarpenterLabelDomain,
	)
	LabelCapacityType = KarpenterLabelDomain + "/capacity-type"
	// WellKnownLabels supported by karpenter
	WellKnownLabels = stringsets.NewString(
		v1.LabelTopologyZone,
		v1.LabelInstanceTypeStable,
		v1.LabelArchStable,
		v1.LabelOSStable,
		LabelCapacityType,
		v1.LabelHostname, // Used internally for hostname topology spread
	)
	// NormalizedLabels translate aliased concepts into the controller's
	// WellKnownLabels. Pod requirements are translated for compatibility,
	// however, Provisioner labels are still restricted to WellKnownLabels.
	// Additional labels may be injected by cloud providers.
	NormalizedLabels = map[string]string{
		v1.LabelFailureDomainBetaZone:   v1.LabelTopologyZone,
		"beta.kubernetes.io/arch":       v1.LabelArchStable,
		"beta.kubernetes.io/os":         v1.LabelOSStable,
		v1.LabelInstanceType:            v1.LabelInstanceTypeStable,
		v1.LabelFailureDomainBetaRegion: v1.LabelTopologyRegion,
	}
	// IgnoredLables are not considered in scheduling decisions
	// and prevent validation errors when specified
	IgnoredLabels = stringsets.NewString(
		v1.LabelTopologyRegion,
	)
)

// Requirements are an alias type that wrap []v1.NodeSelectorRequirement and
// include an efficient set representation under the hood. Since its underlying
// types are slices and maps, this type should not be used as a pointer.
type Requirements struct {
	// Requirements are layered with Labels and applied to every node.
	Requirements []v1.NodeSelectorRequirement `json:"requirements,omitempty"`
	requirements map[string]sets.Set          `json:"-"`
}

// NewRequirements constructs requiremnets from NodeSelectorRequirements
func NewRequirements(requirements ...v1.NodeSelectorRequirement) Requirements {
	return Requirements{requirements: map[string]sets.Set{}}.Add(requirements...)
}

// NewLabelRequirements constructs requriements from labels
func NewLabelRequirements(labels map[string]string) Requirements {
	requirements := []v1.NodeSelectorRequirement{}
	for key, value := range labels {
		requirements = append(requirements, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	return NewRequirements(requirements...)
}

// NewPodRequirements constructs requirements from a pod
func NewPodRequirements(pod *v1.Pod) Requirements {
	requirements := []v1.NodeSelectorRequirement{}
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

func (r Requirements) WellKnown() Requirements {
	requirements := []v1.NodeSelectorRequirement{}
	for _, requirement := range r.Requirements {
		if WellKnownLabels.Has(requirement.Key) {
			requirements = append(requirements, requirement)
		}
	}
	return NewRequirements(requirements...)
}

// Add function returns a new Requirements object with new requirements inserted.
func (r Requirements) Add(requirements ...v1.NodeSelectorRequirement) Requirements {
	// Deep copy to avoid mutating existing requirements
	r = *r.DeepCopy()
	// This fail-safe measurement can be removed later when we implement test webhook.
	if r.requirements == nil {
		r.requirements = map[string]sets.Set{}
	}
	for _, requirement := range requirements {
		if normalized, ok := NormalizedLabels[requirement.Key]; ok {
			requirement.Key = normalized
		}
		if IgnoredLabels.Has(requirement.Key) {
			continue
		}
		r.Requirements = append(r.Requirements, requirement)
		switch requirement.Operator {
		case v1.NodeSelectorOpIn:
			r.requirements[requirement.Key] = r.Get(requirement.Key).Intersection(sets.NewSet(requirement.Values...))
		case v1.NodeSelectorOpNotIn:
			r.requirements[requirement.Key] = r.Get(requirement.Key).Intersection(sets.NewComplementSet(requirement.Values...))
		}
	}
	return r
}

// Keys returns unique set of the label keys from the requirements
func (r Requirements) Keys() stringsets.String {
	keys := stringsets.NewString()
	for _, requirement := range r.Requirements {
		keys.Insert(requirement.Key)
	}
	return keys
}

// Get returns the sets of values allowed by all included requirements
// following a denylist method. Values are allowed except specified
func (r Requirements) Get(key string) sets.Set {
	if _, ok := r.requirements[key]; !ok {
		return sets.NewComplementSet()
	}
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
	return r.Get(LabelCapacityType).Values()
}

// Validate validates the feasibility of the requirements.
//gocyclo:ignore
func (r Requirements) Validate() (errs error) {
	for _, requirement := range r.Requirements {
		for _, err := range validation.IsQualifiedName(requirement.Key) {
			errs = multierr.Append(errs, fmt.Errorf("key %s is not a qualified name, %s", requirement.Key, err))
		}
		for _, value := range requirement.Values {
			for _, err := range validation.IsValidLabelValue(value) {
				errs = multierr.Append(errs, fmt.Errorf("invalid value %s for key %s, %s", value, requirement.Key, err))
			}
		}
		if !SupportedNodeSelectorOps.Has(string(requirement.Operator)) {
			errs = multierr.Append(errs, fmt.Errorf("operator %s not in %s for key %s", requirement.Operator, SupportedNodeSelectorOps.UnsortedList(), requirement.Key))
		}
		// Excludes cases when DoesNotExists appears together with In, NotIn, Exists
		if requirement.Operator == v1.NodeSelectorOpDoesNotExist && (r.hasRequirement(withKeyAndOperator(requirement.Key, v1.NodeSelectorOpIn)) ||
			r.hasRequirement(withKeyAndOperator(requirement.Key, v1.NodeSelectorOpNotIn)) ||
			r.hasRequirement(withKeyAndOperator(requirement.Key, v1.NodeSelectorOpExists))) {
			errs = multierr.Append(errs, fmt.Errorf("operator %s and %s conflict for key %s", v1.NodeSelectorOpDoesNotExist, v1.NodeSelectorOpDoesNotExist, requirement.Key))
		}
	}
	for key := range r.Keys() {
		if r.Get(key).Len() == 0 {
			errs = multierr.Append(errs, fmt.Errorf("no feasible value for key %s", key))
		}
	}
	return errs
}

// Compatible ensures the provided requirements can be met. It is
// non-commutative (i.e., A.Compatible(B) != B.Compatible(A))
//gocyclo:ignore
func (r Requirements) Compatible(requirements Requirements) (errs error) {
	for _, key := range r.Keys().Union(requirements.Keys()).UnsortedList() {
		// Key must be defined if required
		if values := requirements.Get(key); values.Len() != 0 && !values.IsComplement() && !r.hasRequirement(withKey(key)) {
			errs = multierr.Append(errs, fmt.Errorf("require values for key %s but is not defined", key))
		}
		// Values must overlap
		if values := r.Get(key); values.Intersection(requirements.Get(key)).Len() == 0 {
			errs = multierr.Append(errs, fmt.Errorf("%s not in %s, key %s", values, requirements.Get(key), key))
		}
		// Exists incompatible with DoesNotExist or undefined
		if requirements.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpExists)) {
			if r.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) || !r.hasRequirement(withKey(key)) {
				errs = multierr.Append(errs, fmt.Errorf("%s prohibits %s, key %s", v1.NodeSelectorOpExists, v1.NodeSelectorOpDoesNotExist, key))
			}
		}
		// DoesNotExist requires DoesNotExist or undefined
		if requirements.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) {
			if !(r.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) || !r.hasRequirement(withKey(key))) {
				errs = multierr.Append(errs, fmt.Errorf("%s requires %s, key %s", v1.NodeSelectorOpDoesNotExist, v1.NodeSelectorOpDoesNotExist, key))
			}
		}
		// Repeat for the other direction
		// Exists incompatible with DoesNotExist or undefined
		if r.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpExists)) {
			if requirements.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) || !requirements.hasRequirement(withKey(key)) {
				errs = multierr.Append(errs, fmt.Errorf("%s prohibits %s, key %s", v1.NodeSelectorOpExists, v1.NodeSelectorOpDoesNotExist, key))
			}
		}
		// DoesNotExist requires DoesNotExist or undefined
		if r.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) {
			if !(requirements.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) || !requirements.hasRequirement(withKey(key))) {
				errs = multierr.Append(errs, fmt.Errorf("%s requires %s, key %s", v1.NodeSelectorOpDoesNotExist, v1.NodeSelectorOpDoesNotExist, key))
			}
		}
	}
	return errs
}

func (r Requirements) hasRequirement(f func(v1.NodeSelectorRequirement) bool) bool {
	for _, requirement := range r.Requirements {
		if f(requirement) {
			return true
		}
	}
	return false
}

func withKey(key string) func(v1.NodeSelectorRequirement) bool {
	return func(requirement v1.NodeSelectorRequirement) bool { return requirement.Key == key }
}

func withKeyAndOperator(key string, operator v1.NodeSelectorOperator) func(v1.NodeSelectorRequirement) bool {
	return func(requirement v1.NodeSelectorRequirement) bool {
		return key == requirement.Key && requirement.Operator == operator
	}
}

func (r *Requirements) MarshalJSON() ([]byte, error) {
	if r.Requirements == nil {
		r.Requirements = []v1.NodeSelectorRequirement{}
	}
	return json.Marshal(r.Requirements)
}

func (r *Requirements) UnmarshalJSON(b []byte) error {
	var requirements []v1.NodeSelectorRequirement
	if err := json.Unmarshal(b, &requirements); err != nil {
		return err
	}
	*r = NewRequirements(requirements...)
	return nil
}
