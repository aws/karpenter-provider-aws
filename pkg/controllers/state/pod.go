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

package state

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var stateRetryPeriod = 1 * time.Minute

// PodController reconciles pods for the purpose of maintaining state regarding pods that is expensive to compute.
type PodController struct {
	kubeClient client.Client
	cluster    *Cluster
}

func NewPodController(kubeClient client.Client, cluster *Cluster) *PodController {
	return &PodController{
		kubeClient: kubeClient,
		cluster:    cluster,
	}
}

func (c *PodController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(nodeControllerName).With("pod", req.NamespacedName))
	stored := &v1.Pod{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, stored); err != nil {
		if errors.IsNotFound(err) {
			c.cluster.handlePodDeletion(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	c.cluster.handlePodUpdate(stored)

	if stored.DeletionTimestamp != nil {
		// pod is deleting, so keep checking more frequently to ensure we notice as soon as its gone
		return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
	}

	return reconcile.Result{Requeue: true, RequeueAfter: stateRetryPeriod}, nil
}

func (c *PodController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named("state-pod").
		For(&v1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(c)
}
