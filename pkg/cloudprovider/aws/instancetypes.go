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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/patrickmn/go-cache"
)

type InstanceTypeProvider struct {
	ec2               ec2iface.EC2API
	vpc               *VPCProvider
	instanceTypeCache *cache.Cache
}

func NewInstanceTypeProvider(ec2 ec2iface.EC2API, vpcProvider *VPCProvider) *InstanceTypeProvider {
	return &InstanceTypeProvider{
		ec2:               ec2,
		vpc:               vpcProvider,
		instanceTypeCache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

// Get instance types that are availble per availability zone
func (p *InstanceTypeProvider) Get(ctx context.Context, clusterName string, constraints *cloudprovider.Constraints) (map[string][]*ec2.InstanceTypeInfo, error) {
	zones, err := p.retrieveZonesFrom(ctx, constraints, clusterName)
	if err != nil {
		return nil, fmt.Errorf("retrieving availability zones, %w", err)
	}

	zoneToInstanceTypeInfo := map[string][]*ec2.InstanceTypeInfo{}
	for _, zone := range zones {
		if instanceTypes, ok := p.instanceTypeCache.Get(zone); ok {
			zoneToInstanceTypeInfo[zone] = instanceTypes.([]*ec2.InstanceTypeInfo)
			continue
		}

		// populate the cache by zonal keys
		instanceTypes, err := p.getAllInstanceTypes(ctx)
		if err != nil {
			return nil, fmt.Errorf("retrieving all instance types, %w", err)
		}
		instanceTypes, err = p.filterByLocation(ctx, instanceTypes, zone)
		if err != nil {
			return nil, err
		}
		p.instanceTypeCache.SetDefault(zone, instanceTypes)
		zoneToInstanceTypeInfo[zone] = instanceTypes
	}

	for zone, instanceTypes := range zoneToInstanceTypeInfo {
		zoneToInstanceTypeInfo[zone] = p.filterFrom(instanceTypes, constraints)
	}
	return zoneToInstanceTypeInfo, nil
}

// UniqueInstanceTypesFrom returns a unique slice of InstanceTypeInfo structs from the values of the instancePools passed in
func (p *InstanceTypeProvider) UniqueInstanceTypesFrom(instancePools map[string][]*ec2.InstanceTypeInfo) []*ec2.InstanceTypeInfo {
	result := []*ec2.InstanceTypeInfo{}
	uniqueInstanceTypes := map[string]bool{}
	for _, instanceTypes := range instancePools {
		for _, instanceType := range instanceTypes {
			if _, ok := uniqueInstanceTypes[*instanceType.InstanceType]; !ok {
				uniqueInstanceTypes[*instanceType.InstanceType] = true
				result = append(result, instanceType)
			}
		}
	}
	return result
}

// InstanceTypesPerZoneFrom returns a mapping of zone to InstanceTypeInfo based on the provided slice of instance types names and mapping of all zones to InstanceTypeInfo
func (p *InstanceTypeProvider) InstanceTypesPerZoneFrom(instanceTypeNames []string, instancePools map[string][]*ec2.InstanceTypeInfo) map[string][]*ec2.InstanceTypeInfo {
	result := map[string][]*ec2.InstanceTypeInfo{}
	for zone, instanceTypes := range instancePools {
		for _, instanceTypeName := range instanceTypeNames {
			for _, instanceType := range instanceTypes {
				if instanceType != nil && *instanceType.InstanceType == instanceTypeName {
					result[zone] = append(result[zone], instanceType)
				}
			}
		}
	}
	return result
}

// getAllInstanceTypes retrieves all instance types from the ec2 DescribeInstanceTypes API using some opinionated filters
func (p *InstanceTypeProvider) getAllInstanceTypes(ctx context.Context) ([]*ec2.InstanceTypeInfo, error) {
	instanceTypes := []*ec2.InstanceTypeInfo{}
	describeInstanceTypesInput := &ec2.DescribeInstanceTypesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("supported-virtualization-type"),
				Values: []*string{aws.String("hvm")},
			},
			{
				Name:   aws.String("bare-metal"),
				Values: []*string{aws.String("false")},
			},
			{
				Name:   aws.String("instance-type"),
				Values: aws.StringSlice([]string{"m5.*", "c5.*", "r5.*", "m5a.*", "c5a.*", "r5a.*", "m4.*", "c4.*", "r4.*", "m6g.*", "c6g.*", "r6g.*"}),
			},
		},
	}
	err := p.ec2.DescribeInstanceTypesPagesWithContext(ctx, describeInstanceTypesInput, func(page *ec2.DescribeInstanceTypesOutput, lastPage bool) bool {
		instanceTypes = append(instanceTypes, page.InstanceTypes...)
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("fetching instance types using ec2.DescribeInstanceTypes, %w", err)
	}
	return instanceTypes, nil
}

// filterFrom returns a filtered list of instance types based on the provided resource constraints
func (p *InstanceTypeProvider) filterFrom(instanceTypes []*ec2.InstanceTypeInfo, constraints *cloudprovider.Constraints) []*ec2.InstanceTypeInfo {
	filteredInstancePools := []*ec2.InstanceTypeInfo{}
	architecture := "x86_64"
	if *constraints.Architecture == v1alpha1.ArchitectureArm64 {
		architecture = string(*constraints.Architecture)
	}

	for _, instanceTypeInfo := range instanceTypes {
		if (len(constraints.InstanceTypes) != 0 && !functional.ContainsString(constraints.InstanceTypes, *instanceTypeInfo.InstanceType)) ||
			!functional.ContainsString(aws.StringValueSlice(instanceTypeInfo.ProcessorInfo.SupportedArchitectures), architecture) ||
			!functional.ContainsString(aws.StringValueSlice(instanceTypeInfo.SupportedUsageClasses), "on-demand") ||
			*instanceTypeInfo.BurstablePerformanceSupported ||
			instanceTypeInfo.FpgaInfo != nil ||
			instanceTypeInfo.GpuInfo != nil {
			continue
		}
		filteredInstancePools = append(filteredInstancePools, instanceTypeInfo)
	}
	return filteredInstancePools
}

// filterByLocation returns a list of instance types that are supported in the provided availability zone using the ec2 DescribeInstanceTypeOfferings API
func (p *InstanceTypeProvider) filterByLocation(ctx context.Context, instanceTypes []*ec2.InstanceTypeInfo, zone string) ([]*ec2.InstanceTypeInfo, error) {
	inputs := &ec2.DescribeInstanceTypeOfferingsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("location"),
				Values: aws.StringSlice([]string{zone}),
			},
		},
		LocationType: aws.String("availability-zone"),
	}

	instanceTypeNamesSupported := []string{}

	err := p.ec2.DescribeInstanceTypeOfferingsPagesWithContext(ctx, inputs, func(output *ec2.DescribeInstanceTypeOfferingsOutput, lastPage bool) bool {
		for _, offerings := range output.InstanceTypeOfferings {
			instanceTypeNamesSupported = append(instanceTypeNamesSupported, *offerings.InstanceType)
		}
		return true
	})
	if err != nil {
		return instanceTypes, fmt.Errorf("describing instance type location offerings, %w", err)
	}

	instanceTypeInfoSupported := []*ec2.InstanceTypeInfo{}
	for _, instanceTypeInfo := range instanceTypes {
		for _, instanceTypeName := range instanceTypeNamesSupported {
			if instanceTypeName == *instanceTypeInfo.InstanceType {
				instanceTypeInfoSupported = append(instanceTypeInfoSupported, instanceTypeInfo)
			}
		}
	}
	return instanceTypeInfoSupported, nil
}

func (p *InstanceTypeProvider) retrieveZonesFrom(ctx context.Context, constraints *cloudprovider.Constraints, clusterName string) ([]string, error) {
	zones := constraints.Zones
	// If no zone constraints were specified, use all zones that the cluster spans
	if len(zones) == 0 {
		var err error
		zones, err = p.vpc.GetZones(ctx, clusterName)
		if err != nil {
			return nil, err
		}
	}
	// zones could be zone names and zone ids, so normalize them to all be zone names
	zones, err := p.vpc.NormalizeZones(ctx, zones)
	if err != nil {
		return nil, err
	}
	return zones, nil
}
