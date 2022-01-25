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

package selection

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

func NewAntiAffinity() *AntiAffinity {
	return &AntiAffinity{}
}

var AllowedAntiAffinityKeys = sets.NewString(v1.LabelHostname)

type AntiAffinity struct{}

// Validate that the affinity terms are supported
func (a *AntiAffinity) Validate(pod *v1.Pod) (errs *apis.FieldError) {
	for i, term := range a.termsFor(pod) {
		errs = errs.Also(a.validateTerm(term).ViaIndex(i))
	}
	return errs
}

func (a *AntiAffinity) validateTerm(term v1.PodAffinityTerm) (errs *apis.FieldError) {
	if term.NamespaceSelector != nil {
		errs = errs.Also(apis.ErrDisallowedFields("namespaceSelector"))
	}
	if len(term.Namespaces) != 0 {
		errs = errs.Also(apis.ErrDisallowedFields("namespaces"))
	}
	if !AllowedAntiAffinityKeys.Has(term.TopologyKey) {
		errs = errs.Also(apis.ErrInvalidKeyName(fmt.Sprintf("%s not in %v", term.TopologyKey, AllowedAntiAffinityKeys.UnsortedList()), "topologyKey"))
	}
	return errs
}

// Transform pod anti affinity into topology rules
func (a *AntiAffinity) Transform(ctx context.Context, pod *v1.Pod) {
	for _, term := range a.termsFor(pod) {
		pod.Spec.TopologySpreadConstraints = append(pod.Spec.TopologySpreadConstraints, v1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       term.TopologyKey,
			WhenUnsatisfiable: v1.DoNotSchedule,
			LabelSelector:     term.LabelSelector,
		})
	}
}

func (a *AntiAffinity) termsFor(pod *v1.Pod) []v1.PodAffinityTerm {
	if pod.Spec.Affinity == nil {
		return nil
	}
	if pod.Spec.Affinity.PodAntiAffinity == nil {
		return nil
	}
	terms := pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	for _, term := range pod.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		terms = append(terms, term.PodAffinityTerm)
	}
	return terms
}
