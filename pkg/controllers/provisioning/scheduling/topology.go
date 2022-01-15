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
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/pod"
	"github.com/mitchellh/hashstructure/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Topology struct {
	kubeClient client.Client
}

// Inject injects topology rules into pods using supported NodeSelectors
func (t *Topology) Inject(ctx context.Context, constraints *v1alpha5.Constraints, pods []*v1.Pod) error {
	// Group pods by equivalent topology spread constraints
	topologyGroups := t.getTopologyGroups(pods)
	// Compute spread
	for _, topologyGroup := range topologyGroups {
		if err := t.computeCurrentTopology(ctx, constraints, topologyGroup); err != nil {
			return fmt.Errorf("computing topology, %w", err)
		}
		for _, pod := range topologyGroup.Pods {
			domain := topologyGroup.NextDomain(constraints.Requirements.Merge(v1alpha5.PodRequirements(pod)).Allow(topologyGroup.Constraint.TopologyKey))
			pod.Spec.NodeSelector = functional.UnionStringMaps(pod.Spec.NodeSelector, map[string]string{topologyGroup.Constraint.TopologyKey: domain})
		}
	}
	return nil
}

// getTopologyGroups separates pods with equivalent topology rules
func (t *Topology) getTopologyGroups(pods []*v1.Pod) []*TopologyGroup {
	topologyGroupMap := map[uint64]*TopologyGroup{}
	for _, pod := range pods {
		for _, constraint := range pod.Spec.TopologySpreadConstraints {
			// Add to existing group if exists, using a hash for efficient collision detection
			key := topologyGroupKey(pod.Namespace, constraint)
			if topologyGroup, ok := topologyGroupMap[key]; ok {
				topologyGroup.Pods = append(topologyGroup.Pods, pod)
			} else {
				topologyGroupMap[key] = NewTopologyGroup(pod, constraint)
			}
		}
	}
	topologyGroups := []*TopologyGroup{}
	for _, topologyGroup := range topologyGroupMap {
		topologyGroups = append(topologyGroups, topologyGroup)
	}
	return topologyGroups
}

func (t *Topology) computeCurrentTopology(ctx context.Context, constraints *v1alpha5.Constraints, topologyGroup *TopologyGroup) error {
	switch topologyGroup.Constraint.TopologyKey {
	case v1.LabelHostname:
		return t.computeHostnameTopology(topologyGroup, constraints)
	case v1.LabelTopologyZone:
		return t.computeZonalTopology(ctx, constraints.Requirements, topologyGroup)
	default:
		return nil
	}
}

// computeHostnameTopology for the topology group. Hostnames are guaranteed to
// be unique when new nodes join the cluster. Nodes that join the cluster do not
// contain any pods, so we can assume that the global minimum domain count for
// `hostname` is 0. Thus, we can always improve topology skew (computed against
// the global minimum) by adding pods to the cluster. We will generate
// len(pods)/MaxSkew number of domains, to ensure that skew is not violated for
// new instances.
func (t *Topology) computeHostnameTopology(topologyGroup *TopologyGroup, constraints *v1alpha5.Constraints) error {
	domains := []string{}
	for i := 0; i < int(math.Ceil(float64(len(topologyGroup.Pods))/float64(topologyGroup.Constraint.MaxSkew))); i++ {
		domains = append(domains, strings.ToLower(randomdata.Alphanumeric(8)))
	}
	topologyGroup.Register(domains...)
	// This is a bit of a hack that allows the constraints to recognize viable hostname topologies
	constraints.Requirements = constraints.Requirements.
		Merge(v1alpha5.NewRequirements(v1.NodeSelectorRequirement{Key: topologyGroup.Constraint.TopologyKey, Operator: v1.NodeSelectorOpIn, Values: domains}))
	return nil
}

// computeZonalTopology for the topology group. Zones include viable zones for
// the { cloudprovider, provisioner, pod }. If these zones change over time,
// topology skew calculations will only include the current viable zone
// selection. For example, if a cloud provider or provisioner changes the viable
// set of nodes, topology calculations will rebalance the new set of zones.
func (t *Topology) computeZonalTopology(ctx context.Context, requirements *v1alpha5.Requirements, topologyGroup *TopologyGroup) error {
	zones, err := requirements.Zones().Values()
	if err != nil {
		return fmt.Errorf("getting zone list, %w", err)

	}
	topologyGroup.Register(zones...)
	if err := t.countMatchingPods(ctx, topologyGroup); err != nil {
		return fmt.Errorf("getting matching pods, %w", err)
	}
	return nil
}

func (t *Topology) countMatchingPods(ctx context.Context, topologyGroup *TopologyGroup) error {
	pods := &v1.PodList{}
	if err := t.kubeClient.List(ctx, pods, TopologyListOptions(topologyGroup.Pods[0].Namespace, &topologyGroup.Constraint)); err != nil {
		return fmt.Errorf("listing pods, %w", err)
	}
	for i, p := range pods.Items {
		if IgnoredForTopology(&pods.Items[i]) {
			continue
		}
		node := &v1.Node{}
		if err := t.kubeClient.Get(ctx, types.NamespacedName{Name: p.Spec.NodeName}, node); err != nil {
			return fmt.Errorf("getting node %s, %w", p.Spec.NodeName, err)
		}
		domain, ok := node.Labels[topologyGroup.Constraint.TopologyKey]
		if !ok {
			continue // Don't include pods if node doesn't contain domain https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/#conventions
		}
		topologyGroup.Increment(domain)
	}
	return nil
}

func TopologyListOptions(namespace string, constraint *v1.TopologySpreadConstraint) *client.ListOptions {
	selector := labels.Everything()
	if constraint.LabelSelector == nil {
		return &client.ListOptions{Namespace: namespace, LabelSelector: selector}
	}
	for key, value := range constraint.LabelSelector.MatchLabels {
		requirement, _ := labels.NewRequirement(key, selection.Equals, []string{value})
		selector = selector.Add(*requirement)
	}
	for _, expression := range constraint.LabelSelector.MatchExpressions {
		requirement, _ := labels.NewRequirement(expression.Key, selection.Operator(expression.Operator), expression.Values)
		selector = selector.Add(*requirement)
	}
	return &client.ListOptions{Namespace: namespace, LabelSelector: selector}
}

func IgnoredForTopology(p *v1.Pod) bool {
	return !pod.IsScheduled(p) || pod.IsTerminal(p) || pod.IsTerminating(p)
}

func topologyGroupKey(namespace string, constraint v1.TopologySpreadConstraint) uint64 {
	hash, err := hashstructure.Hash(struct {
		Namespace  string
		Constraint v1.TopologySpreadConstraint
	}{namespace, constraint}, hashstructure.FormatV2, nil)
	if err != nil {
		panic(fmt.Errorf("unexpected failure hashing topology, %w", err))
	}
	return hash
}
