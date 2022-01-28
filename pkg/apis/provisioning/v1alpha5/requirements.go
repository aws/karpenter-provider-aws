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

	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/sets"
	v1 "k8s.io/api/core/v1"
	stringsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
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
		v1.LabelFailureDomainBetaZone: v1.LabelTopologyZone,
		"beta.kubernetes.io/arch":     v1.LabelArchStable,
		"beta.kubernetes.io/os":       v1.LabelOSStable,
		v1.LabelInstanceType:          v1.LabelInstanceTypeStable,
	}
	// IgnoredLables are not considered in scheduling decisions
	// and prevent validation errors when specified
	IgnoredLabels = sets.NewString(
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
	if r.requirements == nil {
		r.requirements = map[string]sets.Set{}
	}
	for _, requirement := range requirements {
		if normalized, ok := NormalizedLabels[requirement.Key]; ok {
			requirement.Key = normalized
		}
		r.Requirements = append(r.Requirements, requirement)
		switch requirement.Operator {
		case v1.NodeSelectorOpIn:
			r.requirements[requirement.Key] = r.Values(requirement.Key).Intersection(sets.NewSet(requirement.Values...))
		case v1.NodeSelectorOpNotIn:
			r.requirements[requirement.Key] = r.Values(requirement.Key).Intersection(sets.NewComplementSet(requirement.Values...))
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

// Values returns the sets of values allowed by all included requirements
// following a denylist method. Values are allowed except specified
func (r Requirements) Values(key string) sets.Set {
	if _, ok := r.requirements[key]; !ok {
		return sets.NewComplementSet()
	}
	return r.requirements[key]
}

func (r Requirements) Zones() stringsets.String {
	return r.Values(v1.LabelTopologyZone).Values()
}

func (r Requirements) InstanceTypes() stringsets.String {
	return r.Values(v1.LabelInstanceTypeStable).Values()
}

func (r Requirements) Architectures() stringsets.String {
	return r.Values(v1.LabelArchStable).Values()
}

func (r Requirements) OperatingSystems() stringsets.String {
	return r.Values(v1.LabelOSStable).Values()
}

func (r Requirements) CapacityTypes() stringsets.String {
	return r.Values(LabelCapacityType).Values()
}

// Validate validates the feasibility of the requirements.
func (r Requirements) Validate() (errs *apis.FieldError) {
	for i, requirement := range r.Requirements {
		for _, err := range validation.IsQualifiedName(requirement.Key) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s, %s", requirement.Key, err), "key"))
		}
		for _, value := range requirement.Values {
			for _, err := range validation.IsValidLabelValue(value) {
				errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s, %s", value, err), "values", i))
			}
		}
		if !functional.ContainsString(SupportedNodeSelectorOps, string(requirement.Operator)) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %s", requirement.Operator, SupportedNodeSelectorOps), "operator"))
		}
		if requirement.Operator == v1.NodeSelectorOpDoesNotExist && !r.Values(requirement.Key).Complement {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("operator %s and %s conflict", v1.NodeSelectorOpDoesNotExist, v1.NodeSelectorOpDoesNotExist), "operator"))
		}
	}
	for key := range r.Keys() {
		values := r.Values(key)
		if values.Len() == 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("no feasible value for requirement, %s", key), "values"))
		}
	}
	return errs
}

// Compatible ensures the provided requirements can be met. It is
// non-commutative (i.e., A.Compatible(B) != B.Compatible(A))
func (r Requirements) Compatible(requirements Requirements) (errs *apis.FieldError) {
	for i, key := range r.Keys().Union(requirements.Keys()).UnsortedList() {
		// Key must be defined if required
		if values := requirements.Values(key); values.Len() != 0 && !values.Complement && !r.hasRequirement(withKey(key)) {
			errs = errs.Also(apis.ErrInvalidValue("is not defined", "key")).ViaFieldIndex("requirements", i)
		}
		// Values must overlap
		if values := r.Values(key); values.Intersection(requirements.Values(key)).Len() == 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %s", values, requirements.Values(key)), "values")).ViaFieldIndex("requirements", i)
		}
		// Exists incompatible with DoesNotExist or undefined
		if requirements.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpExists)) {
			if r.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) || !r.hasRequirement(withKey(key)) {
				errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s prohibits %s", v1.NodeSelectorOpExists, v1.NodeSelectorOpDoesNotExist), "operator")).ViaFieldIndex("requirements", i)
			}
		}
		// DoesNotExist requires DoesNotExist or undefined
		if requirements.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) {
			if !(r.hasRequirement(withKeyAndOperator(key, v1.NodeSelectorOpDoesNotExist)) || !r.hasRequirement(withKey(key))) {
				errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s requires %s", v1.NodeSelectorOpDoesNotExist, v1.NodeSelectorOpDoesNotExist), "operator")).ViaFieldIndex("requirements", i)
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
	return json.Marshal(r.Requirements)
}

func (r *Requirements) UnmarshalJSON(b []byte) error {
	var requirements []v1.NodeSelectorRequirement
	json.Unmarshal(b, &requirements)
	*r = NewRequirements(requirements...)
	return nil
}
