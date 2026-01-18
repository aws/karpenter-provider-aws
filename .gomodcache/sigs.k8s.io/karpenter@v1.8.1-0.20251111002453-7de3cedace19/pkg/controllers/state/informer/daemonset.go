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

package informer

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	utilscontroller "sigs.k8s.io/karpenter/pkg/utils/controller"
)

type DaemonSetController struct {
	kubeClient client.Client
	cluster    *state.Cluster
}

func NewDaemonSetController(kubeClient client.Client, cluster *state.Cluster) *DaemonSetController {
	return &DaemonSetController{
		kubeClient: kubeClient,
		cluster:    cluster,
	}
}

func (c *DaemonSetController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "state.daemonset")

	daemonSet := appsv1.DaemonSet{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, &daemonSet); err != nil {
		if errors.IsNotFound(err) {
			// notify cluster state of the daemonset deletion
			c.cluster.DeleteDaemonSet(req.NamespacedName)
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if err := c.cluster.UpdateDaemonSet(ctx, &daemonSet); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{RequeueAfter: time.Minute}, nil
}

func (c *DaemonSetController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("state.daemonset").
		For(&appsv1.DaemonSet{}).
		// We only want to watch the DaemonSet on Create and then re-poll every 1m
		WithEventFilter(predicate.Funcs{
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
		}).
		WithOptions(controller.Options{MaxConcurrentReconciles: utilscontroller.LinearScaleReconciles(utilscontroller.CPUCount(ctx), minReconciles, maxReconciles)}).
		Complete(c)
}
