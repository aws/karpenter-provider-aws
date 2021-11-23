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
package counter

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
)

// Controller for the resource
type Controller struct {
	kubeClient client.Client
}

// NewController is a constructor
func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		kubeClient: kubeClient,
	}
}

// Reconcile a control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// Retrieve the provisioner
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, provisioner); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	persisted := provisioner.DeepCopy()

	// Determine resource usage and update provisioner.status.resources
	resourceCounts, err := c.resourceCountsFor(ctx, provisioner.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	provisioner.Status.Resources = resourceCounts
	if err := c.kubeClient.Status().Patch(ctx, provisioner, client.MergeFrom(persisted)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to persist changes to %s, %w", req.NamespacedName, err)
	}

	return reconcile.Result{}, nil
}

func (c *Controller) resourceCountsFor(ctx context.Context, provisionerName string) (v1.ResourceList, error) {
	nodes := v1.NodeList{}
	if err := c.kubeClient.List(ctx, &nodes, client.MatchingLabels{v1alpha5.ProvisionerNameLabelKey: provisionerName}); err != nil {
		return nil, err
	}

	var cpu = resource.NewScaledQuantity(0, 0)
	var memory = resource.NewScaledQuantity(0, resource.Giga)

	for _, node := range nodes.Items {
		cpu.Add(*node.Status.Capacity.Cpu())
		memory.Add(*node.Status.Capacity.Memory())
	}
	return v1.ResourceList{
		v1.ResourceCPU:    *cpu,
		v1.ResourceMemory: *memory,
	}, nil

}

// Register the controller to the manager
func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named("counter").
		For(&v1alpha5.Provisioner{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				// No need to reconcile the provisioner when nodes are updated since those don't affect the status of the provisioner.
				return false
			},
		}).
		Watches(
			// Reconcile provisioner state when a node managed by it is created or deleted.
			&source.Kind{Type: &v1.Node{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				if name, ok := o.GetLabels()[v1alpha5.ProvisionerNameLabelKey]; ok {
					return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
				}
				return nil
			}),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(c)
}
