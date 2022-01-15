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
		ProvisionerNameLabelKey,
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
	// Ground truth record of requirements
	Requirements map[string][]v1.NodeSelectorRequirement `json:"requirements,omitempty"`
	Allows       map[string]*sets.Set                    `json:"allows,omitempty"`
}

func NewRequirements(requirements ...v1.NodeSelectorRequirement) *Requirements {
	r := Requirements{
		Requirements: map[string][]v1.NodeSelectorRequirement{},
		Allows:       map[string]*sets.Set{},
	}
	r.insert(requirements...)
	r.Normalize()
	return &r
}

// Allow returns the sets of values allowed by all included requirements
func (r *Requirements) Allow(key string) *sets.Set {
	if _, ok := r.Allows[key]; !ok {
		// init set to contains every possible values
		r.Allows[key] = sets.NewSet(true)
	}
	return r.Allows[key]
}

// insert inserts v1.NodeSelectorRequirement type requirements
func (r *Requirements) insert(requirements ...v1.NodeSelectorRequirement) {
	for _, requirement := range requirements {
		r.Requirements[requirement.Key] = append(r.Requirements[requirement.Key], requirement)
		switch requirement.Operator {
		case v1.NodeSelectorOpIn:
			r.Allows[requirement.Key] = r.Allow(requirement.Key).Intersection(sets.NewSet(false, requirement.Values...))
		case v1.NodeSelectorOpNotIn:
			r.Allows[requirement.Key] = r.Allow(requirement.Key).Intersection(sets.NewSet(true, requirement.Values...))
		}
	}
}

// Merge combines two requirement sets
func (r *Requirements) Merge(requirements *Requirements) *Requirements {
	if r == nil {
		r = NewRequirements()
	}
	combined := []v1.NodeSelectorRequirement{}
	allKeys := stringsets.NewString(r.Keys()...).Union(stringsets.NewString(requirements.Keys()...))
	for key := range allKeys {
		if req, ok := r.Requirements[key]; ok {
			combined = append(combined, req...)
		}
		if req, ok := requirements.Requirements[key]; ok {
			combined = append(combined, req...)
		}
	}
	return NewRequirements(combined...)
}

// Normalize the requirements to use WellKnownLabels
func (r *Requirements) Normalize() {
	allows := map[string]*sets.Set{}
	requirements := map[string][]v1.NodeSelectorRequirement{}
	for _, key := range r.Keys() {
		if label, ok := NormalizedLabels[key]; ok {
			if set, has := r.Allows[key]; has {
				allows[label] = r.Allow(label).Intersection(set)
			}
			requirements[label] = r.Requirements[key]
		} else {
			if _, has := r.Allows[key]; has {
				allows[key] = r.Allow(key)
			}
			requirements[key] = r.Requirements[key]
		}
	}
	r.Allows = allows
	r.Requirements = requirements
}

// Validate validates the feasibility of the requirements.
func (r *Requirements) Validate() (errs *apis.FieldError) {
	if r == nil {
		r = NewRequirements()
	}
	for _, key := range r.Keys() {

		// Disable checking for WellKnownLabels because label is represented as In opeartor requirement
		/*
			if !WellKnownLabels.Has(key) {
				errs = errs.Also(apis.ErrInvalidKeyName(fmt.Sprintf("%s not in %v", key, WellKnownLabels.UnsortedList()), "key"))
			}
		*/
		for _, err := range validation.IsQualifiedName(key) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s, %s", key, err), "key"))
		}
		values := r.Allow(key)
		if values.Len() == 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("no feasible value for requirement, %s", key), "values"))
		}
		i := 0
		for value := range values.RawValues() {
			for _, err := range validation.IsValidLabelValue(value) {
				errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s, %s", value, err), "values", i))
			}
			i++
		}
		for _, requirement := range r.Requirements[key] {
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
func LabelRequirements(labels map[string]string) *Requirements {
	requirements := []v1.NodeSelectorRequirement{}
	for key, value := range labels {
		requirements = append(requirements, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}})
	}
	return NewRequirements(requirements...)
}

func PodRequirements(pod *v1.Pod) *Requirements {
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

// Compatible returns errors with detailed messages when requirements are not compatible
// Compatible is non-commutative (i.e., A.Compatible(B) != B.Compatible(A))
func (r *Requirements) Compatible(requirements *Requirements) (errs *apis.FieldError) {
	if r == nil {
		r = NewRequirements()
	}
	allKeys := stringsets.NewString(r.Keys()...).Union(stringsets.NewString(requirements.Keys()...))
	for key := range allKeys {

		if r.Allow(key).Intersection(requirements.Allow(key)).Len() == 0 {
			rValues, _ := r.Allow(key).Values()
			sValues, _ := requirements.Allow(key).Values()
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("no common values for key %s, %v not in %v", key, sValues, rValues), "values"))
		}
		if err := r.compatible(key, requirements); err != nil {
			errs = errs.Also(err)
		}
		// Directional condition:
		// This is to capture pod specify a label selector but provisioner does not have the label
		// Cases that are ignored: r has Allow values but requirements doesn't specify
		// Cases that are caught: requirements has Allow values but r doesn't specify
		if _, ok := requirements.Requirements[key]; ok {
			_, exist := r.Requirements[key]
			if !requirements.Allow(key).IsComplement && requirements.Allow(key).Len() != 0 && !exist {
				values, _ := requirements.Allow(key).Values()
				errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("key %s, need %v but not defined", key, values), "values"))
			}
		}

	}
	return errs
}

func (r *Requirements) compatible(key string, requirements *Requirements) (errs *apis.FieldError) {
	for r1, r2 := range map[*Requirements]*Requirements{
		r:            requirements,
		requirements: r,
	} {
		if records, ok := r1.Requirements[key]; ok {
			for _, requirement := range records {
				switch requirement.Operator {
				case v1.NodeSelectorOpExists:
					if _, exist := r2.Requirements[key]; !exist {
						errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("Exist operator violation, %s", key), "values"))
					}
				case v1.NodeSelectorOpDoesNotExist:
					if _, exist := r2.Requirements[key]; exist {
						errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("DoesNotExist operator violation, %s", key), "values"))
					}
				}
			}
		}
	}
	return errs
}

func (r *Requirements) Zones() *sets.Set {
	return r.Allow(v1.LabelTopologyZone)
}

func (r *Requirements) InstanceTypes() *sets.Set {
	return r.Allow(v1.LabelInstanceTypeStable)
}

func (r *Requirements) Architectures() *sets.Set {
	return r.Allow(v1.LabelArchStable)
}

func (r *Requirements) OperatingSystems() *sets.Set {
	return r.Allow(v1.LabelOSStable)
}

func (r *Requirements) CapacityTypes() *sets.Set {
	return r.Allow(LabelCapacityType)
}

func (r *Requirements) WellKnown() *Requirements {
	requirements := []v1.NodeSelectorRequirement{}
	for _, key := range r.Keys() {
		if WellKnownLabels.Has(key) {
			requirements = append(requirements, r.Requirements[key]...)
		}
	}
	return NewRequirements(requirements...)
}

// Keys returns unique set of the label keys from the requirements
func (r *Requirements) Keys() []string {
	if r == nil {
		r = NewRequirements()
	}
	keys := make([]string, 0, len(r.Requirements))
	for k := range r.Requirements {
		keys = append(keys, k)
	}
	return keys
}
