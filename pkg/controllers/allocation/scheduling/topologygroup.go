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

	v1 "k8s.io/api/core/v1"
)

func NewTopologyGroup(pod *v1.Pod, constraint v1.TopologySpreadConstraint) *TopologyGroup {
	return &TopologyGroup{
		Constraint: constraint,
		Pods:       []*v1.Pod{pod},
		spread:     map[string]int{},
	}
}

// TopologyGroup is a set of pods that share a topology spread constraint
type TopologyGroup struct {
	Constraint v1.TopologySpreadConstraint
	Pods       []*v1.Pod
	// spread is an internal field used to track current spread
	spread map[string]int // TODO(ellistarn) explore replacing with a minheap
}

func (t *TopologyGroup) Register(domains ...string) {
	for _, domain := range domains {
		t.spread[domain] = 0
	}
}

// Increment increments the spread of a registered domain
func (t *TopologyGroup) Increment(domain string) {
	_, ok := t.spread[domain]
	if ok {
		t.spread[domain]++
	}
}

// NextDomain chooses a domain that minimizes skew and increments its count
func (t *TopologyGroup) NextDomain() string {
	minDomain := ""
	minCount := math.MaxInt64
	for domain, count := range t.spread {
		if count <= minCount {
			minDomain = domain
			minCount = count
		}
	}
	t.spread[minDomain]++
	return minDomain
}
