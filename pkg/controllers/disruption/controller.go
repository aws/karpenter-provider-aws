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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/cloudprovider"
)

type Controller struct {
	kubeClient    client.Client
	clock         clock.Clock
	cloudProvider cloudprovider.CloudProvider
	eventChan     chan Event
}

func NewController(ctx context.Context, clk clock.Clock, kubeClient client.Client,
	cloudProvider cloudprovider.CloudProvider, startAsync <-chan struct{}) *Controller {
	c := &Controller{
		kubeClient:    kubeClient,
		clock:         clk,
		cloudProvider: cloudProvider,
		eventChan:     make(chan Event, 10000),
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("disruption"))
	logging.FromContext(ctx).Infof("Starting controller")

	go func() {
		ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("processor"))
		defer func() {
			logging.FromContext(ctx).Infof("Shutting down")
		}()
		select {
		case <-ctx.Done():
			return
		case <-startAsync:
			c.process(ctx)
		}
	}()
	go func() {
		ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("subscriber"))
		defer func() {
			logging.FromContext(ctx).Infof("Shutting down")
		}()
		select {
		case <-ctx.Done():
			return
		case <-startAsync:
			c.subscribe(ctx)
		}
	}()

	return c
}

func (c *Controller) subscribe(ctx context.Context) {
	for msg := range c.cloudProvider.NodeEventWatcher() {
		if ctx.Err() != nil {
			return
		}
		c.eventChan <- NewEventFromCloudProviderEvent(msg, c.clock)
	}
}

func (c *Controller) process(ctx context.Context) {
	for msg := range c.eventChan {
		if ctx.Err() != nil {
			return
		}
		ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("source", msg.Source, "eventType", msg.Type, "node", msg.InvolvedObject))

		// This allows us to re-process a message that has completed but hasn't run its onComplete action yet
		if !msg.Completed {
			switch msg.Type {
			case DeleteEvent:
				if err := c.onDelete(ctx, msg.InvolvedObject); err != nil {
					logging.FromContext(ctx).Errorf("Deleting node from cloudprovider event-watcher, %v", err)
					// Kickoff a backoff goroutine that will re-add the message to the channel after the next backoff
					// duration stored in the message
					go c.backoff(ctx, msg)
				}
			default:
				logging.FromContext(ctx).Errorf("Received an unknown event type on cloudprovider event-watcher")
				continue
			}
			msg.Completed = true
		}
		if err := msg.OnComplete(); err != nil {
			logging.FromContext(ctx).Errorf("Running the complete action, %v", err)
			go c.backoff(ctx, msg)
		}
	}
}

func (c *Controller) backoff(ctx context.Context, msg Event) {
	select {
	case <-ctx.Done():
		return
	case <-c.clock.After(msg.Backoff.NextBackOff()):
		c.eventChan <- msg
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
