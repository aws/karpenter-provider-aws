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

package disruption

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
	"sigs.k8s.io/karpenter/pkg/utils/result"
)

type nodeClaimReconciler interface {
	Reconcile(context.Context, *v1.NodePool, *v1.NodeClaim) (reconcile.Result, error)
}

// Controller is a disruption controller that adds StatusConditions to nodeclaims when they meet certain disruption conditions
// e.g. When the NodeClaim has become empty, then it is marked as "Empty" in the StatusConditions
type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider

	drift         *Drift
	consolidation *Consolidation
}

// NewController constructs a nodeclaim disruption controller. Note that every sub-controller has a dependency on its nodepool.
// Disruption mechanisms that don't depend on the nodepool (like expiration), should live elsewhere.
func NewController(clk clock.Clock, kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		drift:         &Drift{clock: clk, cloudProvider: cloudProvider, instanceTypeNotFoundCheckCache: cache.New(time.Minute*30, time.Minute)},
		consolidation: &Consolidation{kubeClient: kubeClient, clock: clk},
	}
}

// Reconcile executes a control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, nodeClaim *v1.NodeClaim) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclaim.disruption")
	if nodeClaim.Status.NodeName != "" {
		ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Node", klog.KRef("", nodeClaim.Status.NodeName)))
	}

	if !nodeclaimutils.IsManaged(nodeClaim, c.cloudProvider) {
		return reconcile.Result{}, nil
	}
	if !nodeClaim.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	stored := nodeClaim.DeepCopy()
	nodePoolName, ok := nodeClaim.Labels[v1.NodePoolLabelKey]
	if !ok {
		return reconcile.Result{}, nil
	}
	nodePool := &v1.NodePool{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: nodePoolName}, nodePool); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	var results []reconcile.Result
	var errs error
	reconcilers := []nodeClaimReconciler{
		c.drift,
		c.consolidation,
	}
	for _, reconciler := range reconcilers {
		res, err := reconciler.Reconcile(ctx, nodePool, nodeClaim)
		errs = multierr.Append(errs, err)
		results = append(results, res)
	}
	if !equality.Semantic.DeepEqual(stored, nodeClaim) {
		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the status condition list
		if err := c.kubeClient.Status().Patch(ctx, nodeClaim, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, client.IgnoreNotFound(err)
		}
	}
	if errs != nil {
		return reconcile.Result{}, errs
	}
	return result.Min(results...), nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	b := controllerruntime.NewControllerManagedBy(m).
		Named("nodeclaim.disruption").
		For(&v1.NodeClaim{}, builder.WithPredicates(nodeclaimutils.IsManagedPredicateFuncs(c.cloudProvider))).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Watches(&v1.NodePool{}, nodeclaimutils.NodePoolEventHandler(c.kubeClient, c.cloudProvider)).
		Watches(&corev1.Pod{}, nodeclaimutils.PodEventHandler(c.kubeClient, c.cloudProvider))

	for _, nodeClass := range c.cloudProvider.GetSupportedNodeClasses() {
		b.Watches(nodeClass, nodeclaimutils.NodeClassEventHandler(c.kubeClient))
	}
	return b.Complete(reconcile.AsReconciler(m.GetClient(), c))
}

func (c *Controller) Reset() {
	c.drift.instanceTypeNotFoundCheckCache.Flush()
}
