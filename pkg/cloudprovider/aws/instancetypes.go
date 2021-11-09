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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/patrickmn/go-cache"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"
)

const (
	instanceTypesCacheKey         = "types"
	instanceTypeZonesCacheKey     = "zones"
	instanceTypesAndZonesCacheTTL = 5 * time.Minute
)

type InstanceTypeProvider struct {
	ec2api         ec2iface.EC2API
	subnetProvider *SubnetProvider
	cache          *cache.Cache
}

func NewInstanceTypeProvider(ec2api ec2iface.EC2API, subnetProvider *SubnetProvider) *InstanceTypeProvider {
	return &InstanceTypeProvider{
		ec2api:         ec2api,
		subnetProvider: subnetProvider,
		cache:          cache.New(instanceTypesAndZonesCacheTTL, CacheCleanupInterval),
	}
}

// Get instance type options given the constraints
func (p *InstanceTypeProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints) ([]cloudprovider.InstanceType, error) {
	// Get InstanceTypes from EC2
	instanceTypes, err := p.getInstanceTypes(ctx)
	if err != nil {
		return nil, err
	}
	// Get Viable AZs from subnets
	subnets, err := p.subnetProvider.Get(ctx, constraints)
	if err != nil {
		return nil, err
	}
	subnetZones := sets.NewString()
	for _, subnet := range subnets {
		subnetZones.Insert(aws.StringValue(subnet.AvailabilityZone))
	}
	// Get Viable EC2 Purchase offerings
	instanceTypeZones, err := p.getInstanceTypeZones(ctx)
	if err != nil {
		return nil, err
	}
	// Convert to cloudprovider.InstanceType
	result := []cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		//TODO filter out possible zones and capacity types using an ICE cache https://github.com/awslabs/karpenter/issues/371
		offerings := []cloudprovider.Offering{}
		for zone := range subnetZones.Intersection(instanceTypeZones[instanceType.Name()]) {
			// while usage classes should be a distinct set, there's no guarantee of that
			for capacityType := range sets.NewString(aws.StringValueSlice(instanceType.SupportedUsageClasses)...) {
				offerings = append(offerings, cloudprovider.Offering{Zone: zone, CapacityType: capacityType})
			}
		}
		instanceType.AvailableOfferings = offerings
		result = append(result, instanceType)
	}
	return result, nil
}

func (p *InstanceTypeProvider) getInstanceTypeZones(ctx context.Context) (result map[string]sets.String, err error) {
	if cached, ok := p.cache.Get(instanceTypeZonesCacheKey); ok {
		return cached.(map[string]sets.String), nil
	}
	zones := map[string]sets.String{}
	if err := p.ec2api.DescribeInstanceTypeOfferingsPagesWithContext(ctx, &ec2.DescribeInstanceTypeOfferingsInput{LocationType: aws.String("availability-zone")},
		func(output *ec2.DescribeInstanceTypeOfferingsOutput, lastPage bool) bool {
			for _, offering := range output.InstanceTypeOfferings {
				if _, ok := zones[aws.StringValue(offering.InstanceType)]; !ok {
					zones[aws.StringValue(offering.InstanceType)] = sets.NewString()
				}
				zones[aws.StringValue(offering.InstanceType)].Insert(aws.StringValue(offering.Location))
			}
			return true
		}); err != nil {
		return nil, fmt.Errorf("describing instance type zone offerings, %w", err)
	}
	logging.FromContext(ctx).Debugf("Discovered EC2 instance types zonal offerings")
	p.cache.SetDefault(instanceTypeZonesCacheKey, zones)
	return zones, nil
}

// getInstanceTypes retrieves all instance types from the ec2 DescribeInstanceTypes API using some opinionated filters
func (p *InstanceTypeProvider) getInstanceTypes(ctx context.Context) (result map[string]*InstanceType, err error) {
	if cached, ok := p.cache.Get(instanceTypesCacheKey); ok {
		return cached.(map[string]*InstanceType), nil
	}
	instanceTypes := map[string]*InstanceType{}
	if err := p.ec2api.DescribeInstanceTypesPagesWithContext(ctx, &ec2.DescribeInstanceTypesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("supported-virtualization-type"),
				Values: []*string{aws.String("hvm")},
			},
		},
	}, func(page *ec2.DescribeInstanceTypesOutput, lastPage bool) bool {
		for _, instanceType := range page.InstanceTypes {
			if p.filter(instanceType) {
				instanceTypes[aws.StringValue(instanceType.InstanceType)] = &InstanceType{InstanceTypeInfo: *instanceType}
			}
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("fetching instance types using ec2.DescribeInstanceTypes, %w", err)
	}
	logging.FromContext(ctx).Debugf("Discovered %d EC2 instance types", len(instanceTypes))
	p.cache.SetDefault(instanceTypesCacheKey, instanceTypes)
	return instanceTypes, nil
}

// filter the instance types to include useful ones for Kubernetes
func (p *InstanceTypeProvider) filter(instanceType *ec2.InstanceTypeInfo) bool {
	if instanceType.FpgaInfo != nil {
		return false
	}
	if aws.BoolValue(instanceType.BareMetal) {
		return false
	}
	// TODO exclude if not available for spot
	return functional.HasAnyPrefix(aws.StringValue(instanceType.InstanceType),
		"m", "c", "r", "a", // Standard
		"t3", "t4", // Burstable
		"p", "inf", "g", // Accelerators
	)
}
