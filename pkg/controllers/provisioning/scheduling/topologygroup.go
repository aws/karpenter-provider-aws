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
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/aws/karpenter/pkg/utils/sets"

	"math"

	"github.com/mitchellh/hashstructure/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"

	"github.com/aws/karpenter/pkg/utils/pod"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// Topology is used to track pod counts that match a selector by the topology domain (e.g. SELECT COUNT(*) FROM pods GROUP BY(topology_ke
type TopologyGroup struct {
	Key     string
	MaxSkew int32
	Type    TopologyType

	rawSelector    *metav1.LabelSelector // stored so we can easily hash
	selector       labels.Selector       // non-hashable, but fast version used for selecting
	domains        map[string]int32      // TODO(ellistarn) explore replacing with a minheap
	maxCount       int32
	namespaces     sets.Set
	owners         map[client.ObjectKey]struct{}
	isInverse      bool // if true, this is tracking an inverse anti-affinity
	originatorHash uint64
	preferred      bool
}

func (t *TopologyGroup) Matches(namespace string, podLabels labels.Set) bool {
	if !t.namespaces.Has(namespace) {
		return false
	}
	return t.selector.Matches(podLabels)
}

func (t *TopologyGroup) RecordUsage(domain string) {
	t.domains[domain]++
	if t.domains[domain] > t.maxCount {
		t.maxCount = t.domains[domain]
	}
}

// NextDomainMinimizeSkew returns the best domain to choose next and what the max-skew would be if we
// chose that domain
//gocyclo:ignore
func (t *TopologyGroup) NextDomainMinimizeSkew(requirements v1alpha5.Requirements) (minDomain string, maxSkew int32, increasingSkew bool) {
	var requirement sets.Set
	if requirements.Has(t.Key) {
		requirement = requirements.Get(t.Key)
	} else {
		requirement = sets.NewComplementSet()
	}

	minCount := int32(math.MaxInt32)
	globalMin := int32(math.MaxInt32)
	for domain, count := range t.domains {
		if count < globalMin {
			globalMin = count
		}

		// we only consider domains that match the pod's node affinity for the purposes
		// of what we select as our next domain
		if requirement.Has(domain) {
			if count < minCount {
				minCount = count
				minDomain = domain
			}
		}
	}

	// can we create new domains?
	creatable := t.Key == v1.LabelHostname

	// if it's a topology key that we can create a new domain for, there's effectively always a global min
	// of zero
	if creatable {
		globalMin = 0
	}

	if len(t.domains) == 1 {
		// There is only a single item in the domain.  If the domain is creatable we treat it as increasing skew since
		// there is a hypothetical domain with a count of zero out there.  If it's not creatable, we treat it as
		// not increasing skew since the domain has both the min/max counts and a skew of zero
		if creatable {
			return minDomain, t.maxCount + 1, true
		}
		return minDomain, 0, false
	}

	// none of the topology domains have any pods assigned, so we'll just be at
	// a max-skew of 1 when we create something
	if t.maxCount == 0 {
		return minDomain, 1, true
	}

	// Calculate what the max skew will be if we chose the min domain.
	maxSkew = t.maxCount - (minCount + 1)
	if globalMin != minCount {
		// if the global min is less than the count of pods in domains that match the node selector
		// the max-skew is based on the global min as we can't change it
		maxSkew = t.maxCount - globalMin
		// the domain we're allowed to pick happens to be the maximum value, so by picking it we are increasing skew
		// even more
		if minCount == t.maxCount {
			maxSkew++
		}
	}

	// if max = min, skew would be negative since our new domain selection would be the new max
	if maxSkew < 0 {
		maxSkew = 1
	}

	// We need to know if we are increasing or decreasing skew.  If we are above the max-skew, but assigning this
	// topology domain decreases skew, we should do it.
	oldMaxSkew := t.maxCount - globalMin
	increasingSkew = maxSkew > oldMaxSkew
	return
}

func (t *TopologyGroup) AnyDomain(requirements v1alpha5.Requirements) string {
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

func (t *TopologyGroup) MaxNonZeroDomain(requirements v1alpha5.Requirements) string {
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

func (t *TopologyGroup) RegisterDomain(zone string) {
	if _, ok := t.domains[zone]; !ok {
		t.domains[zone] = 0
	}
}

func (t *TopologyGroup) EmptyDomain(requirements v1alpha5.Requirements) string {
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

func (t *TopologyGroup) HasNonEmptyDomains() bool {
	for _, count := range t.domains {
		if count != 0 {
			return true
		}
	}
	return false
}

func (t *TopologyGroup) AddOwner(key client.ObjectKey) {
	t.owners[key] = struct{}{}
}

func (t *TopologyGroup) RemoveOwner(key client.ObjectKey) {
	delete(t.owners, key)
}

func (t *TopologyGroup) IsOwnedBy(object client.ObjectKey) bool {
	_, ok := t.owners[object]
	return ok
}

func (t *TopologyGroup) Hash() uint64 {
	hash, err := hashstructure.Hash(struct {
		TopologyKey   string
		Type          TopologyType
		Namespaces    sets.Set
		LabelSelector *metav1.LabelSelector
	}{
		TopologyKey:   t.Key,
		Type:          t.Type,
		Namespaces:    t.namespaces,
		LabelSelector: t.rawSelector,
	}, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	runtime.Must(err)
	return hash
}

// initializeWellKnown causes the topology group to initialize its domains with the valid well known domains from the
// provisioner spec
func (t *TopologyGroup) initializeWellKnown(constraints *v1alpha5.Constraints) {
	// add our well known domain values
	if t.Key == v1.LabelTopologyZone {
		// we know about the zones that we can schedule to regardless of if nodes exist there
		for _, zone := range constraints.Requirements.Zones().List() {
			t.RegisterDomain(zone)
		}
	} else if t.Key == v1alpha5.LabelCapacityType {
		for _, ct := range constraints.Requirements.CapacityTypes().List() {
			t.RegisterDomain(ct)
		}
	}
}

func IgnoredForTopology(p *v1.Pod) bool {
	return !pod.IsScheduled(p) || pod.IsTerminal(p) || pod.IsTerminating(p)
}

func TopologyListOptions(namespace string, labelSelector *metav1.LabelSelector) *client.ListOptions {
	selector := labels.Everything()
	if labelSelector == nil {
		return &client.ListOptions{Namespace: namespace, LabelSelector: selector}
	}
	for key, value := range labelSelector.MatchLabels {
		requirement, _ := labels.NewRequirement(key, selection.Equals, []string{value})
		selector = selector.Add(*requirement)
	}
	for _, expression := range labelSelector.MatchExpressions {
		requirement, _ := labels.NewRequirement(expression.Key, selection.Operator(expression.Operator), expression.Values)
		selector = selector.Add(*requirement)
	}
	return &client.ListOptions{Namespace: namespace, LabelSelector: selector}
}
