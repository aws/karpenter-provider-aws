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

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (e *Recorder) EventRecorder() record.EventRecorder                          { return e.rec }
func (e *Recorder) WaitingOnReadinessForConsolidation(v *v1.Node)                {}
func (e *Recorder) TerminatingNodeForConsolidation(node *v1.Node, reason string) {}
func (e *Recorder) LaunchingNodeForConsolidation(node *v1.Node, reason string)   {}
func (e *Recorder) WaitingOnDeletionForConsolidation(node *v1.Node)              {}

func (e *Recorder) NominatePod(pod *v1.Pod, node *v1.Node) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bindings = append(e.bindings, Binding{pod, node})
}

func (e *Recorder) EvictPod(pod *v1.Pod) {}

func (e *Recorder) PodFailedToSchedule(pod *v1.Pod, err error) {}

func (e *Recorder) NodeFailedToDrain(node *v1.Node, err error) {}

func (e *Recorder) Reset() {
	e.ResetBindings()
}

func (e *Recorder) ResetBindings() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bindings = nil
}
func (e *Recorder) ForEachBinding(f func(pod *v1.Pod, node *v1.Node)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range e.bindings {
		f(b.Pod, b.Node)
	}
}
