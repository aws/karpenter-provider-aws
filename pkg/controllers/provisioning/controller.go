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
package provisioning

import (
	"context"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/binpacking"
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/awslabs/karpenter/pkg/utils/functional"
)

// Controller for the resource
type Controller struct {
	ctx           context.Context
	provisioners  *sync.Map
	scheduler     *scheduling.Scheduler
	launcher      *Launcher
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

// NewController is a constructor
func NewController(ctx context.Context, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		ctx:           ctx,
		provisioners:  &sync.Map{},
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		scheduler:     scheduling.NewScheduler(kubeClient, cloudProvider),
		launcher:      &Launcher{KubeClient: kubeClient, CoreV1Client: coreV1Client, CloudProvider: cloudProvider, Packer: &binpacking.Packer{}},
	}
}

// Reconcile a control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("provisioning").With("provisioner", req.Name))
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, provisioner); err != nil {
		if errors.IsNotFound(err) {
			c.Delete(ctx, req.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if err := c.Apply(ctx, provisioner); err != nil {
		return reconcile.Result{}, err
	}
	// Requeue in order to discover any changes from GetInstanceTypes.
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

// Delete stops and removes a provisioner. Enqueued pods will be provisioned.
func (c *Controller) Delete(ctx context.Context, name string) {
	if p, ok := c.provisioners.LoadAndDelete(name); ok {
		p.(*Provisioner).Stop()
	}
}

// Apply creates or updates the provisioner to the latest configuration
func (c *Controller) Apply(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	// Refresh global requirements using instance type availability
	instanceTypes, err := c.cloudProvider.GetInstanceTypes(ctx, &provisioner.Spec.Constraints)
	if err != nil {
		return err
	}
	provisioner.Spec.Labels = functional.UnionStringMaps(provisioner.Spec.Labels, map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})
	provisioner.Spec.Requirements = provisioner.Spec.Requirements.
		With(scheduling.GlobalRequirements(instanceTypes)). // TODO(etarn) move GlobalRequirements to this file
		With(v1alpha5.LabelRequirements(provisioner.Spec.Labels))
	if currentProvisioner, ok := c.provisioners.Load(provisioner.Name); ok && currentProvisioner.(*Provisioner).Spec.EqualTo(&provisioner.Spec) {
		// If the provisionerSpecs haven't changed, we don't need to stop and drain the current Provisioner.
		return nil
	}
	ctx, cancelFunc := context.WithCancel(ctx)
	p := &Provisioner{
		Provisioner:   provisioner,
		pods:          make(chan *v1.Pod),
		results:       make(chan error),
		done:          ctx.Done(),
		Stop:          cancelFunc,
		cloudProvider: c.cloudProvider,
		scheduler:     c.scheduler,
		launcher:      c.launcher,
	}
	p.Start(ctx)
	// Update the provisioner; stop and drain an existing provisioner if exists.
	if existing, ok := c.provisioners.LoadOrStore(provisioner.Name, p); ok {
		c.provisioners.Store(provisioner.Name, p)
		existing.(*Provisioner).Stop()
	}
	return nil
}

// List the active provisioners
func (c *Controller) List(ctx context.Context) []*Provisioner {
	provisioners := []*Provisioner{}
	c.provisioners.Range(func(key, value interface{}) bool {
		provisioners = append(provisioners, value.(*Provisioner))
		return true
	})
	return provisioners
}

// Register the controller to the manager
func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named("provisioning").
		For(&v1alpha5.Provisioner{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(c)
}
