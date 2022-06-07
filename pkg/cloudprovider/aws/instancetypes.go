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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
)

const (
	InstanceTypesCacheKey              = "types"
	InstanceTypeZonesCacheKey          = "zones"
	InstanceTypesAndZonesCacheTTL      = 5 * time.Minute
	UnfulfillableCapacityErrorCacheTTL = 3 * time.Minute
)

type InstanceTypeProvider struct {
	sync.Mutex
	ec2api         ec2iface.EC2API
	subnetProvider *SubnetProvider
	// Has two entries: one for all the instance types and one for all zones; values cached *before* considering insufficient capacity errors
	// from the unavailableOfferings cache
	cache *cache.Cache
	// key: <capacityType>:<instanceType>:<zone>, value: struct{}{}
	unavailableOfferings *cache.Cache
}

func NewInstanceTypeProvider(ec2api ec2iface.EC2API, subnetProvider *SubnetProvider) *InstanceTypeProvider {
	return &InstanceTypeProvider{
		ec2api:               ec2api,
		subnetProvider:       subnetProvider,
		cache:                cache.New(InstanceTypesAndZonesCacheTTL, CacheCleanupInterval),
		unavailableOfferings: cache.New(UnfulfillableCapacityErrorCacheTTL, CacheCleanupInterval),
	}
}

// Get all instance type options
func (p *InstanceTypeProvider) Get(ctx context.Context, provider *v1alpha1.AWS) ([]cloudprovider.InstanceType, error) {
	p.Lock()
	defer p.Unlock()
	// Get InstanceTypes from EC2
	instanceTypes, err := p.getInstanceTypes(ctx, provider)
	if err != nil {
		return nil, err
	}
	// Get Viable EC2 Purchase offerings
	instanceTypeZones, err := p.getInstanceTypeZones(ctx, provider)
	if err != nil {
		return nil, err
	}
	var result []cloudprovider.InstanceType
	for _, i := range instanceTypes {
		result = append(result, p.newInstanceType(ctx, i, provider, p.createOfferings(i, instanceTypeZones[aws.StringValue(i.InstanceType)])))
	}
	return result, nil
}

func (p *InstanceTypeProvider) newInstanceType(ctx context.Context, info *ec2.InstanceTypeInfo, provider *v1alpha1.AWS, offerings []cloudprovider.Offering) *InstanceType {
	instanceType := &InstanceType{
		InstanceTypeInfo: info,
		provider:         provider,
		offerings:        offerings,
	}
	// Precompute to minimize memory/compute overhead
	instanceType.resources = instanceType.computeResources(injection.GetOptions(ctx).AWSEnablePodENI)
	instanceType.overhead = instanceType.computeOverhead()
	instanceType.requirements = instanceType.computeRequirements()
	if !injection.GetOptions(ctx).AWSENILimitedPodDensity {
		instanceType.maxPods = ptr.Int32(110)
	}
	return instanceType
}

func (p *InstanceTypeProvider) createOfferings(instanceType *ec2.InstanceTypeInfo, zones sets.String) []cloudprovider.Offering {
	offerings := []cloudprovider.Offering{}
	for zone := range zones {
		// while usage classes should be a distinct set, there's no guarantee of that
		for capacityType := range sets.NewString(aws.StringValueSlice(instanceType.SupportedUsageClasses)...) {
			// exclude any offerings that have recently seen an insufficient capacity error from EC2
			if _, isUnavailable := p.unavailableOfferings.Get(UnavailableOfferingsCacheKey(*instanceType.InstanceType, zone, capacityType)); !isUnavailable {
				offerings = append(offerings, cloudprovider.Offering{Zone: zone, CapacityType: capacityType})
			}
		}
	}
	return offerings
}

func (p *InstanceTypeProvider) getInstanceTypeZones(ctx context.Context, provider *v1alpha1.AWS) (map[string]sets.String, error) {
	if cached, ok := p.cache.Get(InstanceTypeZonesCacheKey); ok {
		return cached.(map[string]sets.String), nil
	}

	// Constrain AZs from subnets
	subnets, err := p.subnetProvider.Get(ctx, provider)
	if err != nil {
		return nil, err
	}
	zones := sets.NewString(lo.Map(subnets, func(subnet *ec2.Subnet, _ int) string {
		return aws.StringValue(subnet.AvailabilityZone)
	})...)

	// Get offerings from EC2
	instanceTypeZones := map[string]sets.String{}
	if err := p.ec2api.DescribeInstanceTypeOfferingsPagesWithContext(ctx, &ec2.DescribeInstanceTypeOfferingsInput{LocationType: aws.String("availability-zone")},
		func(output *ec2.DescribeInstanceTypeOfferingsOutput, lastPage bool) bool {
			for _, offering := range output.InstanceTypeOfferings {
				if zones.Has(aws.StringValue(offering.Location)) {
					if _, ok := instanceTypeZones[aws.StringValue(offering.InstanceType)]; !ok {
						instanceTypeZones[aws.StringValue(offering.InstanceType)] = sets.NewString()
					}
					instanceTypeZones[aws.StringValue(offering.InstanceType)].Insert(aws.StringValue(offering.Location))
				}
			}
			return true
		}); err != nil {
		return nil, fmt.Errorf("describing instance type zone offerings, %w", err)
	}
	logging.FromContext(ctx).Debugf("Discovered EC2 instance types zonal offerings")
	p.cache.SetDefault(InstanceTypeZonesCacheKey, instanceTypeZones)
	return instanceTypeZones, nil
}

// getInstanceTypes retrieves all instance types from the ec2 DescribeInstanceTypes API using some opinionated filters
func (p *InstanceTypeProvider) getInstanceTypes(ctx context.Context, provider *v1alpha1.AWS) (map[string]*ec2.InstanceTypeInfo, error) {
	if cached, ok := p.cache.Get(InstanceTypesCacheKey); ok {
		return cached.(map[string]*ec2.InstanceTypeInfo), nil
	}
	instanceTypes := map[string]*ec2.InstanceTypeInfo{}
	if err := p.ec2api.DescribeInstanceTypesPagesWithContext(ctx, &ec2.DescribeInstanceTypesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("supported-virtualization-type"),
				Values: []*string{aws.String("hvm")},
			},
			{
				Name:   aws.String("processor-info.supported-architecture"),
				Values: aws.StringSlice([]string{"x86_64", "arm64"}),
			},
		},
	}, func(page *ec2.DescribeInstanceTypesOutput, lastPage bool) bool {
		for _, instanceType := range page.InstanceTypes {
			if p.filter(instanceType) {
				instanceTypes[aws.StringValue(instanceType.InstanceType)] = instanceType
			}
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("fetching instance types using ec2.DescribeInstanceTypes, %w", err)
	}
	logging.FromContext(ctx).Debugf("Discovered %d EC2 instance types", len(instanceTypes))
	p.cache.SetDefault(InstanceTypesCacheKey, instanceTypes)
	return instanceTypes, nil
}

// filter the instance types to include useful ones for Kubernetes
func (p *InstanceTypeProvider) filter(instanceType *ec2.InstanceTypeInfo) bool {
	if instanceType.FpgaInfo != nil {
		return false
	}
	if functional.HasAnyPrefix(aws.StringValue(instanceType.InstanceType),
		// G2 instances have an older GPU not supported by the nvidia plugin. This causes the allocatable # of gpus
		// to be set to zero on startup as the plugin considers the GPU unhealthy.
		"g2",
	) {
		return false
	}
	return true
}

// CacheUnavailable allows the InstanceProvider to communicate recently observed temporary capacity shortages in
// the provided offerings
func (p *InstanceTypeProvider) CacheUnavailable(ctx context.Context, fleetErr *ec2.CreateFleetError, capacityType string) {
	instanceType := aws.StringValue(fleetErr.LaunchTemplateAndOverrides.Overrides.InstanceType)
	zone := aws.StringValue(fleetErr.LaunchTemplateAndOverrides.Overrides.AvailabilityZone)
	logging.FromContext(ctx).Debugf("%s for offering { instanceType: %s, zone: %s, capacityType: %s }, avoiding for %s",
		aws.StringValue(fleetErr.ErrorCode),
		instanceType,
		zone,
		capacityType,
		UnfulfillableCapacityErrorCacheTTL)
	// even if the key is already in the cache, we still need to call Set to extend the cached entry's TTL
	p.unavailableOfferings.SetDefault(UnavailableOfferingsCacheKey(instanceType, zone, capacityType), struct{}{})
}

func UnavailableOfferingsCacheKey(instanceType string, zone string, capacityType string) string {
	return fmt.Sprintf("%s:%s:%s", capacityType, instanceType, zone)
}
