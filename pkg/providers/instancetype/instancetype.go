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

package instancetype

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	awscache "github.com/aws/karpenter/pkg/cache"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/providers/pricing"
	"github.com/aws/karpenter/pkg/providers/subnet"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

const (
	InstanceTypesCacheKey           = "types"
	InstanceTypeZonesCacheKeyPrefix = "zones:"
)

type Provider struct {
	region          string
	ec2api          ec2iface.EC2API
	subnetProvider  *subnet.Provider
	pricingProvider *pricing.Provider
	// Has one cache entry for all the instance types (key: InstanceTypesCacheKey)
	// Has one cache entry for all the zones for each subnet selector (key: InstanceTypesZonesCacheKeyPrefix:<hash_of_selector>)
	// Values cached *before* considering insufficient capacity errors from the unavailableOfferings cache.
	// Fully initialized Instance Types are also cached based on the set of all instance types, zones, unavailableOfferings cache,
	// node template, and kubelet configuration from the provisioner

	mu    sync.Mutex
	cache *cache.Cache

	unavailableOfferings *awscache.UnavailableOfferings
	cm                   *pretty.ChangeMonitor
	// instanceTypesSeqNum is a monotonically increasing change counter used to avoid the expensive hashing operation on instance types
	instanceTypesSeqNum uint64
}

func NewProvider(region string, cache *cache.Cache, ec2api ec2iface.EC2API, subnetProvider *subnet.Provider,
	unavailableOfferingsCache *awscache.UnavailableOfferings, pricingProvider *pricing.Provider) *Provider {
	return &Provider{
		ec2api:               ec2api,
		region:               region,
		subnetProvider:       subnetProvider,
		pricingProvider:      pricingProvider,
		cache:                cache,
		unavailableOfferings: unavailableOfferingsCache,
		cm:                   pretty.NewChangeMonitor(),
		instanceTypesSeqNum:  0,
	}
}

func (p *Provider) List(ctx context.Context, kc *corev1beta1.KubeletConfiguration, nodeClass *v1beta1.NodeClass) ([]*cloudprovider.InstanceType, error) {
	// Get InstanceTypes from EC2
	instanceTypes, err := p.GetInstanceTypes(ctx)
	if err != nil {
		return nil, err
	}
	// Get Viable EC2 Purchase offerings
	instanceTypeZones, err := p.getInstanceTypeZones(ctx, nodeClass)
	if err != nil {
		return nil, err
	}

	// Compute fully initialized instance types hash key
	instanceTypeZonesHash, _ := hashstructure.Hash(instanceTypeZones, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	kcHash, _ := hashstructure.Hash(kc, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	key := fmt.Sprintf("%d-%d-%s-%016x-%016x", p.instanceTypesSeqNum, p.unavailableOfferings.SeqNum, nodeClass.UID, instanceTypeZonesHash, kcHash)

	if item, ok := p.cache.Get(key); ok {
		return item.([]*cloudprovider.InstanceType), nil
	}
	// Reject any instance types that don't have any offerings due to zone
	result := lo.Reject(lo.Map(instanceTypes, func(i *ec2.InstanceTypeInfo, _ int) *cloudprovider.InstanceType {
		return NewInstanceType(ctx, i, kc, p.region, nodeClass, p.createOfferings(ctx, i, instanceTypeZones[aws.StringValue(i.InstanceType)]))
	}), func(i *cloudprovider.InstanceType, _ int) bool {
		return len(i.Offerings) == 0
	})
	for _, instanceType := range instanceTypes {
		InstanceTypeVCPU.With(prometheus.Labels{
			InstanceTypeLabel: *instanceType.InstanceType,
		}).Set(float64(aws.Int64Value(instanceType.VCpuInfo.DefaultVCpus)))
		InstanceTypeMemory.With(prometheus.Labels{
			InstanceTypeLabel: *instanceType.InstanceType,
		}).Set(float64(aws.Int64Value(instanceType.MemoryInfo.SizeInMiB) * 1024 * 1024))
	}
	p.cache.SetDefault(key, result)
	return result, nil
}

func (p *Provider) LivenessProbe(req *http.Request) error {
	if err := p.subnetProvider.LivenessProbe(req); err != nil {
		return err
	}
	return p.pricingProvider.LivenessProbe(req)
}

func (p *Provider) createOfferings(ctx context.Context, instanceType *ec2.InstanceTypeInfo, zones sets.Set[string]) []cloudprovider.Offering {
	var offerings []cloudprovider.Offering
	for zone := range zones {
		// while usage classes should be a distinct set, there's no guarantee of that
		for capacityType := range sets.NewString(aws.StringValueSlice(instanceType.SupportedUsageClasses)...) {
			// exclude any offerings that have recently seen an insufficient capacity error from EC2
			isUnavailable := p.unavailableOfferings.IsUnavailable(*instanceType.InstanceType, zone, capacityType)
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

func (p *Provider) getInstanceTypeZones(ctx context.Context, nodeClass *v1beta1.NodeClass) (map[string]sets.Set[string], error) {
	// DO NOT REMOVE THIS LOCK ----------------------------------------------------------------------------
	// We lock here so that multiple callers to getInstanceTypeZones do not result in cache misses and multiple
	// calls to EC2 when we could have just made one call.
	// TODO @joinnis: This can be made more efficient by holding a Read lock and only obtaining the Write if not in cache
	p.mu.Lock()
	defer p.mu.Unlock()

	subnetSelectorHash, err := hashstructure.Hash(nodeClass.Spec.SubnetSelectorTerms, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, fmt.Errorf("failed to hash the subnet selector: %w", err)
	}
	cacheKey := fmt.Sprintf("%s%016x", InstanceTypeZonesCacheKeyPrefix, subnetSelectorHash)
	if cached, ok := p.cache.Get(cacheKey); ok {
		return cached.(map[string]sets.Set[string]), nil
	}

	// Constrain AZs from subnets
	subnets, err := p.subnetProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	if len(subnets) == 0 {
		return nil, nil
	}
	zones := sets.NewString(lo.Map(subnets, func(subnet *ec2.Subnet, _ int) string {
		return aws.StringValue(subnet.AvailabilityZone)
	})...)

	// Get offerings from EC2
	instanceTypeZones := map[string]sets.Set[string]{}
	if err := p.ec2api.DescribeInstanceTypeOfferingsPagesWithContext(ctx, &ec2.DescribeInstanceTypeOfferingsInput{LocationType: aws.String("availability-zone")},
		func(output *ec2.DescribeInstanceTypeOfferingsOutput, lastPage bool) bool {
			for _, offering := range output.InstanceTypeOfferings {
				if zones.Has(aws.StringValue(offering.Location)) {
					if _, ok := instanceTypeZones[aws.StringValue(offering.InstanceType)]; !ok {
						instanceTypeZones[aws.StringValue(offering.InstanceType)] = sets.New[string]()
					}
					instanceTypeZones[aws.StringValue(offering.InstanceType)].Insert(aws.StringValue(offering.Location))
				}
			}
			return true
		}); err != nil {
		return nil, fmt.Errorf("describing instance type zone offerings, %w", err)
	}
	if p.cm.HasChanged("zonal-offerings", nodeClass.Spec.SubnetSelectorTerms) {
		logging.FromContext(ctx).With("zones", zones.List(), "instance-type-count", len(instanceTypeZones), "node-template", nodeClass.Name).Debugf("discovered offerings for instance types")
	}
	p.cache.SetDefault(cacheKey, instanceTypeZones)
	return instanceTypeZones, nil
}

// GetInstanceTypes retrieves all instance types from the ec2 DescribeInstanceTypes API using some opinionated filters
func (p *Provider) GetInstanceTypes(ctx context.Context) ([]*ec2.InstanceTypeInfo, error) {
	// DO NOT REMOVE THIS LOCK ----------------------------------------------------------------------------
	// We lock here so that multiple callers to GetInstanceTypes do not result in cache misses and multiple
	// calls to EC2 when we could have just made one call. This lock is here because multiple callers to EC2 result
	// in A LOT of extra memory generated from the response for simultaneous callers.
	// TODO @joinnis: This can be made more efficient by holding a Read lock and only obtaining the Write if not in cache
	p.mu.Lock()
	defer p.mu.Unlock()

	if cached, ok := p.cache.Get(InstanceTypesCacheKey); ok {
		return cached.([]*ec2.InstanceTypeInfo), nil
	}
	var instanceTypes []*ec2.InstanceTypeInfo
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
		instanceTypes = append(instanceTypes, page.InstanceTypes...)
		return true
	}); err != nil {
		return nil, fmt.Errorf("fetching instance types using ec2.DescribeInstanceTypes, %w", err)
	}
	if p.cm.HasChanged("instance-types", instanceTypes) {
		logging.FromContext(ctx).With(
			"count", len(instanceTypes)).Debugf("discovered instance types")
	}
	atomic.AddUint64(&p.instanceTypesSeqNum, 1)
	p.cache.SetDefault(InstanceTypesCacheKey, instanceTypes)
	return instanceTypes, nil
}
