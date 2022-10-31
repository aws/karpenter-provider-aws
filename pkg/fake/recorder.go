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

	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter-core/pkg/test"
	interruptionevents "github.com/aws/karpenter/pkg/controllers/interruption/events"
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

func (e *EventRecorder) Publish(evt events.Event) {
	switch evt.Reason {
	case interruptionevents.InstanceSpotInterrupted(test.Node()).Reason:
		e.InstanceSpotInterruptedCalled.Add(1)
	case interruptionevents.InstanceRebalanceRecommendation(test.Node()).Reason:
		e.InstanceRebalanceRecommendationCalled.Add(1)
	case interruptionevents.InstanceUnhealthy(test.Node()).Reason:
		e.InstanceUnhealthyCalled.Add(1)
	case interruptionevents.InstanceTerminating(test.Node()).Reason:
		e.InstanceTerminatingCalled.Add(1)
	case interruptionevents.InstanceStopping(test.Node()).Reason:
		e.InstanceStoppingCalled.Add(1)
	case interruptionevents.NodeTerminatingOnInterruption(test.Node()).Reason:
		e.NodeTerminatingOnInterruptionCalled.Add(1)
	}
}
