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

package debug

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
)

type NodeController struct {
	kubeClient client.Client
}

func NewNodeController(kubeClient client.Client) *NodeController {
	return &NodeController{
		kubeClient: kubeClient,
	}
}

func (c *NodeController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	n := &corev1.Node{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, n); err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("[DELETED %s] NODE %s\n", time.Now().Format(time.RFC3339), req.String())
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	fmt.Printf("[CREATED/UPDATED %s] NODE %s %s\n", time.Now().Format(time.RFC3339), req.Name, c.GetInfo(ctx, n))
	return reconcile.Result{}, nil
}

func (c *NodeController) GetInfo(ctx context.Context, n *corev1.Node) string {
	pods, _ := nodeutils.GetPods(ctx, c.kubeClient, n)
	return fmt.Sprintf("ready=%s schedulable=%t initialized=%s pods=%d taints=%v", nodeutils.GetCondition(n, corev1.NodeReady).Status, !n.Spec.Unschedulable, n.Labels[karpv1.NodeInitializedLabelKey], len(pods), n.Spec.Taints)
}

func (c *NodeController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("node").
		For(&corev1.Node{}).
		WithEventFilter(predicate.And(
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldNode := e.ObjectOld.(*corev1.Node)
					newNode := e.ObjectNew.(*corev1.Node)
					return c.GetInfo(ctx, oldNode) != c.GetInfo(ctx, newNode)
				},
			},
			predicate.NewPredicateFuncs(func(o client.Object) bool {
				return o.GetLabels()[karpv1.NodePoolLabelKey] != ""
			}),
		)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10, SkipNameValidation: lo.ToPtr(true)}).
		Complete(c)
}
