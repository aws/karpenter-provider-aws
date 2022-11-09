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
	"github.com/aws/karpenter/pkg/errors"
)

type InfrastructureReconciler struct {
	kubeClient          client.Client
	sqsProvider         *providers.SQS
	eventBridgeProvider *providers.EventBridge

	lastInfrastructureReconcile time.Time // Keeps track of the last reconcile time for infra, so we don't keep calling APIs
}

func NewInfrastructureReconciler(kubeClient client.Client, sqsProvider *providers.SQS, eventBridgeProvider *providers.EventBridge) *InfrastructureReconciler {
	return &InfrastructureReconciler{
		kubeClient:          kubeClient,
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
	}
}

// Reconcile reconciles the infrastructure based on whether interruption handling is enabled and deletes
// the infrastructure by ref-counting when the last AWSNodeTemplate is removed
func (i *InfrastructureReconciler) Reconcile(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	if !awssettings.FromContext(ctx).EnableInterruptionHandling {
		// TODO: Implement an alerting mechanism for settings updates; until then, just poll
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}
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
		if i.lastInfrastructureReconcile.Add(time.Minute * 5).Before(time.Now()) {
			if err := i.CreateInfrastructure(ctx); err != nil {
				if errors.IsRecentlyDeleted(err) {
					logging.FromContext(ctx).Errorf("Interruption queue recently deleted, retrying after one minute")
					return reconcile.Result{RequeueAfter: time.Minute}, nil
				}
				return reconcile.Result{}, err
			}
			i.lastInfrastructureReconcile = time.Now()
		}
	}
	// TODO: Implement an alerting mechanism for settings updates; until then, just poll
	return reconcile.Result{RequeueAfter: time.Second * 10}, nil
}

// CreateInfrastructure provisions an SQS queue and EventBridge rules to enable interruption handling
func (i *InfrastructureReconciler) CreateInfrastructure(ctx context.Context) error {
	defer metrics.Measure(infrastructureCreateDuration)()
	if err := i.ensureQueue(ctx); err != nil {
		return fmt.Errorf("ensuring queue, %w", err)
	}
	if err := i.ensureEventBridge(ctx); err != nil {
		return fmt.Errorf("ensuring eventBridge rules and targets, %w", err)
	}
	logging.FromContext(ctx).Debugf("Reconciled the interruption-handling infrastructure")
	return nil
}

// DeleteInfrastructure removes the infrastructure that was stood up and reconciled
// by the infrastructure controller for SQS message polling
func (i *InfrastructureReconciler) DeleteInfrastructure(ctx context.Context) error {
	defer metrics.Measure(infrastructureDeleteDuration)()
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
	logging.FromContext(ctx).Debugf("Deleted the interruption-handling infrastructure")
	return nil
}

// ensureQueue reconciles the SQS queue with the configuration prescribed by Karpenter
func (i *InfrastructureReconciler) ensureQueue(ctx context.Context) error {
	// Attempt to find the queue. If we can't find it, assume it isn't created and try to create it
	// If we did find it, then just set the queue attributes on the existing queue
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("queueName", i.sqsProvider.QueueName(ctx)))
	queueExists, err := i.sqsProvider.QueueExists(ctx)
	if err != nil {
		return fmt.Errorf("checking the SQS interruption queue existence, %w", err)
	}
	if !queueExists {
		logging.FromContext(ctx).Debugf("Interruption queue not found, creating the SQS interruption queue")
		if err := i.sqsProvider.CreateQueue(ctx); err != nil {
			return fmt.Errorf("creating the SQS interruption queue with policy, %w", err)
		}
	}
	// Always attempt to set the queue attributes, even after creation to help set the queue policy
	if err := i.sqsProvider.SetQueueAttributes(ctx, nil); err != nil {
		return fmt.Errorf("setting queue attributes for interruption queue, %w", err)
	}
	return nil
}

func (i *InfrastructureReconciler) deleteQueue(ctx context.Context) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("queueName", i.sqsProvider.QueueName(ctx)))
	if err := i.sqsProvider.DeleteQueue(ctx); err != nil {
		return fmt.Errorf("deleting the the SQS interruption queue, %w", err)
	}
	return nil
}

// ensureEventBridge reconciles the EventBridge rules with the configuration prescribed by Karpenter
func (i *InfrastructureReconciler) ensureEventBridge(ctx context.Context) error {
	if err := i.eventBridgeProvider.CreateRules(ctx); err != nil {
		return fmt.Errorf("creating EventBridge interruption rules, %w", err)
	}
	return nil
}

func (i *InfrastructureReconciler) deleteEventBridge(ctx context.Context) error {
	if err := i.eventBridgeProvider.DeleteRules(ctx); err != nil {
		return fmt.Errorf("deleting the EventBridge interruption rules, %w", err)
	}
	return nil
}
