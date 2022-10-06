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
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/utils/atomic"
)

type EventQueue struct {
	clock clock.Clock

	// We use an atomic event queue here so that we can dynamically size our event queue as we get
	// more events, and we don't block when we need to backoff
	queue *atomic.List[apis.Event]

	mu         sync.Mutex
	watchables []<-chan apis.ConvertibleToEvent
}

func NewEventQueue(clk clock.Clock) *EventQueue {
	return &EventQueue{
		clock: clk,
		queue: atomic.NewList[apis.Event](),
	}
}

func (e *EventQueue) Start(ctx context.Context, startAsync <-chan struct{}) {
	go func() {
		innerCtx := logging.WithLogger(ctx, logging.FromContext(ctx).Named("watcher"))
		defer logging.FromContext(innerCtx).Infof("Shutting down")
		select {
		case <-innerCtx.Done():
			return
		case <-startAsync:
			e.Watch(innerCtx)
		}
	}()
}

// Watch causes the controller to start watching every channel that has been registered in watchables
// prior to the initialization of the controller
func (e *EventQueue) Watch(ctx context.Context) {
	wg := &sync.WaitGroup{}
	e.mu.Lock()
	for _, subscriber := range e.watchables {
		wg.Add(1)
		go func(s <-chan apis.ConvertibleToEvent) {
			defer wg.Done()
			e.subscribeToWatcher(ctx, s)
		}(subscriber)
	}
	e.mu.Unlock()
	wg.Wait()
}

// RegisterWatcher adds the channel as a watchable for the controller
func (e *EventQueue) RegisterWatcher(watchable <-chan apis.ConvertibleToEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.watchables = append(e.watchables, watchable)
}

// subscribeToWatcher maps the process for reading for the watchable channel and adding events
// from the watchable in the eventQueue
func (e *EventQueue) subscribeToWatcher(ctx context.Context, watchable <-chan apis.ConvertibleToEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-watchable:
			e.queue.PushBack(msg.ToEvent().WithBackoff(newBackoff(e.clock)))
		}
	}
}

func (e *EventQueue) Backoff(ctx context.Context, msg apis.Event) {
	select {
	case <-ctx.Done():
		return
	case <-e.clock.After(msg.Backoff.NextBackOff()):
		e.queue.PushBack(msg)
	}
}

func (e *EventQueue) WaitForElems() <-chan struct{} {
	return e.queue.WaitForElems()
}

func (e *EventQueue) ReadAll() []apis.Event {
	return e.queue.PopAll()
}

func newBackoff(clk clock.Clock) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Minute
	b.MaxElapsedTime = time.Minute * 30
	b.Clock = clk
	return b
}
