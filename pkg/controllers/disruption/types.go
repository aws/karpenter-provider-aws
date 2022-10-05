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

package disruption

import (
	"time"

	"github.com/cenkalti/backoff/v4"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"

	"github.com/aws/karpenter/pkg/cloudprovider"
)

type EventType string

const (
	CreateEvent EventType = "Create"
	DeleteEvent EventType = "Delete"
)

type Event struct {
	Source         string
	Type           EventType
	InvolvedObject types.NamespacedName
	Completed      bool
	OnComplete     func() error
	Backoff        *backoff.ExponentialBackOff
}

func NewEventFromCloudProviderEvent(e cloudprovider.NodeEvent, clk clock.Clock) Event {
	return Event{
		Source:         e.Source,
		Type:           EventType(e.Type),
		InvolvedObject: e.InvolvedObject,
		OnComplete:     e.OnComplete,
		Backoff:        newBackoff(clk),
	}
}

func newBackoff(clk clock.Clock) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Minute
	b.MaxElapsedTime = time.Minute * 30
	b.Clock = clk
	return b
}
