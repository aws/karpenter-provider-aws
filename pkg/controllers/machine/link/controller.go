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

package link

import (
	"context"
	"fmt"
	"math"

	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/errors"

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
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

func (c *Controller) Name() string {
	return "machine.link"
}

func (c *Controller) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	machineList := &v1alpha5.MachineList{}
	if err := c.kubeClient.List(ctx, machineList); err != nil {
		return reconcile.Result{}, err
	}
	retrieved, err := c.cloudProvider.List(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing cloudprovider machines, %w", err)
	}
	// Filter out any machines that shouldn't be linked
	retrieved = lo.Filter(retrieved, func(m *v1alpha5.Machine, _ int) bool {
		return c.shouldLink(m, machineList.Items)
	})
	errs := make([]error, len(retrieved))
	workqueue.ParallelizeUntil(ctx, 20, len(retrieved), func(i int) {
		errs[i] = c.link(ctx, retrieved[i])
	})
	// Effectively, don't requeue this again once it succeeds
	return reconcile.Result{RequeueAfter: math.MaxInt64}, multierr.Combine(errs...)
}

func (c *Controller) link(ctx context.Context, retrieved *v1alpha5.Machine) error {
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: retrieved.Labels[v1alpha5.ProvisionerNameLabelKey]}, provisioner); err != nil {
		if errors.IsNotFound(err) {
			return corecloudprovider.IgnoreMachineNotFoundError(c.cloudProvider.Delete(ctx, retrieved))
		}
		return err
	}
	machine := machineutil.New(&v1.Node{}, provisioner)
	machine.GenerateName = fmt.Sprintf("%s-", provisioner.Name)
	// This annotation communicates to the machine controller that this is a machine linking scenario, not
	// a case where we want to provision a new machine
	machine.Annotations = lo.Assign(machine.Annotations, map[string]string{
		v1alpha5.MachineLinkedAnnotationKey: retrieved.Status.ProviderID,
	})
	return c.kubeClient.Create(ctx, machine)
}

// shouldLink checks if the cloudprovider machine is owned by a provisioner but if it wasn't created
// by a machine. If both of these are true, then we should generate a Machine
func (c *Controller) shouldLink(retrieved *v1alpha5.Machine, existing []v1alpha5.Machine) bool {
	if _, ok := retrieved.Labels[v1alpha5.ManagedLabelKey]; ok {
		return false
	}
	// We have a machine registered for this, so no need to hydrate it
	if _, ok := lo.Find(existing, func(m v1alpha5.Machine) bool {
		return m.Annotations[v1alpha5.MachineLinkedAnnotationKey] == retrieved.Status.ProviderID ||
			m.Status.ProviderID == retrieved.Status.ProviderID
	}); ok {
		return false
	}
	return true
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) controller.Builder {
	return controller.NewSingletonManagedBy(m)
}
