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

package rebalance

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
)

// Controller watches NodeClaims annotated with a rebalance grace period deadline and
// deletes them once the deadline has elapsed.
type Controller struct {
	kubeClient client.Client
	clk        clock.Clock
}

func NewController(kubeClient client.Client, clk clock.Clock) *Controller {
	return &Controller{
		kubeClient: kubeClient,
		clk:        clk,
	}
}

func (c *Controller) Reconcile(ctx context.Context, nodeClaim *karpv1.NodeClaim) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclaim.rebalance")
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("NodeClaim", klog.KObj(nodeClaim)))

	deadlineStr, exists := nodeClaim.Annotations[v1.AnnotationRebalanceGracePeriodEnd]
	if !exists {
		return reconcile.Result{}, nil
	}
	// Already being deleted
	if !nodeClaim.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}
	deadline, err := time.Parse(time.RFC3339, deadlineStr)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("parsing rebalance grace period deadline, %w", err)
	}
	remaining := deadline.Sub(c.clk.Now())
	if remaining > 0 {
		return reconcile.Result{RequeueAfter: remaining}, nil
	}
	if err := c.kubeClient.Delete(ctx, nodeClaim); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(fmt.Errorf("deleting nodeclaim after rebalance grace period, %w", err))
	}
	log.FromContext(ctx).Info("deleted nodeclaim after rebalance grace period elapsed")
	metrics.NodeClaimsDisruptedTotal.Inc(map[string]string{
		metrics.ReasonLabel:       string(messages.RebalanceRecommendationKind),
		metrics.NodePoolLabel:     nodeClaim.Labels[karpv1.NodePoolLabelKey],
		metrics.CapacityTypeLabel: nodeClaim.Labels[karpv1.CapacityTypeLabelKey],
	})
	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclaim.rebalance").
		For(&karpv1.NodeClaim{}, builder.WithPredicates(
			predicate.NewPredicateFuncs(func(o client.Object) bool {
				_, exists := o.GetAnnotations()[v1.AnnotationRebalanceGracePeriodEnd]
				return exists
			}),
		)).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
