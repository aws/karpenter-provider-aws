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
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/aggregatedparser"
	statechangev0 "github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/statechange"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/utils"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/controllers/polling"
	"github.com/aws/karpenter/pkg/controllers/state"
)

type Action = string

var Actions = struct {
	CordonAndDrain,
	Cordon,
	NoAction Action
}{
	CordonAndDrain: "CordonAndDrain",
	Cordon:         "Cordon",
	NoAction:       "NoAction",
}

// Reconciler is an AWS notification reconciler.
// It plugs into the polling controller to periodically poll the SQS queue for notification messages
type Reconciler struct {
	kubeClient           client.Client
	cluster              *state.Cluster
	recorder             events.Recorder
	provider             *aws.SQSProvider
	instanceTypeProvider *aws.InstanceTypeProvider
	parser               event.Parser

	infraController polling.ControllerWithHealthInterface
}

// pollingPeriod that we go to the SQS queue to check if there are any new events
const pollingPeriod = 2 * time.Second

func NewReconciler(kubeClient client.Client, recorder events.Recorder, cluster *state.Cluster,
	sqsProvider *aws.SQSProvider, instanceTypeProvider *aws.InstanceTypeProvider,
	infraController polling.ControllerWithHealthInterface) *Reconciler {

	return &Reconciler{
		kubeClient:           kubeClient,
		cluster:              cluster,
		recorder:             recorder,
		provider:             sqsProvider,
		instanceTypeProvider: instanceTypeProvider,
		parser:               aggregatedparser.NewAggregatedParser(aggregatedparser.DefaultParsers...),
		infraController:      infraController,
	}
}

func (r *Reconciler) Metadata() controllers.Metadata {
	return controllers.Metadata{
		Name:             "aws.notification",
		MetricsSubsystem: subsystem,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	// We rely on the infrastructure, so it needs to be healthy before proceeding to poll the queue
	if !r.infraController.Healthy() {
		return reconcile.Result{}, nil
	}
	sqsMessages, err := r.provider.GetSQSMessages(ctx)
	if err != nil {
		// If the queue isn't found, we should trigger the infrastructure controller to re-reconcile
		if aws.IsNotFound(err) {
			r.infraController.Trigger()
		}
		return reconcile.Result{}, err
	}
	if len(sqsMessages) == 0 {
		return reconcile.Result{RequeueAfter: pollingPeriod}, nil
	}
	instanceIDMap := r.makeInstanceIDMap()
	errs := make([]error, len(sqsMessages))
	workqueue.ParallelizeUntil(ctx, 10, len(sqsMessages), func(i int) {
		errs[i] = r.handleMessage(ctx, instanceIDMap, sqsMessages[i])
	})
	return reconcile.Result{RequeueAfter: pollingPeriod}, multierr.Combine(errs...)
}

// handleMessage gets the node names of the instances involved in the queue message and takes the
// assigned action on the instances based on the message event
func (r *Reconciler) handleMessage(ctx context.Context, instanceIDMap map[string]*v1.Node, msg *sqsapi.Message) (err error) {
	// No message to parse in this case
	if msg == nil || msg.Body == nil {
		return nil
	}
	evt := r.parser.Parse(ctx, *msg.Body)
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("event", evt.Kind()))

	nodes := getInvolvedNodes(evt.EC2InstanceIDs(), instanceIDMap)
	// There's no action to take here since the event doesn't pertain to any of our instances
	if len(nodes) == 0 {
		receivedMessages.WithLabelValues(evt.Kind().String(), "false").Inc()

		// Since there's no action, just delete the message
		err = r.provider.DeleteSQSMessage(ctx, msg)
		if err != nil {
			return fmt.Errorf("failed to delete message from queue, %w", err)
		}
		deletedMessages.Inc()
		return
	}
	receivedMessages.WithLabelValues(evt.Kind().String(), "true").Inc()

	nodeNames := lo.Map(nodes, func(n *v1.Node, _ int) string { return n.Name })
	logging.FromContext(ctx).Infof("Received actionable event from SQS queue for node(s) [%s%s]",
		strings.Join(lo.Slice(nodeNames, 0, 3), ","),
		lo.Ternary(len(nodeNames) > 3, "...", ""))

	for i := range nodes {
		node := nodes[i]
		err = multierr.Append(err, r.handleNode(ctx, evt, node))
	}
	if err != nil {
		return fmt.Errorf("failed to act on nodes [%s%s], %w",
			strings.Join(lo.Slice(nodeNames, 0, 3), ","),
			lo.Ternary(len(nodeNames) > 3, "...", ""), err)
	}
	err = r.provider.DeleteSQSMessage(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to delete message from queue, %w", err)
	}
	deletedMessages.Inc()
	return nil
}

func (r *Reconciler) handleNode(ctx context.Context, evt event.Interface, node *v1.Node) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("node", node.Name))
	action := actionForEvent(evt)

	// Record metric and event for this action
	r.notifyForEvent(evt, node)
	actionsPerformed.WithLabelValues(action).Inc()

	// Mark the offering as unavailable in the ICE cache since we got a spot interruption warning
	if evt.Kind() == event.SpotInterruptionKind {
		zone := node.Labels[v1.LabelTopologyZone]
		instanceType := node.Labels[v1.LabelInstanceTypeStable]
		if zone != "" && instanceType != "" {
			r.instanceTypeProvider.MarkOfferingUnavailable(instanceType, zone, awsv1alpha1.CapacityTypeSpot)
		}
	}
	if action != Actions.NoAction {
		return r.deleteInstance(ctx, node)
	}
	return nil
}

func (r *Reconciler) deleteInstance(ctx context.Context, node *v1.Node) error {
	r.recorder.TerminatingNodeOnNotification(node)
	if err := r.kubeClient.Delete(ctx, node); err != nil {
		return fmt.Errorf("deleting the node on notification, %w", err)
	}
	return nil
}

func (r *Reconciler) notifyForEvent(evt event.Interface, n *v1.Node) {
	switch evt.Kind() {
	case event.RebalanceRecommendationKind:
		r.recorder.EC2SpotRebalanceRecommendation(n)

	case event.ScheduledChangeKind:
		r.recorder.EC2HealthWarning(n)

	case event.SpotInterruptionKind:
		r.recorder.EC2SpotInterruptionWarning(n)

	case event.StateChangeKind:
		typed := evt.(statechangev0.EC2InstanceStateChangeNotification)
		if lo.Contains([]string{"stopping", "stopped"}, typed.State()) {
			r.recorder.EC2StateStopping(n)
		} else {
			r.recorder.EC2StateTerminating(n)
		}

	default:
	}
}

// makeInstanceIDMap builds a map between the instance id that is stored in the
// node .sper.providerID and the node name stored on the host
func (r *Reconciler) makeInstanceIDMap() map[string]*v1.Node {
	m := map[string]*v1.Node{}
	r.cluster.ForEachNode(func(n *state.Node) bool {
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
	case event.RebalanceRecommendationKind:
		return Actions.NoAction

	case event.ScheduledChangeKind:
		return Actions.CordonAndDrain

	case event.SpotInterruptionKind:
		return Actions.CordonAndDrain

	case event.StateChangeKind:
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
