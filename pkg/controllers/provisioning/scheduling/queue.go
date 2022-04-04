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
	"sort"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/utils/resources"
)

// Queue is a queue of pods that is scheduled.  It's used to attempt to schedule pods as long as we are making progress
// in scheduling. This is sometimes required to maintain zonal topology spreads with constrained pods, and can satisfy
// pod affinities that occur in a batch of pods if there are enough constraints provided.
type Queue struct {
	pods             []*v1.Pod
	errors           map[*v1.Pod]error
	overrideProgress bool
	lastRoundCount   int
}

// NewQueue constructs a new queue given the input pods, sorting them to optimize for bin-packing into nodes.
func NewQueue(pods ...*v1.Pod) *Queue {
	sort.Slice(pods, byCPUAndMemoryDescending(pods))
	return &Queue{
		pods:   pods,
		errors: map[*v1.Pod]error{},
	}
}

// IsProgressing returns true if scheduling made progress on the last scheduling round.  This is determined by checking
// if we have scheduled if we still have pods remaining, but have fewer pods remaining than were scheduled in the previous
// round.  This can be overriden for a scheduling round by the MarkProgress method.
func (q *Queue) IsProgressing() bool {
	return q.overrideProgress || (len(q.pods) > 0 && q.lastRoundCount != len(q.pods))
}

// MarkProgress forces the IsProgressing method to return true for the next scheduling round.  This is used when we think
// we may continue to make progress in scheduling, even though we didn't schedule any additional pods this round
// (e.g. successfully relaxing a pod may cause it to schedule in the next round)
func (q *Queue) MarkProgress() {
	q.overrideProgress = true
}

// PopAll pops all pods from the queue, resetting it for the next scheduling round.
func (q *Queue) PopAll() []*v1.Pod {
	q.lastRoundCount = len(q.pods)
	result := q.pods
	q.pods = nil
	q.overrideProgress = false
	return result
}

// PushWithError is used to record pods that have failed to schedule along with the error received.  These pods will be
// tried in future scheduling rounds (calls to PopAll), and once scheduling has finished any unschedulable pods and their
// errors can be retrieved with ForEach
func (q *Queue) PushWithError(pod *v1.Pod, err error) {
	q.errors[pod] = err
	q.pods = append(q.pods, pod)
}

// ForEach is a non-mutating method that iterates over the current set of pods and calls the function provided with each
// pod and the last error recorded for this pod with PushWithError
func (q *Queue) ForEach(fn func(p *v1.Pod, err error)) {
	for _, p := range q.pods {
		fn(p, q.errors[p])
	}
}

func byCPUAndMemoryDescending(pods []*v1.Pod) func(i int, j int) bool {
	return func(i, j int) bool {
		lhs := resources.RequestsForPods(pods[i])
		rhs := resources.RequestsForPods(pods[j])

		cpuCmp := resources.Cmp(lhs[v1.ResourceCPU], rhs[v1.ResourceCPU])
		if cpuCmp < 0 {
			// LHS has less CPU, so it should be sorted after
			return false
		} else if cpuCmp > 0 {
			return true
		}
		memCmp := resources.Cmp(lhs[v1.ResourceMemory], rhs[v1.ResourceMemory])

		if memCmp < 0 {
			return false
		} else if memCmp > 0 {
			return true
		}
		return false
	}
}
