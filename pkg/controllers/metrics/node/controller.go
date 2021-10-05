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

package node

import (
	"context"
	"time"

	karpenterv1 "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	controllerName = "NodeMetrics"

	requeueInterval = 10 * time.Second
)

type Controller struct {
	KubeClient client.Client
}

func NewController(kubeClient client.Client) *Controller {
	return &Controller{KubeClient: kubeClient}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName))

	provisionerName := req.NamespacedName.Name

	// 1. Has the provisioner been deleted?
	if err := c.provisionerExists(ctx, req); err != nil {
		if !errors.IsNotFound(err) {
			// Unable to determine existence of the provisioner, try again later.
			return reconcile.Result{Requeue: true}, err
		}

		// The provisioner has been deleted. Reset all the associated counts to zero.
		if err := publishNodeCountsForProvisioner(provisionerName, consumeNoNodes); err != nil {
			// One or more metrics were not zeroed. Try again later.
			return reconcile.Result{Requeue: true}, err
		}

		// Since the provisioner is gone, do not requeue.
		return reconcile.Result{}, nil
	}

	// 2. Update node counts associated with this provisioner.
	if err := publishNodeCountsForProvisioner(provisionerName, c.consumeNodesFromKubeClientWithContext(ctx)); err != nil {
		// An updated value for one or more metrics was not published. Try again later.
		return reconcile.Result{Requeue: true}, err
	}

	// 3. Schedule the next run.
	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&karpenterv1.Provisioner{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
				UpdateFunc:  func(_ event.UpdateEvent) bool { return false },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			},
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(c)
}

// provisionerExists simply attempts to retrieve the provisioner from the Controller's Client
// and returns any resulting error.
func (c *Controller) provisionerExists(ctx context.Context, req reconcile.Request) error {
	provisioner := karpenterv1.Provisioner{}
	return c.KubeClient.Get(ctx, req.NamespacedName, &provisioner)
}

// consumeNodesFromKubeClientWithContext will retrieve matching nodes from the Controller's Client then
// pass the nodes to `consume` and returns any resulting error. If Client returns an error when
// retrieving nodes then the error is returned without calling `consume`.
func (c *Controller) consumeNodesFromKubeClientWithContext(ctx context.Context) consumeNodesWithFunc {
	return func(nodeLabels client.MatchingLabels, consume nodeListConsumerFunc) error {
		nodes := corev1.NodeList{}
		if err := c.KubeClient.List(ctx, &nodes, nodeLabels); err != nil {
			return err
		}
		return consume(nodes.Items)
	}
}

// consumeNoNodes calls `consume` with an empty slice and returns any resulting error.
func consumeNoNodes(_ client.MatchingLabels, consume nodeListConsumerFunc) error {
	return consume([]corev1.Node{})
}
