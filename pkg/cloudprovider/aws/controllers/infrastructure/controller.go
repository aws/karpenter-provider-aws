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
	"github.com/cenkalti/backoff/v4"
	"go.uber.org/multierr"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
	"github.com/aws/karpenter/pkg/metrics"
)

// Controller is the AWS infrastructure controller.  It is not a standard controller-runtime controller in that it doesn't
// have a reconcile method.
type Controller struct {
	kubeClient client.Client
	recorder   events.Recorder
	clock      clock.Clock

	sqsProvider         *aws.SQSProvider
	eventBridgeProvider *aws.EventBridgeProvider

	mutex         *sync.RWMutex
	backoff       *backoff.ExponentialBackOff
	readinessChan chan struct{} // A signal to other controllers that infrastructure is in a good state
	ready         bool
	trigger       chan struct{}
}

// pollingPeriod is the period that we go to AWS APIs to ensure that the appropriate AWS infrastructure is provisioned
// This period can be reduced to a backoffPeriod if there is an error in ensuring the infrastructure
const pollingPeriod = time.Hour

func NewController(ctx context.Context, kubeClient client.Client, clk clock.Clock,
	recorder events.Recorder, sqsProvider *aws.SQSProvider, eventBridgeProvider *aws.EventBridgeProvider,
	startAsync <-chan struct{}) *Controller {

	c := &Controller{
		kubeClient:          kubeClient,
		recorder:            recorder,
		clock:               clk,
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
		mutex:               &sync.RWMutex{},
		backoff:             newBackoff(clk),
		readinessChan:       make(chan struct{}),
		trigger:             make(chan struct{}, 1),
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("infrastructure"))
	logging.FromContext(ctx).Infof("Starting controller")

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

func newBackoff(clk clock.Clock) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Minute
	b.MaxElapsedTime = time.Minute * 30
	b.Clock = clk
	return b
}

func (c *Controller) run(ctx context.Context) {
	defer logging.FromContext(ctx).Infof("Shutting down")
	for {
		if err := c.Reconcile(ctx); err != nil {
			logging.FromContext(ctx).Errorf("ensuring infrastructure established, %v", err)
			c.setReady(ctx, false)
			backoffPeriod := c.getBackoff(err)

			// Backoff with a shorter polling interval if we fail to ensure the infrastructure
			select {
			case <-ctx.Done():
				return
			case <-c.trigger:
				continue
			case <-c.clock.After(backoffPeriod):
				continue
			}
		}
		c.setReady(ctx, true)
		c.backoff.Reset()
		select {
		case <-ctx.Done():
			return
		case <-c.trigger:
		case <-c.clock.After(pollingPeriod):
		}
	}
}

// Ready returns a channel that serves as a gate for other controllers
// to wait on the infrastructure to be in a good state. When the infrastructure is ready,
// this channel is closed so other controllers can proceed with their operations
func (c *Controller) Ready() <-chan struct{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.readinessChan
}

func (c *Controller) Trigger() {
	c.trigger <- struct{}{}
}

func (c *Controller) setReady(ctx context.Context, ready bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If the infrastructure we close the readiness channel to let all
	// other channels that are waiting on Ready() proceed; otherwise, open
	// a channel to tell the other goroutines to wait
	if ready {
		healthy.Set(1)
		if c.ready != ready {
			logging.FromContext(ctx).Infof("Infrastructure is healthy")
			c.recorder.InfrastructureHealthy(ctx, c.kubeClient)
			close(c.readinessChan)
		}
	} else {
		healthy.Set(0)
		if c.ready != ready {
			logging.FromContext(ctx).Infof("Infrastructure is unhealthy")
			c.recorder.InfrastructureUnhealthy(ctx, c.kubeClient)
		}
		c.readinessChan = make(chan struct{})
	}
	c.ready = ready
}

// Reconcile reconciles the SQS queue and the EventBridge rules with the expected
// configuration prescribed by Karpenter
func (c *Controller) Reconcile(ctx context.Context) (err error) {
	defer metrics.Measure(reconcileDuration)()

	wg := &sync.WaitGroup{}
	m := &sync.Mutex{}

	wg.Add(2)
	go func() {
		defer wg.Done()
		e := c.ensureQueue(ctx)
		m.Lock()
		err = multierr.Append(err, e)
		m.Unlock()
	}()
	go func() {
		defer wg.Done()
		e := c.ensureEventBridge(ctx)
		m.Lock()
		err = multierr.Append(err, e)
		m.Unlock()
	}()
	wg.Wait()
	return err
}

// DeleteInfrastructure removes the infrastructure that was stood up and reconciled
// by the infrastructure controller for SQS message polling
func (c *Controller) DeleteInfrastructure(ctx context.Context) (err error) {
	logging.FromContext(ctx).Infof("Deprovisioning the infrastructure...")
	wg := &sync.WaitGroup{}
	m := &sync.Mutex{}

	wg.Add(2)
	go func() {
		defer wg.Done()
		logging.FromContext(ctx).Debugf("Deleting the SQS notification queue...")
		e := c.sqsProvider.DeleteQueue(ctx)

		// If we get access denied, nothing we can do so just log and don't return the error
		if aws.IsAccessDenied(e) {
			logging.FromContext(ctx).Errorf("Access denied while trying to delete SQS queue, %v", err)
		} else if err != nil {
			m.Lock()
			err = multierr.Append(err, e)
			m.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		logging.FromContext(ctx).Debugf("Deleting the EventBridge notification rules...")
		e := c.eventBridgeProvider.DeleteEC2NotificationRules(ctx)

		// If we get access denied, nothing we can do so just log and don't return the error
		if aws.IsAccessDenied(e) {
			logging.FromContext(ctx).Errorf("Access denied while trying to delete notification rules, %v", err)
		} else if err != nil {
			m.Lock()
			err = multierr.Append(err, e)
			m.Unlock()
		}
	}()
	wg.Wait()
	if err != nil {
		c.recorder.InfrastructureDeletionFailed(ctx, c.kubeClient)
		return err
	}
	logging.FromContext(ctx).Infof("Completed deprovisioning the infrastructure")
	c.recorder.InfrastructureDeletionSucceeded(ctx, c.kubeClient)
	return nil
}

// ensureQueue reconciles the SQS queue with the configuration prescribed by Karpenter
func (c *Controller) ensureQueue(ctx context.Context) error {
	// Attempt to find the queue. If we can't find it, assume it isn't created and try to create it
	// If we did find it, then just set the queue attributes on the existing queue
	logging.FromContext(ctx).Debugf("Reconciling the SQS notification queue")
	if _, err := c.sqsProvider.DiscoverQueueURL(ctx, true); err != nil {
		switch {
		case aws.IsNotFound(err):
			logging.FromContext(ctx).Debugf("Queue not found, creating the SQS notification queue...")
			if err := c.sqsProvider.CreateQueue(ctx); err != nil {
				return fmt.Errorf("creating sqs queue with policy, %w", err)
			}
			logging.FromContext(ctx).Debugf("Successfully created the SQS notification queue")
			return nil
		case aws.IsAccessDenied(err):
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

// ensureEventBridge reconciles the Eventbridge rules with the configuration prescribed by Karpenter
func (c *Controller) ensureEventBridge(ctx context.Context) error {
	logging.FromContext(ctx).Debugf("Reconciling the EventBridge notification rules")
	if err := c.eventBridgeProvider.CreateEC2NotificationRules(ctx); err != nil {
		switch {
		case aws.IsAccessDenied(err):
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
		return c.backoff.NextBackOff()
	}
	switch awsErr.Code() {
	case sqs.ErrCodeQueueDeletedRecently:
		// We special-case this error since the queue can be created here much quicker
		return time.Minute
	default:
		return c.backoff.NextBackOff()
	}
}
