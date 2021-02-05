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

package fleet

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
)

// Capacity cloud provider implementation using AWS Fleet.
type Capacity struct {
	spec             *v1alpha1.ProvisionerSpec
	nodeFactory      *NodeFactory
	packing          cloudprovider.Packer
	instanceProvider *InstanceProvider
	vpc              *VPCProvider
}

// Create a set of nodes given the constraints.
func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.CapacityConstraints) (cloudprovider.Packings, error) {
	// 1. Compute Packing given the constraints
	instancePackings, err := c.packing.Get(ctx, constraints)
	if err != nil {
		return nil, fmt.Errorf("computing bin packing, %w", err)
	}

	// 2. Create Instances
	var instanceIds []*string
	for tempID, instancePacking := range instancePackings {
		ec2InstanceID, err := c.instanceProvider.Create(ctx, instancePacking.InstanceTypeOptions, constraints, c.spec)
		if err != nil {
			// TODO Aggregate errors and continue
			return nil, fmt.Errorf("creating capacity %w", err)
		}
		instancePackings[*ec2InstanceID] = instancePacking
		delete(instancePackings, tempID)
		instanceIds = append(instanceIds, ec2InstanceID)
	}

	// 3. Convert to Nodes
	nodes, err := c.nodeFactory.For(ctx, instanceIds)
	if err != nil {
		return nil, fmt.Errorf("determining nodes, %w", err)
	}
	for instanceID, node := range nodes {
		instancePackings[instanceID].Node = node
	}
	return instancePackings, nil
}

// GetTopologyDomains returns a set of supported domains.
// e.g. us-west-2 -> [ us-west-2a, us-west-2b ]
func (c *Capacity) GetTopologyDomains(ctx context.Context, key cloudprovider.TopologyKey) ([]string, error) {
	return c.vpc.GetTopologyDomains(ctx, key, c.spec.Cluster.Name)
}
