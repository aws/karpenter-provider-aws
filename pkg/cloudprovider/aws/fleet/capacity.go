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
	"math/rand"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

// Capacity cloud provider implementation using AWS Fleet.
type Capacity struct {
	spec                   *v1alpha1.ProvisionerSpec
	ec2                    ec2iface.EC2API
	launchTemplateProvider *LaunchTemplateProvider
	subnetProvider         *SubnetProvider
	nodeFactory            *NodeFactory
}

// Create a set of nodes given the constraints.
func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.CapacityConstraints) ([]*v1.Node, error) {
	// 1. Select a zone
	zone, err := c.selectZone(ctx, constraints)
	if err != nil {
		return nil, fmt.Errorf("getting zone, %w", err)
	}

	// 2. Select a subnet, limited to selected zone.
	subnet, err := c.selectSubnet(ctx, zone, constraints)
	if err != nil {
		return nil, fmt.Errorf("getting subnet, %w", err)
	}

	// 3. Detect launch template.
	launchTemplate, err := c.launchTemplateProvider.Get(ctx, c.spec.Cluster)
	if err != nil {
		return nil, fmt.Errorf("getting launch template, %w", err)
	}

	// 3. Create Fleet.
	createFleetOutput, err := c.ec2.CreateFleetWithContext(ctx, &ec2.CreateFleetInput{
		Type: aws.String(ec2.FleetTypeInstant),
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(ec2.DefaultTargetCapacityTypeOnDemand), // TODO support SPOT
			TotalTargetCapacity:       aws.Int64(1),                                      // TODO construct this more intelligently
		},
		LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{{
			LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: launchTemplate.LaunchTemplateName,
				Version:            aws.String("$Default"),
			},
			Overrides: []*ec2.FleetLaunchTemplateOverridesRequest{{
				AvailabilityZone: aws.String(zone),
				InstanceType:     aws.String("m5.large"), // TODO construct this more intelligently
				SubnetId:         subnet.SubnetId,
			}},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("creating fleet %w", err)
	}
	if len(createFleetOutput.Errors) > 0 {
		// TODO hande case if createFleetOutput.Instances > 0
		return nil, fmt.Errorf("errors while creating fleet, %v", createFleetOutput.Errors)
	}
	if len(createFleetOutput.Instances) == 0 {
		return nil, fmt.Errorf("create fleet returned 0 instances")
	}
	// 4. Transform to Nodes.
	var instanceIds []*string
	for _, fleetInstance := range createFleetOutput.Instances {
		instanceIds = append(instanceIds, fleetInstance.InstanceIds...)
	}
	nodes, err := c.nodeFactory.For(ctx, instanceIds)
	if err != nil {
		return nil, fmt.Errorf("determining nodes, %w", err)
	}
	zap.S().Infof("Successfully requested %d nodes", len(nodes))
	return nodes, nil
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

// seletZone chooses a zone for the given constraints.
func (c *Capacity) selectZone(ctx context.Context, constraints *cloudprovider.CapacityConstraints) (string, error) {
	// 1. Return zone if specified.
	if zone, ok := constraints.Topology[cloudprovider.TopologyKeyZone]; ok {
		return zone, nil
	}
	// 2. Randomly choose from available zones.
	zones, err := c.getZones(ctx)
	if err != nil {
		return "", err
	}
	return zones[rand.Intn(len(zones))], nil
}

// selectSubnet chooses a subnet for the given constraints.
func (c *Capacity) selectSubnet(ctx context.Context, zone string, constraints *cloudprovider.CapacityConstraints) (*ec2.Subnet, error) {
	// 1. Get all subnets
	zonalSubnets, err := c.subnetProvider.Get(ctx, c.spec.Cluster.Name)
	if err != nil {
		return nil, err
	}

	// 2. Return specific subnet if specified.
	if subnetId, ok := constraints.Topology[cloudprovider.TopologyKeySubnet]; ok {
		for _, subnets := range zonalSubnets {
			for _, subnet := range subnets {
				if subnetId == *subnet.SubnetId {
					return subnet, nil
				}
			}
		}
		return nil, fmt.Errorf("no subnet exists named %s", subnetId)
	}

	// 3. Return random subnet in the zone.
	subnets, ok := zonalSubnets[zone]
	if !ok || len(subnets) == 0 {
		return nil, fmt.Errorf("no subnets exists for zone %s", zone)
	}
	return subnets[rand.Intn(len(subnets))], nil
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
