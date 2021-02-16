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
	provisioning "github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// Controller for the resource
type Controller struct {
	filter        *Filter
	collector     *Collector
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
		collector:     &Collector{kubeClient: kubeClient},
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes an allocation control loop for the resource
func (c *Controller) Reconcile(object controllers.Object) error {
	provisioner := object.(*v1alpha1.Provisioner)
	ctx := context.TODO()

	// 1. Filter all nodes with Karpenter labels
	ttlNodes, err := c.filter.getNodesWithLabels(ctx, []string{
		provisioning.ProvisionerNamespaceLabelKey,
		provisioning.ProvisionerNameLabelKey,
	})
	if err != nil {
		return fmt.Errorf("listing ttl provisioner nodes, %w", err)
	}

	// 2. If node has TTL annotation, and time is after TTL, delete node
	// TODO: instead of delete - drain then delete
	for _, node := range ttlNodes.Items {
		if err := c.collector.ParseTTL(ctx, &node); err != nil {
			return fmt.Errorf("handling ttl for node, %w", err)
		}
	}

	// 3. Filter under-utilized nodes
	underutilized, err := c.filter.GetUnderutilizedNodes(ctx, provisioner.Name, provisioner.Namespace)
	if err != nil {
		return fmt.Errorf("filtering nodes, %w", err)
	}
	if len(underutilized) == 0 {
		return nil
	}

	// TODO: 3.5. Filter underutilized nodes that haven't been cordoned/ttl'd to not spam logs
	zap.S().Infof("Found %d underutilized nodes", len(underutilized))

	// 4. Cordon each node
	// TODO: Go routines to CordonNode quicker
	for _, node := range underutilized {
		// 4. Cordon each node
		if err := c.collector.CordonNode(ctx, node); err != nil {
			return fmt.Errorf("cordoning nodes with merge patch, %w", err)
		}
	}

	// 5. Set TTL of 300s from now in annotations if not set
	// TODO: Go routines to AddTTL quicker
	for _, node := range underutilized {
		if err := c.collector.AddTTL(ctx, node); err != nil {
			return fmt.Errorf("managing ttl on underutilized node, %w", err)
		}
	}
	return nil
}
