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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/packing"
	"github.com/awslabs/karpenter/pkg/utils/apiobject"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Controller for the resource
type Controller struct {
	filter        *Filter
	binder        *Binder
	constraints   *Constraints
	packer        packing.Packer
	cloudProvider cloudprovider.CloudProvider
}

// For returns the resource this controller is for.
func (c *Controller) For() client.Object {
	return &v1alpha1.Provisioner{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []client.Object {
	return nil
}

func (c *Controller) Interval() time.Duration {
	return 5 * time.Second
}

func (c *Controller) Name() string {
	return "provisioner/allocator"
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		filter:        &Filter{kubeClient: kubeClient},
		binder:        &Binder{kubeClient: kubeClient, coreV1Client: coreV1Client},
		constraints:   &Constraints{kubeClient: kubeClient},
		packer:        packing.NewPacker(),
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes an allocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, object client.Object) (reconcile.Result, error) {
	provisioner := object.(*v1alpha1.Provisioner)
	// 1. Filter pods
	pods, err := c.filter.GetProvisionablePods(ctx, provisioner)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("filtering pods, %w", err)
	}
	if len(pods) == 0 {
		return reconcile.Result{RequeueAfter: c.Interval()}, nil
	}
	zap.S().Infof("Found %d provisionable pods", len(pods))

	// 2. Group by constraints
	constraintGroups, err := c.constraints.Group(ctx, provisioner, pods)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("building constraint groups, %w", err)
	}

	// 3. Binpack each group
	packings := []*cloudprovider.Packing{}
	for _, constraintGroup := range constraintGroups {
		instanceTypes, err := c.cloudProvider.GetInstanceTypes(ctx)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("getting instance types, %w", err)
		}
		packings = append(packings, c.packer.Pack(ctx, constraintGroup, instanceTypes)...)
	}

	// 4. Create packedNodes for packings
	packedNodes, err := c.cloudProvider.Create(ctx, provisioner, packings)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("creating capacity, %w", err)
	}

	// 5. Bind pods to nodes
	for _, packedNode := range packedNodes {
		zap.S().Infof("Binding pods %v to node %s", apiobject.PodNamespacedNames(packedNode.Pods), packedNode.Node.Name)
		if err := c.binder.Bind(ctx, packedNode.Node, packedNode.Pods); err != nil {
			zap.S().Errorf("Continuing after failing to bind, %s", err.Error())
		}
	}
	return reconcile.Result{RequeueAfter: c.Interval()}, nil
}

func (c *Controller) Watches() (source.Source, handler.EventHandler, builder.WatchesOption) {
	return &source.Kind{Type: &v1.Pod{}},
		&handler.EnqueueRequestForObject{},
		builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool { return false }))
}
