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

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
)

// Capacity cloud provider implementation using AWS Fleet.
type Capacity struct {
	spec                   *v1alpha1.ProvisionerSpec
	launchTemplateProvider *LaunchTemplateProvider
	subnetProvider         *SubnetProvider
	nodeFactory            *NodeFactory
	instanceProvider       *InstanceProvider
}

// Create a set of nodes given the constraints.
func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.CapacityConstraints) (cloudprovider.CapacityPacking, error) {
	// 1. Compute Constraints
	launchTemplate, err := c.launchTemplateProvider.Get(ctx, c.spec.Cluster)
	if err != nil {
		return nil, fmt.Errorf("getting launch template, %w", err)
	}
	instancePackings, err := c.instanceProvider.GetPackings(ctx, constraints.Pods, constraints.Overhead)
	if err != nil {
		return nil, fmt.Errorf("computing bin packing, %w", err)
	}
	zonalSubnets, err := c.getConstrainedZonalSubnets(ctx, constraints)
	if err != nil {
		return nil, fmt.Errorf("getting zonal subnets, %w", err)
	}

	// 2. Create Instances
	var instanceIds []*string
	for _, instancePacking := range instancePackings {
		instanceId, err := c.instanceProvider.Create(ctx, launchTemplate, instancePacking.InstanceTypeOptions, zonalSubnets)
		if err != nil {
			// TODO Aggregate errors and continue
			return nil, fmt.Errorf("creating capacity %w", err)
		}
		instanceIds = append(instanceIds, instanceId)
	}

	// 3. Convert to Nodes
	nodes, err := c.nodeFactory.For(ctx, instanceIds)
	if err != nil {
		return nil, fmt.Errorf("determining nodes, %w", err)
	}

	// 4. Construct capacity packing
	capacityPacking := cloudprovider.CapacityPacking{}
	for i, node := range nodes {
		capacityPacking[node] = instancePackings[i].Pods
	}
	return capacityPacking, nil
}

// GetTopologyDomains returns a set of supported domains.
// e.g. us-west-2 -> [ us-west-2a, us-west-2b ]
func (c *Capacity) GetTopologyDomains(ctx context.Context, key cloudprovider.TopologyKey) ([]string, error) {
	switch key {
	case cloudprovider.TopologyKeyZone:
		zones, err := c.getZones(ctx)
		if err != nil {
			return nil, err
		}
		return zones, nil
	case cloudprovider.TopologyKeySubnet:
		subnets, err := c.getSubnetIds(ctx)
		if err != nil {
			return nil, err
		}
		return subnets, nil
	default:
		return nil, fmt.Errorf("unrecognized topology key %s", key)
	}
}

func (c *Capacity) getConstrainedZonalSubnets(ctx context.Context, constraints *cloudprovider.CapacityConstraints) (map[string][]*ec2.Subnet, error) {
	// 1. Get all subnets
	zonalSubnets, err := c.subnetProvider.Get(ctx, c.spec.Cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("getting zonal subnets, %w", err)
	}
	// 2. Return specific subnet if specified.
	if subnetId, ok := constraints.Topology[cloudprovider.TopologyKeySubnet]; ok {
		for zone, subnets := range zonalSubnets {
			for _, subnet := range subnets {
				if subnetId == *subnet.SubnetId {
					return map[string][]*ec2.Subnet{zone: {subnet}}, nil
				}
			}
		}
		return nil, fmt.Errorf("no subnet exists named %s", subnetId)
	}
	// 3. Constrain by zones
	constrainedZones, err := c.getConstrainedZones(ctx, constraints)
	if err != nil {
		return nil, fmt.Errorf("getting zones, %w", err)
	}
	constrainedZonalSubnets := map[string][]*ec2.Subnet{}
	for zone, subnets := range zonalSubnets {
		for _, constrainedZone := range constrainedZones {
			if zone == constrainedZone {
				constrainedZonalSubnets[constrainedZone] = subnets
			}
		}
	}
	if len(constrainedZonalSubnets) == 0 {
		return nil, fmt.Errorf("failed to find viable zonal subnet pairing")
	}
	return constrainedZonalSubnets, nil
}

func (c *Capacity) getConstrainedZones(ctx context.Context, constraints *cloudprovider.CapacityConstraints) ([]string, error) {
	// 1. Return zone if specified.
	if zone, ok := constraints.Topology[cloudprovider.TopologyKeyZone]; ok {
		return []string{zone}, nil
	}
	// 2. Return all zone options
	zones, err := c.getZones(ctx)
	if err != nil {
		return nil, err
	}
	return zones, nil
}

func (c *Capacity) getZones(ctx context.Context) ([]string, error) {
	zonalSubnets, err := c.subnetProvider.Get(ctx, c.spec.Cluster.Name)
	if err != nil {
		return nil, err
	}
	zones := []string{}
	for zone := range zonalSubnets {
		zones = append(zones, zone)
	}
	return zones, nil
}

func (c *Capacity) getSubnetIds(ctx context.Context) ([]string, error) {
	zonalSubnets, err := c.subnetProvider.Get(ctx, c.spec.Cluster.Name)
	if err != nil {
		return nil, err
	}
	subnetIds := []string{}
	for _, subnets := range zonalSubnets {
		for _, subnet := range subnets {
			subnetIds = append(subnetIds, *subnet.SubnetId)
		}
	}
	return subnetIds, nil
}
