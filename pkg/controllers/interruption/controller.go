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

package interruption

import (
	"context"
	"fmt"
	"time"

	sqsapi "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	"go.uber.org/multierr"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	"github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/sqs"
)

// Controller is an AWS interruption controller.
// It continually polls an SQS queue for events from aws.ec2 and aws.health that
// trigger node health events, spot interruption/rebalance events, and capacity reservation interruptions.
type Controller struct {
	InterruptionHandler
	sqsProvider sqs.Provider
	sqsAPI      *sqsapi.Client
	parser      *EventParser
	cm          *pretty.ChangeMonitor
}

func NewController(
	kubeClient client.Client,
	cloudProvider cloudprovider.CloudProvider,
	recorder events.Recorder,
	sqsProvider sqs.Provider,
	sqsAPI *sqsapi.Client,
	unavailableOfferingsCache *cache.UnavailableOfferings,
	capacityReservationProvider capacityreservation.Provider,
) *Controller {
	return &Controller{
		InterruptionHandler: InterruptionHandler{
			kubeClient:                  kubeClient,
			cloudProvider:               cloudProvider,
			recorder:                    recorder,
			unavailableOfferingsCache:   unavailableOfferingsCache,
			capacityReservationProvider: capacityReservationProvider,
		},
		sqsProvider: sqsProvider,
		sqsAPI:      sqsAPI,
		parser:      NewEventParser(DefaultParsers...),
		cm:          pretty.NewChangeMonitor(),
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, "interruption")

	if c.sqsProvider == nil {
		prov, err := sqs.NewSQSProvider(ctx, c.sqsAPI)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to create valid sqs provider")
			return reconciler.Result{}, fmt.Errorf("creating sqs provider, %w", err)
		}
		c.sqsProvider = prov
	}
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("queue", c.sqsProvider.Name()))
	if c.cm.HasChanged(c.sqsProvider.Name(), nil) {
		log.FromContext(ctx).V(1).Info("watching interruption queue")
	}
	sqsMessages, err := c.sqsProvider.GetSQSMessages(ctx)
	if err != nil {
		return reconciler.Result{}, fmt.Errorf("getting messages from queue, %w", err)
	}
	if len(sqsMessages) == 0 {
		return reconciler.Result{RequeueAfter: singleton.RequeueImmediately}, nil
	}

	errs := make([]error, len(sqsMessages))
	workqueue.ParallelizeUntil(ctx, 10, len(sqsMessages), func(i int) {
		msg, e := c.parseMessage(sqsMessages[i])
		if e != nil {
			// If we fail to parse, then we should delete the message but still log the error
			log.FromContext(ctx).Error(e, "failed parsing interruption message")
			errs[i] = c.deleteMessage(ctx, sqsMessages[i])
			return
		}
		ReceivedMessages.Inc(map[string]string{messageTypeLabel: string(msg.Kind())})
		if e = c.handleMessage(ctx, msg); e != nil {
			errs[i] = fmt.Errorf("handling message, %w", e)
			return
		}
		MessageLatency.Observe(time.Since(msg.StartTime()).Seconds(), nil)
		errs[i] = c.deleteMessage(ctx, sqsMessages[i])
	})
	if err = multierr.Combine(errs...); err != nil {
		return reconciler.Result{}, err
	}
	return reconciler.Result{RequeueAfter: singleton.RequeueImmediately}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("interruption").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}

// parseMessage parses the passed SQS message into an internal Message interface
func (c *Controller) parseMessage(raw *sqstypes.Message) (messages.Message, error) {
	// No message to parse in this case
	if raw == nil || raw.Body == nil {
		return nil, fmt.Errorf("message or message body is nil")
	}
	msg, err := c.parser.Parse(*raw.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing sqs message, %w", err)
	}
	return msg, nil
}

func (c *Controller) deleteMessage(ctx context.Context, msg *sqstypes.Message) error {
	if err := c.sqsProvider.DeleteSQSMessage(ctx, msg); err != nil {
		return fmt.Errorf("deleting sqs message, %w", err)
	}
	DeletedMessages.Inc(nil)
	return nil
}
