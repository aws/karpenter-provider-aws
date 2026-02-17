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

package static

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	disruptionutils "sigs.k8s.io/karpenter/pkg/utils/disruption"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
	nodepoolutils "sigs.k8s.io/karpenter/pkg/utils/nodepool"

	"sigs.k8s.io/karpenter/pkg/utils/pod"
)

const (
	TerminationReason = "overprovisioned"
)

type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	cluster       *state.Cluster
	clock         clock.Clock
}

func NewController(kubeClient client.Client, cluster *state.Cluster, cloudProvider cloudprovider.CloudProvider, clock clock.Clock) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		cluster:       cluster,
		clock:         clock,
	}
}

// Reconcile the resource
// Requeue after computing Static NodePool to ensure we don't miss any events
func (c *Controller) Name() string {
	return "static.deprovisioning"
}

func (c *Controller) Reconcile(ctx context.Context, np *v1.NodePool) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, c.Name())

	if !nodepoolutils.IsManaged(np, c.cloudProvider) || np.Spec.Replicas == nil {
		return reconcile.Result{}, nil
	}

	// We dont have to wait for cluster sync as we cannot really have internal state representing more NodeClaims than actual
	// During controller crashes we gradually populate our cluster/NodePoolState, as and when we populate we delete NC if we are over-provisioned
	runningNodeClaims, _, _ := c.cluster.NodePoolState.GetNodeCount(np.Name)
	desiredReplicas := lo.FromPtr(np.Spec.Replicas)
	// To avoid race conditions between deprovisioning and the disruption controller,
	// we only include running NodeClaims when counting for deprovisioning purposes.
	// Including both active NodeClaims and those pending disruption could cause us
	// to temporarily exceed the desired replica count while replacements are being created.
	nodeClaimsToDeprovision := int64(runningNodeClaims) - desiredReplicas

	// Only handle scale down - scale up is handled by provisioning controller
	if nodeClaimsToDeprovision <= 0 {
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	log.FromContext(ctx).WithValues("current", runningNodeClaims, "desired", desiredReplicas, "deprovision-count", nodeClaimsToDeprovision).
		Info("deprovisioning nodeclaims to satisfy replica count")

	// Get deprovisioning candidates
	candidates := c.getDeprovisioningCandidates(ctx, np, int(nodeClaimsToDeprovision))

	scaleDownErrs := make([]error, len(candidates))
	// Terminate selected NodeClaims
	workqueue.ParallelizeUntil(ctx, len(candidates), len(candidates), func(i int) {
		candidate := candidates[i]

		if err := retry.OnError(retry.DefaultBackoff, func(err error) bool { return client.IgnoreNotFound(err) != nil }, func() error {
			return c.kubeClient.Delete(ctx, candidate)
		}); err != nil && client.IgnoreNotFound(err) != nil {
			scaleDownErrs[i] = err
			return
		}

		log.FromContext(ctx).WithValues("NodeClaim", klog.KObj(candidate)).V(1).Info("deleting nodeclaim")

		// Mark the NodeClaim as Deleting in StateNodePool
		c.cluster.NodePoolState.MarkNodeClaimDeleting(np.Name, candidate.Name)
	})

	if scaleDownErr := multierr.Combine(scaleDownErrs...); scaleDownErr != nil {
		return reconcile.Result{}, fmt.Errorf("failed to deprovision nodeclaims, %w", scaleDownErr)
	}

	return reconcile.Result{RequeueAfter: time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named(c.Name()).
		// Reoncile on NodePool Create and Update (when replicas change)
		For(&v1.NodePool{}, builder.WithPredicates(nodepoolutils.IsManagedPredicateFuncs(c.cloudProvider), nodepoolutils.IsStaticPredicateFuncs(),
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldNP := e.ObjectOld.(*v1.NodePool)
					newNP := e.ObjectNew.(*v1.NodePool)
					return HasNodePoolReplicaCountChanged(oldNP, newNP)
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			})).
		// We care about Static NodeClaims creating as we might have over provisioned and need to deprovision
		Watches(&v1.NodeClaim{}, nodepoolutils.NodeClaimEventHandler(nodepoolutils.WithClient(c.kubeClient), nodepoolutils.WithStaticOnly), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}

func HasNodePoolReplicaCountChanged(oldNP, newNP *v1.NodePool) bool {
	return lo.FromPtr(oldNP.Spec.Replicas) != lo.FromPtr(newNP.Spec.Replicas)
}

// Returns NodeClaims suitable for deprovisioning, prioritizing:
// 1. Unresolved NodeClaims (no ProviderID yet - haven't launched)
// 2. Empty nodes (nodes with no pods or only DaemonSet pods without do-not-disrupt annotation)
// 3. If more nodes needed, nodes with lowest disruption cost (nodes with pods that have do-not-disrupt will have highest cost)
func (c *Controller) getDeprovisioningCandidates(ctx context.Context, np *v1.NodePool, count int) []*v1.NodeClaim {
	candidates := make([]*v1.NodeClaim, 0, count)

	// Unresolved NodeClaims (haven't launched yet or that failed Create call)
	unresolvedCandidates := c.unresolvedDeprovisioningCandidates(ctx, np.Name, count)
	candidates = append(candidates, unresolvedCandidates...)
	remaining := count - len(candidates)

	if remaining == 0 {
		return candidates
	}

	// Get all StateNodes for this NodePool
	nodes := make([]*state.StateNode, 0)
	for n := range c.cluster.Nodes() {
		if n.Labels()[v1.NodePoolLabelKey] == np.Name && n.NodeClaim != nil && !n.MarkedForDeletion() {
			nodes = append(nodes, n.DeepCopy())
		}
	}

	// Resolved nodes (empty first, then by disruption cost)
	resolvedCandidates := c.resolvedDeprovisioningCandidates(ctx, nodes, np, remaining)
	candidates = append(candidates, resolvedCandidates...)

	return candidates
}

// unResolvedDeprovisioningCandidates returns unresolved NodeClaims (those without ProviderID) up to the specified count
func (c *Controller) unresolvedDeprovisioningCandidates(ctx context.Context, nodePoolName string, count int) []*v1.NodeClaim {
	nodeClaimList, err := nodeclaimutils.ListManaged(ctx, c.kubeClient, c.cloudProvider, nodeclaimutils.ForNodePool(nodePoolName))
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to list nodeclaims")
		return nil
	}

	unresolvedNodeClaims := make([]*v1.NodeClaim, 0)
	for _, nc := range nodeClaimList {
		if nc.Status.ProviderID == "" && nc.DeletionTimestamp.IsZero() {
			unresolvedNodeClaims = append(unresolvedNodeClaims, nc)
		}
	}

	if len(unresolvedNodeClaims) == 0 {
		return nil
	}

	unresolvedToDelete := lo.Min([]int{count, len(unresolvedNodeClaims)})
	candidates := make([]*v1.NodeClaim, 0, unresolvedToDelete)
	for i := range unresolvedToDelete {
		candidates = append(candidates, unresolvedNodeClaims[i])
	}

	return candidates
}

// resolvedDeprovisioningCandidates returns resolved NodeClaims (those with ProviderID) up to the specified count,
// prioritizing empty nodes first, then nodes with lowest disruption cost
func (c *Controller) resolvedDeprovisioningCandidates(ctx context.Context, nodes []*state.StateNode, np *v1.NodePool, count int) []*v1.NodeClaim {
	if len(nodes) == 0 {
		return nil
	}

	candidates := make([]*v1.NodeClaim, 0, count)

	// Priority 1: Empty nodes
	emptyNodes := lo.Filter(nodes, func(node *state.StateNode, _ int) bool {
		pods, err := node.Pods(ctx, c.kubeClient)
		if err != nil {
			log.FromContext(ctx).WithValues("node", node.Name()).Error(err, "unable to list pods, treating as non-empty")
			return false
		}
		return len(pods) == 0 || lo.EveryBy(pods, pod.IsOwnedByDaemonSet) && lo.NoneBy(pods, pod.HasDoNotDisrupt)
	})

	for _, node := range lo.Slice(emptyNodes, 0, count) {
		candidates = append(candidates, node.NodeClaim)
	}

	remaining := count - len(candidates)

	if remaining == 0 {
		return candidates
	}

	// Get non-empty nodes with their costs
	type NonEmptyNode struct {
		node            *state.StateNode
		pods            []*corev1.Pod
		hasDoNotDisrupt bool
	}

	emptyNodesSet := sets.New(emptyNodes...)
	nonEmptyNodes := lo.FilterMap(nodes, func(node *state.StateNode, _ int) (NonEmptyNode, bool) {
		if emptyNodesSet.Has(node) {
			return NonEmptyNode{}, false
		}

		pods, err := node.Pods(ctx, c.kubeClient)
		if err != nil {
			log.FromContext(ctx).WithValues("node", node.Name()).Error(err, "unable to list pods, skipping node")
			return NonEmptyNode{}, false
		}

		return NonEmptyNode{
			node:            node,
			pods:            pods,
			hasDoNotDisrupt: lo.SomeBy(pods, pod.HasDoNotDisrupt),
		}, true
	})

	slices.SortFunc(nonEmptyNodes, func(i, j NonEmptyNode) int {
		// If one node has do-not-disrupt pods and the other doesn't, the one without should come first
		if i.hasDoNotDisrupt != j.hasDoNotDisrupt {
			return lo.Ternary(i.hasDoNotDisrupt, 1, -1)
		}
		// If neither has do-not-disrupt pods, compare their costs
		return cmp.Compare(
			disruptionutils.ReschedulingCost(ctx, i.pods)*disruptionutils.LifetimeRemaining(c.clock, np, i.node.NodeClaim),
			disruptionutils.ReschedulingCost(ctx, j.pods)*disruptionutils.LifetimeRemaining(c.clock, np, j.node.NodeClaim),
		)
	})

	// Take the remaining needed nodes with lowest cost
	for _, nwc := range lo.Slice(nonEmptyNodes, 0, remaining) {
		candidates = append(candidates, nwc.node.NodeClaim)
	}

	return candidates
}
