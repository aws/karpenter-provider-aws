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

package reallocator

import (
	"context"
	"fmt"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/controllers"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// Controller for the resource
type Controller struct {
	filter        *Filter
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
	return "provisioner/reallocator"
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, cloudProvider cloudprovider.Factory) *Controller {
	return &Controller{
		filter:        &Filter{kubeClient: kubeClient},
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes an allocation control loop for the resource
func (c *Controller) Reconcile(object controllers.Object) error {
	ctx := context.TODO()
	// 1. Filter all nodes with Karpenter TTL label
	// TODO: Filter only nodes with the Provisioner label

	// 2. Remove TTL from nodes with TTL label that have pods and if past TTL, cordon/drain node

	// 3. Filter under-utilized nodes
	nodes, err := c.filter.GetUnderutilizedNodes(ctx)
	if err != nil {
		return fmt.Errorf("filtering nodes, %w", err)
	}
	if len(nodes) == 0 {
		return nil
	}
	zap.S().Infof("Found %d underutilized nodes", len(nodes))

	// 4. Cordon each node

	// 5. Put TTL of 300s on each node

	return nil
}
