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

package allocator

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/controllers"
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

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client *corev1.CoreV1Client, cloudProvider cloudprovider.Factory) *Controller {
	return &Controller{
		cloudProvider: cloudProvider,
		filter:        &Filter{kubeClient: kubeClient},
		binder:        &Binder{kubeClient: kubeClient, coreV1Client: coreV1Client},
		constraints:   &Constraints{},
	}
}

// Reconcile executes an allocation control loop for the resource
func (c *Controller) Reconcile(object controllers.Object) error {
	provisioner := object.(*v1alpha1.Provisioner)
	ctx := context.TODO()

	// 1. Filter pods
	pods, err := c.filter.GetProvisionablePods(ctx)
	if err != nil {
		return fmt.Errorf("filtering pods, %w", err)
	}
	if len(pods) == 0 {
		return nil
	}
	zap.S().Infof("Found %d provisionable pods", len(pods))

	// 2. Group by constraints
	constraintGroups := c.constraints.Group(pods)

	// 3. Create capacity and packings
	var packings []cloudprovider.Packing
	for _, constraints := range constraintGroups {
		packing, err := c.cloudProvider.CapacityFor(&provisioner.Spec).Create(ctx, constraints)
		if err != nil {
			zap.S().Errorf("Continuing after failing to create capacity, %w", err)
		} else {
			packings = append(packings, packing...)
		}
	}

	// 4. Bind pods to nodes
	for _, packing := range packings {
		zap.S().Infof("Binding %d pods to node %s", len(pods), packing.Node.Name)
		if err := c.binder.Bind(ctx, provisioner, packing.Node, packing.Pods); err != nil {
			zap.S().Errorf("Continuing after failing to bind, %w", err)
		}
	}
	return nil
}
