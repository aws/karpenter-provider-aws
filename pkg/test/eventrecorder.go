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
	"k8s.io/client-go/tools/record"

	"github.com/aws/karpenter/pkg/events"
)

// Binding is a potential binding that was reported through event recording.
type Binding struct {
	Pod  *v1.Pod
	Node *v1.Node
}

// Recorder is a mock event recorder that is used to facilitate testing.
type Recorder struct {
	rec      record.EventRecorder
	mu       sync.Mutex
	bindings []Binding
}

var _ events.Recorder = (*Recorder)(nil)

func NewEventRecorder() *Recorder {
	return &Recorder{}
}

func (r *Recorder) EventRecorder() record.EventRecorder                          { return r.rec }
func (r *Recorder) WaitingOnReadinessForConsolidation(v *v1.Node)                {}
func (r *Recorder) TerminatingNodeForConsolidation(node *v1.Node, reason string) {}
func (r *Recorder) LaunchingNodeForConsolidation(node *v1.Node, reason string)   {}
func (r *Recorder) WaitingOnDeletionForConsolidation(node *v1.Node)              {}

func (r *Recorder) NominatePod(pod *v1.Pod, node *v1.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = append(r.bindings, Binding{pod, node})
}

func (r *Recorder) EvictPod(pod *v1.Pod) {}

func (r *Recorder) PodFailedToSchedule(pod *v1.Pod, err error) {}

func (r *Recorder) NodeFailedToDrain(node *v1.Node, err error) {}

func (r *Recorder) Reset() {
	r.ResetBindings()
}

func (r *Recorder) ResetBindings() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = nil
}
func (r *Recorder) ForEachBinding(f func(pod *v1.Pod, node *v1.Node)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.bindings {
		f(b.Pod, b.Node)
	}
}
