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

package machinehydration

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	machineutil "github.com/aws/karpenter-core/pkg/utils/machine"
)

type Controller struct {
	kubeClient    client.Client
	cloudProvider *cloudprovider.CloudProvider
}

func NewController(kubeClient client.Client, cloudProvider *cloudprovider.CloudProvider) controller.Controller {
	return controller.Typed[*v1.Node](kubeClient, &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	})
}

func (c *Controller) Name() string {
	return "machinehydration"
}

func (c *Controller) Reconcile(ctx context.Context, node *v1.Node) (reconcile.Result, error) {
	if node.Spec.ProviderID == "" {
		return reconcile.Result{}, nil
	}
	provisionerName, ok := node.Labels[v1alpha5.ProvisionerNameLabelKey]
	if !ok {
		return reconcile.Result{}, nil
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("provider-id", node.Spec.ProviderID, "provisioner", provisionerName))
	machineList := &v1alpha5.MachineList{}
	if err := c.kubeClient.List(ctx, machineList, client.Limit(1), client.MatchingFields{"status.providerID": node.Spec.ProviderID}); err != nil {
		return reconcile.Result{}, err
	}
	// We have a machine registered for this node so no need to hydrate it
	if len(machineList.Items) > 0 {
		return reconcile.Result{}, nil
	}
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: provisionerName}, provisioner); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if err := c.hydrate(ctx, node, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("hydrating machine from node, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) hydrate(ctx context.Context, node *v1.Node, provisioner *v1alpha5.Provisioner) error {
	machine := machineutil.New(node, provisioner)
	machine.Name = GenerateName(fmt.Sprintf("%s-", provisioner.Name))
	machine.Spec.StartupTaints = nil // Assume that startupTaints are nil so that they won't be re-applied to the node

	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("machine", machine.Name))

	// Hydrates the machine with the correct values if the instance exists at the cloudprovider
	if err := c.cloudProvider.Hydrate(ctx, machine); err != nil {
		return corecloudprovider.IgnoreMachineNotFoundError(fmt.Errorf("hydrating machine, %w", err))
	}
	if err := c.kubeClient.Create(ctx, machine); err != nil {
		return client.IgnoreAlreadyExists(err)
	}
	logging.FromContext(ctx).Debugf("hydrated machine from node")
	return nil
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) controller.Builder {
	return controller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1.Node{}).
		WithOptions(ctrl.Options{MaxConcurrentReconciles: 10}))
}

// GenerateName generates a Machine name with the passed base prefix, appended with a random alphanumeric string
func GenerateName(base string) string {
	const maxNameLength = 63
	const randomLength = 15

	if len(base) > (maxNameLength - randomLength) {
		base = base[:(maxNameLength - randomLength)]
	}
	return fmt.Sprintf("%s%s", base, rand.String(randomLength))
}
