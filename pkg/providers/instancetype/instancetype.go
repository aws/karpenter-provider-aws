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
	"sync"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"

	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/compatibility"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/offering"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"

	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type NodeClass interface {
	client.Object
	AMIFamily() string
	AMIs() []v1.AMI
	BlockDeviceMappings() []*v1.BlockDeviceMapping
	CapacityReservations() []v1.CapacityReservation
	CPUOptions() *v1.CPUOptions
	InstanceStorePolicy() *v1.InstanceStorePolicy
	NetworkInterfaces() []*v1.NetworkInterface
	KubeletConfiguration() *v1.KubeletConfiguration
	PlacementGroupSelector() *v1.PlacementGroupSelector
	ZoneInfo() []v1.ZoneInfo
	ConnectionTracking() *v1.ConnectionTracking
}

type Provider interface {
	Get(context.Context, NodeClass, ec2types.InstanceType) (*cloudprovider.InstanceType, error)
	List(context.Context, NodeClass) ([]*cloudprovider.InstanceType, error)
	FilterForNodeClass(context.Context, []*cloudprovider.InstanceType, NodeClass) []*cloudprovider.InstanceType
	// ValidateKubeletExpressions evaluates the NodeClass' kubelet CEL expressions against every known instance
	// type and returns an error describing the first per-instance-type evaluation failure. A compile-only check
	// cannot catch these because the instance-type variables aren't known until an instance type is in hand.
	ValidateKubeletExpressions(context.Context, NodeClass) error
}

type DefaultProvider struct {
	ec2api                sdk.EC2API
	subnetProvider        subnet.Provider
	instanceTypesResolver Resolver

	// Values stored *before* considering insufficient capacity errors from the unavailableOfferings cache.
	// Fully initialized Instance Types are also cached based on the set of all instance types, zones, unavailableOfferings cache,
	// EC2NodeClass, and kubelet configuration from the NodePool

	muInstanceTypesInfo sync.RWMutex
	// TODO @engedaam: Look into only storing the needed EC2InstanceTypeInfo
	instanceTypesInfo map[ec2types.InstanceType]ec2types.InstanceTypeInfo

	muInstanceTypesOfferings sync.RWMutex
	instanceTypesOfferings   map[ec2types.InstanceType]sets.Set[string]
	allZones                 sets.Set[string]

	instanceTypesCache      *cache.Cache
	discoveredCapacityCache *cache.Cache
	cm                      *pretty.ChangeMonitor

	placementGroupProvider placementgroup.Provider
	offeringProvider       *offering.DefaultProvider
}

func NewDefaultProvider(
	instanceTypesCache *cache.Cache,
	offeringCache *cache.Cache,
	discoveredCapacityCache *cache.Cache,
	ec2api sdk.EC2API,
	subnetProvider subnet.Provider,
	pricingProvider pricing.Provider,
	capacityReservationProvider capacityreservation.Provider,
	placementGroupProvider placementgroup.Provider,
	unavailableOfferingsCache *awscache.UnavailableOfferings,
	instanceTypesResolver Resolver,
	zonalshiftProvider arczonalshift.Provider,
	kubeClient client.Client,
) *DefaultProvider {
	return &DefaultProvider{
		ec2api:                  ec2api,
		subnetProvider:          subnetProvider,
		instanceTypesInfo:       map[ec2types.InstanceType]ec2types.InstanceTypeInfo{},
		instanceTypesOfferings:  map[ec2types.InstanceType]sets.Set[string]{},
		instanceTypesResolver:   instanceTypesResolver,
		instanceTypesCache:      instanceTypesCache,
		discoveredCapacityCache: discoveredCapacityCache,
		cm:                      pretty.NewChangeMonitor(),
		placementGroupProvider:  placementGroupProvider,
		offeringProvider: offering.NewDefaultProvider(
			pricingProvider,
			capacityReservationProvider,
			placementGroupProvider,
			unavailableOfferingsCache,
			offeringCache,
			zonalshiftProvider,
			kubeClient,
		),
	}
}

//nolint:gocyclo
func (p *DefaultProvider) List(ctx context.Context, nodeClass NodeClass) ([]*cloudprovider.InstanceType, error) {
	p.muInstanceTypesInfo.RLock()
	p.muInstanceTypesOfferings.RLock()
	defer p.muInstanceTypesInfo.RUnlock()
	defer p.muInstanceTypesOfferings.RUnlock()

	if len(p.instanceTypesInfo) == 0 {
		return nil, fmt.Errorf("no instance types found")
	}
	if len(p.instanceTypesOfferings) == 0 {
		return nil, fmt.Errorf("no instance types offerings found")
	}
	if len(nodeClass.ZoneInfo()) == 0 {
		return nil, fmt.Errorf("no subnets found")
	}

	key := p.cacheKey(nodeClass)
	var instanceTypes []*cloudprovider.InstanceType
	if item, ok := p.instanceTypesCache.Get(key); ok {
		// Ensure what's returned from this function is a shallow-copy of the slice (not a deep-copy of the data itself)
		// so that modifications to the ordering of the data don't affect the original
		instanceTypes = item.([]*cloudprovider.InstanceType)
	} else {
		instanceTypes = lo.FilterMapToSlice(p.instanceTypesInfo, func(name ec2types.InstanceType, info ec2types.InstanceTypeInfo) (*cloudprovider.InstanceType, bool) {
			it, err := p.get(ctx, nodeClass, name)
			if err != nil {
				return nil, false
			}
			return it, true
		})
		p.instanceTypesCache.SetDefault(key, instanceTypes)
	}
	// Offerings aren't cached along with the rest of the instance type info because reserved offerings need to have up to
	// date capacity information. Rather than incurring a cache miss each time an instance is launched into a reserved
	// offering (or terminated), offerings are injected to the cached instance types on each call. Note that on-demand and
	// spot offerings are still cached - only reserved offerings are generated each time.
	return p.offeringProvider.InjectOfferings(
		ctx,
		instanceTypes,
		p.instanceTypesInfo,
		nodeClass,
		p.allZones,
	), nil
}

func (p *DefaultProvider) Get(ctx context.Context, nodeClass NodeClass, name ec2types.InstanceType) (*cloudprovider.InstanceType, error) {
	p.muInstanceTypesInfo.RLock()
	p.muInstanceTypesOfferings.RLock()
	defer p.muInstanceTypesInfo.RUnlock()
	defer p.muInstanceTypesOfferings.RUnlock()

	if len(p.instanceTypesInfo) == 0 {
		return nil, fmt.Errorf("no instance types found")
	}
	if len(p.instanceTypesOfferings) == 0 {
		return nil, fmt.Errorf("no instance types offerings found")
	}
	if len(nodeClass.ZoneInfo()) == 0 {
		return nil, fmt.Errorf("no subnets found")
	}
	var instanceType *cloudprovider.InstanceType
	if item, ok := p.instanceTypesCache.Get(p.cacheKey(nodeClass)); ok {
		instanceType, _ = lo.Find(item.([]*cloudprovider.InstanceType), func(i *cloudprovider.InstanceType) bool {
			return ec2types.InstanceType(i.Name) == name
		})
	}
	if instanceType == nil {
		var err error
		instanceType, err = p.get(ctx, nodeClass, name)
		if err != nil {
			return nil, err
		}
	}
	return p.offeringProvider.InjectOfferings(ctx, []*cloudprovider.InstanceType{instanceType}, p.instanceTypesInfo, nodeClass, p.allZones)[0], nil
}

func (p *DefaultProvider) get(ctx context.Context, nodeClass NodeClass, name ec2types.InstanceType) (*cloudprovider.InstanceType, error) {
	info, ok := p.instanceTypesInfo[name]
	if !ok {
		return nil, fmt.Errorf("instance type %s not found in cache", name)
	}
	it := p.instanceTypesResolver.Resolve(ctx, info, p.instanceTypesOfferings[info.InstanceType].UnsortedList(), nodeClass)
	if it == nil {
		return nil, fmt.Errorf("failed to generate instance type %s", name)
	}
	if cached, ok := p.discoveredCapacityCache.Get(discoveredCapacityCacheKey(it.Name, nodeClass)); ok {
		it.Capacity[corev1.ResourceMemory] = cached.(resource.Quantity)
	}
	InstanceTypeVCPU.Set(float64(lo.FromPtr(info.VCpuInfo.DefaultVCpus)), map[string]string{
		instanceTypeLabel: string(info.InstanceType),
	})
	InstanceTypeMemory.Set(float64(lo.FromPtr(info.MemoryInfo.SizeInMiB)*1024*1024), map[string]string{
		instanceTypeLabel: string(info.InstanceType),
	})
	return it, nil
}

// ENILimits returns the default ENI count and IPv4-addresses-per-ENI for an instance type, sourced from
// the live EC2 network info (DescribeInstanceTypes) held in the provider's cache. It returns ok=false when
// the instance type isn't present (e.g. the cache hasn't hydrated yet). This backs the amifamily resolver's
// CEL evaluation so that kubeReserved expressions in the launch template use the same live ENI values as the
// scheduler's reserved-capacity overhead calculation.
func (p *DefaultProvider) ENILimits(name string) (amifamily.ENILimits, bool) {
	p.muInstanceTypesInfo.RLock()
	defer p.muInstanceTypesInfo.RUnlock()

	info, ok := p.instanceTypesInfo[ec2types.InstanceType(name)]
	if !ok {
		return amifamily.ENILimits{}, false
	}
	defaultENIs, ipsPerENI := extractENILimits(info)
	return amifamily.ENILimits{DefaultENIs: int(defaultENIs), IPv4PerENI: int(ipsPerENI)}, true
}

// ValidateKubeletExpressions evaluates the NodeClass' kubelet CEL expressions against every known instance type.
// It returns the first evaluation failure encountered so the caller can surface it on the NodeClass status,
// rather than silently misconfiguring nodes at resolution time.
func (p *DefaultProvider) ValidateKubeletExpressions(ctx context.Context, nodeClass NodeClass) error {
	kc := nodeClass.KubeletConfiguration()
	if kc == nil {
		return nil
	}
	p.muInstanceTypesInfo.RLock()
	defer p.muInstanceTypesInfo.RUnlock()

	if len(p.instanceTypesInfo) == 0 {
		return fmt.Errorf("no instance types found")
	}
	for _, info := range p.instanceTypesInfo {
		if err := EvaluateKubeletExpressions(ctx, info, kc, nodeClass.NetworkInterfaces()); err != nil {
			return err
		}
	}
	return nil
}

func (p *DefaultProvider) cacheKey(nodeClass NodeClass) string {
	type zonePair struct {
		Zone   string
		ZoneID string
	}
	subnetZonesHash, _ := hashstructure.Hash(
		lo.UniqBy(nodeClass.ZoneInfo(), func(i v1.ZoneInfo) zonePair {
			return zonePair{Zone: i.Zone, ZoneID: i.ZoneID}
		}),
		hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true},
	)
	// Compute hash key against node class AMIs (used to force cache rebuild when AMIs change)
	amiHash, _ := hashstructure.Hash(nodeClass.AMIs(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	return fmt.Sprintf("%016x-%016x-%s",
		amiHash,
		subnetZonesHash,
		p.instanceTypesResolver.CacheKey(nodeClass),
	)
}

func (p *DefaultProvider) UpdateInstanceTypes(ctx context.Context) error {
	// DO NOT REMOVE THIS LOCK ----------------------------------------------------------------------------
	// We lock here so that multiple callers to getInstanceTypeOfferings do not result in cache misses and multiple
	// calls to EC2 when we could have just made one call.
	p.muInstanceTypesInfo.Lock()
	defer p.muInstanceTypesInfo.Unlock()

	var instanceTypes []ec2types.InstanceTypeInfo
	paginator := ec2.NewDescribeInstanceTypesPaginator(p.ec2api, &ec2.DescribeInstanceTypesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("supported-virtualization-type"),
				Values: []string{"hvm"},
			},
			{
				Name:   aws.String("processor-info.supported-architecture"),
				Values: []string{"x86_64", "arm64"},
			},
		},
		// MaxResults for DescribeInstanceTypes is capped at 100
		MaxResults: lo.ToPtr[int32](100),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing instance types, %w", err)
		}
		instanceTypes = append(instanceTypes, page.InstanceTypes...)
	}

	if p.cm.HasChanged("instance-types", instanceTypes) {
		// Only update instanceTypesSeqNun with the instance types have been changed
		// This is to not create new keys with duplicate instance types option
		p.instanceTypesCache.Flush() // None of the cached instance type info is valid when the instance type info changes
		log.FromContext(ctx).WithValues("count", len(instanceTypes)).V(1).Info("discovered instance types")
	}
	p.instanceTypesInfo = lo.SliceToMap(instanceTypes, func(i ec2types.InstanceTypeInfo) (ec2types.InstanceType, ec2types.InstanceTypeInfo) {
		return i.InstanceType, i
	})
	return nil
}

func (p *DefaultProvider) UpdateInstanceTypeOfferings(ctx context.Context) error {
	// DO NOT REMOVE THIS LOCK ----------------------------------------------------------------------------
	// We lock here so that multiple callers to GetInstanceTypes do not result in cache misses and multiple
	// calls to EC2 when we could have just made one call. This lock is here because multiple callers to EC2 result
	// in A LOT of extra memory generated from the response for simultaneous callers.
	// TODO @joinnis: This can be made more efficient by holding a Read lock and only obtaining the Write if not in cache
	p.muInstanceTypesOfferings.Lock()
	defer p.muInstanceTypesOfferings.Unlock()

	// Get offerings from EC2
	instanceTypeOfferings := map[ec2types.InstanceType]sets.Set[string]{}

	paginator := ec2.NewDescribeInstanceTypeOfferingsPaginator(p.ec2api, &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: ec2types.LocationTypeAvailabilityZone,
		// MaxResults for DescribeInstanceTypeOfferings is capped at 1000
		MaxResults: lo.ToPtr[int32](1000),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing instance type zone offerings, %w", err)
		}

		for _, offering := range page.InstanceTypeOfferings {
			if _, ok := instanceTypeOfferings[offering.InstanceType]; !ok {
				instanceTypeOfferings[offering.InstanceType] = sets.New[string]()
			}
			instanceTypeOfferings[offering.InstanceType].Insert(lo.FromPtr(offering.Location))
		}
	}

	if p.cm.HasChanged("instance-type-offering", instanceTypeOfferings) {
		// Only update instanceTypesSeqNun with the instance type offerings  have been changed
		// This is to not create new keys with duplicate instance type offerings option
		p.instanceTypesCache.Flush() // None of the cached instance type info is valid when the instance type offerings info changes
		log.FromContext(ctx).WithValues("instance-type-count", len(instanceTypeOfferings)).V(1).Info("discovered offerings for instance types")
	}
	p.instanceTypesOfferings = instanceTypeOfferings

	allZones := sets.New[string]()
	for _, offeringZones := range instanceTypeOfferings {
		for zone := range offeringZones {
			allZones.Insert(zone)
		}
	}
	if p.cm.HasChanged("zones", allZones) {
		log.FromContext(ctx).WithValues("zones", allZones.UnsortedList()).V(1).Info("discovered zones")
	}
	p.allZones = allZones
	return nil
}

func (p *DefaultProvider) UpdateInstanceTypeCapacityFromNode(ctx context.Context, node *corev1.Node, nodeClaim *karpv1.NodeClaim, nodeClass NodeClass) error {
	// Get mappings for most recent AMIs
	instanceTypeName := node.Labels[corev1.LabelInstanceTypeStable]
	amiMap := amifamily.MapToInstanceTypes([]*cloudprovider.InstanceType{{
		Name:         instanceTypeName,
		Requirements: scheduling.NewLabelRequirements(node.Labels),
	}}, nodeClass.AMIs())
	// Ensure NodeClaim AMI is current
	if !lo.ContainsBy(amiMap[nodeClaim.Status.ImageID], func(i *cloudprovider.InstanceType) bool {
		return i.Name == instanceTypeName
	}) {
		return nil
	}

	key := discoveredCapacityCacheKey(instanceTypeName, nodeClass)
	actualCapacity := node.Status.Capacity.Memory()
	if cachedCapacity, ok := p.discoveredCapacityCache.Get(key); !ok || actualCapacity.Cmp(cachedCapacity.(resource.Quantity)) < 1 {
		// Update the capacity in the cache if it is less than or equal to the current cached capacity. We update when it's equal to refresh the TTL.
		p.discoveredCapacityCache.SetDefault(key, *actualCapacity)
		// Only log if we haven't discovered the capacity for the instance type yet or the discovered capacity is **less** than the cached capacity
		if !ok || actualCapacity.Cmp(cachedCapacity.(resource.Quantity)) < 0 {
			log.FromContext(ctx).WithValues("memory-capacity", actualCapacity, "instance-type", instanceTypeName).V(1).Info("updating discovered capacity cache")
		}
	}
	return nil
}

func (p *DefaultProvider) FilterForNodeClass(ctx context.Context, its []*cloudprovider.InstanceType, nodeClass NodeClass) []*cloudprovider.InstanceType {
	p.muInstanceTypesInfo.RLock()
	defer p.muInstanceTypesInfo.RUnlock()
	// Resolve the placement group for compatibility checking
	var pg *placementgroup.PlacementGroup
	if nodeClass.PlacementGroupSelector() != nil {
		pg, _ = p.placementGroupProvider.Get(ctx, nodeClass)
	}
	compatible := []*cloudprovider.InstanceType{}
	for _, it := range its {
		info, found := p.instanceTypesInfo[ec2types.InstanceType(it.Name)]
		if found && compatibility.IsCompatibleWithNodeClass(info, nodeClass, pg) {
			compatible = append(compatible, it)
		}
	}
	return compatible
}

func (p *DefaultProvider) Reset() {
	p.instanceTypesInfo = map[ec2types.InstanceType]ec2types.InstanceTypeInfo{}
	p.instanceTypesOfferings = map[ec2types.InstanceType]sets.Set[string]{}
	p.instanceTypesCache.Flush()
	p.discoveredCapacityCache.Flush()
	p.offeringProvider.ResetOverlayPrices()
}

func discoveredCapacityCacheKey(instanceType string, nodeClass NodeClass) string {
	amiHash, _ := hashstructure.Hash(nodeClass.AMIs(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	return fmt.Sprintf("%s-%016x", instanceType, amiHash)
}
