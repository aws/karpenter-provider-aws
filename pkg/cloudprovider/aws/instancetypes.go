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
	"net/http"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	instancetypev1alpha1 "github.com/aws/karpenter/pkg/apis/instancetype/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/utils/resources"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/pretty"
)

const (
	InstanceTypesCacheKey              = "types"
	InstanceTypeZonesCacheKeyPrefix    = "zones:"
	InstanceTypesAndZonesCacheTTL      = 5 * time.Minute
	UnfulfillableCapacityErrorCacheTTL = 3 * time.Minute
)

type InstanceTypeProvider struct {
	sync.Mutex
	region          string
	ec2api          ec2iface.EC2API
	subnetProvider  *SubnetProvider
	pricingProvider *PricingProvider
	// Has one cache entry for all the instance types (key: InstanceTypesCacheKey)
	// Has one cache entry for all the zones for each subnet selector (key: InstanceTypesZonesCacheKeyPrefix:<hash_of_selector>)
	// Values cached *before* considering insufficient capacity errors from the unavailableOfferings cache.
	cache *cache.Cache
	// key: <capacityType>:<instanceType>:<zone>, value: struct{}{}
	unavailableOfferings *cache.Cache
	cm                   *pretty.ChangeMonitor
	kubeClient           k8sClient.Client
}

func NewInstanceTypeProvider(ctx context.Context, sess *session.Session, options cloudprovider.Options, ec2api ec2iface.EC2API, subnetProvider *SubnetProvider) *InstanceTypeProvider {
	return &InstanceTypeProvider{
		ec2api:         ec2api,
		region:         *sess.Config.Region,
		subnetProvider: subnetProvider,
		pricingProvider: NewPricingProvider(ctx,
			NewPricingAPI(sess, *sess.Config.Region),
			ec2api,
			*sess.Config.Region,
			injection.GetOptions(ctx).AWSIsolatedVPC, options.StartAsync),
		cache:                cache.New(InstanceTypesAndZonesCacheTTL, CacheCleanupInterval),
		unavailableOfferings: cache.New(UnfulfillableCapacityErrorCacheTTL, CacheCleanupInterval),
		cm:                   pretty.NewChangeMonitor(),
		kubeClient:           options.KubeClient,
	}
}

// Get all instance type options
func (p *InstanceTypeProvider) Get(ctx context.Context, provider *v1alpha1.AWS, kc *v1alpha5.KubeletConfiguration) ([]cloudprovider.InstanceType, error) {
	p.Lock()
	defer p.Unlock()
	// Get InstanceTypes from EC2
	instanceTypes, err := p.getInstanceTypes(ctx)
	if err != nil {
		return nil, err
	}
	// Get Viable EC2 Purchase offerings
	instanceTypeZones, err := p.getInstanceTypeZones(ctx, provider)
	if err != nil {
		return nil, err
	}
	var result []cloudprovider.InstanceType

	// Grab the instance type override values when grabbing instance type details
	overrideMap, err := p.makeOverrideMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance type override map, %w", err)
	}
	for _, i := range instanceTypes {
		instanceTypeName := aws.StringValue(i.InstanceType)
		instanceType := NewInstanceType(ctx, i, kc, p.region, provider, p.createOfferings(ctx, i, instanceTypeZones[instanceTypeName]))

		// Check if there are instance type overrides for this name
		// If there are, then we override them
		if v, ok := overrideMap[instanceType.Name()]; ok {
			instanceType = mergeInstanceTypeOverrides(instanceType, v)
		}
		result = append(result, instanceType)
	}
	return result, nil
}

func (p *InstanceTypeProvider) LivenessProbe(req *http.Request) error {
	p.Lock()
	//nolint: staticcheck
	p.Unlock()
	if err := p.subnetProvider.LivenessProbe(req); err != nil {
		return err
	}
	if err := p.pricingProvider.LivenessProbe(req); err != nil {
		return err
	}
	return nil
}

func (p *InstanceTypeProvider) createOfferings(ctx context.Context, instanceType *ec2.InstanceTypeInfo, zones sets.String) []cloudprovider.Offering {
	offerings := []cloudprovider.Offering{}
	for zone := range zones {
		// while usage classes should be a distinct set, there's no guarantee of that
		for capacityType := range sets.NewString(aws.StringValueSlice(instanceType.SupportedUsageClasses)...) {
			// exclude any offerings that have recently seen an insufficient capacity error from EC2
			_, isUnavailable := p.unavailableOfferings.Get(UnavailableOfferingsCacheKey(*instanceType.InstanceType, zone, capacityType))
			var price float64
			var ok bool
			switch capacityType {
			case ec2.UsageClassTypeSpot:
				price, ok = p.pricingProvider.SpotPrice(*instanceType.InstanceType, zone)
			case ec2.UsageClassTypeOnDemand:
				price, ok = p.pricingProvider.OnDemandPrice(*instanceType.InstanceType)
			default:
				logging.FromContext(ctx).Errorf("Received unknown capacity type %s for instance type %s", capacityType, *instanceType.InstanceType)
				continue
			}
			available := !isUnavailable && ok
			offerings = append(offerings, cloudprovider.Offering{
				Zone:         zone,
				CapacityType: capacityType,
				Price:        price,
				Available:    available,
			})
		}
	}
	return offerings
}

func (p *InstanceTypeProvider) getInstanceTypeZones(ctx context.Context, provider *v1alpha1.AWS) (map[string]sets.String, error) {
	subnetSelectorHash, err := hashstructure.Hash(provider.SubnetSelector, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to hash the subnet selector: %w", err)
	}
	cacheKey := fmt.Sprintf("%s%016x", InstanceTypeZonesCacheKeyPrefix, subnetSelectorHash)
	if cached, ok := p.cache.Get(cacheKey); ok {
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
	if p.cm.HasChanged("zonal-offerings", provider.SubnetSelector) {
		logging.FromContext(ctx).Debugf("Discovered EC2 instance types zonal offerings for subnets %s", pretty.Concise(provider.SubnetSelector))
	}
	p.cache.SetDefault(cacheKey, instanceTypeZones)
	return instanceTypeZones, nil
}

// getInstanceTypes retrieves all instance types from the ec2 DescribeInstanceTypes API using some opinionated filters
func (p *InstanceTypeProvider) getInstanceTypes(ctx context.Context) (map[string]*ec2.InstanceTypeInfo, error) {
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
	if p.cm.HasChanged("instance-types", instanceTypes) {
		logging.FromContext(ctx).Debugf("Discovered %d EC2 instance types", len(instanceTypes))
	}
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

// makeOverrideMap creates an in-memory version of the List of InstancType CRDs on the Kubernetes API server
// for overriding instance type details after the instance type has been created
func (p *InstanceTypeProvider) makeOverrideMap(ctx context.Context) (map[string]instancetypev1alpha1.InstanceType, error) {
	overrides := &instancetypev1alpha1.InstanceTypeList{}
	err := p.kubeClient.List(ctx, overrides)
	if err != nil {
		return nil, err
	}
	return lo.SliceToMap(overrides.Items, func(it instancetypev1alpha1.InstanceType) (string, instancetypev1alpha1.InstanceType) {
		return it.Name, it
	}), nil
}

func UnavailableOfferingsCacheKey(instanceType string, zone string, capacityType string) string {
	return fmt.Sprintf("%s:%s:%s", capacityType, instanceType, zone)
}

// mergeInstanceTypeOverrides merges the values from the InstanceType CRs with the resources currently
// pulled from the EC2 APIs. Capacity should already be validated when they get here, so we will automatically
// add them to the instance type resources and its requirements
func mergeInstanceTypeOverrides(it *InstanceType, instanceType instancetypev1alpha1.InstanceType) *InstanceType {
	for name, quantity := range instanceType.Spec.Capacity {
		it.resources[name] = quantity
		if !resources.WellKnownResourceNames.Has(name.String()) && !v1alpha5.WellKnownLabels.Has(name.String()) {
			it.requirements.Add(scheduling.NewRequirement(name.String(), v1.NodeSelectorOpIn, fmt.Sprint(quantity.Value())))
		}
	}
	return it
}
