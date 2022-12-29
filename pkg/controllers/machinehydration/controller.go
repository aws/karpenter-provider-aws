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

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	machineutil "github.com/aws/karpenter-core/pkg/utils/machine"
	"github.com/aws/karpenter/pkg/cloudprovider"
)

type Controller struct {
	kubeClient    client.Client
	cloudProvider *cloudprovider.CloudProvider
}

func NewController(kubeClient client.Client, cloudProvider *cloudprovider.CloudProvider) corecontroller.Controller {
	return corecontroller.Typed[*v1.Node](kubeClient, &Controller{
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
	machineList := &v1alpha5.MachineList{}
	if err := c.kubeClient.List(ctx, machineList, client.Limit(1), client.MatchingFields{"status.providerID": node.Spec.ProviderID}); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing machines, %w", err)
	}
	if len(machineList.Items) > 0 {
		return reconcile.Result{}, nil
	}
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: provisionerName}, provisioner); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("getting provisioner, %w", err)
	}
	if err := c.hydrate(ctx, node, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("hydrating machine from node, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) hydrate(ctx context.Context, node *v1.Node, provisioner *v1alpha5.Provisioner) error {
	machine := machineutil.New(node, provisioner)
	machine.Name = MachineNameGenerator.GenerateName(provisioner.Name + "-") // so we know the name before creation
	machine.Labels = lo.Assign(machine.Labels, map[string]string{
		v1alpha5.MachineNameLabelKey: machine.Name,
	})
	logging.WithLogger(ctx, logging.FromContext(ctx).With("machine", machine.Name))

	// Hydrates the machine with the correct values if the instance exists at the cloudprovider
	if err := c.cloudProvider.Hydrate(ctx, machine); err != nil {
		if corecloudprovider.IsMachineNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("hydrating machine, %w", err)
	}
	if err := c.kubeClient.Create(ctx, machine); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("creating hydrated machine from node, %w", err)
	}
	logging.FromContext(ctx).Debugf("hydrated machine from node")
	return nil
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1.Node{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}
