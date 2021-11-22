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
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/pretty"
)

// Controller for the resource
type Controller struct {
	ctx           context.Context
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	coreV1Client  corev1.CoreV1Interface
}

// NewController is a constructor
func NewController(ctx context.Context, kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, coreV1Client corev1.CoreV1Interface) *Controller {
	return &Controller{
		ctx:           ctx,
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		coreV1Client:  coreV1Client,
	}
}

// Reconcile a control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logging.FromContext(ctx).Infof("Updating resource counts on provisioner")
	// Retrieve the provisioner
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, provisioner); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// Determine resource usage and update provisioner.status.resources
	resourceCounts, err := c.resourceCountsFor(ctx, provisioner.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	fmt.Printf("Total cpu usage %v and mem %v\n", resourceCounts[v1alpha5.ResourceLimitsCPU], resourceCounts[v1alpha5.ResourceLimitsMemory])

	provisioner.Status.Resources = resourceCounts

	logging.FromContext(ctx).Info(pretty.Concise(provisioner))

	if err := c.kubeClient.Update(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating provisioner %s, %w", provisioner.Name, err)
	}

	// Refresh the reconciler state values every 5 minutes irrespective of node events
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) resourceCountsFor(ctx context.Context, provisionerName string) (v1.ResourceList, error) {
	nodeList := v1.NodeList{}
	withProvisionerName := client.MatchingLabels{v1alpha5.ProvisionerNameLabelKey: provisionerName}
	if err := c.kubeClient.List(ctx, &nodeList, withProvisionerName); err != nil {
		return nil, err
	}

	var cpu = resource.NewScaledQuantity(0, 0)
	var memory = resource.NewScaledQuantity(0, 0)

	for _, node := range nodeList.Items {
		cpu.Add(*node.Status.Capacity.Cpu())
		memory.Add(*node.Status.Capacity.Memory())
	}
	return v1.ResourceList{
		v1alpha5.ResourceLimitsCPU:    *cpu,
		v1alpha5.ResourceLimitsMemory: *memory,
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
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				provisionerName := o.GetLabels()[v1alpha5.ProvisionerNameLabelKey]
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: provisionerName}})
				return requests
			}),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(c)
}
