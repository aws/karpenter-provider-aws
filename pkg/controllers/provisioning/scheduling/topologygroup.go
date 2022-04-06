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

func NewTopologyGroup(topologyType TopologyType, topologyKey string, namespaces utilsets.String, labelSelector *metav1.LabelSelector, maxSkew int32, domains utilsets.String) *TopologyGroup {
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

func (t *TopologyGroup) Get(pod *v1.Pod, podDomains, nodeDomains sets.Set) sets.Set {
	switch t.Type {
	case TopologyTypeSpread:
		return t.nextDomainTopologySpread(pod, podDomains, nodeDomains)
	case TopologyTypePodAffinity:
		return t.nextDomainAffinity(pod, podDomains)
	case TopologyTypePodAntiAffinity:
		return t.nextDomainAntiAffinity(podDomains)
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

func (t *TopologyGroup) nextDomainTopologySpread(pod *v1.Pod, podDomains, nodeDomains sets.Set) sets.Set {
	// min count is calculated across all domains
	min := t.domainMinCount(podDomains)
	selfSelecting := t.Matches(pod.Namespace, pod.Labels)

	minDomain := ""
	minCount := int32(math.MaxInt32)
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
	if minDomain == "" {
		// avoids an error message about 'zone in [""]', preferring 'zone in []'
		return sets.NewSet()
	}
	return sets.NewSet(minDomain)
}

func (t *TopologyGroup) domainMinCount(domains sets.Set) int32 {
	// hostname based topologies always have a min pod count of zero since we can create one
	if t.Key == v1.LabelHostname {
		return 0
	}

	min := int32(math.MaxInt32)
	// determine our current min count
	for domain, count := range t.domains {
		if domains.Has(domain) {
			if count < min {
				min = count
			}
		}
	}
	return min
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
