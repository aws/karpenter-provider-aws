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

	"github.com/aws/karpenter/pkg/utils/apiobject"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/pod"
	"github.com/aws/karpenter/pkg/utils/sets"
)

type Topology struct {
	kubeClient client.Client
	topologies map[uint64]*TopologyGroup
	// Anti-affinity works both ways (if a zone has a pod foo with anti-affinity to a pod bar, we can't schedule bar to
	// that zone, even though bar has no anti affinity terms on it. For this to work, we need to separately track the
	// topologies of pods with anti-affinity terms, so we can prevent scheduling the pods they have anti-affinity to
	// in some cases.
	inverseTopologies map[uint64]*TopologyGroup
	constraints       *v1alpha5.Constraints
}

func NewTopology(kubeClient client.Client, constraints *v1alpha5.Constraints) *Topology {
	return &Topology{
		kubeClient:        kubeClient,
		constraints:       constraints,
		topologies:        map[uint64]*TopologyGroup{},
		inverseTopologies: map[uint64]*TopologyGroup{},
	}
}

// Update scans the pods provided and creates topology groups for any topologies that we need to track based off of
// topology spreads, affinities, and anti-affinities specified in the pods.
func (t *Topology) Update(ctx context.Context, pods ...*v1.Pod) error {
	var errs error
	errs = multierr.Append(errs, t.trackExistingAntiAffinities(ctx))
	for _, p := range pods {
		// need to first ensure that we know all of the topology constraints.  This may require looking up
		// existing pods in the running cluster to determine zonal topology skew.
		if err := t.trackTopologySpread(ctx, p); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("tracking topology spread, %w", err))
		}
		if err := t.trackPodAffinityTopology(ctx, p); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("tracking affinity topology, %w", err))
		}
	}
	return errs
}

// trackExistingAntiAffinities is used to identify pods with anti-affinity terms so we can track those topologies.  We
// have to look at every pod in the cluster as there is no way to query for a pod with anti-affinity terms.
func (t *Topology) trackExistingAntiAffinities(ctx context.Context) error {
	// TODO: if can we determine somehow that the LimitPodHardAntiAffinityTopology admission controller is in place, we
	// don't need to do any of this.  In that case anti-affinity would be hostname only which makes this tracking un-needed.
	var nodeList v1.NodeList
	if err := t.kubeClient.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("listing nodes, %w", err)
	}

	for _, node := range nodeList.Items {
		var podlist v1.PodList
		if err := t.kubeClient.List(ctx, &podlist, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
			return fmt.Errorf("listing pods on node %s, %w", node.Name, err)
		}
		for _, p := range podlist.Items {
			// lint warning is about taking the address of a variable in the for loop, it would matter if
			// HasRequiredPodAntiAffinity modified p, but it doesn't
			//nolint:gosec
			if pod.HasRequiredPodAntiAffinity(&p) {
				//nolint:gosec
				if err := t.trackInverseAntiAffinity(ctx, &node, &p); err != nil {
					return fmt.Errorf("tracking existing pod anti-affinity, %w", err)
				}
			}
		}
	}
	return nil
}

func (t *Topology) trackTopologySpread(ctx context.Context, p *v1.Pod) error {
	for _, cs := range p.Spec.TopologySpreadConstraints {
		topologyGroup, err := t.newForSpread(p.Namespace, cs)
		if err != nil {
			return err
		}
		// a preferred constraint that may be relaxed
		if cs.WhenUnsatisfiable == v1.ScheduleAnyway {
			topologyGroup.originatorHash = apiobject.MustHash(cs)
			topologyGroup.preferred = true
		}
		hash := topologyGroup.Hash()
		if existing, ok := t.topologies[hash]; !ok {
			if err := t.initializeTopologyGroup(ctx, topologyGroup); err != nil {
				return err
			}
			t.topologies[hash] = topologyGroup
		} else {
			topologyGroup = existing
		}
		topologyGroup.AddOwner(client.ObjectKeyFromObject(p))
	}
	return nil
}

func (t *Topology) newForSpread(namespace string, cs v1.TopologySpreadConstraint) (*TopologyGroup, error) {
	selector, err := metav1.LabelSelectorAsSelector(cs.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("creating label selector: %w", err)
	}
	return &TopologyGroup{
		Key:         cs.TopologyKey,
		MaxSkew:     cs.MaxSkew,
		Type:        TopologyTypeSpread,
		rawSelector: cs.LabelSelector,
		selector:    selector,
		domains:     map[string]int32{},
		namespaces:  sets.NewSet(namespace),
		owners:      map[client.ObjectKey]struct{}{},
	}, nil
}

func (t *Topology) newForAffinity(term v1.PodAffinityTerm, namespaces sets.Set, topoType TopologyType) (*TopologyGroup, error) {
	selector, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("creating label selector: %w", err)
	}

	return &TopologyGroup{
		Key:        term.TopologyKey,
		MaxSkew:    math.MaxInt32,
		Type:       topoType,
		selector:   selector,
		domains:    map[string]int32{},
		namespaces: namespaces,
		owners:     map[client.ObjectKey]struct{}{},
	}, nil
}

// trackInverseAntiAffinity is used to track topologies of inverse anti-affinities. Here the domains & counts track the
// pods with the anti-affinity.
func (t *Topology) trackInverseAntiAffinity(ctx context.Context, node *v1.Node, p *v1.Pod) error {
	// We intentionally don't track inverse anti-affinity preferences.  We're not required to enforce them so it
	// just adds complexity for very little value.
	for _, term := range p.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		namespaces, err := t.buildNamespaceList(ctx, term.Namespaces, term.NamespaceSelector)
		if err != nil {
			return err
		}
		namespaces.Insert(p.Namespace)

		topologyGroup, err := t.newForAffinity(term, namespaces, TopologyTypePodAntiAffinity)
		if err != nil {
			return err
		}

		topologyGroup.isInverse = true
		hash := topologyGroup.Hash()
		if existing, ok := t.inverseTopologies[hash]; !ok {
			t.inverseTopologies[hash] = topologyGroup
			if err := t.initializeTopologyGroup(ctx, topologyGroup); err != nil {
				return err
			}
		} else {
			topologyGroup = existing
		}
		if node != nil {
			topologyGroup.RecordUsage(node.Labels[topologyGroup.Key])
		}
		topologyGroup.AddOwner(client.ObjectKeyFromObject(p))
	}
	return nil
}

func (t *Topology) trackPodAffinityTopology(ctx context.Context, p *v1.Pod) error {
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
		if err := t.trackInverseAntiAffinity(ctx, nil, p); err != nil {
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
		tg.AddOwner(client.ObjectKeyFromObject(p))
	}
	return err
}

// newForAffinities returns a list of topology groups that have been constructed based on the input pod and required/preferred affinity terms
func (t *Topology) newForAffinities(ctx context.Context, p *v1.Pod, required []v1.PodAffinityTerm, preferred []v1.WeightedPodAffinityTerm,
	topoType TopologyType) ([]*TopologyGroup, error) {
	var topologyGroups []*TopologyGroup
	for _, term := range required {
		namespaces, err := t.buildNamespaceList(ctx, term.Namespaces, term.NamespaceSelector)
		if err != nil {
			return nil, err
		}
		namespaces.Insert(p.Namespace)
		topology, err := t.newForAffinity(term, namespaces, topoType)
		if err != nil {
			return nil, err
		}
		topologyGroups = append(topologyGroups, topology)
	}

	for _, term := range preferred {
		namespaces, err := t.buildNamespaceList(ctx, term.PodAffinityTerm.Namespaces, term.PodAffinityTerm.NamespaceSelector)
		if err != nil {
			return nil, err
		}
		namespaces.Insert(p.Namespace)
		topology, err := t.newForAffinity(term.PodAffinityTerm, namespaces, topoType)
		if err != nil {
			return nil, err
		}
		// we only store the originator for preferred terms as those are the only topologies that we may need
		// to stop tracking mid-schedule
		topology.originatorHash = apiobject.MustHash(term)
		topology.preferred = true
		topologyGroups = append(topologyGroups, topology)
	}
	return topologyGroups, nil
}

// initializeTopologyGroup initializes the topology group by registereding any well-domains and performing pod counts
// against the cluster for any existing pods.
func (t *Topology) initializeTopologyGroup(ctx context.Context, tg *TopologyGroup) error {
	tg.initializeWellKnown(t.constraints)
	if tg.isInverse {
		return nil
	}

	podList := &v1.PodList{}
	// collect the pods from all the specified namespaces (don't see a way to query multiple namespaces
	// simultaneously)
	var pods []v1.Pod
	for ns := range tg.namespaces.Values() {
		if err := t.kubeClient.List(ctx, podList, TopologyListOptions(ns, tg.rawSelector)); err != nil {
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
func (t *Topology) buildNamespaceList(ctx context.Context, namespaces []string, selector *metav1.LabelSelector) (sets.Set, error) {
	uniq := sets.NewSet()
	uniq.Insert(namespaces...)
	if selector == nil {
		return uniq, nil
	}
	var namespaceList v1.NamespaceList
	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return sets.NewSet(), err
	}
	if err := t.kubeClient.List(ctx, &namespaceList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return sets.NewSet(), err
	}
	for _, v := range namespaceList.Items {
		uniq.Insert(v.Name)
	}
	return uniq, nil
}

// Record records the topology changes given that pod p schedule on node n
func (t *Topology) Record(p *v1.Pod, requirements v1alpha5.Requirements) error {
	// once we've now committed to a domain, we record the usage in every topology that cares about it
	var err error
	podLabels := labels.Set(p.Labels)
	for _, tc := range t.topologies {
		if tc.Matches(p.Namespace, podLabels) {
			domains := requirements.Get(tc.Key)
			if domains.Len() != 0 {
				if domain := domains.Values().List()[0]; domain != "" {
					tc.RecordUsage(domain)
				}
			}
		}
	}
	// for anti-affinities, we need to also record where the pods with the anti-affinity are
	key := client.ObjectKeyFromObject(p)
	for _, tc := range t.inverseTopologies {
		if tc.IsOwnedBy(key) {
			domains := requirements.Get(tc.Key)
			if domains.Len() != 0 {
				if domain := domains.Values().List()[0]; domain != "" {
					tc.RecordUsage(domain)
				} else {
					err = multierr.Append(err, fmt.Errorf("empty or missing domain for topology key %s", tc.Key))
				}
			} else {
				err = multierr.Append(err, fmt.Errorf("empty or missing domain for topology key %s", tc.Key))
			}
		}
	}
	return err
}

// getMatchingTopologies returns a sorted list of topologies that either control the scheduling of pod p, or for which
// the topology selects pod p and the scheduling of p affects the count per topology domain
func (t *Topology) getMatchingTopologies(p *v1.Pod) []matchingTopology {
	var matchingTopologies []matchingTopology
	for _, tc := range t.topologies {
		controlsPodScheduling := tc.IsOwnedBy(client.ObjectKeyFromObject(p))
		matchesPod := tc.Matches(p.Namespace, p.Labels)
		if controlsPodScheduling || matchesPod {
			matchingTopologies = append(matchingTopologies, matchingTopology{
				topology:              tc,
				controlsPodScheduling: controlsPodScheduling,
				selectsPod:            matchesPod,
			})
		}
	}
	for _, tc := range t.inverseTopologies {
		if tc.Matches(p.Namespace, p.Labels) {
			matchingTopologies = append(matchingTopologies, matchingTopology{
				topology:              tc,
				controlsPodScheduling: true,
				selectsPod:            true,
			})
		}
	}
	return matchingTopologies
}

// Requirements tightens the input requirements by adding additional requirements that are being enforced by topology spreads
// affinities, anti-affinities or inverse anti-affinities.  It returns these newly tightened requirements, or an error in
// the case of a set of requirements that cannot be satisfied.
//gocyclo: ignore
func (t *Topology) Requirements(requirements v1alpha5.Requirements, nodeName string, p *v1.Pod) (v1alpha5.Requirements, error) {
	for _, match := range t.getMatchingTopologies(p) {
		topology := match.topology
		if topology.Key == v1.LabelHostname {
			topology.RegisterDomain(nodeName)
		}

		// There is a pod that wants to avoid this pod. To ensure this occurs, we can't leave the topology domain for
		// this pod flexible (e.g zone in [zone-1, zone-2, zone-3] and have to commit.  If we didn't to this, we would
		// need to pre-scan the batch of pods to determine if the pod A has anti-affinity to pod B relationship exists,
		// and fail scheduling pod A until the next batch cycle.
		// TODO: is this worthwhile?  We already fail to schedule the case pod A has affinity to pod B if pod B is not  fully committed
		if match.selectsPod && !match.controlsPodScheduling && topology.Type == TopologyTypePodAntiAffinity {
			nextDomain, _, _ := topology.NextDomainMinimizeSkew(requirements)
			if nextDomain == "" {
				return v1alpha5.Requirements{}, fmt.Errorf("unsatisfiable %s topology constraint for key %s", topology.Type, topology.Key)
			}
			requirements = requirements.Add(v1.NodeSelectorRequirement{
				Key:      topology.Key,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{nextDomain},
			})
			continue
		}

		if !match.controlsPodScheduling {
			continue
		}

		selfSelectingPodAffinity := topology.Type == TopologyTypePodAffinity &&
			match.selectsPod && match.controlsPodScheduling &&
			!topology.HasNonEmptyDomains()

		var nextDomain string
		var maxSkew int32
		var increasingSkew bool
		switch topology.Type {
		case TopologyTypeSpread:
			nextDomain, maxSkew, increasingSkew = topology.NextDomainMinimizeSkew(requirements)
			if maxSkew > topology.MaxSkew && increasingSkew {
				if topology.Key == v1.LabelHostname {
					// we can always create a new hostname, so instead of refusing to schedule, create a new node
					nextDomain = ""
				} else {
					return v1alpha5.Requirements{}, fmt.Errorf("would violate max-skew for topology key %s", topology.Key)
				}
			}
		case TopologyTypePodAffinity:
			nextDomain = topology.MaxNonZeroDomain(requirements)
			// we don't have a valid domain, but it's pod affinity and the pod itself will satisfy the topology
			// constraint, so check for any domain
			if nextDomain == "" && selfSelectingPodAffinity {
				nextDomain = topology.AnyDomain(requirements)
			}
		case TopologyTypePodAntiAffinity:
			nextDomain = topology.EmptyDomain(requirements)
		}

		if nextDomain == "" {
			return v1alpha5.Requirements{}, fmt.Errorf("unsatisfiable %s topology constraint for key %s", topology.Type, topology.Key)
		}

		requirements = requirements.Add(v1.NodeSelectorRequirement{
			Key:      topology.Key,
			Operator: v1.NodeSelectorOpIn,
			Values:   []string{nextDomain},
		})
	}
	return requirements, nil
}

func (t *Topology) Relax(p *v1.Pod) {
	matching := map[uint64]*TopologyGroup{}
	podKey := client.ObjectKeyFromObject(p)
	for _, topology := range t.topologies {
		if !topology.IsOwnedBy(podKey) || !topology.preferred {
			continue
		}
		matching[topology.originatorHash] = topology
	}

	for _, tsc := range p.Spec.TopologySpreadConstraints {
		if tsc.WhenUnsatisfiable != v1.ScheduleAnyway {
			continue
		}
		delete(matching, apiobject.MustHash(tsc))
	}
	if pod.HasPodAffinity(p) {
		for _, term := range p.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			delete(matching, apiobject.MustHash(term))
		}
	}
	if pod.HasPodAntiAffinity(p) {
		for _, term := range p.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			delete(matching, apiobject.MustHash(term))
		}
	}
	for _, topology := range matching {
		topology.RemoveOwner(podKey)
	}
}
