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
	"fmt"
	"math"

	"github.com/mitchellh/hashstructure/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilsets "k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
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
	// Pod Index
	owners map[types.UID]struct{} // Pods that have this topology as a scheduling rule
	// Internal state
	domains map[string]int32 // TODO(ellistarn) explore replacing with a minheap
}

func NewTopologyGroup(topologyType TopologyType, key string, namespaces utilsets.String, labelSelector *metav1.LabelSelector, maxSkew int32, domains utilsets.String) *TopologyGroup {
	domainCounts := map[string]int32{}
	for domain := range domains {
		domainCounts[domain] = 0
	}
	return &TopologyGroup{
		Type:       topologyType,
		Key:        key,
		namespaces: namespaces,
		selector:   labelSelector,
		maxSkew:    maxSkew,
		domains:    domainCounts,
		owners:     map[types.UID]struct{}{},
	}
}

func (t *TopologyGroup) Next(requirements v1alpha5.Requirements, selfSelecting bool) (string, error) {
	var nextDomain string
	var maxSkew int32
	var increasingSkew bool
	switch t.Type {
	case TopologyTypeSpread:
		nextDomain, maxSkew, increasingSkew = t.nextDomainMinimizeSkew(requirements)
		if maxSkew > t.maxSkew && increasingSkew {
			return "", fmt.Errorf("would violate max-skew for topology key %s", t.Key)
		}
	case TopologyTypePodAffinity:
		nextDomain = t.maxNonZeroDomain(requirements)
		// we don't have a valid domain, but it's pod affinity and the pod itself will satisfy the topology
		// constraint , so check for any domain as long as it's the first one the pod will land in
		if nextDomain == "" && selfSelecting && !t.hasNonEmptyDomains() {
			nextDomain = t.anyDomain(requirements)
		}
	case TopologyTypePodAntiAffinity:
		nextDomain = t.emptyDomain(requirements)
	}

	if nextDomain == "" {
		return "", fmt.Errorf("unsatisfiable %s topology constraint for key %s", t.Type, t.Key)
	}
	return nextDomain, nil
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

// nextDomainMinimizeSkew returns the best domain to choose next and what the max-skew would be if we
// chose that domain
func (t *TopologyGroup) nextDomainMinimizeSkew(requirements v1alpha5.Requirements) (minDomain string, maxSkew int32, increasingSkew bool) {
	var requirement sets.Set
	if requirements.Has(t.Key) {
		requirement = requirements.Get(t.Key)
	} else {
		requirement = sets.NewComplementSet()
	}

	localMin := int32(math.MaxInt32)  // minimum count for a domain that is possible given the constraints
	globalMin := int32(math.MaxInt32) // minimum count for the domain
	globalMax := int32(0)
	for domain, count := range t.domains {
		if count < globalMin {
			globalMin = count
		}
		if count > globalMax {
			globalMax = count
		}

		// we only consider domains that match the pod's node affinity for the purposes
		// of what we select as our next domain
		if requirement.Has(domain) {
			if count < localMin {
				localMin = count
				minDomain = domain
			}
		}
	}

	// if the topology key is hostname, we can always create a new node with a unique hotsname so there's effectively
	// always a global min of zero
	if t.Key == v1.LabelHostname {
		globalMin = 0
	}

	// none of the topology domains have any pods assigned, so we'll just be at
	// a max-skew of 1 when we create something
	if globalMax == 0 {
		return minDomain, 1, true
	}

	// Calculate what the max skew will be if we chose the min domain.
	maxSkew = globalMax - (localMin + 1)
	if globalMin != localMin {
		// if the global min is less than the count of pods in domains that match the node selector
		// the max-skew is based on the global min as we can't change it
		maxSkew = globalMax - globalMin
	}
	// the domain we're allowed to pick happens to be the maximum value, so by picking it we are increasing skew
	// even more
	if localMin == globalMax {
		maxSkew++
	}

	// We need to know if we are increasing or decreasing skew.  If we are above the max-skew, but assigning this
	// topology domain decreases skew, we should do it.
	oldMaxSkew := globalMax - globalMin
	increasingSkew = maxSkew > oldMaxSkew
	return
}

func (t *TopologyGroup) anyDomain(requirements v1alpha5.Requirements) string {
	var requirement sets.Set
	if requirements.Has(t.Key) {
		requirement = requirements.Get(t.Key)
	} else {
		requirement = sets.NewComplementSet()
	}
	for domain := range t.domains {
		if requirement.Has(domain) {
			return domain
		}
	}
	return ""
}

func (t *TopologyGroup) maxNonZeroDomain(requirements v1alpha5.Requirements) string {
	var requirement sets.Set
	if requirements.Has(t.Key) {
		requirement = requirements.Get(t.Key)
	} else {
		requirement = sets.NewComplementSet()
	}
	maxCount := int32(math.MinInt32)
	var maxDomain string
	for domain, count := range t.domains {
		// we only consider domains that match the pod's node selectors
		if requirement.Has(domain) {
			if count > maxCount {
				maxCount = count
				maxDomain = domain
			}
		}
	}
	if maxCount == 0 {
		return ""
	}
	return maxDomain
}

func (t *TopologyGroup) emptyDomain(requirements v1alpha5.Requirements) string {
	requirement := requirements.Get(t.Key)
	for domain, count := range t.domains {
		if !requirement.Has(domain) {
			continue
		}
		if count == 0 {
			return domain
		}
	}
	return ""
}

func (t *TopologyGroup) hasNonEmptyDomains() bool {
	for _, count := range t.domains {
		if count != 0 {
			return true
		}
	}
	return false
}
