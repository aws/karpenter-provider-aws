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
)

// Binding is a potential binding that was reported through event recording.
type Binding struct {
	Pod      *v1.Pod
	NodeName string
}

// EventRecorder is a mock event recorder that is used to facilitate testing.
type EventRecorder struct {
	mu       sync.Mutex
	bindings []Binding
}

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{}
}

func (e *EventRecorder) PodShouldSchedule(pod *v1.Pod, nodeName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bindings = append(e.bindings, Binding{pod, nodeName})
}
func (e *EventRecorder) PodFailedToSchedule(pod *v1.Pod, err error) {}

func (e *EventRecorder) ForEachBinding(f func(pod *v1.Pod, nodeName string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range e.bindings {
		f(b.Pod, b.NodeName)
	}
}

func (e *EventRecorder) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bindings = nil
}
