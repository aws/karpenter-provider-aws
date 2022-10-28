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

package providers

import (
	"context"
	"fmt"

	"go.uber.org/multierr"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/errors"
)

type Infrastructure struct {
	sqsProvider         *SQS
	eventBridgeProvider *EventBridge
}

func NewInfrastructure(sqsProvider *SQS, eventBridgeProvider *EventBridge) *Infrastructure {
	return &Infrastructure{
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
	}
}

// Create provisions an SQS queue and EventBridge rules to enable interruption handling
func (p *Infrastructure) Create(ctx context.Context) error {
	if err := p.ensureQueue(ctx); err != nil {
		return fmt.Errorf("ensuring queue, %w", err)
	}
	if err := p.ensureEventBridge(ctx); err != nil {
		return fmt.Errorf("ensuring eventBridge rules and targets, %w", err)
	}
	logging.FromContext(ctx).Infof("Completed reconciliation of infrastructure")
	return nil
}

// Delete removes the infrastructure that was stood up and reconciled
// by the infrastructure controller for SQS message polling
func (p *Infrastructure) Delete(ctx context.Context) error {
	logging.FromContext(ctx).Infof("Deprovisioning the infrastructure...")

	deleteQueueFunc := func() error {
		logging.FromContext(ctx).Debugf("Deleting the SQS notification queue...")
		return p.sqsProvider.DeleteQueue(ctx)
	}
	deleteEventBridgeRulesFunc := func() error {
		logging.FromContext(ctx).Debugf("Deleting the EventBridge notification rules...")
		return p.eventBridgeProvider.DeleteEC2NotificationRules(ctx)
	}
	funcs := []func() error{
		deleteQueueFunc,
		deleteEventBridgeRulesFunc,
	}
	errs := make([]error, len(funcs))
	workqueue.ParallelizeUntil(ctx, len(funcs), len(funcs), func(i int) {
		errs[i] = funcs[i]()
	})

	err := multierr.Combine(errs...)
	if err != nil {
		return err
	}
	logging.FromContext(ctx).Infof("Completed deprovisioning the infrastructure")
	return nil
}

// ensureQueue reconciles the SQS queue with the configuration prescribed by Karpenter
func (p *Infrastructure) ensureQueue(ctx context.Context) error {
	// Attempt to find the queue. If we can't find it, assume it isn't created and try to create it
	// If we did find it, then just set the queue attributes on the existing queue
	logging.FromContext(ctx).Debugf("Reconciling the SQS notification queue...")
	if _, err := p.sqsProvider.DiscoverQueueURL(ctx, true); err != nil {
		switch {
		case errors.IsNotFound(err):
			logging.FromContext(ctx).Debugf("Queue not found, creating the SQS notification queue...")
			if err := p.sqsProvider.CreateQueue(ctx); err != nil {
				return fmt.Errorf("creating sqs queue with policy, %w", err)
			}
			logging.FromContext(ctx).Debugf("Successfully created the SQS notification queue")
		default:
			return fmt.Errorf("discovering sqs queue url, %w", err)
		}
	}
	// Always attempt to set the queue attributes, even after creation to help set the queue policy
	if err := p.sqsProvider.SetQueueAttributes(ctx, nil); err != nil {
		return fmt.Errorf("setting queue attributes for queue, %w", err)
	}
	logging.FromContext(ctx).Debugf("Successfully reconciled SQS queue")
	return nil
}

// ensureEventBridge reconciles the Eventbridge rules with the configuration prescribed by Karpenter
func (p *Infrastructure) ensureEventBridge(ctx context.Context) error {
	logging.FromContext(ctx).Debugf("Reconciling the EventBridge event rules...")
	if err := p.eventBridgeProvider.CreateEC2EventRules(ctx); err != nil {
		return fmt.Errorf("creating EventBridge event rules, %w", err)
	}
	logging.FromContext(ctx).Debugf("Successfully reconciled EventBridge event rules")
	return nil
}
