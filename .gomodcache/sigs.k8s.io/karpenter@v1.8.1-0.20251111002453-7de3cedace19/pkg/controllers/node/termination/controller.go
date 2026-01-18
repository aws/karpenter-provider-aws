/*
Copyright The Kubernetes Authors.

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

package termination

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/node/termination/terminator"
	terminatorevents "sigs.k8s.io/karpenter/pkg/controllers/node/termination/terminator/events"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"
	utilscontroller "sigs.k8s.io/karpenter/pkg/utils/controller"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	"sigs.k8s.io/karpenter/pkg/utils/pod"
	volumeutil "sigs.k8s.io/karpenter/pkg/utils/volume"
)

const (
	minReconciles = 100
	maxReconciles = 5000
)

// Controller for the resource
type Controller struct {
	clock         clock.Clock
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	terminator    *terminator.Terminator
	recorder      events.Recorder
}

// NewController constructs a controller instance
func NewController(clk clock.Clock, kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, terminator *terminator.Terminator, recorder events.Recorder) *Controller {
	return &Controller{
		clock:         clk,
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		terminator:    terminator,
		recorder:      recorder,
	}
}

func (c *Controller) Reconcile(ctx context.Context, n *corev1.Node) (reconcile.Result, error) {
	if !n.GetDeletionTimestamp().IsZero() {
		return c.finalize(ctx, n)
	}
	return reconcile.Result{}, nil
}

//nolint:gocyclo
func (c *Controller) finalize(ctx context.Context, node *corev1.Node) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(node, v1.TerminationFinalizer) {
		return reconcile.Result{}, nil
	}
	if !nodeutils.IsManaged(node, c.cloudProvider) {
		return reconcile.Result{}, nil
	}

	// We're not guaranteed to find a NodeClaim for the node (e.g. if we failed to persist the provider ID to the nodeclaim
	// at launch). If there are duplicate NodeClaims, we will treat it as though there is no NodeClaim since there is no
	// longer a single source of truth.
	nodeClaim, err := nodeutils.NodeClaimForNode(ctx, c.kubeClient, node)
	if nodeutils.IgnoreDuplicateNodeClaimError(nodeutils.IgnoreNodeClaimNotFoundError(err)) != nil {
		return reconcile.Result{}, err
	}
	if nodeClaim != nil {
		ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("NodeClaim", klog.KObj(nodeClaim)))
		if nodeClaim.DeletionTimestamp.IsZero() {
			if err := c.kubeClient.Delete(ctx, nodeClaim); client.IgnoreNotFound(err) != nil {
				return reconcile.Result{}, fmt.Errorf("deleting nodeclaim, %w", err)
			}
		}
	}

	// If the underlying instance no longer exists, we want to delete to avoid trying to gracefully draining the
	// associated node. We do a check on the Ready condition of the node since, even though the CloudProvider says the
	// instance is not around, we know that the kubelet process is still running if the Node Ready condition is true.
	// Similar logic to: https://github.com/kubernetes/kubernetes/blob/3a75a8c8d9e6a1ebd98d8572132e675d4980f184/staging/src/k8s.io/cloud-provider/controllers/nodelifecycle/node_lifecycle_controller.go#L144
	if nodeutils.GetCondition(node, corev1.NodeReady).Status != corev1.ConditionTrue {
		if _, err = c.cloudProvider.Get(ctx, node.Spec.ProviderID); err != nil {
			if cloudprovider.IsNodeClaimNotFoundError(err) {
				return reconcile.Result{}, c.removeFinalizer(ctx, node)
			}
			return reconcile.Result{}, fmt.Errorf("getting nodeclaim, %w", err)
		}
	}

	nodeTerminationTime, err := c.nodeTerminationTime(node, nodeClaim)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err = c.terminator.Taint(ctx, node, v1.DisruptedNoScheduleTaint); err != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, serrors.Wrap(fmt.Errorf("tainting node, %w", err), "taint", pretty.Taint(v1.DisruptedNoScheduleTaint))
	}

	var stored *v1.NodeClaim
	if nodeClaim != nil {
		stored = nodeClaim.DeepCopy()
	}
	var terminationErr error
	var result reconcile.Result
	for _, f := range []terminationFunc{
		c.awaitDrain,
		c.awaitVolumeDetachment,
		c.awaitInstanceTermination,
	} {
		result, terminationErr = f(ctx, nodeClaim, node, nodeTerminationTime)
		if !lo.IsEmpty(result) || terminationErr != nil {
			break
		}
	}
	// If we don't have a NodeClaim, then there's nothing for us to patch here
	if stored != nil && !equality.Semantic.DeepEqual(stored, nodeClaim) {
		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the status condition list
		if err = c.kubeClient.Status().Patch(ctx, nodeClaim, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); client.IgnoreNotFound(err) != nil {
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, fmt.Errorf("updating nodeclaim, %w", err)
		}
		// We only increment the drained metric after we have ensured that we have patched the status condition onto the NodeClaim
		if !stored.StatusConditions().IsTrue(v1.ConditionTypeDrained) && nodeClaim.StatusConditions().IsTrue(v1.ConditionTypeDrained) {
			// We'll only increment this metric if there is a NodeClaim present for the node, but this prevents us from double
			// counting over multiple reconciles.
			NodesDrainedTotal.Inc(map[string]string{
				metrics.NodePoolLabel: node.Labels[v1.NodePoolLabelKey],
			})
		}
		// We sleep here after a patch operation since we want to ensure that we are able to read our own writes
		// so that we avoid duplicating metrics and log lines due to quick re-queues from our node watcher
		// USE CAUTION when determining whether to increase this timeout or remove this line
		time.After(time.Second)
	}
	if terminationErr != nil {
		return reconcile.Result{}, terminationErr
	}
	if !lo.IsEmpty(result) {
		return result, nil
	}
	if err = c.removeFinalizer(ctx, node); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

type terminationFunc func(context.Context, *v1.NodeClaim, *corev1.Node, *time.Time) (reconcile.Result, error)

// awaitDrain initiates the drain of the node and will continue to requeue until the node has been drained. If the
// nodeClaim has a terminationGracePeriod set, pods will be deleted to ensure this function does not requeue past the
// nodeTerminationTime.
func (c *Controller) awaitDrain(
	ctx context.Context,
	nodeClaim *v1.NodeClaim,
	node *corev1.Node,
	nodeTerminationTime *time.Time,
) (reconcile.Result, error) {
	if err := c.terminator.Drain(ctx, node, nodeTerminationTime); err != nil {
		if !terminator.IsNodeDrainError(err) {
			return reconcile.Result{}, fmt.Errorf("draining node, %w", err)
		}
		c.recorder.Publish(terminatorevents.NodeFailedToDrain(node, err))
		if nodeClaim != nil {
			nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeDrained, "Draining", "Draining")
		}
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if nodeClaim != nil {
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrained)
	}
	return reconcile.Result{}, nil
}

// awaitVolumeDetachment will continue to requeue until all volume attachments associated with the node have been
// deleted. The deletion is performed by the upstream attach-detach controller, Karpenter just needs to await deletion.
// This will be skipped once the nodeClaim's terminationGracePeriod has elapsed at nodeTerminationTime.
//
//nolint:gocyclo
func (c *Controller) awaitVolumeDetachment(
	ctx context.Context,
	nodeClaim *v1.NodeClaim,
	node *corev1.Node,
	nodeTerminationTime *time.Time,
) (reconcile.Result, error) {
	// In order for Pods associated with PersistentVolumes to smoothly migrate from the terminating Node, we wait
	// for VolumeAttachments of drain-able Pods to be cleaned up before terminating Node and removing its finalizer.
	// However, if TerminationGracePeriod is configured for Node, and we are past that period, we will skip waiting.
	pendingVolumeAttachments, err := c.pendingVolumeAttachments(ctx, node)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("ensuring no volume attachments, %w", err)
	}
	if len(pendingVolumeAttachments) == 0 {
		// There are no remaining volume attachments blocking instance termination. If we've already updated the status
		// condition, fall through. Otherwise, update the status condition and requeue.
		if nodeClaim != nil {
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeVolumesDetached)
		}
		return reconcile.Result{}, nil
	}

	if !c.hasTerminationGracePeriodElapsed(nodeTerminationTime) {
		// There are volume attachments blocking instance termination remaining. We should set the status condition to
		// unknown (if not already) and requeue. This case should never fall through, to continue to instance termination
		// one of two conditions must be met: all blocking volume attachment objects must be deleted or the nodeclaim's TGP
		// must have expired.
		c.recorder.Publish(terminatorevents.NodeAwaitingVolumeDetachmentEvent(node, pendingVolumeAttachments...))
		if nodeClaim != nil {
			nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeVolumesDetached, "AwaitingVolumeDetachment", "AwaitingVolumeDetachment")
		}
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// There are volume attachments blocking instance termination remaining, but the nodeclaim's TGP has expired. In this
	// case we should set the status condition to false (requeing if it wasn't already) and then fall through to instance
	// termination.
	if nodeClaim != nil {
		nodeClaim.StatusConditions().SetFalse(v1.ConditionTypeVolumesDetached, "TerminationGracePeriodElapsed", "TerminationGracePeriodElapsed")
	}
	return reconcile.Result{}, nil
}

// awaitInstanceTermination will initiate instance termination and continue to requeue until the cloudprovider indicates
// the instance is no longer found. Once gone, the node's finalizer will be removed, unblocking the NodeClaim lifecycle
// controller.
func (c *Controller) awaitInstanceTermination(
	ctx context.Context,
	nodeClaim *v1.NodeClaim,
	_ *corev1.Node,
	_ *time.Time,
) (reconcile.Result, error) {
	if nodeClaim == nil {
		return reconcile.Result{}, nil
	}
	deleteErr := c.cloudProvider.Delete(ctx, nodeClaim)
	if cloudprovider.IgnoreNodeClaimNotFoundError(deleteErr) != nil {
		return reconcile.Result{}, deleteErr
	}
	nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeInstanceTerminating)
	if !cloudprovider.IsNodeClaimNotFoundError(deleteErr) {
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return reconcile.Result{}, nil
}

func (c *Controller) hasTerminationGracePeriodElapsed(nodeTerminationTime *time.Time) bool {
	if nodeTerminationTime == nil {
		return false
	}
	return c.clock.Now().After(*nodeTerminationTime)
}

func (c *Controller) pendingVolumeAttachments(ctx context.Context, node *corev1.Node) ([]*storagev1.VolumeAttachment, error) {
	volumeAttachments, err := nodeutils.GetVolumeAttachments(ctx, c.kubeClient, node)
	if err != nil {
		return nil, err
	}
	// Filter out VolumeAttachments associated with not drain-able Pods
	filteredVolumeAttachments, err := filterVolumeAttachments(ctx, c.kubeClient, node, volumeAttachments, c.clock)
	if err != nil {
		return nil, err
	}
	return filteredVolumeAttachments, nil
}

// filterVolumeAttachments filters out storagev1.VolumeAttachments that should not block the termination
// of the passed corev1.Node
func filterVolumeAttachments(ctx context.Context, kubeClient client.Client, node *corev1.Node, volumeAttachments []*storagev1.VolumeAttachment, clk clock.Clock) ([]*storagev1.VolumeAttachment, error) {
	// No need to filter empty VolumeAttachments list
	if len(volumeAttachments) == 0 {
		return volumeAttachments, nil
	}
	// Create list of non-drain-able Pods associated with Node
	pods, err := nodeutils.GetPods(ctx, kubeClient, node)
	if err != nil {
		return nil, err
	}
	unDrainablePods := lo.Reject(pods, func(p *corev1.Pod, _ int) bool {
		return pod.IsDrainable(p, clk)
	})
	// Filter out VolumeAttachments associated with non-drain-able Pods
	// Match on Pod -> PersistentVolumeClaim -> PersistentVolume Name <- VolumeAttachment
	shouldFilterOutVolume := sets.New[string]()
	for _, p := range unDrainablePods {
		for _, v := range p.Spec.Volumes {
			pvc, err := volumeutil.GetPersistentVolumeClaim(ctx, kubeClient, p, v)
			if errors.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if pvc != nil {
				shouldFilterOutVolume.Insert(pvc.Spec.VolumeName)
			}
		}
	}
	filteredVolumeAttachments := lo.Reject(volumeAttachments, func(v *storagev1.VolumeAttachment, _ int) bool {
		pvName := v.Spec.Source.PersistentVolumeName
		return pvName == nil || shouldFilterOutVolume.Has(*pvName)
	})
	return filteredVolumeAttachments, nil
}

func (c *Controller) removeFinalizer(ctx context.Context, n *corev1.Node) error {
	stored := n.DeepCopy()
	controllerutil.RemoveFinalizer(n, v1.TerminationFinalizer)
	if !equality.Semantic.DeepEqual(stored, n) {
		// We use client.StrategicMergeFrom here since the node object supports it and
		// a strategic merge patch represents the finalizer list as a keyed "set" so removing
		// an item from the list doesn't replace the full list
		// https://github.com/kubernetes/kubernetes/issues/111643#issuecomment-2016489732
		if err := c.kubeClient.Patch(ctx, n, client.StrategicMergeFrom(stored)); err != nil {
			return client.IgnoreNotFound(fmt.Errorf("removing finalizer, %w", err))
		}

		metrics.NodesTerminatedTotal.Inc(map[string]string{
			metrics.NodePoolLabel: n.Labels[v1.NodePoolLabelKey],
		})

		// We use stored.DeletionTimestamp since the api-server may give back a node after the patch without a deletionTimestamp
		DurationSeconds.Observe(time.Since(stored.DeletionTimestamp.Time).Seconds(), map[string]string{
			metrics.NodePoolLabel: n.Labels[v1.NodePoolLabelKey],
		})

		NodeLifetimeDurationSeconds.Observe(time.Since(n.CreationTimestamp.Time).Seconds(), map[string]string{
			metrics.NodePoolLabel: n.Labels[v1.NodePoolLabelKey],
		})

		log.FromContext(ctx).Info("deleted node")
	}
	return nil
}

func (c *Controller) nodeTerminationTime(node *corev1.Node, nodeClaim *v1.NodeClaim) (*time.Time, error) {
	if nodeClaim == nil {
		return nil, nil
	}
	expirationTimeString, exists := nodeClaim.Annotations[v1.NodeClaimTerminationTimestampAnnotationKey]
	if !exists {
		return nil, nil
	}
	c.recorder.Publish(terminatorevents.NodeTerminationGracePeriodExpiring(node, expirationTimeString))
	expirationTime, err := time.Parse(time.RFC3339, expirationTimeString)
	if err != nil {
		return nil, serrors.Wrap(fmt.Errorf("parsing annotation, %w", err), "annotation", v1.NodeClaimTerminationTimestampAnnotationKey)
	}
	return &expirationTime, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	maxConcurrentReconciles := utilscontroller.LinearScaleReconciles(utilscontroller.CPUCount(ctx), minReconciles, maxReconciles)
	qps, bucketSize := utilscontroller.GetTypedBucketConfigs(10, minReconciles, maxConcurrentReconciles)
	return controllerruntime.NewControllerManagedBy(m).
		Named("node.termination").
		For(&corev1.Node{}, builder.WithPredicates(nodeutils.IsManagedPredicateFuncs(c.cloudProvider))).
		WithOptions(
			controller.Options{
				RateLimiter: workqueue.NewTypedMaxOfRateLimiter[reconcile.Request](
					workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 10*time.Second),
					// qps scales linearly at 10% of concurrentReconciles, bucket size is 10 * qps
					&workqueue.TypedBucketRateLimiter[reconcile.Request]{Limiter: rate.NewLimiter(rate.Limit(qps), bucketSize)},
				),
				MaxConcurrentReconciles: maxConcurrentReconciles,
			},
		).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
