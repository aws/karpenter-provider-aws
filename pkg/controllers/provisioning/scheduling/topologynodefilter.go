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
	"github.com/mitchellh/hashstructure/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

// TopologyNodeFilter is used to determine if a given actual node or scheduling node matches the pod's node selectors
// and required node affinity terms.  This is used with topology spread constraints to determine if the node should be
// included for topology counting purposes. This is only used with topology spread constraints as affinities/anti-affinities
// always count across all nodes. A nil or zero-value TopologyNodeFilter behaves well and the filter returns true for
// all nodes.
type TopologyNodeFilter struct {
	nodeSelector      map[string]string
	nodeSelectorTerms []v1.NodeSelectorTerm
}

func NewTopologyNodeFilter(p *v1.Pod) *TopologyNodeFilter {
	selector := &TopologyNodeFilter{
		nodeSelector: p.Spec.NodeSelector,
	}
	if p.Spec.Affinity != nil && p.Spec.Affinity.NodeAffinity != nil && p.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		selector.nodeSelectorTerms = p.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	}
	return selector
}

// Matches returns true if the TopologyNodeFilter doesn't prohibit node from the participating in the topology
func (t *TopologyNodeFilter) Matches(node *v1.Node) bool {
	if t == nil {
		return true
	}
	// if a node selector term is provided, it must match the node
	for k, v := range t.nodeSelector {
		if node.Labels[k] != v {
			return false
		}
	}
	if len(t.nodeSelectorTerms) == 0 {
		return true
	}

	for _, term := range t.nodeSelectorTerms {
		requirement := v1alpha5.NewRequirements(term.MatchExpressions...)
		if t.matchesRequirements(requirement, node) {
			return true
		}
	}

	// at this point we have node selector terms, but didn't match all of the requirements for any individual term
	return false
}

// MatchesRequirements returns true if the TopologyNodeFilter doesn't prohibit a node with the requirements from
// participating in the topology. This method allows checking the requirements from a scheduling.Node to see if the
// node we will soon create participates in this topology.
func (t *TopologyNodeFilter) MatchesRequirements(nodeRequirements v1alpha5.Requirements) bool {
	if t == nil {
		return true
	}
	for k, v := range t.nodeSelector {
		if !nodeRequirements.Get(k).Has(v) {
			return false
		}
	}
	if len(t.nodeSelectorTerms) == 0 {
		return true
	}

	for _, term := range t.nodeSelectorTerms {
		requirement := v1alpha5.NewRequirements(term.MatchExpressions...)
		matchesAllReqs := true
		for key := range requirement.Keys() {
			if requirement.Get(key).Intersection(nodeRequirements.Get(key)).Len() == 0 {
				matchesAllReqs = false
				break
			}
		}
		// these terms are OR'd together, so if we match one full set of requirements, the filter passes
		if matchesAllReqs {
			return true
		}
	}
	return true
}

func (t *TopologyNodeFilter) Hash() uint64 {
	if t == nil || (len(t.nodeSelector) == 0 && len(t.nodeSelectorTerms) == 0) {
		return 0
	}
	hash, err := hashstructure.Hash(struct {
		NodeSelector      map[string]string
		NodeSelectorTerms []v1.NodeSelectorTerm
	}{
		NodeSelector:      t.nodeSelector,
		NodeSelectorTerms: t.nodeSelectorTerms,
	}, hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true})
	runtime.Must(err)
	return hash

}

func (t *TopologyNodeFilter) matchesRequirements(requirement v1alpha5.Requirements, node *v1.Node) bool {
	for key := range requirement.Keys() {
		if !requirement.Get(key).Has(node.Labels[key]) {
			return false
		}
	}
	return true
}
