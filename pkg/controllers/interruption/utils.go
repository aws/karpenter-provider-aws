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

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cache"
	interruptionevents "github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/events"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
)

type Action string

const (
	CordonAndDrain      Action = "CordonAndDrain"
	ForcefulTermination Action = "ForcefulTermination"
	NoAction            Action = "NoAction"
)

// InterruptionHandler contains shared logic for handling interruption messages
// from both the SQS queue and the DescribeInstanceStatus API.
type InterruptionHandler struct {
	kubeClient                  client.Client
	clk                         clock.Clock
	cloudProvider               cloudprovider.CloudProvider
	recorder                    events.Recorder
	unavailableOfferingsCache   *cache.UnavailableOfferings
	capacityReservationProvider capacityreservation.Provider
}

// handleMessage takes an action against every node involved in the message that is owned by a NodePool.
// When dryRun is true, it resolves NodeClaims but skips the actual cordon/drain action.
// Returns true if at least one matching NodeClaim was found in the cluster.
func (h *InterruptionHandler) handleMessage(ctx context.Context, msg messages.Message, dryRun bool) (found bool, err error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("messageKind", msg.Kind()))

	if msg.Kind() == messages.NoOpKind {
		return false, nil
	}
	for _, instanceID := range msg.EC2InstanceIDs() {
		nodeClaimList := &karpv1.NodeClaimList{}
		if e := h.kubeClient.List(ctx, nodeClaimList, client.MatchingFields{"status.instanceID": instanceID}); e != nil {
			err = multierr.Append(err, e)
			continue
		}
		if len(nodeClaimList.Items) == 0 {
			continue
		}
		found = true
		if dryRun {
			continue
		}
		for _, nodeClaim := range nodeClaimList.Items {
			nodeList := &corev1.NodeList{}
			if e := h.kubeClient.List(ctx, nodeList, client.MatchingFields{"spec.instanceID": instanceID}); e != nil {
				err = multierr.Append(err, e)
				continue
			}
			var node *corev1.Node
			if len(nodeList.Items) > 0 {
				node = &nodeList.Items[0]
			}
			if e := h.handleNodeClaim(ctx, msg, &nodeClaim, node); e != nil {
				err = multierr.Append(err, e)
			}
		}
	}
	if err != nil {
		return found, fmt.Errorf("acting on NodeClaims, %w", err)
	}
	return found, nil
}

// handleNodeClaim retrieves the action for the message and then performs the appropriate action against the node
func (h *InterruptionHandler) handleNodeClaim(ctx context.Context, msg messages.Message, nodeClaim *karpv1.NodeClaim, node *corev1.Node) error {
	action := actionForMessage(msg)
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("NodeClaim", klog.KObj(nodeClaim), "action", string(action)))
	if node != nil {
		ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Node", klog.KObj(node)))
	}

	// Record metric and event for this action
	h.notifyForMessage(msg, nodeClaim, node)
	h.markUnavailableOfferings(ctx, msg, nodeClaim)

	switch action {
	case ForcefulTermination:
		// TODO(Node Repair): Once Node Repair (kubernetes-sigs/karpenter#2398) graduates to GA,
		// this should be migrated to use repair policies instead of directly annotating the
		// termination timestamp.
		if err := h.annotateTerminationTimestamp(ctx, nodeClaim); err != nil {
			return err
		}
		return h.deleteNodeClaim(ctx, msg, nodeClaim, node)
	case CordonAndDrain:
		return h.deleteNodeClaim(ctx, msg, nodeClaim, node)
	default:
		return nil
	}
}

// markUnavailableOfferings updates caches when an interruption signals that an offering is no longer available.
func (h *InterruptionHandler) markUnavailableOfferings(ctx context.Context, msg messages.Message, nodeClaim *karpv1.NodeClaim) {
	if msg.Kind() == messages.SpotInterruptionKind && h.unavailableOfferingsCache != nil {
		zone := nodeClaim.Labels[corev1.LabelTopologyZone]
		instanceType := nodeClaim.Labels[corev1.LabelInstanceTypeStable]
		if zone != "" && instanceType != "" {
			unavailableReason := map[string]string{
				"reason": string(msg.Kind()),
			}
			h.unavailableOfferingsCache.MarkUnavailable(ctx, ec2types.InstanceType(instanceType), zone, karpv1.CapacityTypeSpot, unavailableReason)
		}
	}
	if msg.Kind() == messages.CapacityReservationInterruptionKind && h.capacityReservationProvider != nil {
		reservationID := nodeClaim.Labels[v1.LabelCapacityReservationID]
		if reservationID != "" {
			h.capacityReservationProvider.MarkUnavailable(reservationID)
		}
	}
}

// deleteNodeClaim removes the NodeClaim from the api-server
func (h *InterruptionHandler) deleteNodeClaim(ctx context.Context, msg messages.Message, nodeClaim *karpv1.NodeClaim, node *corev1.Node) error {
	if !nodeClaim.DeletionTimestamp.IsZero() {
		return nil
	}
	if err := h.kubeClient.Delete(ctx, nodeClaim); err != nil {
		return client.IgnoreNotFound(fmt.Errorf("deleting the node on interruption message, %w", err))
	}
	log.FromContext(ctx).Info("initiating delete from interruption message")
	h.recorder.Publish(interruptionevents.TerminatingOnInterruption(node, nodeClaim)...)
	metrics.NodeClaimsDisruptedTotal.Inc(map[string]string{
		metrics.ReasonLabel:       string(msg.Kind()),
		metrics.NodePoolLabel:     nodeClaim.Labels[karpv1.NodePoolLabelKey],
		metrics.CapacityTypeLabel: nodeClaim.Labels[karpv1.CapacityTypeLabelKey],
	})
	return nil
}

// annotateTerminationTimestamp sets the NodeClaimTerminationTimestampAnnotationKey annotation
// to the current time, causing the termination controller to bypass graceful drain (PDB-respecting
// eviction) and volume detachment waits. This is used for instance health failures where the
// instance is already broken and graceful drain may not be possible.
func (h *InterruptionHandler) annotateTerminationTimestamp(ctx context.Context, nodeClaim *karpv1.NodeClaim) error {
	if _, exists := nodeClaim.Annotations[karpv1.NodeClaimTerminationTimestampAnnotationKey]; exists {
		return nil
	}
	stored := nodeClaim.DeepCopy()
	nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{
		karpv1.NodeClaimTerminationTimestampAnnotationKey: h.clk.Now().Format(time.RFC3339),
	})
	if err := h.kubeClient.Patch(ctx, nodeClaim, client.MergeFrom(stored)); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

// notifyForMessage publishes the relevant alert based on the message kind
func (h *InterruptionHandler) notifyForMessage(msg messages.Message, nodeClaim *karpv1.NodeClaim, n *corev1.Node) {
	switch msg.Kind() {
	case messages.RebalanceRecommendationKind:
		h.recorder.Publish(interruptionevents.RebalanceRecommendation(n, nodeClaim)...)
	case messages.ScheduledChangeKind, messages.EventStatusKind, messages.InstanceStatusKind, messages.SystemStatusKind:
		h.recorder.Publish(interruptionevents.Unhealthy(n, nodeClaim)...)
	case messages.SpotInterruptionKind:
		h.recorder.Publish(interruptionevents.SpotInterrupted(n, nodeClaim)...)
	case messages.CapacityReservationInterruptionKind:
		h.recorder.Publish(interruptionevents.CapacityReservationInstanceInterrupted(n, nodeClaim)...)
	case messages.InstanceStoppedKind:
		h.recorder.Publish(interruptionevents.Stopping(n, nodeClaim)...)
	case messages.InstanceTerminatedKind:
		h.recorder.Publish(interruptionevents.Terminating(n, nodeClaim)...)
	default:
	}
}

func actionForMessage(msg messages.Message) Action {
	switch msg.Kind() {
	case messages.InstanceStatusKind, messages.SystemStatusKind:
		return ForcefulTermination
	case messages.ScheduledChangeKind, messages.EventStatusKind, messages.SpotInterruptionKind, messages.InstanceStoppedKind, messages.InstanceTerminatedKind, messages.CapacityReservationInterruptionKind:
		return CordonAndDrain
	default:
		return NoAction
	}
}
