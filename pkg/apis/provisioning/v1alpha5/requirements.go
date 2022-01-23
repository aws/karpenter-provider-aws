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

type Requirements struct {
	// Requirements are layered with Labels and applied to every node.
	Requirements []v1.NodeSelectorRequirement `json:"requirements,omitempty"`
	// Ground truth for record keeping
	Records map[string][]v1.NodeSelectorRequirement `json:"-"`
	Allows  map[string]sets.Set                     `json:"-"`
}

func NewRequirements(requirements ...v1.NodeSelectorRequirement) *Requirements {
	result := Requirements{
		Records: map[string][]v1.NodeSelectorRequirement{},
		Allows:  map[string]sets.Set{},
	}
	return result.Add(requirements...)
}

// Add function returns a new Requirements object with new requirements inserted.
// It is critical to keep the original Requirements object immutable to keep the
// requirement hashing behavior consistent.
func (r *Requirements) Add(requirements ...v1.NodeSelectorRequirement) *Requirements {
	if r == nil {
		return NewRequirements(requirements...)
	}
	rd := r.DeepCopy()
	for _, requirement := range requirements {
		if newKey, ok := NormalizedLabels[requirement.Key]; ok {
			requirement.Key = newKey
		}
		rd.Records[requirement.Key] = append(rd.Records[requirement.Key], requirement)
		switch requirement.Operator {
		case v1.NodeSelectorOpIn:
			rd.Allows[requirement.Key] = rd.Allow(requirement.Key).Intersection(sets.NewSet(requirement.Values...))
		case v1.NodeSelectorOpNotIn:
			rd.Allows[requirement.Key] = rd.Allow(requirement.Key).Intersection(sets.NewComplementSet(requirement.Values...))
		}
	}
	return rd
}

func (r *Requirements) MarshalJSON() ([]byte, error) {
	var result []v1.NodeSelectorRequirement
	for _, requirements := range r.Records {
		result = append(result, requirements...)
	}
	return json.Marshal(result)
}

func (r *Requirements) UnmarshalJSON(b []byte) error {
	var requirements []v1.NodeSelectorRequirement
	json.Unmarshal(b, &requirements)
	result := NewRequirements(requirements...)
	r.Records = result.Records
	r.Allows = result.Allows
	return nil
}

// Allow returns the sets of values allowed by all included requirements
// Allow follows a denylist method. Values are allowed except specified
func (r *Requirements) Allow(key string) sets.Set {
	if r == nil {
		return sets.NewComplementSet()
	}
	if _, ok := r.Allows[key]; !ok {
		return sets.NewComplementSet()
		//r.Allows[key] = sets.NewComplementSet()
	}
	return r.Allows[key]
}

// Validate validates the feasibility of the requirements.
func (r *Requirements) Validate() (errs *apis.FieldError) {
	if r == nil {
		return errs
	}
	for _, key := range r.Keys() {
		for _, err := range validation.IsQualifiedName(key) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s, %s", key, err), "key"))
		}
		values := r.Allow(key)
		if values.Len() == 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("no feasible value for requirement, %s", key), "values"))
		}
		i := 0
		for value := range values.Members {
			for _, err := range validation.IsValidLabelValue(value) {
				errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s, %s", value, err), "values", i))
			}
			i++
		}
		for _, requirement := range r.Records[key] {
			if !functional.ContainsString(SupportedNodeSelectorOps, string(requirement.Operator)) {
				errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %s", requirement.Operator, SupportedNodeSelectorOps), "operator"))
			}
			// case when DoesNotExists and In operator appear together
			if requirement.Operator == v1.NodeSelectorOpDoesNotExist && !r.Allow(key).IsComplement {
				errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("Operator In and DoesNotExists conflicts, %s", key), "values"))
			}
		}
	}
	return errs
}

// FromLabels constructs requriements from labels
func LabelRequirements(labels map[string]string) []v1.NodeSelectorRequirement {
	requirements := []v1.NodeSelectorRequirement{}
	for key, value := range labels {
		requirements = append(requirements, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	return requirements
}

func PodRequirements(pod *v1.Pod) []v1.NodeSelectorRequirement {
	requirements := []v1.NodeSelectorRequirement{}
	for key, value := range pod.Spec.NodeSelector {
		requirements = append(requirements, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	if pod.Spec.Affinity == nil || pod.Spec.Affinity.NodeAffinity == nil {
		return requirements
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
	return requirements
}

// Compatible returns errors with detailed messages when requirements are not compatible
// Compatible is non-commutative (i.e., A.Compatible(B) != B.Compatible(A))
func (r *Requirements) Compatible(requirements *Requirements) (errs *apis.FieldError) {
	if r == nil {
		errs = errs.Also(apis.ErrInvalidValue("nil requirements", "values"))
		return errs
	}
	allKeys := stringsets.NewString(r.Keys()...).Union(stringsets.NewString(requirements.Keys()...))
	for key := range allKeys {
		if err := r.isKeyDefined(key, requirements); err != nil {
			errs = errs.Also(err)
		}
		if err := r.hasCommons(key, requirements); err != nil {
			errs = errs.Also(err)
		}
		if err := r.satisfyExistOperators(key, requirements); err != nil {
			errs = errs.Also(err)
		}
	}
	return errs
}

// isKeyDefined returns errors when the given requirements requires a key that is not defined by this requirements
// This is a non-commutative function designed to catch the following cases:
// * Pod Spec has a label selector but provisioner doesn't have the label
// * One pod is not compatible with a schedule (group of pods) due to extra requirements
func (r *Requirements) isKeyDefined(key string, requirements *Requirements) (errs *apis.FieldError) {
	if _, ok := requirements.Records[key]; ok {
		_, exist := r.Records[key]
		if !requirements.Allow(key).IsComplement && requirements.Allow(key).Len() != 0 && !exist {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("key %s, need %s but not defined", key, requirements.Allow(key)), "values"))
		}
	}
	return errs
}

// hasCommons returns errors when there is no overlap among the requirements' allowed values for a provided key.
func (r *Requirements) hasCommons(key string, requirements *Requirements) (errs *apis.FieldError) {
	if r.Allow(key).Intersection(requirements.Allow(key)).Len() == 0 {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("no common values for key %s, %s not in %s", key, r.Allow(key), requirements.Allow(key)), "values"))
	}
	return errs
}

// satisfyExistOperators returns errors with detailed messages when Exist or DoesNotExist operators requirements are violated.
func (r *Requirements) satisfyExistOperators(key string, requirements *Requirements) (errs *apis.FieldError) {
	for r1, r2 := range map[*Requirements]*Requirements{
		r:            requirements,
		requirements: r,
	} {
		if records, ok := r1.Records[key]; ok {
			for _, requirement := range records {
				switch requirement.Operator {
				case v1.NodeSelectorOpExists:
					if _, exist := r2.Records[key]; !exist {
						errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("Exist operator violation, %s", key), "values"))
					}
				case v1.NodeSelectorOpDoesNotExist:
					if _, exist := r2.Records[key]; exist {
						errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("DoesNotExist operator violation, %s", key), "values"))
					}
				}
			}
		}
	}
	return errs
}

func (r *Requirements) Zones() sets.Set {
	return r.Allow(v1.LabelTopologyZone)
}

func (r *Requirements) InstanceTypes() sets.Set {
	return r.Allow(v1.LabelInstanceTypeStable)
}

func (r *Requirements) Architectures() sets.Set {
	return r.Allow(v1.LabelArchStable)
}

func (r *Requirements) OperatingSystems() sets.Set {
	return r.Allow(v1.LabelOSStable)
}

func (r *Requirements) CapacityTypes() sets.Set {
	return r.Allow(LabelCapacityType)
}

func (r *Requirements) WellKnown() *Requirements {
	requirements := []v1.NodeSelectorRequirement{}
	for _, key := range r.Keys() {
		if WellKnownLabels.Has(key) {
			requirements = append(requirements, r.Records[key]...)
		}
	}
	return NewRequirements(requirements...)
}

// Keys returns unique set of the label keys from the requirements
func (r *Requirements) Keys() []string {
	if r == nil {
		return []string{}
	}
	keys := make([]string, 0, len(r.Records))
	for k := range r.Records {
		keys = append(keys, k)
	}
	return keys
}
