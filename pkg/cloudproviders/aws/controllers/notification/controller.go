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
	"strings"
	"time"

	sqsapi "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/cenkalti/backoff/v4"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudproviders/aws/apis/v1alpha1"
	awscache "github.com/aws/karpenter/pkg/cloudproviders/aws/cache"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/events"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/notification/event"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/notification/event/statechange"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/providers"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/utils"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/metrics"
)

type Action byte

const (
	_ Action = iota
	CordonAndDrain
	NoAction
)

func (a Action) String() string {
	switch a {
	case CordonAndDrain:
		return "CordonAndDrain"
	case NoAction:
		return "NoAction"
	default:
		return fmt.Sprintf("Unsupported Action %d", a)
	}
}

// Controller is an AWS notification controller.
// It plugs into the polling controller to periodically poll the SQS queue for notification messages
type Controller struct {
	kubeClient                client.Client
	clk                       clock.Clock
	cluster                   *state.Cluster
	recorder                  events.Recorder
	provider                  *providers.SQSProvider
	unavailableOfferingsCache *awscache.UnavailableOfferings
	parser                    *EventParser
	backoff                   *backoff.ExponentialBackOff
}

func NewController(kubeClient client.Client, clk clock.Clock, recorder events.Recorder, cluster *state.Cluster,
	sqsProvider *providers.SQSProvider, unavailableOfferingsCache *awscache.UnavailableOfferings) *Controller {

	return &Controller{
		kubeClient:                kubeClient,
		clk:                       clk,
		cluster:                   cluster,
		recorder:                  recorder,
		provider:                  sqsProvider,
		unavailableOfferingsCache: unavailableOfferingsCache,
		parser:                    NewEventParser(DefaultParsers...),
		backoff:                   newBackoff(clk),
	}
}

func (c *Controller) Start(ctx context.Context) {
	for {
		list := &v1alpha1.AWSNodeTemplateList{}
		if err := c.kubeClient.List(ctx, list); err != nil {
			logging.FromContext(ctx).Errorf("listing aws node templates, %v", err)
			continue
		}
		if len(list.Items) > 0 {
			// If there are AWSNodeTemplates, we should reconcile the notifications by continually polling
			// the queue for messages
			wait := time.Duration(0) // default is to not wait
			if _, err := c.Reconcile(ctx, reconcile.Request{}); err != nil {
				logging.FromContext(ctx).Errorf("reconciling notification messages, %v", err)
				wait = c.backoff.NextBackOff()
			} else {
				c.backoff.Reset()
			}
			select {
			case <-ctx.Done():
				return
			case <-c.clk.After(wait):
			}
		} else {
			// If there are no AWSNodeTemplates, we can just poll on a one-minute interval to check for any templates
			select {
			case <-ctx.Done():
				return
			case <-c.clk.After(time.Minute):
			}
		}
	}
}

func (c *Controller) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	sqsMessages, err := c.provider.GetSQSMessages(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting messages from queue, %w", err)
	}
	if len(sqsMessages) == 0 {
		return reconcile.Result{}, nil
	}
	instanceIDMap := c.makeInstanceIDMap()
	errs := make([]error, len(sqsMessages))
	workqueue.ParallelizeUntil(ctx, 10, len(sqsMessages), func(i int) {
		errs[i] = c.handleMessage(ctx, instanceIDMap, sqsMessages[i])
	})
	return reconcile.Result{}, multierr.Combine(errs...)
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("aws.notification"))
	go func() {
		defer logging.FromContext(ctx).Infof("Shutting down")
		select {
		case <-ctx.Done():
			return
		case <-m.Elected():
			c.Start(ctx)
		}
	}()
	return nil
}

// handleMessage gets the node names of the instances involved in the queue message and takes the
// assigned action on the instances based on the message event
func (c *Controller) handleMessage(ctx context.Context, instanceIDMap map[string]*v1.Node, msg *sqsapi.Message) error {
	// No message to parse in this case
	if msg == nil || msg.Body == nil {
		return nil
	}
	evt, err := c.parser.Parse(*msg.Body)
	if err != nil {
		// In the scenario where we can't parse the message, we log that we have an error and then are
		// forced to just delete the message from the queue
		logging.FromContext(ctx).Errorf("parsing sqs message, %v", err)
		err = c.provider.DeleteSQSMessage(ctx, msg)
		if err != nil {
			return fmt.Errorf("failed to delete message from queue, %w", err)
		}
		deletedMessages.Inc()
		return nil
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("event", evt.Kind()))

	nodes := getInvolvedNodes(evt.EC2InstanceIDs(), instanceIDMap)
	// There's no action to take here since the event doesn't pertain to any of our instances
	if len(nodes) == 0 {
		receivedMessages.WithLabelValues(evt.Kind().String(), "false").Inc()

		// Since there's no action, just delete the message
		err = c.provider.DeleteSQSMessage(ctx, msg)
		if err != nil {
			return fmt.Errorf("failed to delete message from queue, %w", err)
		}
		deletedMessages.Inc()
		return nil
	}
	receivedMessages.WithLabelValues(evt.Kind().String(), "true").Inc()

	nodeNames := lo.Map(nodes, func(n *v1.Node, _ int) string { return n.Name })
	logging.FromContext(ctx).Infof("Received actionable event from SQS queue for node(s) [%s%s]",
		strings.Join(lo.Slice(nodeNames, 0, 3), ","),
		lo.Ternary(len(nodeNames) > 3, "...", ""))

	for i := range nodes {
		node := nodes[i]
		err = multierr.Append(err, c.handleNode(ctx, evt, node))
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
	deletedMessages.Inc()
	return nil
}

func (c *Controller) handleNode(ctx context.Context, evt event.Interface, node *v1.Node) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("node", node.Name))
	action := actionForEvent(evt)

	// Record metric and event for this action
	c.notifyForEvent(evt, node)
	actionsPerformed.WithLabelValues(action.String()).Inc()

	// Mark the offering as unavailable in the ICE cache since we got a spot interruption warning
	if evt.Kind() == event.SpotInterruptionKind {
		zone := node.Labels[v1.LabelTopologyZone]
		instanceType := node.Labels[v1.LabelInstanceTypeStable]
		if zone != "" && instanceType != "" {
			c.unavailableOfferingsCache.MarkUnavailable(ctx, evt.Kind().String(), instanceType, zone, awsv1alpha1.CapacityTypeSpot)
		}
	}
	if action != NoAction {
		return c.deleteInstance(ctx, node)
	}
	return nil
}

func (c *Controller) deleteInstance(ctx context.Context, node *v1.Node) error {
	if err := c.kubeClient.Delete(ctx, node); err != nil {
		return fmt.Errorf("deleting the node on notification, %w", err)
	}
	c.recorder.TerminatingNodeOnNotification(node)
	metrics.NodesTerminatedCounter.WithLabelValues(terminationReasonLabel).Inc()
	return nil
}

func (c *Controller) notifyForEvent(evt event.Interface, n *v1.Node) {
	switch evt.Kind() {
	case event.RebalanceRecommendationKind:
		c.recorder.EC2SpotRebalanceRecommendation(n)

	case event.ScheduledChangeKind:
		c.recorder.EC2HealthWarning(n)

	case event.SpotInterruptionKind:
		c.recorder.EC2SpotInterruptionWarning(n)

	case event.StateChangeKind:
		typed := evt.(statechange.Event)
		if lo.Contains([]string{"stopping", "stopped"}, typed.State()) {
			c.recorder.EC2StateStopping(n)
		} else {
			c.recorder.EC2StateTerminating(n)
		}

	default:
	}
}

// makeInstanceIDMap builds a map between the instance id that is stored in the
// node .spec.providerID and the node name stored on the host
func (c *Controller) makeInstanceIDMap() map[string]*v1.Node {
	m := map[string]*v1.Node{}
	c.cluster.ForEachNode(func(n *state.Node) bool {
		// If this node isn't owned by a provisioner, we shouldn't handle it
		if _, ok := n.Node.Labels[v1alpha5.ProvisionerNameLabelKey]; !ok {
			return true
		}
		id, err := utils.ParseProviderID(n.Node)
		if err != nil || id == nil {
			return true
		}
		m[ptr.StringValue(id)] = n.Node
		return true
	})
	return m
}

func actionForEvent(evt event.Interface) Action {
	switch evt.Kind() {
	case event.ScheduledChangeKind, event.SpotInterruptionKind, event.StateChangeKind:
		return CordonAndDrain
	default:
		return NoAction
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

func newBackoff(clk clock.Clock) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Second * 5
	b.MaxElapsedTime = time.Minute * 30
	b.Clock = clk
	return b
}
