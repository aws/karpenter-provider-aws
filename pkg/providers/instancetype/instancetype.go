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

	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

const (
	InstanceTypesCacheKey         = "types"
	InstanceTypeOfferingsCacheKey = "offerings"
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
	// EC2NodeClass, and kubelet configuration from the NodePool

	mu    sync.Mutex
	cache *cache.Cache

	unavailableOfferings *awscache.UnavailableOfferings
	cm                   *pretty.ChangeMonitor
	// instanceTypesSeqNum is a monotonically increasing change counter used to avoid the expensive hashing operation on instance types
	instanceTypesSeqNum uint64
	// instanceTypeOfferingsSeqNum is a monotonically increasing change counter used to avoid the expensive hashing operation on instance types
	instanceTypeOfferingsSeqNum uint64
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

func (p *Provider) List(ctx context.Context, kc *corev1beta1.KubeletConfiguration, nodeClass *v1beta1.EC2NodeClass) ([]*cloudprovider.InstanceType, error) {
	// Get InstanceTypes from EC2
	instanceTypes, err := p.GetInstanceTypes(ctx)
	if err != nil {
		return nil, err
	}
	// Get InstanceTypeOfferings from EC2
	instanceTypeOfferings, err := p.getInstanceTypeOfferings(ctx)
	if err != nil {
		return nil, err
	}
	subnets, err := p.subnetProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	subnetZones := sets.New[string](lo.Map(subnets, func(s *ec2.Subnet, _ int) string {
		return aws.StringValue(s.AvailabilityZone)
	})...)

	// Compute fully initialized instance types hash key
	subnetZonesHash, _ := hashstructure.Hash(subnetZones, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	kcHash, _ := hashstructure.Hash(kc, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	// TODO: remove kubeReservedHash and systemReservedHash once v1.ResourceList objects are hashed as strings in KubeletConfiguration
	// For more information on the v1.ResourceList hash issue: https://github.com/kubernetes-sigs/karpenter/issues/1080
	kubeReservedHash, systemReservedHash := uint64(0), uint64(0)
	if kc != nil {
		kubeReservedHash, _ = hashstructure.Hash(resources.StringMap(kc.KubeReserved), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
		systemReservedHash, _ = hashstructure.Hash(resources.StringMap(kc.SystemReserved), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	}
	blockDeviceMappingsHash, _ := hashstructure.Hash(nodeClass.Spec.BlockDeviceMappings, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	key := fmt.Sprintf("%d-%d-%d-%016x-%016x-%016x-%s-%s-%016x-%016x",
		p.instanceTypesSeqNum,
		p.instanceTypeOfferingsSeqNum,
		p.unavailableOfferings.SeqNum,
		subnetZonesHash,
		kcHash,
		blockDeviceMappingsHash,
		aws.StringValue((*string)(nodeClass.Spec.InstanceStorePolicy)),
		aws.StringValue(nodeClass.Spec.AMIFamily),
		kubeReservedHash,
		systemReservedHash,
	)
	if item, ok := p.cache.Get(key); ok {
		return item.([]*cloudprovider.InstanceType), nil
	}

	// Get all zones across all offerings
	// We don't use this in the cache key since this is produced from our instanceTypeOfferings which we do cache
	allZones := sets.New[string]()
	for _, offeringZones := range instanceTypeOfferings {
		for zone := range offeringZones {
			allZones.Insert(zone)
		}
	}
	if p.cm.HasChanged("zones", allZones) {
		logging.FromContext(ctx).With("zones", allZones.UnsortedList()).Debugf("discovered zones")
	}
	result := lo.Map(instanceTypes, func(i *ec2.InstanceTypeInfo, _ int) *cloudprovider.InstanceType {
		instanceTypeVCPU.With(prometheus.Labels{
			instanceTypeLabel: *i.InstanceType,
		}).Set(float64(aws.Int64Value(i.VCpuInfo.DefaultVCpus)))
		instanceTypeMemory.With(prometheus.Labels{
			instanceTypeLabel: *i.InstanceType,
		}).Set(float64(aws.Int64Value(i.MemoryInfo.SizeInMiB) * 1024 * 1024))

		return NewInstanceType(ctx, i, kc, p.region, nodeClass, p.createOfferings(ctx, i, instanceTypeOfferings[aws.StringValue(i.InstanceType)], allZones, subnetZones))
	})
	p.cache.SetDefault(key, result)
	return result, nil
}

func (p *Provider) LivenessProbe(req *http.Request) error {
	if err := p.subnetProvider.LivenessProbe(req); err != nil {
		return err
	}
	return p.pricingProvider.LivenessProbe(req)
}

func (p *Provider) createOfferings(ctx context.Context, instanceType *ec2.InstanceTypeInfo, instanceTypeZones, zones, subnetZones sets.Set[string]) []cloudprovider.Offering {
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
			case "capacity-block":
				// ignore since karpenter doesn't support it yet, but do not log an unknown capacity type error
				continue
			default:
				logging.FromContext(ctx).Errorf("Received unknown capacity type %s for instance type %s", capacityType, *instanceType.InstanceType)
				continue
			}
			available := !isUnavailable && ok && instanceTypeZones.Has(zone) && subnetZones.Has(zone)
			offerings = append(offerings, cloudprovider.Offering{
				Zone:         zone,
				CapacityType: capacityType,
				Price:        price,
				Available:    available,
			})
			instanceTypeOfferingAvailable.With(prometheus.Labels{
				instanceTypeLabel: *instanceType.InstanceType,
				capacityTypeLabel: capacityType,
				zoneLabel:         zone,
			}).Set(float64(lo.Ternary(available, 1, 0)))
			instanceTypeOfferingPriceEstimate.With(prometheus.Labels{
				instanceTypeLabel: *instanceType.InstanceType,
				capacityTypeLabel: capacityType,
				zoneLabel:         zone,
			}).Set(price)
		}
	}
	return offerings
}

func (p *Provider) getInstanceTypeOfferings(ctx context.Context) (map[string]sets.Set[string], error) {
	// DO NOT REMOVE THIS LOCK ----------------------------------------------------------------------------
	// We lock here so that multiple callers to getInstanceTypeOfferings do not result in cache misses and multiple
	// calls to EC2 when we could have just made one call.
	// TODO @joinnis: This can be made more efficient by holding a Read lock and only obtaining the Write if not in cache
	p.mu.Lock()
	defer p.mu.Unlock()
	if cached, ok := p.cache.Get(InstanceTypeOfferingsCacheKey); ok {
		return cached.(map[string]sets.Set[string]), nil
	}

	// Get offerings from EC2
	instanceTypeOfferings := map[string]sets.Set[string]{}
	if err := p.ec2api.DescribeInstanceTypeOfferingsPagesWithContext(ctx, &ec2.DescribeInstanceTypeOfferingsInput{LocationType: aws.String("availability-zone")},
		func(output *ec2.DescribeInstanceTypeOfferingsOutput, lastPage bool) bool {
			for _, offering := range output.InstanceTypeOfferings {
				if _, ok := instanceTypeOfferings[aws.StringValue(offering.InstanceType)]; !ok {
					instanceTypeOfferings[aws.StringValue(offering.InstanceType)] = sets.New[string]()
				}
				instanceTypeOfferings[aws.StringValue(offering.InstanceType)].Insert(aws.StringValue(offering.Location))
			}
			return true
		}); err != nil {
		return nil, fmt.Errorf("describing instance type zone offerings, %w", err)
	}
	if p.cm.HasChanged("instance-type-offering", instanceTypeOfferings) {
		// Only update instanceTypesSeqNun with the instance type offerings  have been changed
		// This is to not create new keys with duplicate instance type offerings option
		atomic.AddUint64(&p.instanceTypeOfferingsSeqNum, 1)
		logging.FromContext(ctx).With("instance-type-count", len(instanceTypeOfferings)).Debugf("discovered offerings for instance types")
	}
	p.cache.SetDefault(InstanceTypeOfferingsCacheKey, instanceTypeOfferings)
	return instanceTypeOfferings, nil
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
		// Only update instanceTypesSeqNun with the instance types have been changed
		// This is to not create new keys with duplicate instance types option
		atomic.AddUint64(&p.instanceTypesSeqNum, 1)
		logging.FromContext(ctx).With(
			"count", len(instanceTypes)).Debugf("discovered instance types")
	}
	p.cache.SetDefault(InstanceTypesCacheKey, instanceTypes)
	return instanceTypes, nil
}
