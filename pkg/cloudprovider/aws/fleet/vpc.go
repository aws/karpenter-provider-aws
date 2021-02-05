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

type VPCProvider struct {
	launchTemplateProvider *LaunchTemplateProvider
	subnetProvider         *SubnetProvider
}

// GetTopologyDomains returns a set of supported domains.
// e.g. us-west-2 -> [ us-west-2a, us-west-2b ]
func (p *VPCProvider) GetTopologyDomains(ctx context.Context, key cloudprovider.TopologyKey, clusterName string) ([]string, error) {
	switch key {
	case cloudprovider.TopologyKeyZone:
		zones, err := p.getZones(ctx, clusterName)
		if err != nil {
			return nil, err
		}
		return zones, nil
	case cloudprovider.TopologyKeySubnet:
		subnets, err := p.getSubnetIds(ctx, clusterName)
		if err != nil {
			return nil, err
		}
		return subnets, nil
	default:
		return nil, fmt.Errorf("unrecognized topology key %s", key)
	}
}

func (p *VPCProvider) getLaunchTemplate(ctx context.Context, clusterSpec *v1alpha1.ClusterSpec) (*ec2.LaunchTemplate, error) {
	return p.launchTemplateProvider.Get(ctx, clusterSpec)
}

func (p *VPCProvider) getConstrainedZonalSubnets(ctx context.Context, constraints *cloudprovider.CapacityConstraints, clusterName string) (map[string][]*ec2.Subnet, error) {
	// 1. Get all subnets
	zonalSubnets, err := p.subnetProvider.Get(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("getting zonal subnets, %w", err)
	}
	// 2. Return specific subnet if specified.
	if subnetID, ok := constraints.Topology[cloudprovider.TopologyKeySubnet]; ok {
		for zone, subnets := range zonalSubnets {
			for _, subnet := range subnets {
				if subnetID == *subnet.SubnetId {
					return map[string][]*ec2.Subnet{zone: {subnet}}, nil
				}
			}
		}
		return nil, fmt.Errorf("no subnet exists named %s", subnetID)
	}
	// 3. Constrain by zones
	constrainedZones, err := p.getConstrainedZones(ctx, constraints, clusterName)
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

func (p *VPCProvider) getConstrainedZones(ctx context.Context, constraints *cloudprovider.CapacityConstraints, clusterName string) ([]string, error) {
	// 1. Return zone if specified.
	if zone, ok := constraints.Topology[cloudprovider.TopologyKeyZone]; ok {
		return []string{zone}, nil
	}
	// 2. Return all zone options
	zones, err := p.getZones(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	return zones, nil
}

func (p *VPCProvider) getZones(ctx context.Context, clusterName string) ([]string, error) {
	zonalSubnets, err := p.subnetProvider.Get(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	zones := []string{}
	for zone := range zonalSubnets {
		zones = append(zones, zone)
	}
	return zones, nil
}

func (p *VPCProvider) getSubnetIds(ctx context.Context, clusterName string) ([]string, error) {
	zonalSubnets, err := p.subnetProvider.Get(ctx, clusterName)
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
