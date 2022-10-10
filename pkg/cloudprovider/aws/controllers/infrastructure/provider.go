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
	"fmt"

	"go.uber.org/multierr"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
)

type Provider struct {
	sqsProvider         *aws.SQSProvider
	eventBridgeProvider *aws.EventBridgeProvider
}

func NewProvider(sqsProvider *aws.SQSProvider, eventBridgeProvider *aws.EventBridgeProvider) *Provider {
	return &Provider{
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
	}
}

func (p *Provider) CreateInfrastructure(ctx context.Context) error {
	funcs := []func() error{
		func() error { return p.ensureQueue(ctx) },
		func() error { return p.ensureEventBridge(ctx) },
	}
	errs := make([]error, len(funcs))
	workqueue.ParallelizeUntil(ctx, len(funcs), len(funcs), func(i int) {
		errs[i] = funcs[i]()
	})
	if err := multierr.Combine(errs...); err != nil {
		return err
	}
	logging.FromContext(ctx).Infof("Successfully completed reconciliation of infrastructure")
	return nil
}

// DeleteInfrastructure removes the infrastructure that was stood up and reconciled
// by the infrastructure controller for SQS message polling
func (p *Provider) DeleteInfrastructure(ctx context.Context) error {
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
func (p *Provider) ensureQueue(ctx context.Context) error {
	// Attempt to find the queue. If we can't find it, assume it isn't created and try to create it
	// If we did find it, then just set the queue attributes on the existing queue
	logging.FromContext(ctx).Debugf("Reconciling the SQS notification queue...")
	if _, err := p.sqsProvider.DiscoverQueueURL(ctx, true); err != nil {
		switch {
		case aws.IsNotFound(err):
			logging.FromContext(ctx).Debugf("Queue not found, creating the SQS notification queue...")
			if err := p.sqsProvider.CreateQueue(ctx); err != nil {
				return fmt.Errorf("creating sqs queue with policy, %w", err)
			}
			logging.FromContext(ctx).Debugf("Successfully created the SQS notification queue")
		case aws.IsAccessDenied(err):
			return fmt.Errorf("failed obtaining permission to discover sqs queue url, %w", err)
		default:
			return fmt.Errorf("failed discovering sqs queue url, %w", err)
		}
	}
	// Always attempt to set the queue attributes, even after creation to help set the queue policy
	if err := p.sqsProvider.SetQueueAttributes(ctx); err != nil {
		return fmt.Errorf("setting queue attributes for queue, %w", err)
	}
	return nil
}

// ensureEventBridge reconciles the Eventbridge rules with the configuration prescribed by Karpenter
func (p *Provider) ensureEventBridge(ctx context.Context) error {
	logging.FromContext(ctx).Debugf("Reconciling the EventBridge notification rules...")
	if err := p.eventBridgeProvider.CreateEC2NotificationRules(ctx); err != nil {
		switch {
		case aws.IsAccessDenied(err):
			return fmt.Errorf("obtaining permission to eventbridge, %w", err)
		default:
			return fmt.Errorf("creating event bridge notification rules, %w", err)
		}
	}
	return nil
}
