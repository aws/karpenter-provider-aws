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
	"fmt"
	"sort"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/mitchellh/hashstructure/v2"
)

const controllerName = "provisioning"

// Controller for the resource
type Controller struct {
	ctx           context.Context
	provisioners  *sync.Map
	scheduler     *scheduling.Scheduler
	coreV1Client  corev1.CoreV1Interface
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

// NewController is a constructor
func NewController(ctx context.Context, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		ctx:           ctx,
		provisioners:  &sync.Map{},
		kubeClient:    kubeClient,
		coreV1Client:  coreV1Client,
		cloudProvider: cloudProvider,
		scheduler:     scheduling.NewScheduler(kubeClient),
	}
}

// Reconcile a control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName).With("provisioner", req.Name))
	ctx = injection.WithNamespacedName(ctx, req.NamespacedName)
	ctx = injection.WithControllerName(ctx, controllerName)

	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, provisioner); err != nil {
		if errors.IsNotFound(err) {
			c.Delete(req.Name)
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
func (c *Controller) Delete(name string) {
	if p, ok := c.provisioners.LoadAndDelete(name); ok {
		p.(*Provisioner).Stop()
	}
}

// Apply creates or updates the provisioner to the latest configuration
func (c *Controller) Apply(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	// Refresh global requirements using instance type availability
	instanceTypes, err := c.cloudProvider.GetInstanceTypes(ctx, provisioner.Spec.Provider)
	if err != nil {
		return err
	}
	provisioner.Spec.Labels = functional.UnionStringMaps(provisioner.Spec.Labels, map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})
	provisioner.Spec.Requirements = provisioner.Spec.Requirements.
		Add(requirements(instanceTypes)...).
		Add(v1alpha5.NewLabelRequirements(provisioner.Spec.Labels).Requirements...)
	if err := provisioner.Spec.Requirements.Validate(); err != nil {
		return fmt.Errorf("provisioner requirements validation failed, %v", err)
	}
	// Update the provisioner if anything has changed
	if c.hasChanged(ctx, provisioner) {
		c.Delete(provisioner.Name)
		c.provisioners.Store(provisioner.Name, NewProvisioner(ctx, provisioner, c.kubeClient, c.coreV1Client, c.cloudProvider))
	}
	return nil
}

// Returns true if the new candidate provisioner is different than the provisioner in memory.
func (c *Controller) hasChanged(ctx context.Context, provisionerNew *v1alpha5.Provisioner) bool {
	oldProvisioner, ok := c.provisioners.Load(provisionerNew.Name)
	if !ok {
		return true
	}
	hashKeyOld, err := hashstructure.Hash(oldProvisioner.(*Provisioner).Spec, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		logging.FromContext(ctx).Fatalf("Unable to hash old provisioner spec: %s", err.Error())
	}
	hashKeyNew, err := hashstructure.Hash(provisionerNew.Spec, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		logging.FromContext(ctx).Fatalf("Unable to hash new provisioner spec: %s", err.Error())
	}
	return hashKeyOld != hashKeyNew
}

// List active provisioners in order of priority
func (c *Controller) List(ctx context.Context) []*Provisioner {
	provisioners := []*Provisioner{}
	c.provisioners.Range(func(key, value interface{}) bool {
		provisioners = append(provisioners, value.(*Provisioner))
		return true
	})
	sort.Slice(provisioners, func(i, j int) bool { return provisioners[i].Name < provisioners[j].Name })
	return provisioners
}

func requirements(instanceTypes []cloudprovider.InstanceType) []v1.NodeSelectorRequirement {
	supported := map[string]sets.String{
		v1.LabelInstanceTypeStable: sets.NewString(),
		v1.LabelTopologyZone:       sets.NewString(),
		v1.LabelArchStable:         sets.NewString(),
		v1.LabelOSStable:           sets.NewString(),
		v1alpha5.LabelCapacityType: sets.NewString(),
	}
	for _, instanceType := range instanceTypes {
		for _, offering := range instanceType.Offerings() {
			supported[v1.LabelTopologyZone].Insert(offering.Zone)
			supported[v1alpha5.LabelCapacityType].Insert(offering.CapacityType)
		}
		supported[v1.LabelInstanceTypeStable].Insert(instanceType.Name())
		supported[v1.LabelArchStable].Insert(instanceType.Architecture())
		supported[v1.LabelOSStable].Insert(instanceType.OperatingSystems().List()...)
	}
	requirements := []v1.NodeSelectorRequirement{}
	for key, values := range supported {
		requirements = append(requirements, v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: values.UnsortedList()})
	}
	return requirements

}

// Register the controller to the manager
func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1alpha5.Provisioner{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(c)
}
