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
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

// Capacity cloud provider implementation using AWS Fleet.
type Capacity struct {
	spec                   *v1alpha1.ProvisionerSpec
	nodeFactory            *NodeFactory
	packer                 packing.Packer
	instanceProvider       *InstanceProvider
	vpcProvider            *VPCProvider
	launchTemplateProvider *LaunchTemplateProvider
}

// Create a set of nodes given the constraints.
func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.Constraints) ([]cloudprovider.Packing, error) {
	// 1. Compute Packing given the constraints
	instancePackings, err := c.packer.Pack(ctx, constraints.Pods)
	if err != nil {
		return nil, fmt.Errorf("computing bin packing, %w", err)
	}

	zap.S().Debugf("Computed packings for %d pod(s) onto %d node(s)", len(constraints.Pods), len(instancePackings))
	launchTemplate, err := c.launchTemplateProvider.Get(ctx, c.spec.Cluster, constraints)
	if err != nil {
		return nil, fmt.Errorf("getting launch template, %w", err)
	}

	zonalSubnetOptions, err := c.vpcProvider.GetZonalSubnets(ctx, constraints, c.spec.Cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("getting zonal subnets, %w", err)
	}

	// 2. Create Instances
	var instanceIds []*string
	podsForInstance := make(map[string][]*v1.Pod)
	for _, packing := range instancePackings {
		instanceID, err := c.instanceProvider.Create(ctx, launchTemplate, packing.InstanceTypes, zonalSubnetOptions)
		if err != nil {
			// TODO Aggregate errors and continue
			return nil, fmt.Errorf("creating capacity %w", err)
		}
		podsForInstance[*instanceID] = packing.Pods
		instanceIds = append(instanceIds, instanceID)
	}

	// 3. Convert to Nodes
	nodes, err := c.nodeFactory.For(ctx, instanceIds)
	if err != nil {
		return nil, fmt.Errorf("determining nodes, %w", err)
	}
	nodePackings := []cloudprovider.Packing{}
	for instanceID, node := range nodes {
		node.Labels = constraints.Labels
		node.Spec.Taints = constraints.Taints
		nodePackings = append(nodePackings, cloudprovider.Packing{
			Node: node,
			Pods: podsForInstance[instanceID],
		})
	}
	return nodePackings, nil
}

func (c *Capacity) Delete(ctx context.Context, nodes []*v1.Node) error {
	return c.instanceProvider.Terminate(ctx, nodes)
}
