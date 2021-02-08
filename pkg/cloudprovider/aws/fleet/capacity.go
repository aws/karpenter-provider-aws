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
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fleet/packing"
	v1 "k8s.io/api/core/v1"
)

// Capacity cloud provider implementation using AWS Fleet.
type Capacity struct {
	spec             *v1alpha1.ProvisionerSpec
	nodeFactory      *NodeFactory
	packer           packing.Packer
	instanceProvider *InstanceProvider
	vpcProvider      *VPCProvider
}

// Create a set of nodes given the constraints.
func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.Constraints) (cloudprovider.NodePackings, error) {
	// 1. Compute Packing given the constraints
	instancePackings, err := c.packer.Pack(ctx, constraints.Pods)
	if err != nil {
		return nil, fmt.Errorf("computing bin packing, %w", err)
	}

	launchTemplate, err := c.vpcProvider.GetLaunchTemplate(ctx, c.spec.Cluster)
	if err != nil {
		return nil, fmt.Errorf("getting launch template, %w", err)
	}

	zonalSubnetOptions, err := c.vpcProvider.GetZonalSubnets(ctx, constraints, c.spec.Cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("getting zonal subnets, %w", err)
	}

	// 2. Create Instances
	var instanceIds []*string
	podsMapped := make(map[string][]*v1.Pod)
	for _, packing := range instancePackings {
		instanceID, err := c.instanceProvider.Create(ctx, launchTemplate, packing.InstanceTypeOptions, zonalSubnetOptions)
		if err != nil {
			// TODO Aggregate errors and continue
			return nil, fmt.Errorf("creating capacity %w", err)
		}
		podsMapped[*instanceID] = packing.Pods
		instanceIds = append(instanceIds, instanceID)
	}

	// 3. Convert to Nodes
	nodes, err := c.nodeFactory.For(ctx, instanceIds)
	if err != nil {
		return nil, fmt.Errorf("determining nodes, %w", err)
	}
	nodePackings := make(cloudprovider.NodePackings)
	for instanceID, node := range nodes {
		nodePackings[node] = podsMapped[instanceID]
	}
	return nodePackings, nil
}

// GetTopologyDomains returns a set of supported domains.
// e.g. us-west-2 -> [ us-west-2a, us-west-2b ]
func (c *Capacity) GetTopologyDomains(ctx context.Context, key cloudprovider.TopologyKey) ([]string, error) {
	switch key {
	case cloudprovider.TopologyKeyZone:
		zones, err := c.vpcProvider.GetZones(ctx, c.spec.Cluster.Name)
		if err != nil {
			return nil, err
		}
		return zones, nil
	case cloudprovider.TopologyKeySubnet:
		subnets, err := c.vpcProvider.GetSubnetIds(ctx, c.spec.Cluster.Name)
		if err != nil {
			return nil, err
		}
		return subnets, nil
	default:
		return nil, fmt.Errorf("unrecognized topology key %s", key)
	}
}
