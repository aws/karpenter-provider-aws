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
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.uber.org/multierr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/injection"
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
	readinessChan chan struct{} // A signal to other controllers that infrastructure is in a good state
	ready         bool
}

// pollingPeriod is the period that we go to AWS APIs to ensure that the appropriate AWS infrastructure is provisioned
// This period can be reduced to a backoffPeriod if there is an error in ensuring the infrastructure
const pollingPeriod = time.Hour

// defaultBackoffPeriod is the default period that we go to AWS APIs to ensure that the appropriate AWS infrastructure
// is provisioned if there is an error in the reconciliation loop
const defaultBackoffPeriod = time.Minute * 10

func NewController(ctx context.Context, cleanupCtx context.Context, kubeClient client.Client, clk clock.Clock,
	recorder events.Recorder, sqsProvider *aws.SQSProvider, eventBridgeProvider *aws.EventBridgeProvider,
	startAsync <-chan struct{}, cleanupAsync <-chan os.Signal) *Controller {
	c := &Controller{
		kubeClient:          kubeClient,
		recorder:            recorder,
		clock:               clk,
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
		mutex:               &sync.RWMutex{},
		readinessChan:       make(chan struct{}),
	}

	go func() {
		<-cleanupAsync
		c.cleanup(cleanupCtx)
	}()

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
			c.setReady(ctx, false)
			backoffPeriod := c.getBackoff(err)

			// Backoff with a shorter polling interval if we fail to ensure the infrastructure
			select {
			case <-ctx.Done():
				return
			case <-c.clock.After(backoffPeriod):
				continue
			}
		}
		c.setReady(ctx, true)
		select {
		case <-ctx.Done():
			return
		case <-c.clock.After(pollingPeriod):
		}
	}
}

func (c *Controller) cleanup(ctx context.Context) {
	logging.WithLogger(ctx, logging.FromContext(ctx).Named("infrastructure.cleanup"))

	dep := &appsv1.Deployment{}
	nn := types.NamespacedName{
		Name:      injection.GetOptions(ctx).DeploymentName,
		Namespace: injection.GetOptions(ctx).DeploymentNamespace,
	}

	err := c.kubeClient.Get(ctx, nn, dep)
	if err != nil {
		logging.FromContext(ctx).Errorf("Getting the deployment %s for cleanup, %v", nn, err)
	}

	// Deployment is deleting so we should cleanup the infrastructure
	if !dep.DeletionTimestamp.IsZero() {
		err = c.deleteInfrastructure(ctx)
		if err != nil {
			logging.FromContext(ctx).Errorf("Deleting the infrastructure, %v", err)
		}
	}
}

func (c *Controller) Ready() <-chan struct{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.readinessChan
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
		}
		close(c.readinessChan)
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

func (c *Controller) ensureInfrastructure(ctx context.Context) (err error) {
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

func (c *Controller) deleteInfrastructure(ctx context.Context) (err error) {
	logging.FromContext(ctx).Infof("Deprovisioning the infrastructure...")
	wg := &sync.WaitGroup{}
	m := &sync.Mutex{}

	wg.Add(2)
	go func() {
		defer wg.Done()
		e := c.sqsProvider.DeleteQueue(ctx)
		m.Lock()
		err = multierr.Append(err, e)
		m.Unlock()
	}()
	go func() {
		defer wg.Done()
		e := c.eventBridgeProvider.DeleteEC2NotificationRules(ctx)
		m.Lock()
		err = multierr.Append(err, e)
		m.Unlock()
	}()
	wg.Wait()
	time.Sleep(time.Minute)
	return err
}

func (c *Controller) ensureQueue(ctx context.Context) error {
	// Attempt to find the queue. If we can't find it, assume it isn't created and try to create it
	// If we did find it, then just set the queue attributes on the existing queue
	if _, err := c.sqsProvider.DiscoverQueueURL(ctx, true); err != nil {
		var awsErr awserr.Error
		if !errors.As(err, &awsErr) {
			// This shouldn't happen, but if it does, we should capture it
			return fmt.Errorf("failed conversion to AWS error, %w", err)
		}
		switch awsErr.Code() {
		case sqs.ErrCodeQueueDoesNotExist:
			logging.FromContext(ctx).Infof("Creating the SQS queue for EC2 notifications...")
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
			// This shouldn't happen, but if it does, we should capture it
			return fmt.Errorf("failed conversion to AWS error, %w", err)
		}
		switch awsErr.Code() {
		case aws.AccessDeniedException:
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
		return defaultBackoffPeriod
	}
	switch awsErr.Code() {
	case sqs.ErrCodeQueueDeletedRecently:
		return time.Minute * 2
	default:
		return defaultBackoffPeriod
	}
}
