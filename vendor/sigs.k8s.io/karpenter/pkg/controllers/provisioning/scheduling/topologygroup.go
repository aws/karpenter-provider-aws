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
	"fmt"
	"math"

	"github.com/awslabs/operatorpkg/option"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/karpenter/pkg/scheduling"
)

type TopologyType byte

const (
	TopologyTypeSpread TopologyType = iota
	TopologyTypePodAffinity
	TopologyTypePodAntiAffinity
)

func (t TopologyType) String() string {
	switch t {
	case TopologyTypeSpread:
		return "topology spread"
	case TopologyTypePodAffinity:
		return "pod affinity"
	case TopologyTypePodAntiAffinity:
		return "pod anti-affinity"
	}
	return ""
}

// TopologyGroup is used to track pod counts that match a selector by the topology domain (e.g. SELECT COUNT(*) FROM pods GROUP BY(topology_ke
type TopologyGroup struct {
	// Hashed Fields
	Key         string
	Type        TopologyType
	maxSkew     int32
	minDomains  *int32
	namespaces  sets.Set[string]
	selector    labels.Selector
	rawSelector *metav1.LabelSelector
	nodeFilter  TopologyNodeFilter
	// Index
	owners       map[types.UID]struct{} // Pods that have this topology as a scheduling rule
	domains      map[string]int32       // TODO(ellistarn) explore replacing with a minheap
	emptyDomains sets.Set[string]       // domains for which we know that no pod exists
}

func NewTopologyGroup(topologyType TopologyType, topologyKey string, pod *v1.Pod, namespaces sets.Set[string], labelSelector *metav1.LabelSelector, maxSkew int32, minDomains *int32, domains sets.Set[string]) *TopologyGroup {
	domainCounts := map[string]int32{}
	for domain := range domains {
		domainCounts[domain] = 0
	}
	// the nil *TopologyNodeFilter always passes which is what we need for affinity/anti-affinity
	var nodeSelector TopologyNodeFilter
	if topologyType == TopologyTypeSpread {
		nodeSelector = MakeTopologyNodeFilter(pod)
	}
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		selector = labels.Nothing()
	}
	return &TopologyGroup{
		Type:         topologyType,
		Key:          topologyKey,
		namespaces:   namespaces,
		selector:     selector,
		rawSelector:  labelSelector,
		nodeFilter:   nodeSelector,
		maxSkew:      maxSkew,
		domains:      domainCounts,
		emptyDomains: domains.Clone(),
		owners:       map[types.UID]struct{}{},
		minDomains:   minDomains,
	}
}

func (t *TopologyGroup) Get(pod *v1.Pod, podDomains, nodeDomains *scheduling.Requirement) *scheduling.Requirement {
	switch t.Type {
	case TopologyTypeSpread:
		return t.nextDomainTopologySpread(pod, podDomains, nodeDomains)
	case TopologyTypePodAffinity:
		return t.nextDomainAffinity(pod, podDomains, nodeDomains)
	case TopologyTypePodAntiAffinity:
		return t.nextDomainAntiAffinity(podDomains, nodeDomains)
	default:
		panic(fmt.Sprintf("Unrecognized topology group type: %s", t.Type))
	}
}

func (t *TopologyGroup) Record(domains ...string) {
	for _, domain := range domains {
		t.domains[domain]++
		t.emptyDomains.Delete(domain)
	}
}

// Counts returns true if the pod would count for the topology, given that it schedule to a node with the provided
// requirements
func (t *TopologyGroup) Counts(pod *v1.Pod, requirements scheduling.Requirements, compatabilityOptions ...option.Function[scheduling.CompatibilityOptions]) bool {
	return t.selects(pod) && t.nodeFilter.MatchesRequirements(requirements, compatabilityOptions...)
}

// Register ensures that the topology is aware of the given domain names.
func (t *TopologyGroup) Register(domains ...string) {
	for _, domain := range domains {
		if _, ok := t.domains[domain]; !ok {
			t.domains[domain] = 0
			t.emptyDomains.Insert(domain)
		}
	}
}

func (t *TopologyGroup) Unregister(domains ...string) {
	for _, domain := range domains {
		delete(t.domains, domain)
		t.emptyDomains.Delete(domain)
	}
}

func (t *TopologyGroup) AddOwner(key types.UID) {
	t.owners[key] = struct{}{}
}

func (t *TopologyGroup) RemoveOwner(key types.UID) {
	delete(t.owners, key)
}

func (t *TopologyGroup) IsOwnedBy(key types.UID) bool {
	_, ok := t.owners[key]
	return ok
}

// Hash is used so we can track single topologies that affect multiple groups of pods.  If a deployment has 100x pods
// with self anti-affinity, we track that as a single topology with 100 owners instead of 100x topologies.
func (t *TopologyGroup) Hash() uint64 {
	return lo.Must(hashstructure.Hash(struct {
		TopologyKey string
		Type        TopologyType
		Namespaces  sets.Set[string]
		RawSelector *metav1.LabelSelector
		MaxSkew     int32
		NodeFilter  TopologyNodeFilter
	}{
		TopologyKey: t.Key,
		Type:        t.Type,
		Namespaces:  t.namespaces,
		RawSelector: t.rawSelector,
		MaxSkew:     t.maxSkew,
		NodeFilter:  t.nodeFilter,
	}, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
}

// nextDomainTopologySpread returns a scheduling.Requirement that includes a node domain that a pod should be scheduled to.
// If there are multiple eligible domains, we return any random domain that satisfies the `maxSkew` configuration.
// If there are no eligible domains, we return a `DoesNotExist` requirement, implying that we could not satisfy the topologySpread requirement.
// nolint:gocyclo
func (t *TopologyGroup) nextDomainTopologySpread(pod *v1.Pod, podDomains, nodeDomains *scheduling.Requirement) *scheduling.Requirement {
	// min count is calculated across all domains
	min := t.domainMinCount(podDomains)
	selfSelecting := t.selects(pod)

	minDomain := ""
	minCount := int32(math.MaxInt32)

	// If we are explicitly selecting on specific node domains ("In" requirement),
	// this is going to be more efficient to iterate through
	// This is particularly useful when considering the hostname topology key that can have a
	// lot of t.domains but only a single nodeDomain
	if nodeDomains.Operator() == v1.NodeSelectorOpIn {
		for _, domain := range nodeDomains.Values() {
			if count, ok := t.domains[domain]; ok {
				if selfSelecting {
					count++
				}
				if count-min <= t.maxSkew && count < minCount {
					minDomain = domain
					minCount = count
				}
			}
		}
	} else {
		for domain := range t.domains {
			// but we can only choose from the node domains
			if nodeDomains.Has(domain) {
				// comment from kube-scheduler regarding the viable choices to schedule to based on skew is:
				// 'existing matching num' + 'if self-match (1 or 0)' - 'global min matching num' <= 'maxSkew'
				count := t.domains[domain]
				if selfSelecting {
					count++
				}
				if count-min <= t.maxSkew && count < minCount {
					minDomain = domain
					minCount = count
				}
			}
		}
	}
	if minDomain == "" {
		// avoids an error message about 'zone in [""]', preferring 'zone in []'
		return scheduling.NewRequirement(podDomains.Key, v1.NodeSelectorOpDoesNotExist)
	}
	return scheduling.NewRequirement(podDomains.Key, v1.NodeSelectorOpIn, minDomain)
}

func (t *TopologyGroup) domainMinCount(domains *scheduling.Requirement) int32 {
	// hostname based topologies always have a min pod count of zero since we can create one
	if t.Key == v1.LabelHostname {
		return 0
	}

	min := int32(math.MaxInt32)
	var numPodSupportedDomains int32
	// determine our current min count
	for domain, count := range t.domains {
		if domains.Has(domain) {
			numPodSupportedDomains++
			if count < min {
				min = count
			}
		}
	}
	if t.minDomains != nil && numPodSupportedDomains < *t.minDomains {
		min = 0
	}
	return min
}

// nolint:gocyclo
func (t *TopologyGroup) nextDomainAffinity(pod *v1.Pod, podDomains *scheduling.Requirement, nodeDomains *scheduling.Requirement) *scheduling.Requirement {
	options := scheduling.NewRequirement(podDomains.Key, v1.NodeSelectorOpDoesNotExist)

	// If we are explicitly selecting on specific node domains ("In" requirement),
	// this is going to be more efficient to iterate through
	// This is particularly useful when considering the hostname topology key that can have a
	// lot of t.domains but only a single nodeDomain
	if nodeDomains.Operator() == v1.NodeSelectorOpIn {
		for _, domain := range nodeDomains.Values() {
			if count, ok := t.domains[domain]; podDomains.Has(domain) && ok && count > 0 {
				options.Insert(domain)
			}
		}
	} else {
		for domain := range t.domains {
			if podDomains.Has(domain) && t.domains[domain] > 0 && nodeDomains.Has(domain) {
				options.Insert(domain)
			}
		}
	}
	if options.Len() != 0 {
		return options
	}

	// If pod is self-selecting and no pod has been scheduled yet OR the pods that have scheduled are
	// incompatible with our podDomains, we can pick a domain at random to bootstrap scheduling.
	if t.selects(pod) && (len(t.domains) == len(t.emptyDomains) || !t.anyCompatiblePodDomain(podDomains)) {
		// First try to find a domain that is within the intersection of pod/node domains. In the case of an in-flight node
		// this causes us to pick the domain that the existing in-flight node is already in if possible instead of picking
		// a random viable domain.
		intersected := podDomains.Intersection(nodeDomains)
		for domain := range t.domains {
			if intersected.Has(domain) {
				options.Insert(domain)
				break
			}
		}

		// and if there are no node domains, just return the first random domain that is viable
		for domain := range t.domains {
			if podDomains.Has(domain) {
				options.Insert(domain)
				break
			}
		}
	}
	return options
}

// anyCompatiblePodDomain validates whether any t.domain is compatible with our podDomains
// This is only useful in affinity checking because it tells us whether we can schedule the pod
// to the current node since it is the first pod that exists in the TopologyGroup OR all other domains
// in the TopologyGroup are incompatible with the podDomains
func (t *TopologyGroup) anyCompatiblePodDomain(podDomains *scheduling.Requirement) bool {
	for domain := range t.domains {
		if podDomains.Has(domain) && t.domains[domain] > 0 {
			return true
		}
	}
	return false
}

// nolint:gocyclo
func (t *TopologyGroup) nextDomainAntiAffinity(podDomains, nodeDomains *scheduling.Requirement) *scheduling.Requirement {
	options := scheduling.NewRequirement(podDomains.Key, v1.NodeSelectorOpDoesNotExist)
	// pods with anti-affinity must schedule to a domain where there are currently none of those pods (an empty
	// domain). If there are none of those domains, then the pod can't schedule and we don't need to walk this
	// list of domains.  The use case where this optimization is really great is when we are launching nodes for
	// a deployment of pods with self anti-affinity.  The domains map here continues to grow, and we continue to
	// fully scan it each iteration.

	// If we are explicitly selecting on specific node domains ("In" requirement) and the number of node domains
	// is less than our empty domains, this is going to be more efficient to iterate through
	// This is particularly useful when considering the hostname topology key that can have a
	// lot of t.domains but only a single nodeDomain
	if nodeDomains.Operator() == v1.NodeSelectorOpIn && nodeDomains.Len() < len(t.emptyDomains) {
		for _, domain := range nodeDomains.Values() {
			if t.emptyDomains.Has(domain) && podDomains.Has(domain) {
				options.Insert(domain)
			}
		}
	} else {
		for domain := range t.emptyDomains {
			if nodeDomains.Has(domain) && podDomains.Has(domain) {
				options.Insert(domain)
			}
		}
	}
	return options
}

// selects returns true if the given pod is selected by this topology
func (t *TopologyGroup) selects(pod *v1.Pod) bool {
	return t.namespaces.Has(pod.Namespace) && t.selector.Matches(labels.Set(pod.Labels))
}
