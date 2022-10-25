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
	"net/http"
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

	"github.com/aws/karpenter-core/pkg/operator/scheme"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	awscache "github.com/aws/karpenter/pkg/cache"
	interruptionevents "github.com/aws/karpenter/pkg/controllers/interruption/events"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/statechange"
	"github.com/aws/karpenter/pkg/controllers/providers"
	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/utils"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/metrics"
	operatorcontroller "github.com/aws/karpenter-core/pkg/operator/controller"
)

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
}

// Controller is an AWS interruption controller.
// It continually polls an provisioned SQS queue for events from aws.ec2 and aws.health that
// trigger node health events or node spot interruption/rebalance events.
type Controller struct {
	kubeClient                client.Client
	clk                       clock.Clock
	cluster                   *state.Cluster
	recorder                  *events.Recorder
	provider                  *providers.SQS
	unavailableOfferingsCache *awscache.UnavailableOfferings
	parser                    *EventParser
	backoff                   *backoff.ExponentialBackOff
}

func NewController(kubeClient client.Client, clk clock.Clock, recorder *events.Recorder, cluster *state.Cluster,
	sqsProvider *providers.SQS, unavailableOfferingsCache *awscache.UnavailableOfferings) *Controller {

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

func (c *Controller) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	defer metrics.Measure(reconcileDuration)()
	list := &v1alpha1.AWSNodeTemplateList{}
	if err := c.kubeClient.List(ctx, list); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing node templates, %w", err)
	}

	if settings.FromContext(ctx).EnableInterruptionHandling && len(list.Items) > 0 {
		active.Set(1)
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
	} else {
		active.Set(0)
	}
	return reconcile.Result{RequeueAfter: time.Second * 10}, nil
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) operatorcontroller.Builder {
	return operatorcontroller.NewSingletonManagedBy(m).
		Named("notification")
}

func (c *Controller) LivenessProbe(_ *http.Request) error {
	return nil
}

// handleMessage gets the node names of the instances involved in the queue message and takes the
// assigned action on the instances based on the message event
func (c *Controller) handleMessage(ctx context.Context, instanceIDMap map[string]*v1.Node, raw *sqsapi.Message) error {
	// No message to parse in this case
	if raw == nil || raw.Body == nil {
		return nil
	}
	msg, err := c.parser.Parse(*raw.Body)
	if err != nil {
		// In the scenario where we can't parse the message, we log that we have an error and then are
		// forced to just delete the message from the queue
		logging.FromContext(ctx).Errorf("parsing sqs message, %v", err)
		err = c.provider.DeleteSQSMessage(ctx, raw)
		if err != nil {
			return fmt.Errorf("failed to delete message from queue, %w", err)
		}
		deletedMessages.Inc()
		return nil
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("event", msg.Kind()))

	nodes := getInvolvedNodes(msg.EC2InstanceIDs(), instanceIDMap)
	// There's no action to take here since the event doesn't pertain to any of our instances
	if len(nodes) == 0 {
		receivedMessages.WithLabelValues(msg.Kind().String(), "false").Inc()

		// Since there's no action, just delete the message
		err = c.provider.DeleteSQSMessage(ctx, raw)
		if err != nil {
			return fmt.Errorf("failed to delete message from queue, %w", err)
		}
		deletedMessages.Inc()
		return nil
	}
	receivedMessages.WithLabelValues(msg.Kind().String(), "true").Inc()

	nodeNames := lo.Map(nodes, func(n *v1.Node, _ int) string { return n.Name })
	logging.FromContext(ctx).Infof("Received actionable event from SQS queue for node(s) [%s%s]",
		strings.Join(lo.Slice(nodeNames, 0, 3), ","),
		lo.Ternary(len(nodeNames) > 3, "...", ""))

	for i := range nodes {
		node := nodes[i]
		err = multierr.Append(err, c.handleNode(ctx, msg, node))
	}
	if err != nil {
		return fmt.Errorf("failed to act on nodes [%s%s], %w",
			strings.Join(lo.Slice(nodeNames, 0, 3), ","),
			lo.Ternary(len(nodeNames) > 3, "...", ""), err)
	}
	err = c.provider.DeleteSQSMessage(ctx, raw)
	if err != nil {
		return fmt.Errorf("failed to delete message from queue, %w", err)
	}
	deletedMessages.Inc()
	return nil
}

func (c *Controller) handleNode(ctx context.Context, evt messages.Interface, node *v1.Node) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("node", node.Name))
	action := actionForEvent(evt)

	// Record metric and event for this action
	c.notifyForEvent(evt, node)
	actionsPerformed.WithLabelValues(action.String()).Inc()

	// Mark the offering as unavailable in the ICE cache since we got a spot interruption warning
	if evt.Kind() == messages.SpotInterruptionKind {
		zone := node.Labels[v1.LabelTopologyZone]
		instanceType := node.Labels[v1.LabelInstanceTypeStable]
		if zone != "" && instanceType != "" {
			c.unavailableOfferingsCache.MarkUnavailable(ctx, evt.Kind().String(), instanceType, zone, v1alpha1.CapacityTypeSpot)
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
	c.recorder.Publish(interruptionevents.NodeTerminatingOnInterruption(node))
	metrics.NodesTerminatedCounter.WithLabelValues(terminationReasonLabel).Inc()
	return nil
}

func (c *Controller) notifyForEvent(evt messages.Interface, n *v1.Node) {
	switch evt.Kind() {
	case messages.RebalanceRecommendationKind:
		c.recorder.Publish(interruptionevents.InstanceRebalanceRecommendation(n))

	case messages.ScheduledChangeKind:
		c.recorder.Publish(interruptionevents.InstanceUnhealthy(n))

	case messages.SpotInterruptionKind:
		c.recorder.Publish(interruptionevents.InstanceSpotInterrupted(n))

	case messages.StateChangeKind:
		typed := evt.(statechange.Event)
		if lo.Contains([]string{"stopping", "stopped"}, typed.State()) {
			c.recorder.Publish(interruptionevents.InstanceStopping(n))
		} else {
			c.recorder.Publish(interruptionevents.InstanceTerminating(n))
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
		id, err := utils.ParseInstanceID(n.Node)
		if err != nil || id == nil {
			return true
		}
		m[ptr.StringValue(id)] = n.Node
		return true
	})
	return m
}

func actionForEvent(evt messages.Interface) Action {
	switch evt.Kind() {
	case messages.ScheduledChangeKind, messages.SpotInterruptionKind, messages.StateChangeKind:
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
