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

package notification

import (
	"context"
	"fmt"
	"time"

	sqsapi "github.com/aws/aws-sdk-go/service/sqs"
	"go.uber.org/multierr"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/aggregatedparser"
	"github.com/aws/karpenter/pkg/events"
)

// Controller is the consolidation controller.  It is not a standard controller-runtime controller in that it doesn't
// have a reconcile method.
type Controller struct {
	kubeClient client.Client
	recorder   events.Recorder
	clock      clock.Clock
	provider   *SQSProvider
	parser     event.Parser
}

// pollingPeriod that we go to the SQS queue to check if there are any new events
const pollingPeriod = 2 * time.Second

func NewController(ctx context.Context, clk clock.Clock, kubeClient client.Client,
	sqsProvider *SQSProvider, recorder events.Recorder, startAsync <-chan struct{}) *Controller {
	c := &Controller{
		clock:      clk,
		kubeClient: kubeClient,
		recorder:   recorder,
		provider:   sqsProvider,
		parser:     aggregatedparser.NewAggregatedParser(aggregatedparser.DefaultParsers...),
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
	logger := logging.FromContext(ctx).Named("notification")
	ctx = logging.WithLogger(ctx, logger)
	for {
		select {
		case <-ctx.Done():
			logger.Infof("Shutting down")
			return
		case <-time.After(pollingPeriod):
			logging.FromContext(ctx).Info("Here")
		}
	}
}

func (c *Controller) Poll(ctx context.Context) error {
	sqsMessages, err := c.provider.GetSQSMessages(ctx)
	if err != nil {
		return err
	}

	for _, msg := range sqsMessages {
		e := c.handleMessage(ctx, msg)
		err = multierr.Append(err, e)
	}
	return nil
}

func (c *Controller) handleMessage(ctx context.Context, msg *sqsapi.Message) (err error) {
	fmt.Printf("Handling the message for %#v\n", msg)

	// No message to parse in this case
	if msg == nil || msg.Body == nil {
		return nil
	}
	evt := c.parser.Parse(ctx, *msg.Body)
	evtAction := actionForEvent(evt)

	// TODO: hand some of this work off to a batcher that will handle the spinning up of a new node
	// and the deletion of the old node separate from this reconciliation loop
	if evtAction != Actions.NoAction {
		for _, ec2InstanceID := range evt.EC2InstanceIDs() {
			e := c.handleInstance(ctx, ec2InstanceID, evtAction)
			err = multierr.Append(err, e)
		}
	}
	if err != nil {
		return err
	}
	return c.provider.DeleteSQSMessage(ctx, msg)
}

// TODO: Handle the instance appropriately, this should be handled with a batcher
func (c *Controller) handleInstance(ctx context.Context, ec2InstanceID string, evtAction Action) error {
	logging.FromContext(ctx).Infof("Got a message for ec2 instance id %s", ec2InstanceID)
	return nil
}

func actionForEvent(evt event.Interface) Action {
	switch evt.Kind() {
	case event.Kinds.RebalanceRecommendation:
		return Actions.NoAction

	case event.Kinds.ScheduledChange:
		return Actions.CordonAndDrain

	case event.Kinds.SpotInterruption:
		return Actions.CordonAndDrain

		// TODO: understand what the state change action is
	case event.Kinds.StateChange:
		return Actions.NoAction

	default:
		return Actions.NoAction
	}
}
