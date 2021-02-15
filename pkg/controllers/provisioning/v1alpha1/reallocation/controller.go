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

package reallocation

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Controller for the resource
type Controller struct {
	filter        *Filter
	terminator    *Terminator
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
		terminator:    &Terminator{kubeClient: kubeClient, cloudprovider: cloudProvider},
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes a reallocation control loop for the resource
func (c *Controller) Reconcile(object controllers.Object) error {
	provisioner := object.(*v1alpha1.Provisioner)
	ctx := context.TODO()

	// 1. Get underutilized nodes
	underutilized, err := c.filter.GetUnderutilizedNodes(ctx, provisioner)
	if err != nil {
		return fmt.Errorf("listing underutilized nodes, %w", err)
	}

	// 2. Set TTL on underutilized nodes
	if err := c.terminator.AddTTLs(ctx, c.filter.GetTTLableNodes(underutilized)); err != nil {
		return fmt.Errorf("adding ttl, %w", err)
	}

	// 3. Find Nodes Past TTL with Karpenter Labels
	expired, err := c.filter.GetExpiredNodes(ctx, provisioner)
	if err != nil {
		return fmt.Errorf("getting expired nodes, %w", err)
	}
	if len(expired) == 0 {
		return nil
	}

	// 4. Cordon each node
	if err := c.terminator.CordonNodes(ctx, c.filter.GetCordonableNodes(expired)); err != nil {
		return fmt.Errorf("cordoning node, %w", err)
	}

	// TODO
	// 5. Drain Nodes past TTL

	// 6. Delete Nodes past TTL
	if err := c.terminator.DeleteNodes(ctx, expired, &provisioner.Spec); err != nil {
		return err
	}

	return nil
}
