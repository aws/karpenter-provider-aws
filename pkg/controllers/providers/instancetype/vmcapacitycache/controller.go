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
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
)

type Controller struct {
	kubeClient           client.Client
	instancetypeProvider *instancetype.DefaultProvider
}

func NewController(kubeClient client.Client, instancetypeProvider *instancetype.DefaultProvider) *Controller {
	return &Controller{
		kubeClient:           kubeClient,
		instancetypeProvider: instancetypeProvider,
	}
}

const initNodeName = "initialize-capacity-cache-nodes"

func (c *Controller) Reconcile(ctx context.Context, r reconcile.Request) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "instancetypes.vmcapacitycache")
	if r.Name == initNodeName {
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
		if _, err := c.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}}); err != nil {
			errs[i] = err
		}
	}
	if err := multierr.Combine(errs...); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating vm capacity cache, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instancetypes.vmcapacitycache").
		WatchesRawSource(initialLoad()).
		For(&corev1.Node{}).
		WithEventFilter(predicate.Funcs{
			// Only trigger reconciliation once a node becomes registered
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldNode, okOld := e.ObjectOld.(*corev1.Node)
				newNode, okNew := e.ObjectNew.(*corev1.Node)
				if !okOld || !okNew {
					return false
				}
				return oldNode.Labels[karpv1.NodeRegisteredLabelKey] == "" && newNode.Labels[karpv1.NodeRegisteredLabelKey] == "true"
			},
			CreateFunc:  func(e event.CreateEvent) bool { return false },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		}).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 1,
		}).
		Complete(c)
}

func initialLoad() source.Source {
	events := make(chan event.GenericEvent, 1)
	events <- event.GenericEvent{Object: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: initNodeName}}}
	return source.Channel(events, &handler.Funcs{
		GenericFunc: func(_ context.Context, evt event.GenericEvent, queue workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: evt.Object.GetName()}})
		},
	})
}
