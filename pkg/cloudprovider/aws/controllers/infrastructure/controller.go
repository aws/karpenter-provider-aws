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

package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	"golang.org/x/sync/errgroup"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
)

// Controller is the AWS infrastructure controller.  It is not a standard controller-runtime controller in that it doesn't
// have a reconcile method.
type Controller struct {
	sqsProvider         *aws.SQSProvider
	eventBridgeProvider *aws.EventBridgeProvider
	recorder            events.Recorder
	clock               clock.Clock

	mutex         *sync.RWMutex
	readinessChan chan struct{} // A signal to other controllers that infrastructure is in a good state
}

// pollingPeriod is the period that we go to AWS APIs to ensure that the appropriate AWS infrastructure is provisioned
const pollingPeriod = time.Hour

func NewController(ctx context.Context, clk clock.Clock, recorder events.Recorder,
	sqsProvider *aws.SQSProvider, eventBridgeProvider *aws.EventBridgeProvider, startAsync <-chan struct{}) *Controller {
	c := &Controller{
		recorder:            recorder,
		clock:               clk,
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
		mutex:               &sync.RWMutex{},
		readinessChan:       make(chan struct{}),
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-startAsync:
			c.run(ctx)
		}
	}()

	return c
}

func (c *Controller) run(ctx context.Context) {
	logger := logging.FromContext(ctx).Named("infrastructure")
	ctx = logging.WithLogger(ctx, logger)

	defer func() {
		logger.Infof("Shutting down")
	}()
	for {
		if err := c.ensureInfrastructure(ctx); err != nil {
			logging.FromContext(ctx).Errorf("ensuring infrastructure established, %v", err)
			c.setReady(false)
			backoffPeriod := c.getBackoff(err)

			// Backoff with a shorter polling interval if we fail to ensure the infrastructure
			select {
			case <-ctx.Done():
				return
			case <-c.clock.After(backoffPeriod):
				continue
			}
		}
		c.setReady(true)
		select {
		case <-ctx.Done():
			return
		case <-c.clock.After(pollingPeriod):
		}
	}
}

func (c *Controller) Ready() <-chan struct{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.readinessChan
}

func (c *Controller) setReady(ready bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If the infrastructure we close the readiness channel to let all
	// other channels that are waiting on Ready() proceed; otherwise, open
	// a channel to tell the other goroutines to wait
	if ready {
		close(c.readinessChan)
	} else {
		c.readinessChan = make(chan struct{})
	}
}

func (c *Controller) ensureInfrastructure(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return c.ensureQueue(ctx) })
	g.Go(func() error { return c.ensureEventBridge(ctx) })
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func (c *Controller) ensureQueue(ctx context.Context) error {
	// Attempt to find the queue. If we can't find it, assume it isn't created and try to create it
	// If we did find it, then just set the queue attributes on the existing queue
	if _, err := c.sqsProvider.DiscoverQueueURL(ctx, true); err != nil {
		var awsErr awserr.Error
		if !errors.As(err, &awsErr) {
			// This shouldn't happen, but if it does we should capture it
			return fmt.Errorf("failed conversion to AWS error, %w", err)
		}
		switch awsErr.Code() {
		case sqs.ErrCodeQueueDoesNotExist:
			if err := c.sqsProvider.CreateQueue(ctx); err != nil {
				return fmt.Errorf("creating sqs queue with policy, %w", err)
			}
			return nil
		case aws.AccessDeniedCode:
			return fmt.Errorf("failed obtaining permission to discover sqs queue url, %w", err)
		default:
			return fmt.Errorf("failed discovering sqs queue url, %w", err)
		}
	}
	if err := c.sqsProvider.SetQueueAttributes(ctx); err != nil {
		return fmt.Errorf("setting queue attributes for queue, %w", err)
	}
	return nil
}

func (c *Controller) ensureEventBridge(ctx context.Context) error {
	if err := c.eventBridgeProvider.CreateEC2NotificationRules(ctx); err != nil {
		var awsErr awserr.Error
		if !errors.As(err, &awsErr) {
			// This shouldn't happen, but if it does we should capture it
			return fmt.Errorf("failed conversion to AWS error, %w", err)
		}
		switch awsErr.Code() {
		case aws.AccessDeniedCode:
			return fmt.Errorf("obtaining permission to eventbridge, %w", err)
		default:
			return fmt.Errorf("creating event bridge notification rules, %w", err)
		}
	}
	return nil
}

// getBackoff gets a dynamic backoff timeframe based on the error
// that we receive from the AWS API
func (c *Controller) getBackoff(err error) time.Duration {
	var awsErr awserr.Error
	if !errors.As(err, &awsErr) {
		return time.Minute
	}
	switch awsErr.Code() {
	case aws.AccessDeniedCode:
		return time.Minute * 10
	default:
		return time.Minute
	}
}
