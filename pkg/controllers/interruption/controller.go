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

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/metrics"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	sqsapi "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	"sigs.k8s.io/karpenter/pkg/events"

	"github.com/aws/karpenter-provider-aws/pkg/cache"
	interruptionevents "github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/events"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/sqs"
	"github.com/aws/karpenter-provider-aws/pkg/providers/webhook"
)

type Action string

const (
	CordonAndDrain Action = "CordonAndDrain"
	NoAction       Action = "NoAction"

	// webhookTimeout is the maximum time allowed for a webhook notification to complete
	// This is set to 30s to allow for 3 retries with 10s HTTP timeout each plus backoff delays (1s + 2s + 4s)
	webhookTimeout = 30 * time.Second

	// maxConcurrentWebhooks limits the number of simultaneous webhook HTTP requests
	// to prevent file descriptor exhaustion during mass interruption events
	maxConcurrentWebhooks = 100
)

var (
	// webhookSemaphore limits concurrent webhook notifications to prevent resource exhaustion
	webhookSemaphore = make(chan struct{}, maxConcurrentWebhooks)
)

// Controller is an AWS interruption controller.
// It continually polls an SQS queue for events from aws.ec2 and aws.health that
// trigger node health events or node spot interruption/rebalance events.
type Controller struct {
	kubeClient                client.Client
	cloudProvider             cloudprovider.CloudProvider
	clk                       clock.Clock
	recorder                  events.Recorder
	sqsProvider               sqs.Provider
	sqsAPI                    *sqsapi.Client
	unavailableOfferingsCache *cache.UnavailableOfferings
	webhookProvider           webhook.Provider
	parser                    *EventParser
	cm                        *pretty.ChangeMonitor
}

func NewController(
	kubeClient client.Client,
	cloudProvider cloudprovider.CloudProvider,
	clk clock.Clock,
	recorder events.Recorder,
	sqsProvider sqs.Provider,
	sqsAPI *sqsapi.Client,
	unavailableOfferingsCache *cache.UnavailableOfferings,
	webhookProvider webhook.Provider,
) *Controller {
	return &Controller{
		kubeClient:                kubeClient,
		cloudProvider:             cloudProvider,
		clk:                       clk,
		recorder:                  recorder,
		sqsProvider:               sqsProvider,
		sqsAPI:                    sqsAPI,
		unavailableOfferingsCache: unavailableOfferingsCache,
		webhookProvider:           webhookProvider,
		parser:                    NewEventParser(DefaultParsers...),
		cm:                        pretty.NewChangeMonitor(),
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
		if e = c.handleMessage(ctx, msg); e != nil {
			errs[i] = fmt.Errorf("handling message, %w", e)
			return
		}
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

// handleMessage takes an action against every node involved in the message that is owned by a NodePool
func (c *Controller) handleMessage(ctx context.Context, msg messages.Message) (err error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("messageKind", msg.Kind()))
	ReceivedMessages.Inc(map[string]string{messageTypeLabel: string(msg.Kind())})

	if msg.Kind() == messages.NoOpKind {
		return nil
	}
	for _, instanceID := range msg.EC2InstanceIDs() {
		nodeClaimList := &karpv1.NodeClaimList{}
		if e := c.kubeClient.List(ctx, nodeClaimList, client.MatchingFields{"status.instanceID": instanceID}); e != nil {
			err = multierr.Append(err, e)
			continue
		}
		if len(nodeClaimList.Items) == 0 {
			continue
		}
		for _, nodeClaim := range nodeClaimList.Items {
			nodeList := &corev1.NodeList{}
			if e := c.kubeClient.List(ctx, nodeList, client.MatchingFields{"spec.instanceID": instanceID}); e != nil {
				err = multierr.Append(err, e)
				continue
			}
			var node *corev1.Node
			if len(nodeList.Items) > 0 {
				node = &nodeList.Items[0]
			}
			if e := c.handleNodeClaim(ctx, msg, &nodeClaim, node); e != nil {
				err = multierr.Append(err, e)
			}
		}
	}
	MessageLatency.Observe(time.Since(msg.StartTime()).Seconds(), nil)
	if err != nil {
		return fmt.Errorf("acting on NodeClaims, %w", err)
	}
	return nil
}

// deleteMessage removes the passed SQS message from the queue and fires a metric for the deletion
func (c *Controller) deleteMessage(ctx context.Context, msg *sqstypes.Message) error {
	if err := c.sqsProvider.DeleteSQSMessage(ctx, msg); err != nil {
		return fmt.Errorf("deleting sqs message, %w", err)
	}
	DeletedMessages.Inc(nil)
	return nil
}

// handleNodeClaim retrieves the action for the message and then performs the appropriate action against the node
func (c *Controller) handleNodeClaim(ctx context.Context, msg messages.Message, nodeClaim *karpv1.NodeClaim, node *corev1.Node) error {
	action := actionForMessage(msg)
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("NodeClaim", klog.KObj(nodeClaim), "action", string(action)))
	if node != nil {
		ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Node", klog.KObj(node)))
	}

	// Record metric and event for this action
	c.notifyForMessage(ctx, msg, nodeClaim, node)

	// Mark the offering as unavailable in the ICE cache since we got a spot interruption warning
	if msg.Kind() == messages.SpotInterruptionKind {
		zone := nodeClaim.Labels[corev1.LabelTopologyZone]
		instanceType := nodeClaim.Labels[corev1.LabelInstanceTypeStable]
		if zone != "" && instanceType != "" {
			unavailableReason := map[string]string{
				"reason": string(msg.Kind()),
			}
			c.unavailableOfferingsCache.MarkUnavailable(ctx, ec2types.InstanceType(instanceType), zone, karpv1.CapacityTypeSpot, unavailableReason)
		}
	}
	if action != NoAction {
		return c.deleteNodeClaim(ctx, msg, nodeClaim, node)
	}
	return nil
}

// deleteNodeClaim removes the NodeClaim from the api-server
func (c *Controller) deleteNodeClaim(ctx context.Context, msg messages.Message, nodeClaim *karpv1.NodeClaim, node *corev1.Node) error {
	if !nodeClaim.DeletionTimestamp.IsZero() {
		return nil
	}
	if err := c.kubeClient.Delete(ctx, nodeClaim); err != nil {
		return client.IgnoreNotFound(fmt.Errorf("deleting the node on interruption message, %w", err))
	}
	log.FromContext(ctx).Info("initiating delete from interruption message")
	c.recorder.Publish(interruptionevents.TerminatingOnInterruption(node, nodeClaim)...)
	metrics.NodeClaimsDisruptedTotal.Inc(map[string]string{
		metrics.ReasonLabel:       string(msg.Kind()),
		metrics.NodePoolLabel:     nodeClaim.Labels[karpv1.NodePoolLabelKey],
		metrics.CapacityTypeLabel: nodeClaim.Labels[karpv1.CapacityTypeLabelKey],
	})
	return nil
}

// notifyForMessage publishes the relevant alert based on the message kind
func (c *Controller) notifyForMessage(ctx context.Context, msg messages.Message, nodeClaim *karpv1.NodeClaim, n *corev1.Node) {
	switch msg.Kind() {
	case messages.RebalanceRecommendationKind:
		c.recorder.Publish(interruptionevents.RebalanceRecommendation(n, nodeClaim)...)

	case messages.ScheduledChangeKind:
		c.recorder.Publish(interruptionevents.Unhealthy(n, nodeClaim)...)

	case messages.SpotInterruptionKind:
		c.recorder.Publish(interruptionevents.SpotInterrupted(n, nodeClaim)...)

	case messages.InstanceStoppedKind:
		c.recorder.Publish(interruptionevents.Stopping(n, nodeClaim)...)

	case messages.InstanceTerminatedKind:
		c.recorder.Publish(interruptionevents.Terminating(n, nodeClaim)...)

	default:
	}

	// Send webhook notification if provider is configured
	if c.webhookProvider != nil && c.webhookProvider.ShouldNotify(msg.Kind()) {
		payload := c.buildWebhookPayload(ctx, msg, nodeClaim, n)
		if payload == nil {
			return // Skip webhook if payload could not be built (e.g., missing cluster name)
		}

		// Send asynchronously to avoid blocking interruption handling
		// Use a detached context with timeout to allow webhook send to complete
		// even if parent context is cancelled, but with a reasonable time limit
		// Capture logger from parent context before entering goroutine
		logger := log.FromContext(ctx)

		// Try to acquire semaphore slot, drop webhook if queue is full
		select {
		case webhookSemaphore <- struct{}{}:
			// Acquired slot, proceed with webhook
		default:
			// Queue full, drop webhook notification
			logger.V(1).Info("webhook queue full, dropping notification",
				"messageKind", msg.Kind(),
				"nodeClaim", nodeClaim.Name)
			WebhookNotificationsTotal.Inc(map[string]string{
				"status":     "dropped",
				"event_type": string(msg.Kind()),
			})
			return
		}

		go func() {
			defer func() { <-webhookSemaphore }() // Release semaphore slot
			defer func() {
				if r := recover(); r != nil {
					WebhookNotificationsTotal.Inc(map[string]string{
						"status":     "panic",
						"event_type": string(msg.Kind()),
					})
					logger.Error(nil, "webhook notification panicked",
						"panic", r,
						"nodeClaim", nodeClaim.Name)
				}
			}()

			webhookCtx, cancel := context.WithTimeout(context.Background(), webhookTimeout)
			defer cancel()

			start := time.Now()
			err := c.webhookProvider.SendNotification(webhookCtx, payload)
			duration := time.Since(start).Seconds()

			if err != nil {
				WebhookNotificationsTotal.Inc(map[string]string{
					"status":     "failure",
					"event_type": string(msg.Kind()),
				})
				WebhookNotificationDuration.Observe(duration, map[string]string{
					"status": "failure",
				})
				logger.Error(err, "failed to send webhook notification",
					"messageKind", msg.Kind(),
					"nodeClaim", nodeClaim.Name)
			} else {
				WebhookNotificationsTotal.Inc(map[string]string{
					"status":     "success",
					"event_type": string(msg.Kind()),
				})
				WebhookNotificationDuration.Observe(duration, map[string]string{
					"status": "success",
				})
			}
		}()
	}
}

func actionForMessage(msg messages.Message) Action {
	switch msg.Kind() {
	case messages.ScheduledChangeKind, messages.SpotInterruptionKind, messages.InstanceStoppedKind, messages.InstanceTerminatedKind:
		return CordonAndDrain
	default:
		return NoAction
	}
}

// buildWebhookPayload creates a webhook notification payload from the message and nodeclaim
func (c *Controller) buildWebhookPayload(ctx context.Context, msg messages.Message, nodeClaim *karpv1.NodeClaim, n *corev1.Node) *webhook.NotificationPayload {
	var eventReason, eventMessage string

	switch msg.Kind() {
	case messages.SpotInterruptionKind:
		eventReason = "Spot Interruption"
		eventMessage = "Spot instance will be terminated in 2 minutes"
	case messages.ScheduledChangeKind:
		eventReason = "Scheduled Change"
		eventMessage = "Instance has a scheduled maintenance event"
	case messages.InstanceStoppedKind:
		eventReason = "Instance Stopped"
		eventMessage = "Instance has been stopped"
	case messages.InstanceTerminatedKind:
		eventReason = "Instance Terminated"
		eventMessage = "Instance has been terminated"
	case messages.RebalanceRecommendationKind:
		eventReason = "Rebalance Recommendation"
		eventMessage = "Instance received a rebalance recommendation"
	default:
		eventReason = string(msg.Kind())
		eventMessage = "Instance interruption event"
	}

	// Extract instance ID from EC2 instance IDs in the message
	instanceID := ""
	if ids := msg.EC2InstanceIDs(); len(ids) > 0 {
		instanceID = ids[0]
	}

	nodeName := ""
	if n != nil {
		nodeName = n.Name
	}

	// Get cluster name safely from context
	opts := options.FromContext(ctx)
	if opts == nil || opts.ClusterName == "" {
		log.FromContext(ctx).V(1).Info("skipping webhook notification: cluster name not configured")
		return nil
	}

	return &webhook.NotificationPayload{
		Timestamp:     msg.StartTime(),
		ClusterName:   opts.ClusterName,
		EventType:     string(msg.Kind()),
		EventReason:   eventReason,
		Message:       eventMessage,
		NodeClaimName: nodeClaim.Name,
		NodeName:      nodeName,
		InstanceID:    instanceID,
		InstanceType:  nodeClaim.Labels[corev1.LabelInstanceTypeStable],
		Zone:          nodeClaim.Labels[corev1.LabelTopologyZone],
		NodePoolName:  nodeClaim.Labels[karpv1.NodePoolLabelKey],
		CapacityType:  nodeClaim.Labels[karpv1.CapacityTypeLabelKey],
	}
}
