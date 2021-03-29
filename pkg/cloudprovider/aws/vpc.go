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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

type VPCProvider struct {
	ec2            ec2iface.EC2API
	subnetProvider *SubnetProvider
	zoneCache      *cache.Cache
}

func NewVPCProvider(ec2api ec2iface.EC2API, subnetProvider *SubnetProvider) *VPCProvider {
	return &VPCProvider{
		ec2:            ec2api,
		subnetProvider: subnetProvider,
		zoneCache:      cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (p *VPCProvider) GetZones(ctx context.Context, clusterName string) ([]string, error) {
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

func (p *VPCProvider) GetZonalSubnets(ctx context.Context, constraints *cloudprovider.Constraints, clusterName string) (map[string][]*ec2.Subnet, error) {
	// 1. Get all subnets
	zonalSubnets, err := p.subnetProvider.Get(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("getting zonal subnets, %w", err)
	}

	// 2. Normalize zone constraints to all be zone names
	zones, err := p.normalizeZones(ctx, constraints.Zones)
	if err != nil {
		return nil, err
	}

	// 3. Constrain by zones
	constrainedZones, err := p.getConstrainedZones(ctx, zones, clusterName)
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

// normalizeZones takes zone names or ids and returns them all as zone names
func (p *VPCProvider) normalizeZones(ctx context.Context, zones []string) ([]string, error) {
	allZonesKey := "all"
	azs, ok := p.zoneCache.Get(allZonesKey)
	if !ok {
		azsOutput, err := p.ec2.DescribeAvailabilityZonesWithContext(ctx, &ec2.DescribeAvailabilityZonesInput{})
		if err != nil {
			return zones, fmt.Errorf("retrieving availability zones, %w", err)
		}
		p.zoneCache.SetDefault(allZonesKey, azsOutput.AvailabilityZones)
		azs = azsOutput.AvailabilityZones
	}

	zoneNames := []string{}
	for _, zone := range zones {
		for _, az := range azs.([]*ec2.AvailabilityZone) {
			if zone == *az.ZoneName || zone == *az.ZoneId {
				zoneNames = append(zoneNames, *az.ZoneName)
			}
		}
	}
	zap.S().Debugf("Successfully normalized %d zone(s) to their respective zone name(s)", len(zones))
	return zoneNames, nil
}

func (p *VPCProvider) getConstrainedZones(ctx context.Context, zoneConstraints []string, clusterName string) ([]string, error) {
	zones, err := p.GetZones(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	// Unconstrained
	if len(zoneConstraints) == 0 {
		return zones, nil
	}
	// Supported by provider and by constraints
	return functional.IntersectStringSlice(zones, zoneConstraints), nil
}

type ZonalSubnets map[string][]*ec2.Subnet

type SubnetProvider struct {
	ec2         ec2iface.EC2API
	subnetCache *cache.Cache
}

func NewSubnetProvider(ec2 ec2iface.EC2API) *SubnetProvider {
	return &SubnetProvider{
		ec2:         ec2,
		subnetCache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (s *SubnetProvider) Get(ctx context.Context, clusterName string) (ZonalSubnets, error) {
	if zonalSubnets, ok := s.subnetCache.Get(clusterName); ok {
		return zonalSubnets.(ZonalSubnets), nil
	}
	return s.getZonalSubnets(ctx, clusterName)
}

func (s *SubnetProvider) getZonalSubnets(ctx context.Context, clusterName string) (ZonalSubnets, error) {
	describeSubnetOutput, err := s.ec2.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("tag-key"),
			Values: []*string{aws.String(fmt.Sprintf(ClusterTagKeyFormat, clusterName))},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("describing subnets, %w", err)
	}

	zonalSubnetMap := ZonalSubnets{}
	for _, subnet := range describeSubnetOutput.Subnets {
		if subnets, ok := zonalSubnetMap[*subnet.AvailabilityZone]; ok {
			zonalSubnetMap[*subnet.AvailabilityZone] = append(subnets, subnet)
		} else {
			zonalSubnetMap[*subnet.AvailabilityZone] = []*ec2.Subnet{subnet}
		}
	}

	s.subnetCache.Set(clusterName, zonalSubnetMap, CacheTTL)
	zap.S().Debugf("Successfully discovered subnets in %d zones for cluster %s", len(zonalSubnetMap), clusterName)
	return zonalSubnetMap, nil
}
