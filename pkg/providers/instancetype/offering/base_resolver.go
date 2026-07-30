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
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/compatibility"
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
	nodeClass NodeClass,
	allZones sets.Set[string],
	shiftedZones sets.Set[string],
	pg *placementgroup.PlacementGroup,
) cloudprovider.Offerings {
	itZones := sets.New(it.Requirements.Get(corev1.LabelTopologyZone).Values()...)
	zoneInfo := nodeClass.ZoneInfo()
	// Not all instance types are compatible with the NodeClass.
	// In the event it is not, we mark the offering as unavailable.
	isCompatibleWithNodeClass := compatibility.IsCompatibleWithNodeClass(instanceTypeInfo, nodeClass, pg)

	// If the sequence number has changed for the unavailable offerings, we know that we can't use the previously cached value
	lastSeqNum, ok := r.LastUnavailableOfferingsSeqNum.Load(ec2types.InstanceType(it.Name))
	if !ok {
		lastSeqNum = 0
	}
	seqNum := r.UnavailableOfferings.SeqNum(ec2types.InstanceType(it.Name))
	if ofs, ok := r.Cache.Get(cacheKeyFromInstanceType(it, nodeClass, shiftedZones)); ok && lastSeqNum == seqNum {
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
				isZonalShifted = r.ZonalshiftProvider.IsZonalShifted(ctx, zonalInfo.ZoneID)
			}
			for _, capacityType := range it.Requirements.Get(karpv1.CapacityTypeLabelKey).Values() {
				// Reserved capacity types are constructed separately
				if capacityType == karpv1.CapacityTypeReserved {
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
				}
				if zonefound {
					offering.Requirements.Add(scheduling.NewRequirement(v1.LabelTopologyZoneID, corev1.NodeSelectorOpIn, zonalInfo.ZoneID))
				}
				cachedOfferings = append(cachedOfferings, offering)
			}
		}
		r.Cache.SetDefault(cacheKeyFromInstanceType(it, nodeClass, shiftedZones), cachedOfferings)
		r.LastUnavailableOfferingsSeqNum.Store(ec2types.InstanceType(it.Name), seqNum)
		offerings = append(offerings, cachedOfferings...)
	}
	return offerings
}

// cacheKeyFromInstanceType generates a cache key based on instance type, node class, and shifted zones.
func cacheKeyFromInstanceType(it *cloudprovider.InstanceType, nodeClass NodeClass, shiftedZones sets.Set[string]) string {
	zonesHash, _ := hashstructure.Hash(
		it.Requirements.Get(corev1.LabelTopologyZone).Values(),
		hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true},
	)
	capacityTypesHash, _ := hashstructure.Hash(
		it.Requirements.Get(karpv1.CapacityTypeLabelKey).Values(),
		hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true},
	)
	networkInterfaceHash, _ := hashstructure.Hash(nodeClass.NetworkInterfaces(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	subnetsHash, _ := hashstructure.Hash(
		lo.Reduce(nodeClass.ZoneInfo(), func(agg []string, i v1.ZoneInfo, _ int) []string {
			return append(agg, i.SubnetIDs...)
		}, []string{}),
		hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true},
	)
	placementGroupPartitionsHash, _ := hashstructure.Hash(
		it.Requirements.Get(v1.LabelPlacementGroupPartition).Values(),
		hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true},
	)
	shiftedZonesHash, _ := hashstructure.Hash(
		shiftedZones,
		hashstructure.FormatV2,
		&hashstructure.HashOptions{SlicesAsSets: true},
	)

	connectionTrackingHash, _ := hashstructure.Hash(nodeClass.ConnectionTracking() != nil, hashstructure.FormatV2, nil)

	return fmt.Sprintf(
		"%s-%016x-%016x-%016x-%016x-%016x-%016x-%016x",
		it.Name,
		zonesHash,
		capacityTypesHash,
		networkInterfaceHash,
		subnetsHash,
		placementGroupPartitionsHash,
		shiftedZonesHash,
		connectionTrackingHash,
	)
}
