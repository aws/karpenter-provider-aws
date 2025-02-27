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

	"github.com/samber/lo"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
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
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	"sigs.k8s.io/karpenter/pkg/utils/pod"
	"sigs.k8s.io/karpenter/pkg/utils/termination"
	volumeutil "sigs.k8s.io/karpenter/pkg/utils/volume"
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
	ctx = injection.WithControllerName(ctx, "node.termination")

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

	nodeClaims, err := nodeutils.GetNodeClaims(ctx, c.kubeClient, node)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing nodeclaims, %w", err)
	}

	if err = c.deleteAllNodeClaims(ctx, nodeClaims...); err != nil {
		return reconcile.Result{}, fmt.Errorf("deleting nodeclaims, %w", err)
	}

	nodeTerminationTime, err := c.nodeTerminationTime(node, nodeClaims...)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err = c.terminator.Taint(ctx, node, v1.DisruptedNoScheduleTaint); err != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, client.IgnoreNotFound(fmt.Errorf("tainting node with %s, %w", pretty.Taint(v1.DisruptedNoScheduleTaint), err))
	}
	if err = c.terminator.Drain(ctx, node, nodeTerminationTime); err != nil {
		if !terminator.IsNodeDrainError(err) {
			return reconcile.Result{}, fmt.Errorf("draining node, %w", err)
		}
		c.recorder.Publish(terminatorevents.NodeFailedToDrain(node, err))
		// If the underlying NodeClaim no longer exists, we want to delete to avoid trying to gracefully draining
		// on nodes that are no longer alive. We do a check on the Ready condition of the node since, even
		// though the CloudProvider says the instance is not around, we know that the kubelet process is still running
		// if the Node Ready condition is true
		// Similar logic to: https://github.com/kubernetes/kubernetes/blob/3a75a8c8d9e6a1ebd98d8572132e675d4980f184/staging/src/k8s.io/cloud-provider/controllers/nodelifecycle/node_lifecycle_controller.go#L144
		if nodeutils.GetCondition(node, corev1.NodeReady).Status != corev1.ConditionTrue {
			if _, err = c.cloudProvider.Get(ctx, node.Spec.ProviderID); err != nil {
				if cloudprovider.IsNodeClaimNotFoundError(err) {
					return reconcile.Result{}, c.removeFinalizer(ctx, node)
				}
				return reconcile.Result{}, fmt.Errorf("getting nodeclaim, %w", err)
			}
		}

		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	NodesDrainedTotal.Inc(map[string]string{
		metrics.NodePoolLabel: node.Labels[v1.NodePoolLabelKey],
	})
	// In order for Pods associated with PersistentVolumes to smoothly migrate from the terminating Node, we wait
	// for VolumeAttachments of drain-able Pods to be cleaned up before terminating Node and removing its finalizer.
	// However, if TerminationGracePeriod is configured for Node, and we are past that period, we will skip waiting.
	if nodeTerminationTime == nil || c.clock.Now().Before(*nodeTerminationTime) {
		areVolumesDetached, err := c.ensureVolumesDetached(ctx, node)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("ensuring no volume attachments, %w", err)
		}
		if !areVolumesDetached {
			return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}
	nodeClaims, err = nodeutils.GetNodeClaims(ctx, c.kubeClient, node)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("deleting nodeclaims, %w", err)
	}
	for _, nodeClaim := range nodeClaims {
		isInstanceTerminated, err := termination.EnsureTerminated(ctx, c.kubeClient, nodeClaim, c.cloudProvider)
		if err != nil {
			// 404 = the nodeClaim no longer exists
			if errors.IsNotFound(err) {
				continue
			}
			// 409 - The nodeClaim exists, but its status has already been modified
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, fmt.Errorf("ensuring instance termination, %w", err)
		}
		if !isInstanceTerminated {
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}
	if err := c.removeFinalizer(ctx, node); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (c *Controller) deleteAllNodeClaims(ctx context.Context, nodeClaims ...*v1.NodeClaim) error {
	for _, nodeClaim := range nodeClaims {
		// If we still get the NodeClaim, but it's already marked as terminating, we don't need to call Delete again
		if nodeClaim.DeletionTimestamp.IsZero() {
			if err := c.kubeClient.Delete(ctx, nodeClaim); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
	}
	return nil
}

func (c *Controller) ensureVolumesDetached(ctx context.Context, node *corev1.Node) (volumesDetached bool, err error) {
	volumeAttachments, err := nodeutils.GetVolumeAttachments(ctx, c.kubeClient, node)
	if err != nil {
		return false, err
	}
	// Filter out VolumeAttachments associated with not drain-able Pods
	filteredVolumeAttachments, err := filterVolumeAttachments(ctx, c.kubeClient, node, volumeAttachments, c.clock)
	if err != nil {
		return false, err
	}
	return len(filteredVolumeAttachments) == 0, nil
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

func (c *Controller) nodeTerminationTime(node *corev1.Node, nodeClaims ...*v1.NodeClaim) (*time.Time, error) {
	if len(nodeClaims) == 0 {
		return nil, nil
	}
	expirationTimeString, exists := nodeClaims[0].ObjectMeta.Annotations[v1.NodeClaimTerminationTimestampAnnotationKey]
	if !exists {
		return nil, nil
	}
	c.recorder.Publish(terminatorevents.NodeTerminationGracePeriodExpiring(node, expirationTimeString))
	expirationTime, err := time.Parse(time.RFC3339, expirationTimeString)
	if err != nil {
		return nil, fmt.Errorf("parsing %s annotation, %w", v1.NodeClaimTerminationTimestampAnnotationKey, err)
	}
	return &expirationTime, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("node.termination").
		For(&corev1.Node{}, builder.WithPredicates(nodeutils.IsManagedPredicateFuncs(c.cloudProvider))).
		WithOptions(
			controller.Options{
				RateLimiter: workqueue.NewTypedMaxOfRateLimiter[reconcile.Request](
					workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 10*time.Second),
					// 10 qps, 100 bucket size
					&workqueue.TypedBucketRateLimiter[reconcile.Request]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
				),
				MaxConcurrentReconciles: 100,
			},
		).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
