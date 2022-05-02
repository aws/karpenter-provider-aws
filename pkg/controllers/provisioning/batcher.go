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

package provisioning

import (
	"context"
	"time"
)

var (
	MaxBatchDuration  = time.Second * 10
	BatchIdleDuration = time.Second * 1
)

// Batcher separates a stream of Trigger() calls into windowed slices. The
// window is dynamic and will be extended if additional items are added up to a
// maximum batch duration.
type Batcher struct {
	running   context.Context
	trigger   chan struct{}
	immediate chan struct{}
}

// NewBatcher is a constructor for the Batcher
func NewBatcher(running context.Context) *Batcher {
	return &Batcher{
		running:   running,
		trigger:   make(chan struct{}), // triggering shouldn't block
		immediate: make(chan struct{}),
	}
}

// Trigger causes the batcher to start a batching window, or extend the current batching window if it hasn't reached the
// maximum length.
func (b *Batcher) Trigger() {
	// it's ok to miss a trigger as that means Wait() already has a trigger inbound
	select {
	case b.trigger <- struct{}{}:
	default:
	}
}

// TriggerImmediate causes the batcher to immediately end the current batching window and causes the waiter on the batching
// window to continue.
func (b *Batcher) TriggerImmediate() {
	b.immediate <- struct{}{}
}

// Wait starts a batching window and returns a slice of items when closed.
func (b *Batcher) Wait() (window time.Duration) {
	var start time.Time
	select {
	case <-b.trigger:
		// start the batching window after the first item is received
		start = time.Now()
	case <-b.immediate:
		// but for immediate triggering and context cancellations, end the batching window
		return
	case <-b.running.Done():
		return
	}

	defer func() {
		window = time.Since(start)
	}()

	timeout := time.NewTimer(MaxBatchDuration)
	idle := time.NewTimer(BatchIdleDuration)
	for {
		select {
		case <-b.trigger:
			if start.IsZero() {
				start = time.Now()
			}
			// correct way to reset an active timer per docs
			if !idle.Stop() {
				<-idle.C
			}
			idle.Reset(BatchIdleDuration)
		case <-b.immediate:
			return
		case <-timeout.C:
			return
		case <-idle.C:
			return
		}
	}
}
