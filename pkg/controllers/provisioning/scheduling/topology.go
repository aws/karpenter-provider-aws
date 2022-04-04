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

	"k8s.io/apimachinery/pkg/util/runtime"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/pod"
)

type Topology struct {
	kubeClient client.Client
	topologies map[uint64]*TopologyGroup
	// Anti-affinity works both ways (if a zone has a pod foo with anti-affinity to a pod bar, we can't schedule bar to
	// that zone, even though bar has no anti affinity terms on it. For this to work, we need to separately track the
	// topologies of pods with anti-affinity terms, so we can prevent scheduling the pods they have anti-affinity to
	// in some cases.
	inverseTopologies map[uint64]*TopologyGroup
	// Universe of options used to compute domains
	requirements *v1alpha5.Requirements
}

func NewTopology(kubeClient client.Client, requirements *v1alpha5.Requirements) *Topology {
	return &Topology{
		kubeClient:        kubeClient,
		requirements:      requirements,
		topologies:        map[uint64]*TopologyGroup{},
		inverseTopologies: map[uint64]*TopologyGroup{},
	}
}

// Initialize scans the pods provided and creates topology groups for any topologies that we need to track based off of
// topology spreads, affinities, and anti-affinities specified in the pods.
func (t *Topology) Initialize(ctx context.Context, pods ...*v1.Pod) (errs error) {
	errs = multierr.Append(errs, t.updateInverseAffinities(ctx))
	for i := range pods {
		errs = multierr.Append(errs, t.Update(ctx, pods[i]))
	}
	return errs
}

func (t *Topology) Update(ctx context.Context, p *v1.Pod) (errs error) {
	for _, topology := range t.topologies {
		if topology.IsOwnedBy(p.UID) {
			topology.RemoveOwner(p.UID)
		}
	}

	if err := t.updateTopologySpread(ctx, p); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("tracking topology spread, %w", err))
	}
	if err := t.updateAffinity(ctx, p); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("tracking affinity topology, %w", err))
	}
	return errs
}

// updateInverseAffinities is used to identify pods with anti-affinity terms so we can track those topologies.  We
// have to look at every pod in the cluster as there is no way to query for a pod with anti-affinity terms.
func (t *Topology) updateInverseAffinities(ctx context.Context) error {
	var nodeList v1.NodeList
	if err := t.kubeClient.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("listing nodes, %w", err)
	}
	for i := range nodeList.Items {
		var podlist v1.PodList
		if err := t.kubeClient.List(ctx, &podlist, client.MatchingFields{"spec.nodeName": nodeList.Items[i].Name}); err != nil {
			return fmt.Errorf("listing pods on node %s, %w", nodeList.Items[i].Name, err)
		}
		for j := range podlist.Items {
			if pod.HasRequiredPodAntiAffinity(&podlist.Items[j]) {
				if err := t.updateInverseAntiAffinity(ctx, &nodeList.Items[i], &podlist.Items[j]); err != nil {
					return fmt.Errorf("tracking existing pod anti-affinity, %w", err)
				}
			}
		}
	}
	return nil
}

func (t *Topology) updateTopologySpread(ctx context.Context, p *v1.Pod) error {
	for _, cs := range p.Spec.TopologySpreadConstraints {
		topologyGroup := t.newForSpread(p.Namespace, cs)
		hash := topologyGroup.Hash()
		if existing, ok := t.topologies[hash]; !ok {
			if err := t.initializeTopologyGroup(ctx, topologyGroup); err != nil {
				return err
			}
			t.topologies[hash] = topologyGroup
		} else {
			topologyGroup = existing
		}
		topologyGroup.AddOwner(p.UID)
	}
	return nil
}

func (t *Topology) newForSpread(namespace string, cs v1.TopologySpreadConstraint) *TopologyGroup {
	return &TopologyGroup{
		Key:        cs.TopologyKey,
		maxSkew:    cs.MaxSkew,
		Type:       TopologyTypeSpread,
		selector:   cs.LabelSelector,
		domains:    map[string]int32{},
		namespaces: sets.NewString(namespace),
		owners:     map[types.UID]struct{}{},
	}
}

func (t *Topology) newForAffinity(term v1.PodAffinityTerm, namespaces sets.String, topoType TopologyType) *TopologyGroup {
	return &TopologyGroup{
		Key:        term.TopologyKey,
		maxSkew:    math.MaxInt32,
		Type:       topoType,
		selector:   term.LabelSelector,
		domains:    map[string]int32{},
		namespaces: namespaces,
		owners:     map[types.UID]struct{}{},
	}
}

// updateInverseAntiAffinity is used to track topologies of inverse anti-affinities. Here the domains & counts track the
// pods with the anti-affinity.
func (t *Topology) updateInverseAntiAffinity(ctx context.Context, node *v1.Node, p *v1.Pod) error {
	// We intentionally don't track inverse anti-affinity preferences.  We're not required to enforce them so it
	// just adds complexity for very little value.
	for _, term := range p.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		namespaces, err := t.buildNamespaceList(ctx, p.Namespace, term.Namespaces, term.NamespaceSelector)
		if err != nil {
			return err
		}

		topologyGroup := t.newForAffinity(term, namespaces, TopologyTypePodAntiAffinity)
		hash := topologyGroup.Hash()
		if existing, ok := t.inverseTopologies[hash]; !ok {
			t.inverseTopologies[hash] = topologyGroup
			topologyGroup.InitializeWellKnown(t.requirements)
		} else {
			topologyGroup = existing
		}
		if node != nil {
			topologyGroup.RecordUsage(node.Labels[topologyGroup.Key])
		}
		topologyGroup.AddOwner(p.UID)
	}
	return nil
}

func (t *Topology) updateAffinity(ctx context.Context, p *v1.Pod) error {
	var topologyGroups []*TopologyGroup
	if pod.HasPodAffinity(p) {
		groups, err := t.newForAffinities(ctx, p, p.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			p.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution, TopologyTypePodAffinity)
		if err != nil {
			return err
		}
		topologyGroups = append(topologyGroups, groups...)
	}

	if pod.HasPodAntiAffinity(p) {
		if err := t.updateInverseAntiAffinity(ctx, nil, p); err != nil {
			return fmt.Errorf("tracking existing pod anti-affinity, %w", err)
		}
		groups, err := t.newForAffinities(ctx, p, p.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			p.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, TopologyTypePodAntiAffinity)
		if err != nil {
			return err
		}
		topologyGroups = append(topologyGroups, groups...)
	}

	var err error
	for _, tg := range topologyGroups {
		hash := tg.Hash()
		// is this a new topology we aren't tracking yet?
		if existing, ok := t.topologies[hash]; !ok {
			t.topologies[hash] = tg
			err = multierr.Append(err, t.initializeTopologyGroup(ctx, tg))
		} else {
			tg = existing
		}
		tg.AddOwner(p.UID)
	}
	return err
}

// newForAffinities returns a list of topology groups that have been constructed based on the input pod and required/preferred affinity terms
func (t *Topology) newForAffinities(ctx context.Context, p *v1.Pod, required []v1.PodAffinityTerm, preferred []v1.WeightedPodAffinityTerm,
	topoType TopologyType) ([]*TopologyGroup, error) {
	var topologyGroups []*TopologyGroup
	for _, term := range required {
		namespaces, err := t.buildNamespaceList(ctx, p.Namespace, term.Namespaces, term.NamespaceSelector)
		if err != nil {
			return nil, err
		}
		topology := t.newForAffinity(term, namespaces, topoType)
		topologyGroups = append(topologyGroups, topology)
	}

	for _, term := range preferred {
		namespaces, err := t.buildNamespaceList(ctx, p.Namespace, term.PodAffinityTerm.Namespaces, term.PodAffinityTerm.NamespaceSelector)
		if err != nil {
			return nil, err
		}
		topology := t.newForAffinity(term.PodAffinityTerm, namespaces, topoType)
		topologyGroups = append(topologyGroups, topology)
	}
	return topologyGroups, nil
}

// initializeTopologyGroup initializes the topology group by registereding any well-domains and performing pod counts
// against the cluster for any existing pods.
func (t *Topology) initializeTopologyGroup(ctx context.Context, tg *TopologyGroup) error {
	tg.InitializeWellKnown(t.requirements)

	podList := &v1.PodList{}
	// collect the pods from all the specified namespaces (don't see a way to query multiple namespaces
	// simultaneously)
	var pods []v1.Pod
	for _, ns := range tg.namespaces.UnsortedList() {
		if err := t.kubeClient.List(ctx, podList, TopologyListOptions(ns, tg.selector)); err != nil {
			return fmt.Errorf("listing pods, %w", err)
		}
		pods = append(pods, podList.Items...)
	}

	for i, p := range pods {
		if IgnoredForTopology(&pods[i]) {
			continue
		}
		if tg.Key == v1.LabelHostname {
			tg.RecordUsage(p.Spec.NodeName)
			continue
		}
		node := &v1.Node{}
		if err := t.kubeClient.Get(ctx, types.NamespacedName{Name: p.Spec.NodeName}, node); err != nil {
			return fmt.Errorf("getting node %s, %w", p.Spec.NodeName, err)
		}
		domain, ok := node.Labels[tg.Key]
		if !ok {
			continue // Don't include pods if node doesn't contain domain https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/#conventions
		}
		tg.RecordUsage(domain)
	}
	return nil
}

// buildNamespaceList constructs a unique list of namespaces consisting of the pod's namespace and the optional list of
// namespaces and those selected by the namespace selector
func (t *Topology) buildNamespaceList(ctx context.Context, namespace string, namespaces []string, selector *metav1.LabelSelector) (sets.String, error) {
	if len(namespaces) == 0 && selector == nil {
		return sets.NewString(namespace), nil
	}
	if selector == nil {
		return sets.NewString(namespaces...), nil
	}
	var namespaceList v1.NamespaceList
	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	if err := t.kubeClient.List(ctx, &namespaceList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return nil, err
	}
	selected := sets.NewString()
	for _, namespace := range namespaceList.Items {
		selected.Insert(namespace.Name)
	}
	selected.Insert(namespaces...)
	return selected, nil
}

// Record records the topology changes given that pod p schedule on node n
func (t *Topology) Record(p *v1.Pod, requirements v1alpha5.Requirements) error {
	// once we've now committed to a domain, we record the usage in every topology that cares about it
	var err error
	podLabels := labels.Set(p.Labels)
	for _, tc := range t.topologies {
		if tc.Matches(p.Namespace, podLabels) {
			domains := requirements.Get(tc.Key)
			if domains.Len() == 1 {
				if domain := domains.Values().List()[0]; domain != "" {
					tc.RecordUsage(domain)
				}
			}
		}
	}
	// for anti-affinities, we need to also record where the pods with the anti-affinity are
	for _, tc := range t.inverseTopologies {
		if tc.IsOwnedBy(p.UID) {
			domains := requirements.Get(tc.Key)
			if domains.Len() == 1 {
				if domain := domains.Values().List()[0]; domain != "" {
					tc.RecordUsage(domain)
				} else {
					err = multierr.Append(err, fmt.Errorf("empty or missing domain for topology key %s", tc.Key))
				}
			} else {
				// TODO(todd): try to construct a case to get here, or convince myself it's not possible.  We treat anti-affinities
				// somewhat like a topology spread in that we need to land in any empty domain.  Topology spread does this so it can
				// keep count, the anti-affinity could create a node selector like in [zone-a, zone-b, zone-c], but currently
				// we just choose one and commit.  If we don't do that, we would need to record usage in all of those domains and prevent
				// some other pods from scheduling this round.
				err = multierr.Append(err, fmt.Errorf("empty or missing domain for topology key %s", tc.Key))
			}
		}
	}
	return err
}

// getMatchingTopologies returns a sorted list of topologies that either control the scheduling of pod p, or for which
// the topology selects pod p and the scheduling of p affects the count per topology domain
func (t *Topology) getMatchingTopologies(p *v1.Pod) []*TopologyGroup {
	var matchingTopologies []*TopologyGroup
	for _, tc := range t.topologies {
		if tc.IsOwnedBy(p.UID) {
			matchingTopologies = append(matchingTopologies, tc)
		}
	}
	for _, tc := range t.inverseTopologies {
		if tc.Matches(p.Namespace, p.Labels) {
			matchingTopologies = append(matchingTopologies, tc)
		}
	}
	return matchingTopologies
}

// Requirements tightens the input requirements by adding additional requirements that are being enforced by topology spreads
// affinities, anti-affinities or inverse anti-affinities.  It returns these newly tightened requirements, or an error in
// the case of a set of requirements that cannot be satisfied.
//gocyclo: ignore
func (t *Topology) Requirements(requirements v1alpha5.Requirements, nodeName string, p *v1.Pod) (v1alpha5.Requirements, error) {
	for _, topology := range t.getMatchingTopologies(p) {
		if topology.Key == v1.LabelHostname {
			topology.RegisterDomain(nodeName)
		}

		nextDomain, err := topology.Next(requirements, topology.Matches(p.Namespace, p.Labels))
		if err != nil {
			return v1alpha5.Requirements{}, err
		}

		requirements = requirements.Add(v1.NodeSelectorRequirement{
			Key:      topology.Key,
			Operator: v1.NodeSelectorOpIn,
			Values:   []string{nextDomain},
		})
	}
	return requirements, nil
}

func TopologyListOptions(namespace string, labelSelector *metav1.LabelSelector) *client.ListOptions {
	selector := labels.Everything()
	if labelSelector == nil {
		return &client.ListOptions{Namespace: namespace, LabelSelector: selector}
	}
	for key, value := range labelSelector.MatchLabels {
		requirement, err := labels.NewRequirement(key, selection.Equals, []string{value})
		runtime.Must(err)
		selector = selector.Add(*requirement)
	}
	for _, expression := range labelSelector.MatchExpressions {
		requirement, err := labels.NewRequirement(expression.Key, selection.Operator(expression.Operator), expression.Values)
		runtime.Must(err)
		selector = selector.Add(*requirement)
	}
	return &client.ListOptions{Namespace: namespace, LabelSelector: selector}
}

func IgnoredForTopology(p *v1.Pod) bool {
	return !pod.IsScheduled(p) || pod.IsTerminal(p) || pod.IsTerminating(p)
}
