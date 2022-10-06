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
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/test"
)

// EventRecorder is a mock event recorder that is used to facilitate testing.
type EventRecorder struct {
	test.Recorder
}

func (e *EventRecorder) EventRecorder() record.EventRecorder { return e.Recorder.EventRecorder() }

func (e *EventRecorder) EC2SpotInterruptionWarning(_ *v1.Node) {}

func (e *EventRecorder) EC2SpotRebalanceRecommendation(_ *v1.Node) {}

func (e *EventRecorder) EC2HealthWarning(_ *v1.Node) {}

func (e *EventRecorder) EC2StateTerminating(_ *v1.Node) {}

func (e *EventRecorder) EC2StateStopping(_ *v1.Node) {}

func (e *EventRecorder) TerminatingNodeOnNotification(_ *v1.Node) {}

func (e *EventRecorder) InfrastructureUnhealthy(_ context.Context, _ client.Client) {}

func (e *EventRecorder) InfrastructureHealthy(_ context.Context, _ client.Client) {}

func (e *EventRecorder) InfrastructureDeletionSucceeded(_ context.Context, _ client.Client) {}

func (e *EventRecorder) InfrastructureDeletionFailed(_ context.Context, _ client.Client) {}

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{
		Recorder: *test.NewRecorder(),
	}
}
