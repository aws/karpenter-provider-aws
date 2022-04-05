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
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/sets"
)

// domainFilter is a filter that matches possible topology domains against the pod's spec.NodeSelector and
// required terms from the spec.Affinity.NodeAffinity
type domainFilter struct {
	validDomains []sets.Set
}

// Matches returns true if the domain is valid given the constraints derived from the pod.
func (n *domainFilter) Matches(domain string) bool {
	// no domain limitations, so everything is valid
	if len(n.validDomains) == 0 {
		return true
	}

	// there are 1-N valid domain sets, if the domain is accepted by any of them it's valid
	for _, vd := range n.validDomains {
		if vd.Has(domain) {
			return true
		}
	}
	return false
}

// newDomainFilter constructs a filter that matches possible domain values against those allowed by the pod's spec.NodeSelector and
// required terms from the spec.Affinity.NodeAffinity.  If neither is specified, the filter will accept all values.
func newDomainFilter(pod *v1.Pod, topologyKey string) domainFilter {
	// the label selector is a single value that overrides everything and restricts us to a single domain
	for key, domain := range pod.Spec.NodeSelector {
		if key == topologyKey {
			return domainFilter{
				validDomains: []sets.Set{sets.NewSet(domain)},
			}
		}
	}

	// required node affinities are OR'd together, so we construct a list of valid domains
	var filter domainFilter
	var nodeRequiredAffinity *v1.NodeSelector
	if pod.Spec.Affinity != nil && pod.Spec.Affinity.NodeAffinity != nil && pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		nodeRequiredAffinity = pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		for _, term := range nodeRequiredAffinity.NodeSelectorTerms {
			requirements := v1alpha5.NewRequirements(term.MatchExpressions...)
			if requirements.Has(topologyKey) {
				// union all of these together into valid domains
				filter.validDomains = append(filter.validDomains, requirements.Get(topologyKey))
			}
		}
	}
	return filter
}
