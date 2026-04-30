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
	"strconv"
	"sync"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	arczonalshiftProvider "github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/compatibility"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

type Provider interface {
	InjectOfferings(context.Context, []*cloudprovider.InstanceType, *v1.EC2NodeClass, []string) []*cloudprovider.InstanceType
}

type NodeClass interface {
	client.Object
	CapacityReservations() []v1.CapacityReservation
	ZoneInfo() []v1.ZoneInfo
	NetworkInterfaces() []*v1.NetworkInterface
	AMIFamily() string
	PlacementGroupSelector() *v1.PlacementGroupSelector
}

type DefaultProvider struct {
	pricingProvider                pricing.Provider
	capacityReservationProvider    capacityreservation.Provider
	zonalshiftProvider             arczonalshiftProvider.Provider
	placementGroupProvider         placementgroup.Provider
	unavailableOfferings           *awscache.UnavailableOfferings
	lastUnavailableOfferingsSeqNum sync.Map // instance type -> seqNum
	cache                          *cache.Cache
}

func NewDefaultProvider(
	pricingProvider pricing.Provider,
	capacityReservationProvider capacityreservation.Provider,
	placementGroupProvider placementgroup.Provider,
	unavailableOfferingsCache *awscache.UnavailableOfferings,
	offeringCache *cache.Cache,
	zonalshiftProvider arczonalshiftProvider.Provider,
) *DefaultProvider {
	return &DefaultProvider{
		pricingProvider:             pricingProvider,
		capacityReservationProvider: capacityReservationProvider,
		placementGroupProvider:      placementGroupProvider,
		unavailableOfferings:        unavailableOfferingsCache,
		cache:                       offeringCache,
		zonalshiftProvider:          zonalshiftProvider,
	}
}

func (p *DefaultProvider) InjectOfferings(
	ctx context.Context,
	instanceTypes []*cloudprovider.InstanceType,
	instanceTypeInfo map[ec2types.InstanceType]ec2types.InstanceTypeInfo,
	nodeClass NodeClass,
	allZones sets.Set[string],
) []*cloudprovider.InstanceType {
	// Resolve the placement group once and pass it through to avoid repeated type assertions and lookups
	var pg *placementgroup.PlacementGroup
	if nodeClass.PlacementGroupSelector() != nil {
		pg, _ = p.placementGroupProvider.Get(ctx, nodeClass)
	}

	cacheKeyBuilder := newCacheKeyBuilder().
		withNetworkInterfaces(nodeClass.NetworkInterfaces()).
		withPlacementGroup(pg)

	var its []*cloudprovider.InstanceType
	for _, it := range instanceTypes {
		info := instanceTypeInfo[ec2types.InstanceType(it.Name)]
		offerings := p.createOfferings(
			ctx,
			it,
			info,
			nodeClass,
			pg,
			allZones,
			cacheKeyBuilder,
		)
		// For partition placement groups, expand each offering into N offerings (one per partition)
		// These expanded offerings are not cached, but the underlying offerings are.
		offerings = p.expandPartitionOfferings(it, offerings, nodeClass.ZoneInfo(), pg)
		// NOTE: By making this copy one level deep, we can modify the offerings without mutating the results from previous
		// GetInstanceTypes calls. This should still be done with caution - it is currently done here in the provider, and
		// once in the instance provider (filterReservedInstanceTypes)

		// Inject placement group requirements into instance type requirements so the scheduler
		// can discover partition topology domains for TopologySpreadConstraints.
		reqs := it.Requirements
		if pg != nil {
			reqs = scheduling.NewRequirements(it.Requirements.Values()...)
			reqs.Add(scheduling.NewRequirement(v1.LabelPlacementGroupID, corev1.NodeSelectorOpIn, pg.ID))
			if pg.Strategy == placementgroup.StrategyPartition && pg.PartitionCount > 0 {
				partitions := make([]string, pg.PartitionCount)
				for i := int32(0); i < pg.PartitionCount; i++ {
					partitions[i] = fmt.Sprintf("%d", i+1)
				}
				reqs.Add(scheduling.NewRequirement(v1.LabelPlacementGroupPartition, corev1.NodeSelectorOpIn, partitions...))
			}
		}
		its = append(its, &cloudprovider.InstanceType{
			Name:         it.Name,
			Requirements: reqs,
			Offerings:    offerings,
			Capacity:     it.Capacity,
			Overhead:     it.Overhead,
		})
	}
	return its
}

//nolint:gocyclo
func (p *DefaultProvider) createOfferings(
	ctx context.Context,
	it *cloudprovider.InstanceType,
	info ec2types.InstanceTypeInfo,
	nodeClass NodeClass,
	pg *placementgroup.PlacementGroup,
	allZones sets.Set[string],
	cacheKeyBuilder *cacheKeyBuilder,
) cloudprovider.Offerings {
	var offerings []*cloudprovider.Offering
	itZones := sets.New(it.Requirements.Get(corev1.LabelTopologyZone).Values()...)
	zoneInfo := nodeClass.ZoneInfo()

	// Build the capacity types key based on what's present
	// This is computed once and passed to cacheKeyFromInstanceType to avoid redundant work
	capacityTypes := it.Requirements.Get(karpv1.CapacityTypeLabelKey).Values()
	var ctKey capacityTypesKey
	for _, ct := range capacityTypes {
		switch ct {
		case karpv1.CapacityTypeOnDemand:
			ctKey.hasOnDemand = true
		case karpv1.CapacityTypeSpot:
			ctKey.hasSpot = true
		}
	}

	// Not all instance types are compatible with the NodeClass.
	// In the event it is not, we mark the offering as unavailable.
	isCompatibleWithNodeClass := compatibility.IsCompatibleWithNodeClass(info, nodeClass, pg)

	// If the sequence number has changed for the unavailable offerings, we know that we can't use the previously cached value
	lastSeqNum, ok := p.lastUnavailableOfferingsSeqNum.Load(ec2types.InstanceType(it.Name))
	if !ok {
		lastSeqNum = 0
	}
	seqNum := p.unavailableOfferings.SeqNum(ec2types.InstanceType(it.Name))
	if ofs, ok := p.cache.Get(cacheKeyBuilder.cacheKeyFromInstanceType(it, nodeClass, &ctKey)); ok && lastSeqNum == seqNum {
		offerings = append(offerings, ofs.([]*cloudprovider.Offering)...)
	} else {
		var pgOpts []awscache.UnavailableOfferingsOption
		if pg != nil {
			pgOpts = append(pgOpts, awscache.WithPlacementGroup(pg.ID))
		}
		var cachedOfferings []*cloudprovider.Offering
		for zone := range allZones {
			var subnetIDs []string
			isZonalShifted := false
			zonalInfo, zonefound := lo.Find(zoneInfo, func(i v1.ZoneInfo) bool {
				return i.Zone == zone
			})
			if zonefound {
				subnetIDs = zonalInfo.SubnetIDs
				isZonalShifted = p.zonalshiftProvider.IsZonalShifted(ctx, zonalInfo.ZoneID)
			}
			for _, capacityType := range capacityTypes {
				// Reserved capacity types are constructed separately
				if capacityType == karpv1.CapacityTypeReserved {
					continue
				}
				// Check both the general ICE signal and the PG-scoped signal.
				// An offering is unavailable if either the general key or the PG-specific key is in the cache.
				isUnavailable := p.unavailableOfferings.IsUnavailable(ec2types.InstanceType(it.Name), zone, subnetIDs, capacityType)
				if !isUnavailable && len(pgOpts) > 0 {
					isUnavailable = p.unavailableOfferings.IsUnavailable(ec2types.InstanceType(it.Name), zone, subnetIDs, capacityType, pgOpts...)
				}
				var price float64
				var hasPrice bool
				switch capacityType {
				case karpv1.CapacityTypeOnDemand:
					price, hasPrice = p.pricingProvider.OnDemandPrice(ec2types.InstanceType(it.Name))
				case karpv1.CapacityTypeSpot:
					price, hasPrice = p.pricingProvider.SpotPrice(ec2types.InstanceType(it.Name), zone)
				default:
					panic(fmt.Sprintf("invalid capacity type %q in requirements for instance type %q", capacityType, it.Name))
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
				}
				if zonefound {
					offering.Requirements.Add(scheduling.NewRequirement(v1.LabelTopologyZoneID, corev1.NodeSelectorOpIn, zonalInfo.ZoneID))
				}
				cachedOfferings = append(cachedOfferings, offering)
			}
		}
		p.cache.SetDefault(cacheKeyBuilder.cacheKeyFromInstanceType(it, nodeClass, &ctKey), cachedOfferings)
		p.lastUnavailableOfferingsSeqNum.Store(ec2types.InstanceType(it.Name), seqNum)
		offerings = append(offerings, cachedOfferings...)
	}
	if options.FromContext(ctx).FeatureGates.ReservedCapacity {
		capacityReservations := nodeClass.CapacityReservations()
		for i := range capacityReservations {
			if capacityReservations[i].InstanceType != it.Name {
				continue
			}
			reservation := &capacityReservations[i]
			price := 0.0
			if odPrice, ok := p.pricingProvider.OnDemandPrice(ec2types.InstanceType(it.Name)); ok {
				// Divide the on-demand price by a sufficiently large constant. This allows us to treat the reservation as "free",
				// while maintaining relative ordering for consolidation. If the pricing details are unavailable for whatever reason,
				// still succeed to create the offering and leave the price at zero. This will break consolidation, but will allow
				// users to utilize the instances they're already paying for.
				price = odPrice / 10_000_000.0
			}
			isZonalShifted := false
			zonalInfo, zoneFound := lo.Find(zoneInfo, func(i v1.ZoneInfo) bool {
				return i.Zone == reservation.AvailabilityZone
			})
			if zoneFound {
				isZonalShifted = p.zonalshiftProvider.IsZonalShifted(ctx, zonalInfo.ZoneID)
			}
			reservationCapacity := p.capacityReservationProvider.GetAvailableInstanceCount(reservation.ID)
			offering := &cloudprovider.Offering{
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeReserved),
					scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, reservation.AvailabilityZone),
					scheduling.NewRequirement(cloudprovider.ReservationIDLabel, corev1.NodeSelectorOpIn, reservation.ID),
					scheduling.NewRequirement(v1.LabelCapacityReservationType, corev1.NodeSelectorOpIn, string(reservation.ReservationType)),
					scheduling.NewRequirement(v1.LabelCapacityReservationInterruptible, corev1.NodeSelectorOpIn, fmt.Sprintf("%t", reservation.Interruptible)),
				),
				Price:               price,
				Available:           isCompatibleWithNodeClass && reservationCapacity != 0 && itZones.Has(reservation.AvailabilityZone) && reservation.State != v1.CapacityReservationStateExpiring && !isZonalShifted,
				ReservationCapacity: reservationCapacity,
			}
			if zoneFound {
				offering.Requirements.Add(scheduling.NewRequirement(v1.LabelTopologyZoneID, corev1.NodeSelectorOpIn, zonalInfo.ZoneID))
			}
			offerings = append(offerings, offering)
		}
	}
	return offerings
}

// expandPartitionOfferings expands each offering into N offerings (one per partition) for partition placement groups.
// This enables the scheduler to use TopologySpreadConstraints with the partition topology key.
func (p *DefaultProvider) expandPartitionOfferings(it *cloudprovider.InstanceType, offerings cloudprovider.Offerings, zoneInfo []v1.ZoneInfo, pg *placementgroup.PlacementGroup) cloudprovider.Offerings {
	if pg == nil || pg.Strategy != placementgroup.StrategyPartition {
		return offerings
	}
	partitionCount := int(pg.PartitionCount)
	if partitionCount <= 0 {
		return offerings
	}
	var expanded []*cloudprovider.Offering
	for _, offering := range offerings {
		capacityType := offering.Requirements.Get(karpv1.CapacityTypeLabelKey).Values()
		zone := offering.Requirements.Get(corev1.LabelTopologyZone).Values()
		if len(capacityType) != 1 {
			panic("Each offering should only have 1 capacity type")
		}
		if len(zone) != 1 {
			panic("Each offering should only have 1 zone")
		}
		zonalInfo, zonefound := lo.Find(zoneInfo, func(i v1.ZoneInfo) bool {
			return i.Zone == zone[0]
		})
		for partition := 1; partition <= partitionCount; partition++ {
			isUnavailable := true
			// if the underlying offering is not available, then the expanded offering won't be either
			if !offering.Available && zonefound {
				pgOpts := []awscache.UnavailableOfferingsOption{awscache.WithPlacementGroupPartition(strconv.Itoa(partition))}
				isUnavailable = p.unavailableOfferings.IsUnavailable(ec2types.InstanceType(it.Name), zone[0], zonalInfo.SubnetIDs, capacityType[0], pgOpts...)
			}
			reqs := scheduling.NewRequirements(offering.Requirements.Values()...)
			reqs.Add(scheduling.NewRequirement(v1.LabelPlacementGroupPartition, corev1.NodeSelectorOpIn, fmt.Sprintf("%d", partition)))
			expanded = append(expanded, &cloudprovider.Offering{
				Requirements:        reqs,
				Price:               offering.Price,
				Available:           offering.Available && isUnavailable,
				ReservationCapacity: offering.ReservationCapacity,
			})
		}
	}
	return expanded
}

// capacityTypesKey is used as a map key for pre-computed capacity type hashes.
type capacityTypesKey struct {
	hasOnDemand bool
	hasSpot     bool
}

// cacheKeyBuilder pre-computes some cache key components to avoid redundant hashing.
type cacheKeyBuilder struct {
	baseSuffix          string                      // non-instance-type specific hashes
	capacityTypesHashes map[capacityTypesKey]uint64 // capacity requirements -> hash
}

func newCacheKeyBuilder() *cacheKeyBuilder {
	b := &cacheKeyBuilder{
		capacityTypesHashes: make(map[capacityTypesKey]uint64),
	}
	for _, hasOnDemand := range []bool{false, true} {
		for _, hasSpot := range []bool{false, true} {
			key := capacityTypesKey{hasOnDemand: hasOnDemand, hasSpot: hasSpot}
			var capacityTypes []string
			if hasOnDemand {
				capacityTypes = append(capacityTypes, karpv1.CapacityTypeOnDemand)
			}
			if hasSpot {
				capacityTypes = append(capacityTypes, karpv1.CapacityTypeSpot)
			}
			hash, _ := hashstructure.Hash(
				capacityTypes,
				hashstructure.FormatV2,
				&hashstructure.HashOptions{SlicesAsSets: true},
			)
			b.capacityTypesHashes[key] = hash
		}
	}
	return b
}

func (b *cacheKeyBuilder) withNetworkInterfaces(networkInterfaces []*v1.NetworkInterface) *cacheKeyBuilder {
	h, _ := hashstructure.Hash(networkInterfaces, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	b.baseSuffix += fmt.Sprintf("-%016x", h)
	return b
}

func (b *cacheKeyBuilder) withPlacementGroup(pg *placementgroup.PlacementGroup) *cacheKeyBuilder {
	if pg == nil {
		return b
	}
	h, _ := hashstructure.Hash(pg.ID, hashstructure.FormatV2, nil)
	b.baseSuffix += fmt.Sprintf("-%016x", h)
	return b
}

func (b *cacheKeyBuilder) cacheKeyFromInstanceType(it *cloudprovider.InstanceType, nodeClass NodeClass, ctKey *capacityTypesKey) string {
	subnetsHash, _ := hashstructure.Hash(
		lo.Reduce(nodeClass.ZoneInfo(), func(agg []string, i v1.ZoneInfo, _ int) []string {
			return append(agg, i.SubnetIDs...)
		}, []string{}),
		hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true},
	)

	// Try to use pre-computed capacity types hash
	capacityTypesHash, ok := b.capacityTypesHashes[lo.FromPtr(ctKey)]
	if !ok {
		// we try to use the pre-computed capacity types hash but fallback
		// on unknown capacity type combinations
		capacityTypes := it.Requirements.Get(karpv1.CapacityTypeLabelKey).Values()
		capacityTypesHash, _ = hashstructure.Hash(
			capacityTypes,
			hashstructure.FormatV2,
			&hashstructure.HashOptions{SlicesAsSets: true},
		)
	}

	key := fmt.Sprintf(
		"%s-%016x-%016x-%s",
		it.Name,
		subnetsHash,
		capacityTypesHash,
		b.baseSuffix,
	)
	return key
}
