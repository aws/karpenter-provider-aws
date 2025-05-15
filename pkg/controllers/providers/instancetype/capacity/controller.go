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

package capacity

import (
	"context"
	"fmt"

	"github.com/awslabs/operatorpkg/reasonable"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
)

type Controller struct {
	kubeClient           client.Client
	cloudProvider        cloudprovider.CloudProvider
	instancetypeProvider *instancetype.DefaultProvider
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, instancetypeProvider *instancetype.DefaultProvider) *Controller {
	return &Controller{
		kubeClient:           kubeClient,
		cloudProvider:        cloudProvider,
		instancetypeProvider: instancetypeProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, node *corev1.Node) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "providers.instancetype.capacity")
	if !nodeutils.IsManaged(node, c.cloudProvider) {
		return reconcile.Result{}, nil
	}
	nodeClaim, err := nodeutils.NodeClaimForNode(ctx, c.kubeClient, node)
	if err != nil {
		return reconcile.Result{}, nodeutils.IgnoreNodeClaimNotFoundError(fmt.Errorf("failed to get nodeclaim for node, %w", err))
	}

	nodeClass := &v1.EC2NodeClass{}
	if err = c.kubeClient.Get(ctx, client.ObjectKey{Name: nodeClaim.Spec.NodeClassRef.Name}, nodeClass); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get ec2nodeclass, %w", err)
	}
	if err := c.instancetypeProvider.UpdateInstanceTypeCapacityFromNode(ctx, node, nodeClaim, nodeClass); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating discovered capacity cache, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("providers.instancetype.capacity").
		For(&corev1.Node{}, builder.WithPredicates(predicate.TypedFuncs[client.Object]{
			// Only trigger reconciliation once a node becomes registered. This is an optimization to omit no-op reconciliations and reduce lock contention on the cache.
			UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
				if e.ObjectOld.GetLabels()[karpv1.NodeRegisteredLabelKey] != "" {
					return false
				}
				return e.ObjectNew.GetLabels()[karpv1.NodeRegisteredLabelKey] == "true"
			},
			// Reconcile against all Nodes added to the informer cache in a registered state. This allows us to hydrate the discovered capacity cache on controller startup.
			CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
				return e.Object.GetLabels()[karpv1.NodeRegisteredLabelKey] == "true"
			},
			DeleteFunc:  func(e event.TypedDeleteEvent[client.Object]) bool { return false },
			GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool { return false },
		}, nodeutils.IsManagedPredicateFuncs(c.cloudProvider))).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 1,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
