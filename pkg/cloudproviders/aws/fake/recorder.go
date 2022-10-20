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

package fake

import (
	"sync/atomic"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	interruptionevents "github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/events"
)

// EventRecorder is a mock event recorder that is used to facilitate testing.
type EventRecorder struct {
	InstanceSpotInterruptedCalled         atomic.Int64
	InstanceRebalanceRecommendationCalled atomic.Int64
	InstanceUnhealthyCalled               atomic.Int64
	InstanceTerminatingCalled             atomic.Int64
	InstanceStoppingCalled                atomic.Int64
	NodeTerminatingOnInterruptionCalled   atomic.Int64
}

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{}
}

func (e *EventRecorder) Event(_ runtime.Object, _, reason, _ string) {
	fakeNode := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "fake"}}
	switch reason {
	case interruptionevents.InstanceSpotInterrupted(fakeNode).Reason:
		e.InstanceSpotInterruptedCalled.Add(1)
	case interruptionevents.InstanceRebalanceRecommendation(fakeNode).Reason:
		e.InstanceRebalanceRecommendationCalled.Add(1)
	case interruptionevents.InstanceUnhealthy(fakeNode).Reason:
		e.InstanceUnhealthyCalled.Add(1)
	case interruptionevents.InstanceTerminating(fakeNode).Reason:
		e.InstanceTerminatingCalled.Add(1)
	case interruptionevents.InstanceStopping(fakeNode).Reason:
		e.InstanceStoppingCalled.Add(1)
	case interruptionevents.NodeTerminatingOnInterruption(fakeNode).Reason:
		e.NodeTerminatingOnInterruptionCalled.Add(1)
	}
}

func (e *EventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, _ ...interface{}) {
	e.Event(object, eventtype, reason, messageFmt)
}

func (e *EventRecorder) AnnotatedEventf(object runtime.Object, _ map[string]string, eventtype, reason, messageFmt string, _ ...interface{}) {
	e.Event(object, eventtype, reason, messageFmt)
}
