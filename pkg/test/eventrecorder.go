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

package test

import (
	"sync"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/events"
)

// Binding is a potential binding that was reported through event recording.
type Binding struct {
	Pod  *v1.Pod
	Node *v1.Node
}

// EventRecorder is a mock event recorder that is used to facilitate testing.
type EventRecorder struct {
	mu       sync.Mutex
	bindings []Binding
}

var _ events.Recorder = (*EventRecorder)(nil)

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{}
}

func (r *EventRecorder) WaitingOnReadinessForConsolidation(v *v1.Node)                {}
func (r *EventRecorder) TerminatingNodeForConsolidation(node *v1.Node, reason string) {}
func (r *EventRecorder) LaunchingNodeForConsolidation(node *v1.Node, reason string)   {}
func (r *EventRecorder) WaitingOnDeletionForConsolidation(node *v1.Node)              {}

func (r *EventRecorder) NominatePod(pod *v1.Pod, node *v1.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = append(r.bindings, Binding{pod, node})
}

func (r *EventRecorder) EvictPod(pod *v1.Pod) {}

func (r *EventRecorder) PodFailedToSchedule(pod *v1.Pod, err error) {}

func (r *EventRecorder) NodeFailedToDrain(node *v1.Node, err error) {}

func (r *EventRecorder) Reset() {
	r.ResetBindings()
}

func (r *EventRecorder) ResetBindings() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = nil
}
func (r *EventRecorder) ForEachBinding(f func(pod *v1.Pod, node *v1.Node)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.bindings {
		f(b.Pod, b.Node)
	}
}
