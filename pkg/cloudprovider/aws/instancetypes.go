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
	"knative.dev/pkg/logging"
)

const (
	allInstanceTypesKey = "all"
)

type InstanceTypeProvider struct {
	ec2api ec2iface.EC2API
	cache  *cache.Cache
}

func NewInstanceTypeProvider(ec2api ec2iface.EC2API) *InstanceTypeProvider {
	return &InstanceTypeProvider{
		ec2api: ec2api,
		cache:  cache.New(CacheTTL, CacheCleanupInterval),
	}
}

// Get instance types that are available per availability zone
func (p *InstanceTypeProvider) Get(ctx context.Context) ([]cloudprovider.InstanceType, error) {
	var instanceTypes []cloudprovider.InstanceType
	if cached, ok := p.cache.Get(allInstanceTypesKey); ok {
		instanceTypes = cached.([]cloudprovider.InstanceType)
	} else {
		var err error
		instanceTypes, err = p.get(ctx)
		if err != nil {
			return nil, err
		}
		p.cache.SetDefault(allInstanceTypesKey, instanceTypes)
		logging.FromContext(ctx).Debugf("Discovered %d EC2 instance types", len(instanceTypes))
	}
	return instanceTypes, nil
}

func (p *InstanceTypeProvider) get(ctx context.Context) ([]cloudprovider.InstanceType, error) {
	// 1. Get InstanceTypes from EC2
	instanceTypes, err := p.getInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieving all instance types, %w", err)
	}

	err = p.ec2api.DescribeInstanceTypeOfferingsPagesWithContext(ctx, &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: aws.String("availability-zone"),
	}, func(output *ec2.DescribeInstanceTypeOfferingsOutput, lastPage bool) bool {
		for _, offering := range output.InstanceTypeOfferings {
			for _, instanceType := range instanceTypes {
				if instanceType.Name() == aws.StringValue(offering.InstanceType) {
					instanceType.ZoneOptions = append(instanceType.ZoneOptions, aws.StringValue(offering.Location))
				}
			}
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("describing instance type zone offerings, %w", err)
	}

	// convert to cloudprovider.InstanceType
	result := []cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		result = append(result, instanceType)
	}
	return result, nil
}

// getInstanceTypes retrieves all instance types from the ec2 DescribeInstanceTypes API using some opinionated filters
func (p *InstanceTypeProvider) getInstanceTypes(ctx context.Context) ([]*InstanceType, error) {
	instanceTypes := []*InstanceType{}
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
				instanceTypes = append(instanceTypes, &InstanceType{InstanceTypeInfo: *instanceType})
			}
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("fetching instance types using ec2.DescribeInstanceTypes, %w", err)
	}
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
