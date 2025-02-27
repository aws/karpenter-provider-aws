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

package hydration

import (
	"context"
	"fmt"

	"github.com/awslabs/operatorpkg/reasonable"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
)

// Controller hydrates information to the Node which is expected in newer versions of Karpenter, but would not exist on
// pre-existing nodes.
type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, n *corev1.Node) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, c.Name())
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Node", klog.KRef(n.Namespace, n.Name)))

	nc, err := nodeutils.NodeClaimForNode(ctx, c.kubeClient, n)
	if err != nil {
		if nodeutils.IsDuplicateNodeClaimError(err) || nodeutils.IsNodeClaimNotFoundError(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("hydrating node, %w", err)
	}
	if !nodeclaimutils.IsManaged(nc, c.cloudProvider) {
		return reconcile.Result{}, nil
	}

	stored := n.DeepCopy()
	n.Labels = lo.Assign(n.Labels, map[string]string{
		v1.NodeClassLabelKey(nc.Spec.NodeClassRef.GroupKind()): nc.Spec.NodeClassRef.Name,
	})
	if !equality.Semantic.DeepEqual(stored, n) {
		if err := c.kubeClient.Patch(ctx, n, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, client.IgnoreNotFound(err)
		}
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Name() string {
	return "node.hydration"
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named(c.Name()).
		For(&corev1.Node{}).
		Watches(&v1.NodeClaim{}, nodeutils.NodeClaimEventHandler(c.kubeClient)).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 1000,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
