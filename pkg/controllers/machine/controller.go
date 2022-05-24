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

package machine

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
)

const controllerName = "machine"

// NewController constructs a controller instance
func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

// Controller manages a set of properties on karpenter provisioned nodes, such as
// taints, labels, finalizers.
type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

// Reconcile executes a reallocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName).With("machine", req.Name))
	machine := &v1alpha5.Machine{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, machine); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if !machine.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, c.finalize(ctx, machine)
	}
	return reconcile.Result{}, c.reconcile(ctx, machine)
}

func (c *Controller) reconcile(ctx context.Context, machine *v1alpha5.Machine) error {
	// Get or Create the corresponding node object
	node, err := c.ensureNode(ctx, machine)
	if err != nil {
		return err
	}
	// Update status
	machine.Status.NodeName = ptr.String(node.Name)
	if err := c.kubeClient.Status().Patch(ctx, machine, client.Merge); err != nil {
		return err
	}
	return nil
}

func (c *Controller) finalize(ctx context.Context, machine *v1alpha5.Machine) error {
	// TODO Cordon
	// TODO Drain
	// Delete the machine
	if err := c.cloudProvider.DeleteMachine(ctx, machine.Name); err != nil {
		return fmt.Errorf("deleting machine %s, %w", machine.Name, err)
	}
	// TODO consider deleting the node for more responsiveness
	machine.Finalizers = lo.Reject(machine.Finalizers, func(finalizer string, _ int) bool { return finalizer == v1alpha5.MachineFinalizer })
	if err := c.kubeClient.Patch(ctx, machine, client.Merge); err != nil {
		return fmt.Errorf("patching machine, %w", err)
	}
	return nil
}

func (c *Controller) ensureNode(ctx context.Context, machine *v1alpha5.Machine) (node *v1.Node, err error) {
	// Check if node is registered
	nodes := v1.NodeList{}
	if err := c.kubeClient.List(ctx, &nodes, client.MatchingLabels{v1alpha5.MachineNameLabelKey: machine.Name}); err != nil {
		return nil, fmt.Errorf("listing nodes, %w", err)
	}
	if len(nodes.Items) > 0 {
		return &nodes.Items[0], nil
	}
	// Check if cloudprovider has the node (in flight)
	node, err = c.cloudProvider.GetMachine(ctx, machine.Name)
	if err != nil {
		return nil, fmt.Errorf("getting machine %s, %w", machine.Name, err)
	}
	if node != nil {
		return node, nil
	}

	// Construct requirements
	requirements, err := c.cloudProvider.GetRequirements(ctx, machine.Spec.Provider)
	if err != nil {
		return nil, fmt.Errorf("getting cloudprovider requirements")
	}
	machine.Spec.Requirements = requirements.ToNodeSelectorRequirements()

	// Create the machine in the cloud provider
	node, err = c.cloudProvider.CreateMachine(ctx, machine)
	if err != nil {
		return nil, fmt.Errorf("creating machine %s, %w", machine.Name, err)
	}
	return node, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1alpha5.Machine{}).
		Watches(&source.Kind{Type: &v1.Node{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: object.GetLabels()[v1alpha5.MachineNameLabelKey]}}}
		})).
		Complete(c)
}
