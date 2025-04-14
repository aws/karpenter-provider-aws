/*
Copyright The Kubernetes Authors.

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

	"github.com/awslabs/operatorpkg/option"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/pod"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type Topology struct {
	kubeClient client.Client
	// Both the topologies and inverseTopologies are maps of the hash from TopologyGroup.Hash() to the topology group
	// itself. This is used to allow us to store one topology group that tracks the topology of many pods instead of
	// having a 1<->1 mapping between topology groups and pods owned/selected by that group.
	topologies map[uint64]*TopologyGroup
	// Anti-affinity works both ways (if a zone has a pod foo with anti-affinity to a pod bar, we can't schedule bar to
	// that zone, even though bar has no anti affinity terms on it. For this to work, we need to separately track the
	// topologies of pods with anti-affinity terms, so we can prevent scheduling the pods they have anti-affinity to
	// in some cases.
	inverseTopologies map[uint64]*TopologyGroup
	// The universe of domains by topology key
	domains map[string]sets.Set[string]
	// excludedPods are the pod UIDs of pods that are excluded from counting.  This is used so we can simulate
	// moving pods to prevent them from being double counted.
	excludedPods sets.Set[string]
	cluster      *state.Cluster
}

func NewTopology(ctx context.Context, kubeClient client.Client, cluster *state.Cluster, domains map[string]sets.Set[string], pods []*corev1.Pod) (*Topology, error) {
	t := &Topology{
		kubeClient:        kubeClient,
		cluster:           cluster,
		domains:           domains,
		topologies:        map[uint64]*TopologyGroup{},
		inverseTopologies: map[uint64]*TopologyGroup{},
		excludedPods:      sets.New[string](),
	}

	// these are the pods that we intend to schedule, so if they are currently in the cluster we shouldn't count them for
	// topology purposes
	for _, p := range pods {
		t.excludedPods.Insert(string(p.UID))
	}

	errs := t.updateInverseAffinities(ctx)
	for i := range pods {
		errs = multierr.Append(errs, t.Update(ctx, pods[i]))
	}
	if errs != nil {
		return nil, errs
	}
	return t, nil
}

// topologyError allows lazily generating the error string in the topology error.  If a pod fails to schedule, most often
// we are only interested in the fact that it failed to schedule and not why.
type topologyError struct {
	topology    *TopologyGroup
	podDomains  *scheduling.Requirement
	nodeDomains *scheduling.Requirement
}

func (t topologyError) Error() string {
	return fmt.Sprintf("unsatisfiable topology constraint for %s, key=%s (counts = %s, podDomains = %v, nodeDomains = %v", t.topology.Type, t.topology.Key,
		pretty.Map(t.topology.domains, 25), t.podDomains, t.nodeDomains)
}

// Update unregisters the pod as the owner of all affinities and then creates any new topologies based on the pod spec
// registered the pod as the owner of all associated affinities, new or old.  This allows Update() to be called after
// relaxation of a preference to properly break the topology <-> owner relationship so that the preferred topology will
// no longer influence scheduling.
func (t *Topology) Update(ctx context.Context, p *corev1.Pod) error {
	for _, topology := range t.topologies {
		topology.RemoveOwner(p.UID)
	}

	if pod.HasPodAntiAffinity(p) {
		if err := t.updateInverseAntiAffinity(ctx, p, nil); err != nil {
			return fmt.Errorf("updating inverse anti-affinities, %w", err)
		}
	}

	topologies := t.newForTopologies(p)
	affinities, err := t.newForAffinities(ctx, p)
	if err != nil {
		return fmt.Errorf("updating affinities, %w", err)
	}

	for _, tg := range append(topologies, affinities...) {
		hash := tg.Hash()
		// Avoid recomputing topology counts if we've already seen this group
		if existing, ok := t.topologies[hash]; !ok {
			if err := t.countDomains(ctx, tg); err != nil {
				return err
			}
			t.topologies[hash] = tg
		} else {
			tg = existing
		}
		tg.AddOwner(p.UID)
	}
	return nil
}

// Record records the topology changes given that pod p schedule on a node with the given requirements
func (t *Topology) Record(p *corev1.Pod, requirements scheduling.Requirements, compatabilityOptions ...option.Function[scheduling.CompatibilityOptions]) {
	// once we've committed to a domain, we record the usage in every topology that cares about it
	for _, tc := range t.topologies {
		if tc.Counts(p, requirements, compatabilityOptions...) {
			domains := requirements.Get(tc.Key)
			if tc.Type == TopologyTypePodAntiAffinity {
				// for anti-affinity topologies we need to block out all possible domains that the pod could land in
				tc.Record(domains.Values()...)
			} else {
				// but for affinity & topology spread, we can only record the domain if we know the specific domain we land in
				if domains.Len() == 1 {
					tc.Record(domains.Values()[0])
				}
			}
		}
	}
	// for anti-affinities, we record where the pods could be, even if
	// requirements haven't collapsed to a single value.
	for _, tc := range t.inverseTopologies {
		if tc.IsOwnedBy(p.UID) {
			tc.Record(requirements.Get(tc.Key).Values()...)
		}
	}
}

// AddRequirements tightens the input requirements by adding additional requirements that are being enforced by topology spreads
// affinities, anti-affinities or inverse anti-affinities.  The nodeHostname is the hostname that we are currently considering
// placing the pod on.  It returns these newly tightened requirements, or an error in the case of a set of requirements that
// cannot be satisfied.
func (t *Topology) AddRequirements(podRequirements, nodeRequirements scheduling.Requirements, p *corev1.Pod, compatabilityOptions ...option.Function[scheduling.CompatibilityOptions]) (scheduling.Requirements, error) {
	requirements := scheduling.NewRequirements(nodeRequirements.Values()...)
	for _, topology := range t.getMatchingTopologies(p, nodeRequirements, compatabilityOptions...) {
		podDomains := scheduling.NewRequirement(topology.Key, corev1.NodeSelectorOpExists)
		if podRequirements.Has(topology.Key) {
			podDomains = podRequirements.Get(topology.Key)
		}
		nodeDomains := scheduling.NewRequirement(topology.Key, corev1.NodeSelectorOpExists)
		if nodeRequirements.Has(topology.Key) {
			nodeDomains = nodeRequirements.Get(topology.Key)
		}
		domains := topology.Get(p, podDomains, nodeDomains)
		if domains.Len() == 0 {
			return nil, topologyError{
				topology:    topology,
				podDomains:  podDomains,
				nodeDomains: nodeDomains,
			}
		}
		requirements.Add(domains)
	}
	return requirements, nil
}

// Register is used to register a domain as available across topologies for the given topology key.
func (t *Topology) Register(topologyKey string, domain string) {
	for _, topology := range t.topologies {
		if topology.Key == topologyKey {
			topology.Register(domain)
		}
	}
	for _, topology := range t.inverseTopologies {
		if topology.Key == topologyKey {
			topology.Register(domain)
		}
	}
}

// Unregister is used to unregister a domain as available across topologies for the given topology key.
func (t *Topology) Unregister(topologyKey string, domain string) {
	for _, topology := range t.topologies {
		if topology.Key == topologyKey {
			topology.Unregister(domain)
		}
	}
	for _, topology := range t.inverseTopologies {
		if topology.Key == topologyKey {
			topology.Unregister(domain)
		}
	}
}

// updateInverseAffinities is used to identify pods with anti-affinity terms so we can track those topologies.  We
// have to look at every pod in the cluster as there is no way to query for a pod with anti-affinity terms.
func (t *Topology) updateInverseAffinities(ctx context.Context) error {
	var errs error
	t.cluster.ForPodsWithAntiAffinity(func(pod *corev1.Pod, node *corev1.Node) bool {
		// don't count the pod we are excluding
		if t.excludedPods.Has(string(pod.UID)) {
			return true
		}
		if err := t.updateInverseAntiAffinity(ctx, pod, node.Labels); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("tracking existing pod anti-affinity, %w", err))
		}
		return true
	})
	return errs
}

// updateInverseAntiAffinity is used to track topologies of inverse anti-affinities. Here the domains & counts track the
// pods with the anti-affinity.
func (t *Topology) updateInverseAntiAffinity(ctx context.Context, pod *corev1.Pod, domains map[string]string) error {
	// We intentionally don't track inverse anti-affinity preferences. We're not
	// required to enforce them so it just adds complexity for very little
	// value.  The problem with them comes from the relaxation process, the pod
	// we are relaxing is not the pod with the anti-affinity term.
	for _, term := range pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		namespaces, err := t.buildNamespaceList(ctx, pod.Namespace, term.Namespaces, term.NamespaceSelector)
		if err != nil {
			return err
		}

		tg := NewTopologyGroup(TopologyTypePodAntiAffinity, term.TopologyKey, pod, namespaces, term.LabelSelector, math.MaxInt32, nil, t.domains[term.TopologyKey])

		hash := tg.Hash()
		if existing, ok := t.inverseTopologies[hash]; !ok {
			t.inverseTopologies[hash] = tg
		} else {
			tg = existing
		}
		if domain, ok := domains[tg.Key]; ok {
			tg.Record(domain)
		}
		tg.AddOwner(pod.UID)
	}
	return nil
}

// countDomains initializes the topology group by registereding any well known domains and performing pod counts
// against the cluster for any existing pods.
//
//nolint:gocyclo
func (t *Topology) countDomains(ctx context.Context, tg *TopologyGroup) error {
	podList := &corev1.PodList{}

	// collect the pods from all the specified namespaces (don't see a way to query multiple namespaces
	// simultaneously)
	var pods []corev1.Pod
	for _, ns := range tg.namespaces.UnsortedList() {
		if err := t.kubeClient.List(ctx, podList, TopologyListOptions(ns, tg.rawSelector)); err != nil {
			return fmt.Errorf("listing pods, %w", err)
		}
		pods = append(pods, podList.Items...)
	}

	for i, p := range pods {
		if IgnoredForTopology(&pods[i]) {
			continue
		}
		// pod is excluded for counting purposes
		if t.excludedPods.Has(string(p.UID)) {
			continue
		}
		node := &corev1.Node{}
		if err := t.kubeClient.Get(ctx, types.NamespacedName{Name: p.Spec.NodeName}, node); err != nil {
			// Pods that cannot be evicted can be leaked in the API Server after
			// a Node is removed. Since pod bindings are immutable, these pods
			// cannot be recovered, and will be deleted by the pod lifecycle
			// garbage collector. These pods are not running, and should not
			// impact future topology calculations.
			if errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("getting node %s, %w", p.Spec.NodeName, err)
		}
		domain, ok := node.Labels[tg.Key]
		// Kubelet sets the hostname label, but the node may not be ready yet so there is no label.  We fall back and just
		// treat the node name as the label.  It probably is in most cases, but even if not we at least count the existence
		// of the pods in some domain, even if not in the correct one.  This is needed to handle the case of pods with
		// self-affinity only fulfilling that affinity if all domains are empty.
		if !ok && tg.Key == corev1.LabelHostname {
			domain = node.Name
			ok = true
		}
		if !ok {
			continue // Don't include pods if node doesn't contain domain https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/#conventions
		}
		// nodes may or may not be considered for counting purposes for topology spread constraints depending on if they
		// are selected by the pod's node selectors and required node affinities.  If these are unset, the node always counts.
		if !tg.nodeFilter.Matches(node) {
			continue
		}
		tg.Record(domain)
	}
	return nil
}

func (t *Topology) newForTopologies(p *corev1.Pod) []*TopologyGroup {
	var topologyGroups []*TopologyGroup
	for _, cs := range p.Spec.TopologySpreadConstraints {
		topologyGroups = append(topologyGroups, NewTopologyGroup(TopologyTypeSpread, cs.TopologyKey, p, sets.New(p.Namespace), cs.LabelSelector, cs.MaxSkew, cs.MinDomains, t.domains[cs.TopologyKey]))
	}
	return topologyGroups
}

// newForAffinities returns a list of topology groups that have been constructed based on the input pod and required/preferred affinity terms
func (t *Topology) newForAffinities(ctx context.Context, p *corev1.Pod) ([]*TopologyGroup, error) {
	var topologyGroups []*TopologyGroup
	// No affinity defined
	if p.Spec.Affinity == nil {
		return topologyGroups, nil
	}
	affinityTerms := map[TopologyType][]corev1.PodAffinityTerm{}

	// include both soft and hard affinity terms
	if p.Spec.Affinity.PodAffinity != nil {
		affinityTerms[TopologyTypePodAffinity] = append(affinityTerms[TopologyTypePodAffinity], p.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution...)
		for _, term := range p.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			affinityTerms[TopologyTypePodAffinity] = append(affinityTerms[TopologyTypePodAffinity], term.PodAffinityTerm)
		}
	}

	// include both soft and hard antiaffinity terms
	if p.Spec.Affinity.PodAntiAffinity != nil {
		affinityTerms[TopologyTypePodAntiAffinity] = append(affinityTerms[TopologyTypePodAntiAffinity], p.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution...)
		for _, term := range p.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			affinityTerms[TopologyTypePodAntiAffinity] = append(affinityTerms[TopologyTypePodAntiAffinity], term.PodAffinityTerm)
		}
	}

	// build topologies
	for topologyType, terms := range affinityTerms {
		for _, term := range terms {
			namespaces, err := t.buildNamespaceList(ctx, p.Namespace, term.Namespaces, term.NamespaceSelector)
			if err != nil {
				return nil, err
			}
			topologyGroups = append(topologyGroups, NewTopologyGroup(topologyType, term.TopologyKey, p, namespaces, term.LabelSelector, math.MaxInt32, nil, t.domains[term.TopologyKey]))
		}
	}
	return topologyGroups, nil
}

// buildNamespaceList constructs a unique list of namespaces consisting of the pod's namespace and the optional list of
// namespaces and those selected by the namespace selector
func (t *Topology) buildNamespaceList(ctx context.Context, namespace string, namespaces []string, selector *metav1.LabelSelector) (sets.Set[string], error) {
	if len(namespaces) == 0 && selector == nil {
		return sets.New(namespace), nil
	}
	if selector == nil {
		return sets.New(namespaces...), nil
	}
	var namespaceList corev1.NamespaceList
	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("parsing selector, %w", err)
	}
	if err := t.kubeClient.List(ctx, &namespaceList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return nil, fmt.Errorf("listing namespaces, %w", err)
	}
	selected := sets.New[string]()
	for _, namespace := range namespaceList.Items {
		selected.Insert(namespace.Name)
	}
	selected.Insert(namespaces...)
	return selected, nil
}

// getMatchingTopologies returns a sorted list of topologies that either control the scheduling of pod p, or for which
// the topology selects pod p and the scheduling of p affects the count per topology domain
func (t *Topology) getMatchingTopologies(p *corev1.Pod, requirements scheduling.Requirements, compatabilityOptions ...option.Function[scheduling.CompatibilityOptions]) []*TopologyGroup {
	var matchingTopologies []*TopologyGroup
	for _, tc := range t.topologies {
		if tc.IsOwnedBy(p.UID) {
			matchingTopologies = append(matchingTopologies, tc)
		}
	}
	for _, tc := range t.inverseTopologies {
		if tc.Counts(p, requirements, compatabilityOptions...) {
			matchingTopologies = append(matchingTopologies, tc)
		}
	}
	return matchingTopologies
}

func TopologyListOptions(namespace string, labelSelector *metav1.LabelSelector) *client.ListOptions {
	selector := labels.Everything()
	if labelSelector == nil {
		return &client.ListOptions{Namespace: namespace, LabelSelector: selector}
	}
	for key, value := range labelSelector.MatchLabels {
		requirement, err := labels.NewRequirement(key, selection.Equals, []string{value})
		if err != nil {
			return &client.ListOptions{Namespace: namespace, LabelSelector: labels.Nothing()}
		}
		selector = selector.Add(*requirement)
	}
	for _, expression := range labelSelector.MatchExpressions {
		requirement, err := labels.NewRequirement(expression.Key, mapOperator(expression.Operator), expression.Values)
		if err != nil {
			return &client.ListOptions{Namespace: namespace, LabelSelector: labels.Nothing()}
		}
		selector = selector.Add(*requirement)
	}
	return &client.ListOptions{Namespace: namespace, LabelSelector: selector}
}

func mapOperator(operator metav1.LabelSelectorOperator) selection.Operator {
	switch operator {
	case metav1.LabelSelectorOpIn:
		return selection.In
	case metav1.LabelSelectorOpNotIn:
		return selection.NotIn
	case metav1.LabelSelectorOpExists:
		return selection.Exists
	case metav1.LabelSelectorOpDoesNotExist:
		return selection.DoesNotExist
	}
	// this shouldn't occur as we cover all valid cases of LabelSelectorOperator that the API allows.  If it still
	// does occur somehow we'll panic just later when the requirement throws an error.,
	return ""
}

func IgnoredForTopology(p *corev1.Pod) bool {
	return !pod.IsScheduled(p) || pod.IsTerminal(p) || pod.IsTerminating(p)
}
