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

package events

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/flowcontrol"
)

func NewLoadSheddingRecorder(r Recorder) Recorder {
	return &loadshedding{
		rec:              r,
		nominationBucket: flowcontrol.NewTokenBucketRateLimiter(5, 10),
	}
}

type loadshedding struct {
	rec              Recorder
	nominationBucket flowcontrol.RateLimiter
}

func (l *loadshedding) NominatePod(pod *v1.Pod, node *v1.Node) {
	// Pod nominations occur very often, especially in large scale-ups.  They normally aren't particularly useful
	// during a scaleup, but are useful when at a steady state where we have a bug and think a pod will schedule
	// that actually won't.  This prevents us from hammering the API server with events that likely aren't useful
	// which can slow down node creation or result in events being dropped anyway by the K8s client.
	if !l.nominationBucket.TryAccept() {
		return
	}
	l.rec.NominatePod(pod, node)
}

func (l *loadshedding) EvictPod(pod *v1.Pod) {
	l.rec.EvictPod(pod)
}

func (l *loadshedding) PodFailedToSchedule(pod *v1.Pod, err error) {
	l.rec.PodFailedToSchedule(pod, err)
}

func (l *loadshedding) NodeFailedToDrain(node *v1.Node, err error) {
	l.rec.NodeFailedToDrain(node, err)
}

func (l *loadshedding) TerminatingNodeForConsolidation(node *v1.Node, reason string) {
	l.rec.TerminatingNodeForConsolidation(node, reason)
}

func (l *loadshedding) LaunchingNodeForConsolidation(node *v1.Node, reason string) {
	l.rec.LaunchingNodeForConsolidation(node, reason)
}

func (l *loadshedding) WaitingOnReadinessForConsolidation(node *v1.Node) {
	l.rec.WaitingOnReadinessForConsolidation(node)
}

func (l *loadshedding) WaitingOnDeletionForConsolidation(node *v1.Node) {
	l.rec.WaitingOnDeletionForConsolidation(node)
}
