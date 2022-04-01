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
	"sort"

	"github.com/mitchellh/hashstructure/v2"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/pod"
	"github.com/aws/karpenter/pkg/utils/resources"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

type NodeSet struct {
	kubeClient      client.Client
	daemonResources v1.ResourceList
	nodes           []*Node
	topologies      map[uint64]*Topology
	// Anti-affinity works both ways (if a zone has a pod foo with anti-affinity to a pod bar, we can't schedule bar to
	// that zone, even though bar has no anti affinity terms on it. For this to work, we need to separately track the
	// topologies of pods with anti-affinity terms, so we can prevent scheduling the pods they have anti-affinity to
	// in some cases.
	inverseAntiAffinityTopologies map[uint64]*Topology
	constraints                   *v1alpha5.Constraints
	instanceTypes                 []cloudprovider.InstanceType
}

func NewNodeSet(ctx context.Context, constraints *v1alpha5.Constraints,
	instanceTypes []cloudprovider.InstanceType, client client.Client) (*NodeSet, error) {
	ns := &NodeSet{
		kubeClient:                    client,
		constraints:                   constraints,
		instanceTypes:                 instanceTypes,
		topologies:                    map[uint64]*Topology{},
		inverseAntiAffinityTopologies: map[uint64]*Topology{},
	}

	daemons, err := ns.getDaemons(ctx, constraints)
	if err != nil {
		return nil, err
	}

	for _, d := range daemons {
		// skip any daemons that our provisioner configured taints would cause to not schedule
		if err := constraints.Taints.Tolerates(d); err != nil {
			continue
		}
		// or that aren't compatible with provisioner requirements
		if err := constraints.Requirements.Compatible(v1alpha5.NewPodRequirements(d)); err != nil {
			continue
		}
		ns.daemonResources = resources.Merge(ns.daemonResources, resources.RequestsForPods(d))
	}
	return ns, nil
}

func (s *NodeSet) getDaemons(ctx context.Context, constraints *v1alpha5.Constraints) ([]*v1.Pod, error) {
	daemonSetList := &appsv1.DaemonSetList{}
	if err := s.kubeClient.List(ctx, daemonSetList); err != nil {
		return nil, fmt.Errorf("listing daemonsets, %w", err)
	}
	// Include DaemonSets that will schedule on this node
	var pods []*v1.Pod
	for _, daemonSet := range daemonSetList.Items {
		p := &v1.Pod{Spec: daemonSet.Spec.Template.Spec}
		if err := constraints.ValidatePod(p); err == nil {
			pods = append(pods, p)
		}
	}
	return pods, nil
}

func (s *NodeSet) Add(node *Node) {
	s.nodes = append(s.nodes, node)
}

func (s *NodeSet) Schedule(ctx context.Context, pod *v1.Pod) error {
	// copy the pod as this method modifies the pod's node selectors to force it to meet topology constraints
	pod = pod.DeepCopy()

	var possibleNodes []*Node
	for _, node := range s.nodes {
		if err := node.Compatible(ctx, pod); err == nil {
			possibleNodes = append(possibleNodes, node)
		}
	}

	tightened := s.constraints.Requirements.Add(v1alpha5.NewPodRequirements(pod).Requirements...)
	var err error
	possibleNodes, err = s.filterByTopologies(ctx, tightened, pod, possibleNodes)
	if err != nil {
		return err
	}

	if len(possibleNodes) > 0 {
		sort.Slice(possibleNodes, byPreferredNode(possibleNodes))
		if err := possibleNodes[0].Add(ctx, pod); err != nil {
			return fmt.Errorf("adding pod to node, %w", err)
		}
	} else {
		n := NewNode(s.constraints, s.daemonResources, s.instanceTypes)
		if err := n.Add(ctx, pod); err != nil {
			return fmt.Errorf("adding pod to node, %w", err)
		}
		s.nodes = append(s.nodes, n)
	}

	if err := s.recordTopologyDecisions(pod); err != nil {
		logging.FromContext(ctx).Errorf("recording topology decision, %s", err)
	}

	return nil
}

func (s *NodeSet) TrackTopologies(ctx context.Context, pods []*v1.Pod) error {
	var errs error
	errs = multierr.Append(errs, s.trackExistingAntiAffinities(ctx))
	for _, p := range pods {
		// need to first ensure that we know all of the topology constraints.  This may require looking up
		// existing pods in the running cluster to determine zonal topology skew.
		if err := s.trackTopologySpread(ctx, p); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("tracking topology spread, %w", err))
		}
		if err := s.trackPodAffinityTopology(ctx, p); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("tracking affinity topology, %w", err))
		}
	}
	return errs
}

// trackExistingAntiAffinities is used to identify pods with anti-affinity terms so we can track those topologies.  We
// have to look at every pod in the cluster as there is no way to query for a pod with anti-affinity terms.
func (s *NodeSet) trackExistingAntiAffinities(ctx context.Context) error {
	// TODO: if can we determine somehow that the LimitPodHardAntiAffinityTopology admission controller is in place, we
	// don't need to do any of this.  In that case anti-affinity would be hostname only which makes this tracking un-needed.
	var nodeList v1.NodeList
	if err := s.kubeClient.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("listing nodes, %w", err)
	}

	for _, node := range nodeList.Items {
		var podlist v1.PodList
		if err := s.kubeClient.List(ctx, &podlist, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
			return fmt.Errorf("listing pods on node %s, %w", node.Name, err)
		}
		for _, p := range podlist.Items {
			// lint warning is about taking the address of a variable in the for loop, it would matter if
			// HasRequiredPodAntiAffinity modified p, but it doesn't
			//nolint:gosec
			if pod.HasRequiredPodAntiAffinity(&p) {
				//nolint:gosec
				if err := s.trackExistingPodAntiAffinityTopology(ctx, &node, &p); err != nil {
					return fmt.Errorf("tracking existing pod anti-affinity, %w", err)
				}
			}
		}
	}
	return nil
}

func (s *NodeSet) trackTopologySpread(ctx context.Context, p *v1.Pod) error {
	for _, cs := range p.Spec.TopologySpreadConstraints {
		// constraints apply within a single namespace so we have to ensure we include the namespace in our key
		key := struct {
			Namespace string
			Cs        v1.TopologySpreadConstraint
		}{p.Namespace, cs}

		hash, err := hashstructure.Hash(key, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
		if err != nil {
			return fmt.Errorf("hashing topology constraint: %w", err)
		}
		topology, found := s.topologies[hash]
		if !found {
			topology, err = s.buildAndUpdateTopology(ctx, cs, p.Namespace)
			if err != nil {
				return err
			}
			s.topologies[hash] = topology
		}
		topology.AddOwner(client.ObjectKeyFromObject(p))
	}
	return nil
}

func (s *NodeSet) trackExistingPodAntiAffinityTopology(ctx context.Context, node *v1.Node, p *v1.Pod) error {
	type affinityKey struct {
		Preferred     bool
		Weight        int32
		AntiAffinity  bool
		TopologyKey   string
		Namespaces    []string
		LabelSelector *metav1.LabelSelector
	}

	for _, v := range p.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		key := affinityKey{
			TopologyKey:   v.TopologyKey,
			AntiAffinity:  true,
			Namespaces:    buildNamespaceList(ctx, s.kubeClient, p, v.Namespaces, v.NamespaceSelector),
			LabelSelector: v.LabelSelector}

		hash, err := hashstructure.Hash(key, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
		if err != nil {
			return fmt.Errorf("hashing topology constraint: %w", err)
		}
		tsc, found := s.inverseAntiAffinityTopologies[hash]
		if !found {
			var cs v1.TopologySpreadConstraint
			cs.TopologyKey = key.TopologyKey
			cs.LabelSelector = key.LabelSelector
			cs.WhenUnsatisfiable = v1.DoNotSchedule
			cs.MaxSkew = math.MaxInt32

			topology, err := s.buildAndUpdateTopology(ctx, cs, key.Namespaces...)
			if err != nil {
				return err
			}
			topology.Type = TopologyTypePodAntiAffinity
			tsc = topology
			s.inverseAntiAffinityTopologies[hash] = topology
		}
		if node != nil {
			tsc.RecordUsage(node.Labels[tsc.Key])
		}
		tsc.AddOwner(client.ObjectKeyFromObject(p))
	}
	return nil
}

func (s *NodeSet) buildAndUpdateTopology(ctx context.Context, cs v1.TopologySpreadConstraint, namespaces ...string) (*Topology, error) {
	topology, err := NewTopology(cs, namespaces)
	if err != nil {
		return nil, err
	}
	if topology.Key == v1.LabelTopologyZone {
		// we know about the zones that we can schedule to regardless of if nodes exist there
		for _, zone := range s.constraints.Requirements.Zones().List() {
			topology.RegisterDomain(zone)
		}
	} else if topology.Key == v1alpha5.LabelCapacityType {
		for _, ct := range s.constraints.Requirements.CapacityTypes().List() {
			topology.RegisterDomain(ct)
		}
	}
	// get existing zone spread information
	if err := topology.UpdateFromCluster(ctx, s.kubeClient); err != nil {
		logging.FromContext(ctx).Errorf("unable to get existing %s topology information, %s", topology.Key, err)
	}
	return topology, nil
}

//gocyclo:ignore
func (s *NodeSet) trackPodAffinityTopology(ctx context.Context, p *v1.Pod) error {
	type affinityKey struct {
		Preferred     bool
		Weight        int32
		AntiAffinity  bool
		TopologyKey   string
		Namespaces    []string
		LabelSelector *metav1.LabelSelector
	}

	var keys []affinityKey
	if pod.HasPodAffinity(p) {
		for _, v := range p.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			key := affinityKey{
				TopologyKey:   v.TopologyKey,
				Namespaces:    buildNamespaceList(ctx, s.kubeClient, p, v.Namespaces, v.NamespaceSelector),
				LabelSelector: v.LabelSelector}
			keys = append(keys, key)
		}
		for _, v := range p.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			key := affinityKey{
				Preferred:     true,
				Weight:        v.Weight,
				TopologyKey:   v.PodAffinityTerm.TopologyKey,
				Namespaces:    buildNamespaceList(ctx, s.kubeClient, p, v.PodAffinityTerm.Namespaces, v.PodAffinityTerm.NamespaceSelector),
				LabelSelector: v.PodAffinityTerm.LabelSelector}
			keys = append(keys, key)
		}
	}

	if pod.HasPodAntiAffinity(p) {
		if err := s.trackExistingPodAntiAffinityTopology(ctx, nil, p); err != nil {
			return err
		}

		for _, v := range p.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			key := affinityKey{
				AntiAffinity:  true,
				TopologyKey:   v.TopologyKey,
				Namespaces:    buildNamespaceList(ctx, s.kubeClient, p, v.Namespaces, v.NamespaceSelector),
				LabelSelector: v.LabelSelector}
			keys = append(keys, key)
		}
		for _, v := range p.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			key := affinityKey{
				AntiAffinity:  true,
				Preferred:     true,
				Weight:        v.Weight,
				TopologyKey:   v.PodAffinityTerm.TopologyKey,
				Namespaces:    buildNamespaceList(ctx, s.kubeClient, p, v.PodAffinityTerm.Namespaces, v.PodAffinityTerm.NamespaceSelector),
				LabelSelector: v.PodAffinityTerm.LabelSelector}
			keys = append(keys, key)
		}
	}

	for _, key := range keys {
		hash, err := hashstructure.Hash(key, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
		if err != nil {
			return fmt.Errorf("hashing topology constraint: %w", err)
		}
		tsc, found := s.topologies[hash]
		if !found {
			var cs v1.TopologySpreadConstraint
			cs.TopologyKey = key.TopologyKey
			cs.LabelSelector = key.LabelSelector
			if key.Preferred {
				cs.WhenUnsatisfiable = v1.ScheduleAnyway
			} else {
				cs.WhenUnsatisfiable = v1.DoNotSchedule
			}

			// MaxSkew is unused by the affinity constraint itself, but it's useful to set it to a large value.  This
			// is because we have to commit to topology decisions for the target pods of a pod-affinity.  If nothing
			// else affects it, we don't really care how many of these pods end up per domain.  We only care about what
			// the affinity owner does in relation to those pods and the domain.
			cs.MaxSkew = math.MaxInt32

			topology, err := s.buildAndUpdateTopology(ctx, cs, key.Namespaces...)
			if err != nil {
				return err
			}
			if key.AntiAffinity {
				topology.Type = TopologyTypePodAntiAffinity
			} else {
				topology.Type = TopologyTypePodAffinity
			}
			topology.Weight = key.Weight

			tsc = topology
			s.topologies[hash] = topology
		}
		tsc.AddOwner(client.ObjectKeyFromObject(p))
	}

	return nil
}

// buildNamespaceList constructs a unique list of namespaces consisting of the pod's namespace and the optional list of
// namespaces and those selected by the namespace selector
func buildNamespaceList(ctx context.Context, c client.Client, p *v1.Pod, namespaces []string, selector *metav1.LabelSelector) []string {
	uniq := map[string]struct{}{}
	uniq[p.Namespace] = struct{}{}

	for _, n := range namespaces {
		uniq[n] = struct{}{}
	}

	logger := logging.FromContext(ctx).With("pod", client.ObjectKeyFromObject(p))
	if selector != nil {
		var namespaceList v1.NamespaceList
		lsel, err := metav1.LabelSelectorAsSelector(selector)
		if err != nil {
			logger.Errorf("constructing namespace label selector: %s", err)
			return nil
		}

		if err := c.List(context.Background(), &namespaceList, &client.ListOptions{LabelSelector: lsel}); err != nil {
			logger.Errorf("fetching namespace list: %s", err)
			return nil
		}
		for _, v := range namespaceList.Items {
			uniq[v.Name] = struct{}{}
		}
	}

	uniqNamespaces := make([]string, 0, len(uniq))
	for k := range uniq {
		uniqNamespaces = append(uniqNamespaces, k)
	}
	return uniqNamespaces
}

func (s *NodeSet) recordTopologyDecisions(p *v1.Pod) error {
	// once we've now committed to a domain, we record the usage in every topology that cares about it
	var err error
	podLabels := labels.Set(p.Labels)
	for _, tc := range s.topologies {
		if tc.Matches(p.Namespace, podLabels) {
			domain, ok := p.Spec.NodeSelector[tc.Key]
			if !ok || domain == "" {
				err = multierr.Append(err, fmt.Errorf("empty or missing domain for topology key %s", tc.Key))
			} else {
				tc.RecordUsage(domain)
			}
		}
	}

	// for anti-affinities, we need to also record where the pods with the anti-affinity are
	key := client.ObjectKeyFromObject(p)
	for _, tc := range s.inverseAntiAffinityTopologies {
		if tc.IsOwnedBy(key) {
			domain, ok := p.Spec.NodeSelector[tc.Key]
			if !ok || domain == "" {
				err = multierr.Append(err, fmt.Errorf("empty or missing domain for topology key %s", tc.Key))
			} else {
				tc.RecordUsage(domain)
			}
		}
	}
	return err
}

type matchingTopology struct {
	topology              *Topology
	controlsPodScheduling bool
	selectsPod            bool
}

// filterByTopologies filters the list of all compatible nodes returning the list of compatible nodes that also satisfy
// topology constraints (topology spread, pod affinity & pod anti-affinity) and an error.  If the list of returned nodes
// is empty, then the constraints can't be satisfied given the existing nodes and a new node must be created. If error
// is non-nil then the topology constraints can't be satisfied even by creating a new node and the pod is unschedulable.
//gocyclo:ignore
func (s *NodeSet) filterByTopologies(ctx context.Context, tightened v1alpha5.Requirements, p *v1.Pod, nodes []*Node) ([]*Node, error) {
	if p.Spec.NodeSelector == nil {
		p.Spec.NodeSelector = map[string]string{}
	}
	var err error
	for _, match := range s.getMatchingTopologies(p) {

		topology := match.topology
		// tighten requirements based on any existing pod node selector for the topology key as well as trying to require
		// that the pod land on an existing node. If this fails, we can always create a new node.
		tightened = match.tightenByPodAndCompatibleNodes(tightened, p, nodes, topology)
		if err != nil {
			return nil, err
		}

		// possibly select a domain for the topology key if the pod is selected by the topology, but not controlled
		// by the topology
		nodes, err = match.applyNodeSelectorIfNotControlled(ctx, tightened, p, nodes, topology)
		if err != nil {
			return nil, err
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
			nextDomain, maxSkew, increasingSkew = topology.NextDomainMinimizeSkew(topology.Key == v1.LabelHostname, tightened)
			if maxSkew > topology.MaxSkew && increasingSkew && !topology.AllowViolation {
				if topology.Key == v1.LabelHostname {
					// we can always create a new hostname, so instead of refusing to schedule, create a new node
					nextDomain = ""
				} else {
					topology.NextDomainMinimizeSkew(topology.Key == v1.LabelHostname, tightened)
					return nil, fmt.Errorf("would violate max-skew for topology key %s", topology.Key)
				}
			}
		case TopologyTypePodAffinity:
			nextDomain = topology.MaxNonZeroDomain(tightened)
			// we don't have a valid domain, but it's pod affinity and the pod itself will satisfy the topology
			// constraint, so check for any domain
			if nextDomain == "" && selfSelectingPodAffinity {
				nextDomain = topology.AnyDomain(tightened)
			}
		case TopologyTypePodAntiAffinity:
			nextDomain = topology.EmptyDomain(tightened)
		}

		if nextDomain == "" {
			// For topology spread and anti-affinity, we can always create a new hostname to satisfy the constraint. For
			// a self-selecting pod affinity, the pod itself satisfies the affinity.
			if topology.Key == v1.LabelHostname && (topology.Type == TopologyTypeSpread || topology.Type == TopologyTypePodAntiAffinity ||
				selfSelectingPodAffinity) {
				nodes = nil
				continue
			}
			// the affinity is only a preference so we don't mind violating it
			if topology.AllowViolation {
				continue
			}

			return nil, fmt.Errorf("unsatisfiable %s topology constraint for key %s", topology.Type, topology.Key)
		}

		p.Spec.NodeSelector[topology.Key] = nextDomain
		nodes = filterCompatibleNodes(ctx, nodes, p)
	}

	return nodes, nil
}

// tightenByPodAndCompatibleNodes tightens the requirements by adding any node selectors that may already exist on the pod and adding a node
// selector for hostname that limits the pod to landing on any from the list of compatible nodes
func (match *matchingTopology) tightenByPodAndCompatibleNodes(tightened v1alpha5.Requirements, p *v1.Pod, nodes []*Node, topology *Topology) v1alpha5.Requirements {
	// the pod already has a key defined for this domain, so we add it to our tightened requirements to limit what
	// topology domain selection can return
	if existingDomain, ok := p.Spec.NodeSelector[topology.Key]; ok {
		tightened = tightened.Add(v1.NodeSelectorRequirement{
			Key:      topology.Key,
			Operator: v1.NodeSelectorOpIn,
			Values:   []string{existingDomain},
		})
	}

	// To be able to re-use nodes that we have already created with hostname based topologies, we need to ensure
	// that our topology domain selection is both aware of the valid domains it can choose from which are the
	// compatible nodes and filters down via the tightened requirements to select from only those valid domains. If this
	// causes no domain to be selectable, we can always create a new node to create a new domain.
	if topology.Key == v1.LabelHostname {
		var validNodeNames []string
		for _, n := range nodes {
			topology.RegisterDomain(n.Name)
			validNodeNames = append(validNodeNames, n.Name)
		}
		tightened = tightened.Add(v1.NodeSelectorRequirement{
			Key:      v1.LabelHostname,
			Operator: v1.NodeSelectorOpIn,
			Values:   validNodeNames,
		})
	}
	return tightened
}

// applyNodeSelectorIfNotControlled adds a node selector to the pod if the pod isn't controlled by the topology, but is
// selected by the topology. If the pod doesn't have a domain defined for the topology key yet, we need to select one
// so that some future pod can know where to land as well.
func (match *matchingTopology) applyNodeSelectorIfNotControlled(ctx context.Context, tightened v1alpha5.Requirements, p *v1.Pod, nodes []*Node, topology *Topology) ([]*Node, error) {
	// if the topology key is already set, we can't change it
	_, topologyAlreadySet := p.Spec.NodeSelector[topology.Key]
	if topologyAlreadySet || !match.selectsPod || match.controlsPodScheduling {
		return nodes, nil
	}

	// Some other pod is selecting based on how this pod is scheduled, but this particular pod is not itself
	// controlled by the current topology constraint.
	var nextDomain string
	if topology.Type == TopologyTypePodAntiAffinity {
		// pod anti-affinity is bidirectional so we need to select an empty domain for this pod to land in, if it doesn't
		// exist, it's unschedulable
		nextDomain = topology.EmptyDomain(tightened)
	} else {
		nextDomain, _, _ = topology.NextDomainMinimizeSkew(topology.Key == v1.LabelHostname, tightened)
	}

	if nextDomain == "" {
		// no valid hostname domain, but we can always generate a new one, return no nodes as we know none are compatible
		if topology.Key == v1.LabelHostname {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to determine valid topology key for domain %s", topology.Key)
	}

	// whenever we add a node selector for a topology constraint, we have to re-filter out list of compatible
	// nodes as we may have just excluded some and don't want to pick from them with a future topology
	// selector.  Suppose we had nodes in zone-a and zone-b, and just selected zone-b as our domain.  We need
	// to ensure that the list of compatible nodes only contains nodes in zone-b.
	p.Spec.NodeSelector[topology.Key] = nextDomain
	nodes = filterCompatibleNodes(ctx, nodes, p)
	return nodes, nil
}

func filterCompatibleNodes(ctx context.Context, nodes []*Node, p *v1.Pod) []*Node {
	var possibleNodes []*Node
	for _, n := range nodes {
		if err := n.Compatible(ctx, p); err == nil {
			possibleNodes = append(possibleNodes, n)
		}
	}
	return possibleNodes
}

// getMatchingTopologies returns a sorted list of topologies that either control the scheduling of pod p, or for which
// the topology selects pod p and the scheduling of p affects the count per topology domain
func (s *NodeSet) getMatchingTopologies(p *v1.Pod) []matchingTopology {
	var matchingTopologies []matchingTopology
	for _, tc := range s.topologies {
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
	for _, tc := range s.inverseAntiAffinityTopologies {
		if tc.Matches(p.Namespace, p.Labels) {
			matchingTopologies = append(matchingTopologies, matchingTopology{
				topology:              tc,
				controlsPodScheduling: true,
				selectsPod:            true,
			})
		}
	}
	sort.Slice(matchingTopologies, byTopologyPriority(matchingTopologies))
	return matchingTopologies
}

// byTopologyPriority returns a comparator for topologies which prioritizes solving for topology constraints in order of
// those that control a pod's scheduling, followed by those that can't be violated followed by any remaining in order
// of weight
func byTopologyPriority(matchingTopologies []matchingTopology) func(a int, b int) bool {
	return func(a, b int) bool {
		lhs := matchingTopologies[a]
		rhs := matchingTopologies[b]

		// topologies that control this pods scheduling are applied first
		if lhs.controlsPodScheduling != rhs.controlsPodScheduling {
			return lhs.controlsPodScheduling
		}

		// then topologies that we can't violate
		if lhs.topology.AllowViolation != rhs.topology.AllowViolation {
			return !lhs.topology.AllowViolation
		}
		// Apply any non-hostname topologies before any hostname topology derived constraints.  Picking a hostname
		// effectively fixes every other topology key as well (e.g. zone or capacity-type) so we want to apply the
		// other constraints first.
		if lhs.topology.Key != rhs.topology.Key {
			if lhs.topology.Key == v1.LabelHostname {
				return false
			}
			if rhs.topology.Key == v1.LabelHostname {
				return true
			}
		}
		// descending order by weight
		return lhs.topology.Weight > rhs.topology.Weight
	}
}

// byPreferredNode sorts nodes by preference so we prefer to schedule on nodes that have fewer pods.
func byPreferredNode(nodes []*Node) func(i int, j int) bool {
	return func(i, j int) bool {
		lhs := nodes[i]
		rhs := nodes[j]
		return len(lhs.Pods) < len(rhs.Pods)
	}
}
