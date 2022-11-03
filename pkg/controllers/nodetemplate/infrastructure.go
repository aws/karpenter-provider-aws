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

package nodetemplate

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/multierr"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/metrics"
	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/controllers/providers"
)

type Infrastructure struct {
	kubeClient          client.Client
	sqsProvider         *providers.SQS
	eventBridgeProvider *providers.EventBridge

	lastInfrastructureReconcile time.Time // Keeps track of the last reconcile time for infra, so we don't keep calling APIs
}

// Reconcile reconciles the infrastructure based on whether interruption handling is enabled and deletes
// the infrastructure by ref-counting when the last AWSNodeTemplate is removed
func (i *Infrastructure) Reconcile(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	if awssettings.FromContext(ctx).EnableInterruptionHandling {
		list := &v1alpha1.AWSNodeTemplateList{}
		if err := i.kubeClient.List(ctx, list); err != nil {
			return reconcile.Result{}, err
		}
		if !nodeTemplate.DeletionTimestamp.IsZero() && len(list.Items) == 1 {
			if err := i.DeleteInfrastructure(ctx); err != nil {
				return reconcile.Result{}, err
			}
			i.lastInfrastructureReconcile = time.Time{}
			return reconcile.Result{}, nil
		} else if len(list.Items) >= 1 {
			if i.lastInfrastructureReconcile.Add(time.Hour).Before(time.Now()) {
				if err := i.CreateInfrastructure(ctx); err != nil {
					infrastructureHealthy.Set(0)
					return reconcile.Result{}, err
				}
				i.lastInfrastructureReconcile = time.Now()
				infrastructureHealthy.Set(1)
			}
		}
	} else {
		infrastructureHealthy.Set(0)
	}

	// TODO: Implement an alerting mechanism for settings updates; until then, just poll
	return reconcile.Result{RequeueAfter: time.Second * 10}, nil
}

// CreateInfrastructure provisions an SQS queue and EventBridge rules to enable interruption handling
func (i *Infrastructure) CreateInfrastructure(ctx context.Context) error {
	defer metrics.Measure(infrastructureCreateDuration)()
	if err := i.ensureQueue(ctx); err != nil {
		return fmt.Errorf("ensuring queue, %w", err)
	}
	if err := i.ensureEventBridge(ctx); err != nil {
		return fmt.Errorf("ensuring eventBridge rules and targets, %w", err)
	}
	logging.FromContext(ctx).Infof("Completed reconciliation of infrastructure")
	return nil
}

// DeleteInfrastructure removes the infrastructure that was stood up and reconciled
// by the infrastructure controller for SQS message polling
func (i *Infrastructure) DeleteInfrastructure(ctx context.Context) error {
	defer metrics.Measure(infrastructureDeleteDuration)()
	logging.FromContext(ctx).Infof("Deprovisioning the infrastructure...")
	funcs := []func(context.Context) error{
		i.deleteQueue,
		i.deleteEventBridge,
	}
	errs := make([]error, len(funcs))
	workqueue.ParallelizeUntil(ctx, len(funcs), len(funcs), func(i int) {
		errs[i] = funcs[i](ctx)
	})

	err := multierr.Combine(errs...)
	if err != nil {
		return err
	}
	logging.FromContext(ctx).Infof("Completed deprovisioning the infrastructure")
	return nil
}

// ensureQueue reconciles the SQS queue with the configuration prescribed by Karpenter
func (i *Infrastructure) ensureQueue(ctx context.Context) error {
	// Attempt to find the queue. If we can't find it, assume it isn't created and try to create it
	// If we did find it, then just set the queue attributes on the existing queue
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("queueName", i.sqsProvider.QueueName(ctx)))
	logging.FromContext(ctx).Debugf("Reconciling the SQS interruption queue...")
	queueExists, err := i.sqsProvider.QueueExists(ctx)
	if err != nil {
		return fmt.Errorf("checking queue existence, %w", err)
	}
	if !queueExists {
		logging.FromContext(ctx).Debugf("Queue not found, creating the SQS interruption queue...")
		if err := i.sqsProvider.CreateQueue(ctx); err != nil {
			return fmt.Errorf("creating sqs queue with policy, %w", err)
		}
		logging.FromContext(ctx).Debugf("Successfully created the SQS interruption queue")
	}
	// Always attempt to set the queue attributes, even after creation to help set the queue policy
	if err := i.sqsProvider.SetQueueAttributes(ctx, nil); err != nil {
		return fmt.Errorf("setting queue attributes for queue, %w", err)
	}
	logging.FromContext(ctx).Debugf("Successfully reconciled SQS queue")
	return nil
}

func (i *Infrastructure) deleteQueue(ctx context.Context) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("queueName", i.sqsProvider.QueueName(ctx)))
	logging.FromContext(ctx).Debugf("Deleting the SQS interruption queue...")
	return i.sqsProvider.DeleteQueue(ctx)
}

// ensureEventBridge reconciles the Eventbridge rules with the configuration prescribed by Karpenter
func (i *Infrastructure) ensureEventBridge(ctx context.Context) error {
	logging.FromContext(ctx).Debugf("Reconciling the EventBridge event rules...")
	if err := i.eventBridgeProvider.CreateRules(ctx); err != nil {
		return fmt.Errorf("creating EventBridge event rules, %w", err)
	}
	logging.FromContext(ctx).Debugf("Successfully reconciled EventBridge event rules")
	return nil
}

func (i *Infrastructure) deleteEventBridge(ctx context.Context) error {
	logging.FromContext(ctx).Debugf("Deleting the EventBridge interruption rules...")
	return i.eventBridgeProvider.DeleteRules(ctx)
}
