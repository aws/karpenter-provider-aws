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

package allocation

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/packing"
	"github.com/awslabs/karpenter/pkg/utils/apiobject"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	MaxBatchWindow   = 10 * time.Second
	BatchIdleTimeout = 2 * time.Second
)

// Controller for the resource
type Controller struct {
	filter        *Filter
	binder        *Binder
	batcher       *Batcher
	constraints   *Constraints
	packer        packing.Packer
	cloudProvider cloudprovider.CloudProvider
}

// For returns the resource this controller is for.
func (c *Controller) For() client.Object {
	return &v1alpha2.Provisioner{}
}

func (c *Controller) Interval() time.Duration {
	return 5 * time.Second
}

// ConcurrentReconciles controls the number of concurrent reconciles that can occur
func (c *Controller) ConcurrentReconciles() int {
	return 4
}

func (c *Controller) Name() string {
	return "provisioner/allocator"
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		filter:        &Filter{kubeClient: kubeClient},
		binder:        &Binder{kubeClient: kubeClient, coreV1Client: coreV1Client},
		batcher:       NewBatcher(MaxBatchWindow, BatchIdleTimeout),
		constraints:   &Constraints{kubeClient: kubeClient},
		packer:        packing.NewPacker(),
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes an allocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, object client.Object) (reconcile.Result, error) {
	persistedProvisioner := object.(*v1alpha2.Provisioner)

	// 1. Hydrate provisioner with (dynamic) default values, which must not
	//    be persisted into the original CRD as they might change with each reconciliation
	//    loop iteration.
	provisionerWithDefaults, err := persistedProvisioner.WithDynamicDefaults()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("setting dynamic default values, %w", err)
	}

	c.batcher.Wait(persistedProvisioner.UID)

	// 2. Filter pods
	pods, err := c.filter.GetProvisionablePods(ctx, &provisionerWithDefaults)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("filtering pods, %w", err)
	}
	if len(pods) == 0 {
		return reconcile.Result{}, nil
	}
	zap.S().Infof("Found %d provisionable pods", len(pods))

	// 3. Group by constraints
	constraintGroups, err := c.constraints.Group(ctx, &provisionerWithDefaults, pods)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("building constraint groups, %w", err)
	}

	// 4. Binpack each group
	packings := []*cloudprovider.Packing{}
	for _, constraintGroup := range constraintGroups {
		instanceTypes, err := c.cloudProvider.GetInstanceTypes(ctx)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("getting instance types, %w", err)
		}
		packings = append(packings, c.packer.Pack(ctx, constraintGroup, instanceTypes)...)
	}

	// 5. Create packedNodes for packings and also copy all Status changes made by the
	//    cloud provider to the original provisioner instance.
	packedNodes, err := c.cloudProvider.Create(ctx, &provisionerWithDefaults, packings)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("creating capacity, %w", err)
	}

	// 6. Bind pods to nodes
	var errs error
	for _, packedNode := range packedNodes {
		zap.S().Infof("Binding pods %v to node %s", apiobject.PodNamespacedNames(packedNode.Pods), packedNode.Node.Name)
		if err := c.binder.Bind(ctx, packedNode.Node, packedNode.Pods); err != nil {
			zap.S().Errorf("Continuing after failing to bind, %s", err.Error())
			errs = multierr.Append(errs, err)
		}
	}

	return reconcile.Result{}, errs
}

// Watches returns the necessary information to create a watch
//   a. source: the resource that is being watched
//   b. eventHandler: which controller objects to be reconciled
//   c. predicates: which events can be filtered out before processed
func (c *Controller) Watches(ctx context.Context) (source.Source, handler.EventHandler, builder.WatchesOption) {
	return &source.Kind{Type: &v1.Pod{}},
		handler.EnqueueRequestsFromMapFunc(handler.MapFunc(
			func(obj client.Object) []reconcile.Request {
				pod := obj.(*v1.Pod)
				provisioner, err := c.filter.GetProvisionerFor(ctx, pod)
				if err != nil {
					zap.S().Errorf("Retrieving provisioner, %s", err.Error())
					return nil
				}
				err = c.filter.isProvisionable(ctx, pod, provisioner)
				if err != nil {
					return nil
				}
				c.batcher.Start(ctx)
				c.batcher.Add(provisioner)
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: provisioner.Name, Namespace: provisioner.Namespace}}}
			},
		)),
		builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool { return true }))
}
