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
	"k8s.io/client-go/tools/record"
)

// Recorder is used to record events that occur about pods so they can be viewed by looking at the pod's events so our
// actions are more observable without requiring log inspection
type Recorder interface {
	// NominatePod is called when we have determined that a pod should schedule against an existing node and don't
	// currently need to provision new capacity for the pod.
	NominatePod(*v1.Pod, *v1.Node)
	// PodFailedToSchedule is called when a pod has failed to schedule entirely.
	PodFailedToSchedule(*v1.Pod, error)
	// NodeFailedToDrain is called when a pod causes a node draining to fail
	NodeFailedToDrain(*v1.Node, error)
}

type recorder struct {
	rec record.EventRecorder
}

func NewRecorder(rec record.EventRecorder) Recorder {
	return &recorder{rec: rec}
}

func (r recorder) NominatePod(pod *v1.Pod, node *v1.Node) {
	r.rec.Eventf(pod, "Normal", "NominatePod", "Pod should schedule on %s", node.Name)
}

func (r recorder) PodFailedToSchedule(pod *v1.Pod, err error) {
	r.rec.Eventf(pod, "Warning", "FailedProvisioning", "Failed to provision new node, %s", err)
}

func (r recorder) NodeFailedToDrain(node *v1.Node, err error) {
	r.rec.Eventf(node, "Warning", "FailedDraining", "Failed to drain node, %s", err)
}
