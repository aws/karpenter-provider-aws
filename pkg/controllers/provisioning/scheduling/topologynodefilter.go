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
)

// TopologyNodeFilter is used to determine if a given actual node or scheduling node matches the pod's node selectors
// and required node affinity terms.  This is used with topology spread constraints to determine if the node should be
// included for topology counting purposes. This is only used with topology spread constraints as affinities/anti-affinities
// always count across all nodes. A nil or zero-value TopologyNodeFilter behaves well and the filter returns true for
// all nodes.
type TopologyNodeFilter []v1alpha5.Requirements

func MakeTopologyNodeFilter(p *v1.Pod) TopologyNodeFilter {
	nodeSelectorRequirements := v1alpha5.NewLabelRequirements(p.Spec.NodeSelector)
	// if we only have a label selector, that's the only requirement that must match
	if p.Spec.Affinity == nil || p.Spec.Affinity.NodeAffinity == nil || p.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return TopologyNodeFilter{nodeSelectorRequirements}
	}

	// otherwise, we need to match the combination of label selector and any term of the required node affinities since
	// those terms are OR'd together
	var filter TopologyNodeFilter
	for _, term := range p.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		requirements := nodeSelectorRequirements.Add(v1alpha5.NewRequirements(term.MatchExpressions...).Requirements...)
		filter = append(filter, requirements)
	}

	return filter
}

// Matches returns true if the TopologyNodeFilter doesn't prohibit node from the participating in the topology
func (t TopologyNodeFilter) Matches(node *v1.Node) bool {
	nodeLabels := v1alpha5.NewLabelRequirements(node.Labels)
	return t.MatchesRequirements(nodeLabels)
}

// MatchesRequirements returns true if the TopologyNodeFilter doesn't prohibit a node with the requirements from
// participating in the topology. This method allows checking the requirements from a scheduling.Node to see if the
// node we will soon create participates in this topology.
func (t TopologyNodeFilter) MatchesRequirements(requirements v1alpha5.Requirements) bool {
	// no requirements, so it always matches
	if len(t) == 0 {
		return true
	}
	// these are an OR, so if any passes the filter passes
	for _, req := range t {
		if err := requirements.Compatible(req); err == nil {
			return true
		}
	}
	return false
}
