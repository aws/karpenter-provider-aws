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

	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/events"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/notification"
)

// EventRecorder is a mock event recorder that is used to facilitate testing.
type EventRecorder struct {
	EC2SpotInterruptionWarningCalled     atomic.Int64
	EC2SpotRebalanceRecommendationCalled atomic.Int64
	EC2HealthWarningCalled               atomic.Int64
	EC2StateStoppingCalled               atomic.Int64
	EC2StateTerminatingCalled            atomic.Int64
	TerminatingOnNotificationCalled      atomic.Int64
}

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{}
}

func (e *EventRecorder) Create(evt events.Event) {
	switch evt.Reason() {
	case notification.EC2SpotInterruptionWarning{}.Reason():
		e.EC2SpotInterruptionWarningCalled.Add(1)
	case notification.EC2SpotRebalanceRecommendation{}.Reason():
		e.EC2SpotRebalanceRecommendationCalled.Add(1)
	case notification.EC2HealthWarning{}.Reason():
		e.EC2HealthWarningCalled.Add(1)
	case notification.EC2StateTerminating{}.Reason():
		e.EC2StateTerminatingCalled.Add(1)
	case notification.EC2StateStopping{}.Reason():
		e.EC2StateStoppingCalled.Add(1)
	case notification.TerminatingNodeOnNotification{}.Reason():
		e.TerminatingOnNotificationCalled.Add(1)
	}
}
