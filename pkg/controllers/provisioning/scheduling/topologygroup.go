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
	"math"

	"github.com/mitchellh/hashstructure/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilsets "k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter/pkg/utils/sets"
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
	Key        string
	Type       TopologyType
	maxSkew    int32
	namespaces utilsets.String
	selector   *metav1.LabelSelector
	// Index
	owners  map[types.UID]struct{} // Pods that have this topology as a scheduling rule
	domains map[string]int32       // TODO(ellistarn) explore replacing with a minheap
}

func NewTopologyGroup(pod *v1.Pod, topologyType TopologyType, topologyKey string, namespaces utilsets.String, labelSelector *metav1.LabelSelector, maxSkew int32, domains utilsets.String) *TopologyGroup {
	domainCounts := map[string]int32{}
	for domain := range domains {
		domainCounts[domain] = 0
	}
	return &TopologyGroup{
		Type:       topologyType,
		Key:        topologyKey,
		namespaces: namespaces,
		selector:   labelSelector,
		maxSkew:    maxSkew,
		domains:    domainCounts,
		owners:     map[types.UID]struct{}{},
	}
}

func (t *TopologyGroup) Next(pod *v1.Pod, nodeHostname string, domains sets.Set) sets.Set {
	switch t.Type {
	case TopologyTypeSpread:
		// We are only considering putting the pod on a single node. Intersecting the list of viable domains with the node's
		// hostname solves a problem where we have multiple nodes that the pod could land on because of a min-domain tie, but
		// some of them are not viable due to arch/os.  Since we look at each node one at a time and this returns a min
		// domain at random, we potentially fail to schedule as it may return a node hostname that doesn't correspond to the
		// node that we are considering.  We can't add a node selector upstream to limit our available domains to just the node
		// under consideration as this as would break pod self-affinity since it needs to consider the universe of valid
		// domains to ensure that it only satisfies pod self-affinity the first time.  With the additional node selector, the
		// universe of domains would always be a single hostname and it would repeatedly allow pods to provide self affinity
		// across different nodes
		if t.Key == v1.LabelHostname {
			domains = domains.Intersection(sets.NewSet(nodeHostname))
		}
		return t.nextDomainTopologySpread(domains)
	case TopologyTypePodAffinity:
		return t.nextDomainAffinity(pod, domains)
	case TopologyTypePodAntiAffinity:
		return t.nextDomainAntiAffinity(domains)
	default:
		return sets.NewSet()
	}
}

func (t *TopologyGroup) Record(domains ...string) {
	for _, domain := range domains {
		t.domains[domain]++
	}
}

func (t *TopologyGroup) Matches(namespace string, podLabels labels.Set) bool {
	selector, err := metav1.LabelSelectorAsSelector(t.selector)
	runtime.Must(err)
	return t.namespaces.Has(namespace) && selector.Matches(podLabels)
}

// Register ensures that the topology is aware of the given domain names.
func (t *TopologyGroup) Register(domains ...string) {
	for _, domain := range domains {
		t.domains[domain] = 0
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
	hash, err := hashstructure.Hash(struct {
		TopologyKey   string
		Type          TopologyType
		Namespaces    utilsets.String
		LabelSelector *metav1.LabelSelector
		MaxSkew       int32
	}{
		TopologyKey:   t.Key,
		Type:          t.Type,
		Namespaces:    t.namespaces,
		LabelSelector: t.selector,
		MaxSkew:       t.maxSkew,
	}, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	runtime.Must(err)
	return hash
}

func (t *TopologyGroup) nextDomainTopologySpread(domains sets.Set) sets.Set {
	// Pick the domain that minimizes skew.
	min := int32(math.MaxInt32)
	minDomain := ""

	// Need to count skew for hostname
	for domain := range t.domains {
		if domains.Has(domain) && t.domains[domain] < min {
			min = t.domains[domain]
			minDomain = domain
		}
	}
	if t.Key == v1.LabelHostname && min+1 > t.maxSkew {
		return sets.NewSet()
	}
	return sets.NewSet(minDomain)
}

func (t *TopologyGroup) nextDomainAffinity(pod *v1.Pod, domains sets.Set) sets.Set {
	options := sets.NewSet()
	for domain := range t.domains {
		if domains.Has(domain) && t.domains[domain] > 0 {
			options.Insert(domain)
		}
	}
	// If pod is self selecting and no pod has been scheduled yet, pick a domain at random to bootstrap scheduling
	if options.Len() == 0 && t.Matches(pod.Namespace, pod.Labels) {
		for domain := range t.domains {
			if domains.Has(domain) {
				options.Insert(domain)
				break
			}
		}
	}
	return options
}

func (t *TopologyGroup) nextDomainAntiAffinity(domains sets.Set) sets.Set {
	options := sets.NewSet()
	for domain := range t.domains {
		if domains.Has(domain) && t.domains[domain] == 0 {
			options.Insert(domain)
		}
	}
	return options
}
