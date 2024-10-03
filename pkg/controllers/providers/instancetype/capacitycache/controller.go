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

package capacitycache

import (
	"context"
	"fmt"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/awslabs/operatorpkg/reasonable"
	"github.com/awslabs/operatorpkg/singleton"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

func (c *Controller) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "instancetypes.capacitycache")
	if err := c.instancetypeProvider.UpdateInstanceTypeCapacityCache(ctx, c.kubeClient); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating instancetype capacity cache, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instancetypes.capacitycache").
		WatchesRawSource(singleton.Source()).
		Watches(&corev1.Node{}, handler.Funcs{
			// Only trigger reconciliation once a node becomes registered
			UpdateFunc: func(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				oldNode, okOld := e.ObjectOld.(*corev1.Node)
				newNode, okNew := e.ObjectNew.(*corev1.Node)
				if !okOld || !okNew {
					return
				}
				if oldNode.Labels[karpv1.NodeRegisteredLabelKey] == "" && newNode.Labels[karpv1.NodeRegisteredLabelKey] == "true" {
					q.Add(reconcile.Request{})
				}
			},
		}).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 1,
		}).
		Complete(c)
}
