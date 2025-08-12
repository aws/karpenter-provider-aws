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

package lifecycle

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
)

type Registration struct {
	kubeClient client.Client
	recorder   events.Recorder
}

func (r *Registration) Reconcile(ctx context.Context, nodeClaim *v1.NodeClaim) (reconcile.Result, error) {
	if cond := nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered); !cond.IsUnknown() {
		// Ensure that we always set the status condition to the latest generation
		nodeClaim.StatusConditions().Set(*cond)
		return reconcile.Result{}, nil
	}
	node, err := nodeclaimutils.NodeForNodeClaim(ctx, r.kubeClient, nodeClaim)
	if err != nil {
		if nodeclaimutils.IsNodeNotFoundError(err) {
			nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeRegistered, "NodeNotFound", "Node not registered with cluster")
			return reconcile.Result{}, nil
		}
		if nodeclaimutils.IsDuplicateNodeError(err) {
			nodeClaim.StatusConditions().SetFalse(v1.ConditionTypeRegistered, "MultipleNodesFound", "Invariant violated, matched multiple nodes")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("getting node for nodeclaim, %w", err)
	}
	_, hasStartupTaint := lo.Find(node.Spec.Taints, func(t corev1.Taint) bool {
		return t.MatchTaint(&v1.UnregisteredNoExecuteTaint)
	})
	// if the sync hasn't happened yet and the race protecting startup taint isn't present then log it as missing and proceed
	// if the sync has happened then the startup taint has been removed if it was present
	if _, ok := node.Labels[v1.NodeRegisteredLabelKey]; !ok && !hasStartupTaint {
		log.FromContext(ctx).WithValues("taint", v1.UnregisteredTaintKey).Error(fmt.Errorf("missing taint prevents registration-related race conditions on Karpenter-managed nodes"), "node claim registration error")
		r.recorder.Publish(UnregisteredTaintMissingEvent(nodeClaim))
	}
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Node", klog.KObj(node)))
	if err = r.syncNode(ctx, nodeClaim, node); err != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, err
	}
	log.FromContext(ctx).Info("registered nodeclaim")
	nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeRegistered)
	nodeClaim.Status.NodeName = node.Name

	metrics.NodesCreatedTotal.Inc(map[string]string{
		metrics.NodePoolLabel: nodeClaim.Labels[v1.NodePoolLabelKey],
	})
	if err := r.updateNodePoolRegistrationHealth(ctx, nodeClaim); client.IgnoreNotFound(err) != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// updateNodePoolRegistrationHealth sets the NodeRegistrationHealthy=True
// on the NodePool if the nodeClaim that registered is owned by a NodePool
func (r *Registration) updateNodePoolRegistrationHealth(ctx context.Context, nodeClaim *v1.NodeClaim) error {
	nodePoolName := nodeClaim.Labels[v1.NodePoolLabelKey]
	if nodePoolName != "" {
		nodePool := &v1.NodePool{}
		if err := r.kubeClient.Get(ctx, types.NamespacedName{Name: nodePoolName}, nodePool); err != nil {
			return err
		}
		stored := nodePool.DeepCopy()
		if nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeRegistrationHealthy) {
			// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
			// can cause races due to the fact that it fully replaces the list on a change
			// Here, we are updating the status condition list
			if err := r.kubeClient.Status().Patch(ctx, nodePool, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Registration) syncNode(ctx context.Context, nodeClaim *v1.NodeClaim, node *corev1.Node) error {
	stored := node.DeepCopy()
	controllerutil.AddFinalizer(node, v1.TerminationFinalizer)

	node = nodeclaimutils.UpdateNodeOwnerReferences(nodeClaim, node)

	// We do not sync the taints if this label is present. We instead assume that the karpenter provider
	// is managing taints. We still manage/remove the unregistered taint to signal the end of syncing.
	if value, ok := node.Labels[v1.NodeDoNotSyncTaintsLabelKey]; !ok || value != "true" {
		// Sync all taints inside NodeClaim into the Node taints
		node.Spec.Taints = scheduling.Taints(node.Spec.Taints).Merge(nodeClaim.Spec.Taints)
		node.Spec.Taints = scheduling.Taints(node.Spec.Taints).Merge(nodeClaim.Spec.StartupTaints)
	}

	node.Annotations = lo.Assign(node.Annotations, nodeClaim.Annotations)
	// Remove karpenter.sh/unregistered taint
	node.Spec.Taints = lo.Reject(node.Spec.Taints, func(t corev1.Taint, _ int) bool {
		return t.MatchTaint(&v1.UnregisteredNoExecuteTaint)
	})
	node.Labels = lo.Assign(node.Labels, nodeClaim.Labels, map[string]string{
		v1.NodeRegisteredLabelKey: "true",
	})
	if !equality.Semantic.DeepEqual(stored, node) {
		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		if err := r.kubeClient.Patch(ctx, node, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
			return fmt.Errorf("syncing node, %w", err)
		}
	}
	return nil
}
