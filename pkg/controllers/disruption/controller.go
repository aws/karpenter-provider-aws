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
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/utils/atomic"
)

// Controller is the disruption controller.
// It is not a standard controller-runtime controller in that it doesn't have a reconcile method.
type Controller struct {
	kubeClient client.Client
	clock      clock.Clock

	// We use an atomic event queue here so that we can dynamically size our event queue as we get
	// more events, and we don't block when we need to backoff
	eventQueue *atomic.List[apis.Event]

	mu            sync.Mutex
	subscriptions []<-chan apis.ConvertibleToEvent
}

func NewController(ctx context.Context, clk clock.Clock, kubeClient client.Client,
	startAsync <-chan struct{}) *Controller {
	c := &Controller{
		kubeClient: kubeClient,
		clock:      clk,
		eventQueue: atomic.NewList[apis.Event](),
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("disruption"))
	logging.FromContext(ctx).Infof("Starting controller")

	go func() {
		innerCtx := logging.WithLogger(ctx, logging.FromContext(ctx).Named("processor"))
		defer logging.FromContext(innerCtx).Infof("Shutting down")
		select {
		case <-innerCtx.Done():
			return
		case <-startAsync:
			c.Process(innerCtx)
		}
	}()
	go func() {
		innerCtx := logging.WithLogger(ctx, logging.FromContext(ctx).Named("subscriber"))
		defer logging.FromContext(innerCtx).Infof("Shutting down")
		select {
		case <-innerCtx.Done():
			return
		case <-startAsync:
			c.Subscribe(innerCtx)
		}
	}()

	return c
}

// Subscribe causes the controller to start watching every channel that has been registered in subscriptions
// prior to the initialization of the controller
func (c *Controller) Subscribe(ctx context.Context) {
	wg := &sync.WaitGroup{}
	c.mu.Lock()
	for _, subscriber := range c.subscriptions {
		wg.Add(1)
		go func(s <-chan apis.ConvertibleToEvent) {
			defer wg.Done()
			c.subscribeToSubscription(ctx, s)
		}(subscriber)
	}
	c.mu.Unlock()
	wg.Wait()
}

// RegisterSubscription adds the channel as a subscription for the controller
func (c *Controller) RegisterSubscription(subscription <-chan apis.ConvertibleToEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions = append(c.subscriptions, subscription)
}

// subscribeToSubscription maps the process for reading for the subscription channel and adding events
// from the subscription in the eventQueue
func (c *Controller) subscribeToSubscription(ctx context.Context, subscription <-chan apis.ConvertibleToEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-subscription:
			c.eventQueue.PushBack(msg.ToEvent().WithBackoff(newBackoff(c.clock)))
		}
	}
}

// Process waits for events to be added to the event queue on a channel and then processes every message on the channel
func (c *Controller) Process(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.eventQueue.WaitForElems():
			for c.eventQueue.Len() > 0 {
				c.ProcessMessage(ctx, c.eventQueue.PopFront())
			}
		}
	}
}

// ProcessMessage determines the appropriate action to take for a message event, executes that action,
// and runs the onComplete action as an acknowledgement of processing the message
func (c *Controller) ProcessMessage(ctx context.Context, msg apis.Event) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("source", msg.Source, "eventType", msg.Type, "node", msg.InvolvedObject))

	// This allows us to re-process a message that has completed but hasn't run its onComplete action yet
	if !msg.Completed {
		switch msg.Type {
		case apis.DeleteEvent:
			if err := c.onDelete(ctx, msg.InvolvedObject); err != nil {
				logging.FromContext(ctx).Errorf("Deleting node from cloudprovider event-watcher, %v", err)
				// Kickoff a backoff goroutine that will re-add the message to the channel after the next backoff
				// duration stored in the message
				go c.backoffMessage(ctx, msg)
			}
		default:
			logging.FromContext(ctx).Errorf("Received an unknown event type on cloudprovider event-watcher")
			return
		}
		msg.Completed = true
	}
	if err := msg.OnComplete(); err != nil {
		logging.FromContext(ctx).Errorf("Running the complete action, %v", err)
		go c.backoffMessage(ctx, msg)
	}
}

func (c *Controller) backoffMessage(ctx context.Context, msg apis.Event) {
	select {
	case <-ctx.Done():
		return
	case <-c.clock.After(msg.Backoff.NextBackOff()):
		c.eventQueue.PushBack(msg)
	}
}

func (c *Controller) onDelete(ctx context.Context, key types.NamespacedName) error {
	n := &v1.Node{}
	if err := c.kubeClient.Get(ctx, key, n); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("getting node for deletion, %w", err)
	}
	if err := c.kubeClient.Delete(ctx, n); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("deleting node, %w", err)
	}
	return nil
}

func newBackoff(clk clock.Clock) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Minute
	b.MaxElapsedTime = time.Minute * 30
	b.Clock = clk
	return b
}
