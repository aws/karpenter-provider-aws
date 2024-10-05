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

package vmcapacitycache

import (
	"context"
	"fmt"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/awslabs/operatorpkg/reasonable"
	"github.com/awslabs/operatorpkg/singleton"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
)

type Controller struct {
	kubeClient           client.Client
	instancetypeProvider *instancetype.DefaultProvider
	initialized          bool
}

func NewController(kubeClient client.Client, instancetypeProvider *instancetype.DefaultProvider) *Controller {
	return &Controller{
		kubeClient:           kubeClient,
		instancetypeProvider: instancetypeProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, r reconcile.Request) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "instancetypes.vmcapacitycache")
	if !c.initialized {
		return c.reconcileAllNodes(ctx)
	}
	if err := c.instancetypeProvider.UpdateVMCapacityCache(ctx, c.kubeClient, r.Name); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating vm capacity cache, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) reconcileAllNodes(ctx context.Context) (reconcile.Result, error) {
	nodeList := &corev1.NodeList{}
	if err := c.kubeClient.List(ctx, nodeList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{karpv1.NodeRegisteredLabelKey: "true"}),
	}); err != nil {
		return reconcile.Result{}, err
	}
	errs := make([]error, len(nodeList.Items))
	for i, node := range nodeList.Items {
		if err := c.instancetypeProvider.UpdateVMCapacityCache(ctx, c.kubeClient, node.Name); err != nil {
			errs[i] = err
		}
	}
	if err := multierr.Combine(errs...); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating vm capacity cache, %w", err)
	}
	c.initialized = true
	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instancetypes.vmcapacitycache").
		For(&corev1.Node{}, builder.WithPredicates(predicate.TypedFuncs[client.Object]{
			// Only trigger reconciliation once a node becomes registered. This is an optimization to omit no-op reconciliations and reduce lock contention on the cache.
			UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			    if e.ObjectOld.GetLabels()[karpv1.NodeRegisteredLabelKey] != "" {
			        return false
			    }
			    return e.ObjectNew.GetLabels()[karpv1.NodeRegisteredLabelKey] == "true"
			},
			// Reconcile against all Nodes added to the cache in a registered state. This allows us to hydrate the cache on controller startup.
			CreateFunc:  func(e event.TypedCreateEvent[client.Object]) bool { 
			    return e.Object.GetLabels()[karpv1.NodeRegisteredLabelKey] == "true" 
			},
			DeleteFunc:  func(e event.TypedDeleteEvent[client.Object]) bool { return false },
			GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool { return false },
		}).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 1,
		}).
		Complete(c)
}
