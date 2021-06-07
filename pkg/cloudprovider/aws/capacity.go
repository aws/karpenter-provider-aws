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

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
)

// Capacity cloud provider implementation using AWS Fleet.
type Capacity struct {
	provisioner            *v1alpha1.Provisioner
	nodeFactory            *NodeFactory
	instanceProvider       *InstanceProvider
	subnetProvider         *SubnetProvider
	launchTemplateProvider *LaunchTemplateProvider
	instanceTypeProvider   *InstanceTypeProvider
}

// Create a set of nodes given the constraints.
func (c *Capacity) Create(ctx context.Context, packings []*cloudprovider.Packing) ([]*cloudprovider.PackedNode, error) {
	instanceIDs := []*string{}
	instancePackings := map[string]*cloudprovider.Packing{}
	for _, packing := range packings {
		constraints := Constraints(*packing.Constraints)
		// 1. Get Subnets and constrain by zones
		zonalSubnets, err := c.subnetProvider.GetZonalSubnets(ctx, c.provisioner.Spec.Cluster.Name)
		if err != nil {
			return nil, fmt.Errorf("getting zonal subnets, %w", err)
		}
		zonalSubnetOptions := map[string][]*ec2.Subnet{}
		for zone, subnets := range zonalSubnets {
			if len(constraints.Zones) == 0 || functional.ContainsString(constraints.Zones, zone) {
				zonalSubnetOptions[zone] = subnets
			}
		}
		// 2. Get Launch Template
		launchTemplate, err := c.launchTemplateProvider.Get(ctx, c.provisioner, &constraints)
		if err != nil {
			return nil, fmt.Errorf("getting launch template, %w", err)
		}
		// 3. Create instance
		instanceID, err := c.instanceProvider.Create(ctx, launchTemplate, packing.InstanceTypeOptions, zonalSubnets, constraints.GetCapacityType())
		if err != nil {
			// TODO Aggregate errors and continue
			return nil, fmt.Errorf("creating capacity %w", err)
		}
		instancePackings[*instanceID] = packing
		instanceIDs = append(instanceIDs, instanceID)
	}

	// 4. Convert to Nodes
	nodes, err := c.nodeFactory.For(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("determining nodes, %w", err)
	}
	// 5. Convert to PackedNodes, TODO: move this logic into NodeFactory
	packedNodes := []*cloudprovider.PackedNode{}
	for instanceID, node := range nodes {
		packing := instancePackings[instanceID]
		node.Labels = packing.Constraints.Labels
		node.Spec.Taints = packing.Constraints.Taints
		packedNodes = append(packedNodes, &cloudprovider.PackedNode{
			Node: node,
			Pods: packing.Pods,
		})
	}
	return packedNodes, nil
}

func (c *Capacity) GetInstanceTypes(ctx context.Context) ([]cloudprovider.InstanceType, error) {
	return c.instanceTypeProvider.Get(ctx)
}
