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
	v1 "k8s.io/api/core/v1"
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
func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.Constraints) (cloudprovider.PodPackings, error) {
	// 1. Compute Packing given the constraints
	packings, err := c.packing.Pack(ctx, constraints.Pods)
	if err != nil {
		return nil, fmt.Errorf("computing bin packing, %w", err)
	}

	launchTemplate, err := c.vpc.GetLaunchTemplate(ctx, c.spec.Cluster)
	if err != nil {
		return nil, fmt.Errorf("getting launch template, %w", err)
	}

	zonalSubnetOptions, err := c.vpc.GetZonalSubnets(ctx, constraints, c.spec.Cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("getting zonal subnets, %w", err)
	}

	// 2. Create Instances
	var instanceIds []*string
	podsMapped := make(map[string][]*v1.Pod)
	for _, packing := range packings {
		ec2InstanceID, err := c.instanceProvider.Create(ctx, packing.InstanceTypeOptions, launchTemplate, zonalSubnetOptions)
		if err != nil {
			// TODO Aggregate errors and continue
			return nil, fmt.Errorf("creating capacity %w", err)
		}
		podsMapped[*ec2InstanceID] = packing.Pods
		instanceIds = append(instanceIds, ec2InstanceID)
	}

	// 3. Convert to Nodes
	nodes, err := c.nodeFactory.For(ctx, instanceIds)
	if err != nil {
		return nil, fmt.Errorf("determining nodes, %w", err)
	}
	nodePackings := make(cloudprovider.PodPackings)
	for instanceID, node := range nodes {
		nodePackings[node] = podsMapped[instanceID]
	}
	return nodePackings, nil
}

// GetTopologyDomains returns a set of supported domains.
// e.g. us-west-2 -> [ us-west-2a, us-west-2b ]
func (c *Capacity) GetTopologyDomains(ctx context.Context, key cloudprovider.TopologyKey) ([]string, error) {
	return c.vpc.GetTopologyDomains(ctx, key, c.spec.Cluster.Name)
}
