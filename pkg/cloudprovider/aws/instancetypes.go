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
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/utils"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
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
func (p *InstanceTypeProvider) Get(ctx context.Context, zonalSubnetOptions map[string][]*ec2.Subnet, constraints *cloudprovider.Constraints) ([]*cloudprovider.Instance, error) {
	zones := []string{}
	for zone := range zonalSubnetOptions {
		zones = append(zones, zone)
	}

	zoneToInstanceTypeInfo := map[string][]*ec2.InstanceTypeInfo{}
	for _, zone := range zones {
		if instanceTypes, ok := p.instanceTypeCache.Get(zone); ok {
			zoneToInstanceTypeInfo[zone] = instanceTypes.([]*ec2.InstanceTypeInfo)
			continue
		}

		// populate the cache by zonal keys
		instanceTypes, err := p.getInstanceTypesForZone(ctx, zone)
		if err != nil {
			return nil, err
		}
		p.instanceTypeCache.SetDefault(zone, instanceTypes)
		zoneToInstanceTypeInfo[zone] = instanceTypes
	}

	ec2InstanceTypes := map[string]*cloudprovider.Instance{}
	for zone, instanceTypes := range zoneToInstanceTypeInfo {
		for _, it := range p.filterFrom(instanceTypes, constraints) {
			if instanceType, ok := ec2InstanceTypes[*it.InstanceType]; ok {
				instanceType.Zones = append(instanceType.Zones, zone)
			} else {
				ec2InstanceTypes[*it.InstanceType] = &cloudprovider.Instance{InstanceTypeInfo: *it, Zones: []string{zone}}
			}
		}
	}

	instanceTypes := []*cloudprovider.Instance{}
	for _, instanceType := range ec2InstanceTypes {
		instanceTypes = append(instanceTypes, instanceType)
	}

	return instanceTypes, nil
}

// GetAllInstanceTypeNames returns all instance type names without filtering based on constraints
func (p *InstanceTypeProvider) GetAllInstanceTypeNames(ctx context.Context, clusterName string) ([]string, error) {
	zones, err := p.vpc.GetZones(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	instanceTypeNames := []string{}
	for _, zone := range zones {
		instanceTypes, ok := p.instanceTypeCache.Get(zone)
		if !ok {
			instanceTypes, err = p.getInstanceTypesForZone(ctx, zone)
			if err != nil {
				return nil, err
			}
			p.instanceTypeCache.SetDefault(zone, instanceTypes)
		}
		for _, instanceType := range instanceTypes.([]*ec2.InstanceTypeInfo) {
			instanceTypeNames = append(instanceTypeNames, *instanceType.InstanceType)
		}
	}
	return instanceTypeNames, nil
}

func (p *InstanceTypeProvider) getInstanceTypesForZone(ctx context.Context, zone string) ([]*ec2.InstanceTypeInfo, error) {
	instanceTypes, err := p.getAllInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieving all instance types, %w", err)
	}
	instanceTypes, err = p.filterByZoneOfferings(ctx, instanceTypes, zone)
	if err != nil {
		return nil, fmt.Errorf("filtering instance types by zone offerings, %w", err)
	}
	return instanceTypes, nil
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
		},
	}
	err := p.ec2.DescribeInstanceTypesPagesWithContext(ctx, describeInstanceTypesInput, func(page *ec2.DescribeInstanceTypesOutput, lastPage bool) bool {
		instanceTypes = append(instanceTypes, page.InstanceTypes...)
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("fetching instance types using ec2.DescribeInstanceTypes, %w", err)
	}
	zap.S().Debugf("Successfully discovered %d EC2 instance types", len(instanceTypes))
	return instanceTypes, nil
}

// filterFrom returns a filtered list of instance types based on the provided resource constraints
func (p *InstanceTypeProvider) filterFrom(instanceTypes []*ec2.InstanceTypeInfo, constraints *cloudprovider.Constraints) []*ec2.InstanceTypeInfo {
	filtered := []*ec2.InstanceTypeInfo{}
	architecture := utils.NormalizeArchitecture(*constraints.Architecture)

	for _, instanceTypeInfo := range instanceTypes {
		if (len(constraints.InstanceTypes) == 0 || functional.ContainsString(constraints.InstanceTypes, *instanceTypeInfo.InstanceType)) &&
			(len(constraints.InstanceTypes) != 0 || p.isDefaultInstanceType(instanceTypeInfo)) &&
			functional.ContainsString(aws.StringValueSlice(instanceTypeInfo.ProcessorInfo.SupportedArchitectures), architecture) &&
			functional.ContainsString(aws.StringValueSlice(instanceTypeInfo.SupportedUsageClasses), "on-demand") {
			filtered = append(filtered, instanceTypeInfo)
		}
	}
	return filtered
}

// isDefaultInstanceType returns true if the instance type provided conforms to the default instance type criteria
// This function is used to make sure we launch instance types that are suited for general workloads
func (p *InstanceTypeProvider) isDefaultInstanceType(instanceTypeInfo *ec2.InstanceTypeInfo) bool {
	if instanceTypeInfo.FpgaInfo == nil &&
		instanceTypeInfo.GpuInfo == nil &&
		!*instanceTypeInfo.BareMetal &&
		functional.HasAnyPrefix(*instanceTypeInfo.InstanceType, "m", "c", "r", "a", "t3", "t4") {
		return true
	}
	return false
}

// filterByZoneOfferings returns a list of instance types that are supported in the provided availability zone using the ec2 DescribeInstanceTypeOfferings API
func (p *InstanceTypeProvider) filterByZoneOfferings(ctx context.Context, instanceTypes []*ec2.InstanceTypeInfo, zone string) ([]*ec2.InstanceTypeInfo, error) {
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
	zap.S().Debugf("Successfully discovered %d EC2 instance types supported in the %s availability zone", len(instanceTypeInfoSupported), zone)
	return instanceTypeInfoSupported, nil
}
