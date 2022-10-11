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
	"k8s.io/client-go/tools/record"

	"github.com/aws/karpenter/pkg/test"
)

// EventRecorder is a mock event recorder that is used to facilitate testing.
type EventRecorder struct {
	test.Recorder

	EC2SpotInterruptionWarningCalled     atomic.Int64
	EC2SpotRebalanceRecommendationCalled atomic.Int64
	EC2HealthWarningCalled               atomic.Int64
	EC2StateStoppingCalled               atomic.Int64
	EC2StateTerminatingCalled            atomic.Int64
}

func (e *EventRecorder) EventRecorder() record.EventRecorder { return e.Recorder.EventRecorder() }

func (e *EventRecorder) EC2SpotInterruptionWarning(_ *v1.Node) {
	e.EC2SpotInterruptionWarningCalled.Add(1)
}

func (e *EventRecorder) EC2SpotRebalanceRecommendation(_ *v1.Node) {
	e.EC2SpotRebalanceRecommendationCalled.Add(1)
}

func (e *EventRecorder) EC2HealthWarning(_ *v1.Node) {
	e.EC2HealthWarningCalled.Add(1)
}

func (e *EventRecorder) EC2StateTerminating(_ *v1.Node) {
	e.EC2StateTerminatingCalled.Add(1)
}

func (e *EventRecorder) EC2StateStopping(_ *v1.Node) {
	e.EC2StateStoppingCalled.Add(1)
}

func (e *EventRecorder) TerminatingNodeOnNotification(_ *v1.Node) {}

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{
		Recorder: *test.NewRecorder(),
	}
}
