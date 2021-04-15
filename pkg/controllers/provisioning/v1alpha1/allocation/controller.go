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
	"github.com/awslabs/karpenter/pkg/controllers"
	"github.com/awslabs/karpenter/pkg/utils/apiobject"
	"go.uber.org/zap"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Controller for the resource
type Controller struct {
	filter        *Filter
	binder        *Binder
	constraints   *Constraints
	cloudProvider cloudprovider.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() controllers.Object {
	return &v1alpha1.Provisioner{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []controllers.Object {
	return []controllers.Object{}
}

func (c *Controller) Interval() time.Duration {
	return 5 * time.Second
}

func (c *Controller) Name() string {
	return "provisioner/allocator"
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.Factory) *Controller {
	return &Controller{
		cloudProvider: cloudProvider,
		filter:        &Filter{kubeClient: kubeClient, cloudProvider: cloudProvider},
		binder:        &Binder{kubeClient: kubeClient, coreV1Client: coreV1Client},
		constraints:   &Constraints{kubeClient: kubeClient},
	}
}

// Reconcile executes an allocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, object controllers.Object) error {
	provisioner := object.(*v1alpha1.Provisioner)
	// 1. Filter pods
	pods, err := c.filter.GetProvisionablePods(ctx, provisioner)
	if err != nil {
		return fmt.Errorf("filtering pods, %w", err)
	}
	if len(pods) == 0 {
		return nil
	}
	zap.S().Infof("Found %d provisionable pods", len(pods))

	// 2. Group by constraints
	groups, err := c.constraints.Group(ctx, provisioner, pods)
	if err != nil {
		return fmt.Errorf("building constraint groups, %w", err)
	}

	// 3. Create capacity and packings
	var packings []cloudprovider.Packing
	for _, constraints := range groups {
		packing, err := c.cloudProvider.CapacityFor(&provisioner.Spec).Create(ctx, constraints)
		if err != nil {
			zap.S().Errorf("Continuing after failing to create capacity, %s", err.Error())
		} else {
			packings = append(packings, packing...)
		}
	}

	// 4. Bind pods to nodes
	for _, packing := range packings {
		zap.S().Infof("Binding pods %v to node %s", apiobject.PodNamespacedNames(packing.Pods), packing.Node.Name)
		if err := c.binder.Bind(ctx, packing.Node, packing.Pods); err != nil {
			zap.S().Errorf("Continuing after failing to bind, %s", err.Error())
		}
	}
	return nil
}
