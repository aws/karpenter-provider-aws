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

import v1 "k8s.io/api/core/v1"

// Recorder is used to record events that occur about pods so they can be viewed by looking at the pod's events so our
// actions are more observable without requiring log inspection
type Recorder interface {
	// PodShouldSchedule is called when we have determined that a pod should schedule against an existing node and don't
	// currently need to provision new capacity for the pod.
	PodShouldSchedule(pod *v1.Pod, node *v1.Node)
	// PodFailedToSchedule is called when a pod has failed to schedule entirely.
	PodFailedToSchedule(pod *v1.Pod, err error)
}

// TODO: Remove this type and actually record events onto pods as part of https://github.com/aws/karpenter/issues/1584
// this will require some new permissions in order to create events
type NoOpRecorder struct {
}

func (n *NoOpRecorder) PodShouldSchedule(pod *v1.Pod, node *v1.Node) {}
func (n *NoOpRecorder) PodFailedToSchedule(pod *v1.Pod, err error)   {}
