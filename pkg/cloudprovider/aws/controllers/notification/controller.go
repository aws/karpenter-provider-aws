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
	"regexp"
	"strings"
	"time"

	sqsapi "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/aggregatedparser"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/metrics"
)

type Action = string

var Actions = struct {
	CordonAndDrain,
	NoAction Action
}{
	CordonAndDrain: "CordonAndDrain",
	NoAction:       "NoAction",
}

// Controller is the notification controller. It is not a standard controller-runtime controller in that it doesn't
// have a reconcile method.
type Controller struct {
	kubeClient client.Client
	cluster    *state.Cluster
	recorder   events.Recorder
	clock      clock.Clock
	provider   *aws.SQSProvider
	parser     event.Parser

	infraController *infrastructure.Controller
}

// pollingPeriod that we go to the SQS queue to check if there are any new events
const pollingPeriod = 2 * time.Second

func NewController(ctx context.Context, kubeClient client.Client, clk clock.Clock,
	recorder events.Recorder, cluster *state.Cluster, sqsProvider *aws.SQSProvider,
	infraController *infrastructure.Controller, startAsync <-chan struct{}) *Controller {
	c := &Controller{
		kubeClient:      kubeClient,
		cluster:         cluster,
		recorder:        recorder,
		clock:           clk,
		provider:        sqsProvider,
		parser:          aggregatedparser.NewAggregatedParser(aggregatedparser.DefaultParsers...),
		infraController: infraController,
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
		<-c.infraController.Ready() // block until the infrastructure is up and ready
		err := c.pollSQS(ctx)
		if err != nil {
			logging.FromContext(ctx).Errorf("Handling notification messages from SQS queue, %v", err)
		}

		select {
		case <-ctx.Done():
			logger.Infof("Shutting down")
			return
		case <-c.clock.After(pollingPeriod):
		}
	}
}

func (c *Controller) pollSQS(ctx context.Context) error {
	defer metrics.Measure(reconcileDuration.WithLabelValues())()

	sqsMessages, err := c.provider.GetSQSMessages(ctx)
	if err != nil {
		// If the queue isn't found, we should trigger the infrastructure controller to re-reconcile
		if aws.IsNotFound(err) {
			c.infraController.Trigger()
		}
		return err
	}
	if len(sqsMessages) == 0 {
		return nil
	}
	instanceIDMap := c.makeInstanceIDMap()
	for _, msg := range sqsMessages {
		e := c.handleMessage(ctx, instanceIDMap, msg)
		err = multierr.Append(err, e)
	}
	return nil
}

// handleMessage gets the node names of the instances involved in the queue message and takes the
// assigned action on the instances based on the message event
func (c *Controller) handleMessage(ctx context.Context, instanceIDMap map[string]*v1.Node, msg *sqsapi.Message) (err error) {
	// No message to parse in this case
	if msg == nil || msg.Body == nil {
		return nil
	}
	evt := c.parser.Parse(ctx, *msg.Body)
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("event", evt.Kind()))

	nodes := getInvolvedNodes(evt.EC2InstanceIDs(), instanceIDMap)
	// There's no action to take here since the event doesn't pertain to any of our instances
	if len(nodes) == 0 {
		receivedMessages.WithLabelValues(evt.Kind(), "false").Inc()
		return
	}
	receivedMessages.WithLabelValues(evt.Kind(), "true").Inc()

	action := actionForEvent(evt)
	nodeNames := lo.Map(nodes, func(n *v1.Node, _ int) string { return n.Name })
	logging.FromContext(ctx).Infof("Received actionable event from SQS queue for node(s) [%s%s]",
		strings.Join(lo.Slice(nodeNames, 0, 3), ","),
		lo.Ternary(len(nodeNames) > 3, "...", ""))

	for i := range nodes {
		node := nodes[i]
		nodeCtx := logging.WithLogger(ctx, logging.FromContext(ctx).With("node", node.Name))

		// Record metric and event for this action
		c.notifyForEvent(evt, node)
		actionsTaken.WithLabelValues(action).Inc()

		if action != Actions.NoAction {
			e := c.deleteInstance(nodeCtx, node)
			err = multierr.Append(err, e)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to act on nodes [%s%s], %w",
			strings.Join(lo.Slice(nodeNames, 0, 3), ","),
			lo.Ternary(len(nodeNames) > 3, "...", ""), err)
	}
	err = c.provider.DeleteSQSMessage(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to delete message from queue, %w", err)
	}
	deletedMessages.WithLabelValues().Inc()
	return nil
}

func (c *Controller) deleteInstance(ctx context.Context, node *v1.Node) error {
	c.recorder.TerminatingNodeOnNotification(node)
	if err := c.kubeClient.Delete(ctx, node); err != nil {
		return fmt.Errorf("deleting the spot interrupted node, %w", err)
	}
	return nil
}

func (c *Controller) notifyForEvent(evt event.Interface, n *v1.Node) {
	switch evt.Kind() {
	case event.Kinds.RebalanceRecommendation:
		c.recorder.EC2SpotRebalanceRecommendation(n)

	case event.Kinds.ScheduledChange:
		c.recorder.EC2HealthWarning(n)

	case event.Kinds.SpotInterruption:
		c.recorder.EC2SpotInterruptionWarning(n)

	case event.Kinds.StateChange:
		c.recorder.EC2StateChange(n)
	default:
	}
}

func actionForEvent(evt event.Interface) Action {
	switch evt.Kind() {
	case event.Kinds.RebalanceRecommendation:
		return Actions.NoAction

	case event.Kinds.ScheduledChange:
		return Actions.CordonAndDrain

	case event.Kinds.SpotInterruption:
		return Actions.CordonAndDrain

	case event.Kinds.StateChange:
		return Actions.CordonAndDrain

	default:
		return Actions.NoAction
	}
}

// getInvolvedNodes gets all the nodes that are involved in an event based
// on the instanceIDs passed in from the event
func getInvolvedNodes(instanceIDs []string, instanceIDMap map[string]*v1.Node) []*v1.Node {
	var nodes []*v1.Node
	for _, id := range instanceIDs {
		if node, ok := instanceIDMap[id]; ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// buildInstanceIDMap builds a map between the instance name that is stored in the
// node .spec.providerID and the node name stored on the host
func (c *Controller) makeInstanceIDMap() map[string]*v1.Node {
	m := map[string]*v1.Node{}
	c.cluster.ForEachNode(func(n *state.Node) bool {
		// If this node isn't owned by a provisioner, we shouldn't handle it
		if _, ok := n.Node.Labels[v1alpha5.ProvisionerNameLabelKey]; !ok {
			return true
		}
		id := parseProviderID(n.Node.Spec.ProviderID)
		if id == "" {
			return true
		}
		m[id] = n.Node
		return true
	})
	return m
}

// parseProviderID parses the provider ID stored on the node to get the instance ID
// associated with a node
func parseProviderID(pid string) string {
	r := regexp.MustCompile(`aws:///(?P<AZ>.*)/(?P<InstanceID>.*)`)
	matches := r.FindStringSubmatch(pid)
	if matches == nil {
		return ""
	}
	for i, name := range r.SubexpNames() {
		if name == "InstanceID" {
			return matches[i]
		}
	}
	return ""
}
