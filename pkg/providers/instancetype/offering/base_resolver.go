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

package offering

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	arczonalshiftProvider "github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"

	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

// BaseResolver resolves base offerings (on-demand and spot) with caching.
// It is the first resolver in the chain and generates offerings from scratch.
type BaseResolver struct {
	PricingProvider                pricing.Provider
	UnavailableOfferings           *awscache.UnavailableOfferings
	LastUnavailableOfferingsSeqNum *sync.Map // instance type -> seqNum
	Cache                          *cache.Cache
	ZonalshiftProvider             arczonalshiftProvider.Provider
	GetOverlayPrice                func(ctx context.Context, instanceTypeName string) (float64, bool)
}

//nolint:gocyclo
func (r *BaseResolver) ResolveOfferings(
	ctx context.Context,
	it *cloudprovider.InstanceType,
	offerings cloudprovider.Offerings,
	instanceTypeInfo ec2types.InstanceTypeInfo,
	_ NodeClass,
	allZones sets.Set[string],
	_ sets.Set[string],
	pg *placementgroup.PlacementGroup,
	rc *ResolveContext,
) cloudprovider.Offerings {
	// ZoneInfo and the cache-key builder are pre-computed once per InjectOfferings call and passed
	// in via ResolveContext, since they're constant across instance types and expensive to recompute.
	keyBuilder := rc.cacheKeyBuilder

	// Values() allocates, so read each requirement once.
	itZoneValues := it.Requirements.Get(corev1.LabelTopologyZone).Values()
	capacityTypeValues := it.Requirements.Get(karpv1.CapacityTypeLabelKey).Values()

	// If the sequence number has changed for the unavailable offerings, we know that we can't use the previously cached value
	lastSeqNum, ok := r.LastUnavailableOfferingsSeqNum.Load(ec2types.InstanceType(it.Name))
	if !ok {
		lastSeqNum = 0
	}
	seqNum := r.UnavailableOfferings.SeqNum(ec2types.InstanceType(it.Name))
	cacheKey := keyBuilder.cacheKeyFromInstanceType(it, itZoneValues, capacityTypeValues)
	if ofs, ok := r.Cache.Get(cacheKey); ok && lastSeqNum == seqNum {
		return append(offerings, ofs.([]*cloudprovider.Offering)...)
	}
	itZones := sets.New(itZoneValues...)
	// Not all instance types are compatible with the NodeClass.
	// In the event it is not, we mark the offering as unavailable.
	isCompatibleWithNodeClass := rc.Compatibility.IsCompatible(instanceTypeInfo)
	var pgOpts []awscache.UnavailableOfferingsOption
	if pg != nil {
		pgOpts = append(pgOpts, awscache.WithPlacementGroup(pg.ID))
	}
	var cachedOfferings []*cloudprovider.Offering
	for zone := range allZones {
		var subnetIDs []string
		isZonalShifted := false
		zonalInfo, zonefound := rc.ZoneInfoByZone[zone]
		if zonefound {
			subnetIDs = zonalInfo.SubnetIDs
			isZonalShifted = r.ZonalshiftProvider.IsZonalShifted(ctx, zonalInfo.ZoneID)
		}
		for _, capacityType := range capacityTypeValues {
			// Reserved capacity types are constructed separately
			if capacityType == karpv1.CapacityTypeReserved {
				continue
			}
			// Additional capacity types registered by the cloud provider get their offerings from an additional resolver
			if _, builtIn := cloudprovider.DefaultCapacityTypeClasses[capacityType]; !builtIn && lo.Contains(cloudprovider.CapacityTypesOf(cloudprovider.CapacityTypeClassOnDemand), capacityType) {
				continue
			}
			// Check both the general ICE signal and the PG-scoped signal.
			// An offering is unavailable if either the general key or the PG-specific key is in the cache.
			isUnavailable := r.UnavailableOfferings.IsUnavailable(ec2types.InstanceType(it.Name), zone, subnetIDs, capacityType)
			if !isUnavailable && len(pgOpts) > 0 {
				isUnavailable = r.UnavailableOfferings.IsUnavailable(ec2types.InstanceType(it.Name), zone, subnetIDs, capacityType, pgOpts...)
			}
			var price float64
			var hasPrice bool
			switch capacityType {
			case karpv1.CapacityTypeOnDemand:
				price, hasPrice = r.PricingProvider.OnDemandPrice(ec2types.InstanceType(it.Name))
			case karpv1.CapacityTypeSpot:
				price, hasPrice = r.PricingProvider.SpotPrice(ec2types.InstanceType(it.Name), zone)
			default:
				panic(fmt.Sprintf("invalid capacity type %q in requirements for instance type %q", capacityType, it.Name))
			}
			if !hasPrice {
				if overlayPrice, ok := r.GetOverlayPrice(ctx, it.Name); ok {
					price = overlayPrice
					hasPrice = true
				}
			}
			offering := &cloudprovider.Offering{
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityType),
					scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, zone),
					scheduling.NewRequirement(cloudprovider.ReservationIDLabel, corev1.NodeSelectorOpDoesNotExist),
					scheduling.NewRequirement(v1.LabelCapacityReservationType, corev1.NodeSelectorOpDoesNotExist),
					scheduling.NewRequirement(v1.LabelCapacityReservationInterruptible, corev1.NodeSelectorOpDoesNotExist),
				),
				Price:     price,
				Available: isCompatibleWithNodeClass && !isUnavailable && hasPrice && itZones.Has(zone) && !isZonalShifted,
				// Only NodeClass incompatibility is permanent; ICE, a zonal shift and a missing price may all recover, so
				// they leave the offering Compatible. A zone the type isn't offered in (or has no subnet in) also leaves
				// it Compatible: that zone is already outside the instance type's zone requirement, so nothing waits on it.
				Compatible: isCompatibleWithNodeClass,
			}
			if zonefound {
				offering.Requirements.Add(scheduling.NewRequirement(v1.LabelTopologyZoneID, corev1.NodeSelectorOpIn, zonalInfo.ZoneID))
			}
			cachedOfferings = append(cachedOfferings, offering)
		}
	}
	r.Cache.SetDefault(cacheKey, cachedOfferings)
	r.LastUnavailableOfferingsSeqNum.Store(ec2types.InstanceType(it.Name), seqNum)
	return append(offerings, cachedOfferings...)
}

// cacheKeyBuilder pre-computes the NodeClass-level cache key components to avoid redundant hashing.
type cacheKeyBuilder struct {
	baseSuffix string // non-instance-type specific hashes
}

func newCacheKeyBuilder(nodeClass NodeClass, zoneInfo []v1.ZoneInfo, shiftedZones sets.Set[string], pg *placementgroup.PlacementGroup) *cacheKeyBuilder {
	networkInterfaceHash, _ := hashstructure.Hash(nodeClass.NetworkInterfaces(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	subnetsHash, _ := hashstructure.Hash(
		lo.Reduce(zoneInfo, func(agg []string, i v1.ZoneInfo, _ int) []string {
			return append(agg, i.SubnetIDs...)
		}, []string{}),
		hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true},
	)
	shiftedZonesHash, _ := hashstructure.Hash(
		shiftedZones,
		hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true},
	)
	connectionTrackingHash, _ := hashstructure.Hash(nodeClass.ConnectionTracking() != nil, hashstructure.FormatV2, nil)
	enclaveOptions := nodeClass.EnclaveOptions()
	enclavesEnabled := enclaveOptions != nil && enclaveOptions.Enabled
	enclavesEnabledHash, _ := hashstructure.Hash(enclavesEnabled, hashstructure.FormatV2, nil)
	cpuOptions := nodeClass.CPUOptions()
	nestedVirtualizationEnabled := cpuOptions != nil && lo.FromPtr(cpuOptions.NestedVirtualization) == "enabled"
	nestedVirtualizationEnabledHash, _ := hashstructure.Hash(nestedVirtualizationEnabled, hashstructure.FormatV2, nil)
	var placementGroupHash uint64
	if pg != nil {
		placementGroupHash, _ = hashstructure.Hash(pg.ID, hashstructure.FormatV2, nil)
	}
	// The AMI family affects compatibility (a1 is unsupported on AL2023), which the cached offerings carry.
	amiFamilyHash, _ := hashstructure.Hash(nodeClass.AMIFamily(), hashstructure.FormatV2, nil)

	baseHash, _ := hashstructure.Hash([]uint64{
		networkInterfaceHash,
		subnetsHash,
		shiftedZonesHash,
		connectionTrackingHash,
		enclavesEnabledHash,
		nestedVirtualizationEnabledHash,
		placementGroupHash,
		amiFamilyHash,
	}, hashstructure.FormatV2, nil)

	return &cacheKeyBuilder{
		baseSuffix: fmt.Sprintf("%016x", baseHash),
	}
}

// cacheKeyFromInstanceType writes the zone and capacity type values into the key literally; both sets are
// small, so this is cheaper than hashing them.
func (b *cacheKeyBuilder) cacheKeyFromInstanceType(it *cloudprovider.InstanceType, zoneValues, capacityTypeValues []string) string {
	var key strings.Builder
	key.Grow(len(it.Name) + valuesLen(zoneValues) + valuesLen(capacityTypeValues) + len(b.baseSuffix) + 1)
	key.WriteString(it.Name)
	writeSortedValues(&key, zoneValues)
	writeSortedValues(&key, capacityTypeValues)
	key.WriteByte(keySeparator)
	key.WriteString(b.baseSuffix)
	return key.String()
}

const (
	keySeparator   = '|'
	valueSeparator = ','
)

func valuesLen(values []string) int {
	n := 1
	for _, v := range values {
		n += len(v) + 1
	}
	return n
}

// writeSortedValues sorts values in place.
func writeSortedValues(key *strings.Builder, values []string) {
	slices.Sort(values)
	key.WriteByte(keySeparator)
	for i, v := range values {
		if i > 0 {
			key.WriteByte(valueSeparator)
		}
		key.WriteString(v)
	}
}
