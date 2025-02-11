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
	"runtime"
	"sync"
	"weak"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

type Provider interface {
	InjectOfferings(context.Context, []*cloudprovider.InstanceType, *v1.EC2NodeClass, []string, *types.UID) []*cloudprovider.InstanceType
}

type DefaultProvider struct {
	sync.RWMutex
	snapshots map[types.UID]weak.Pointer[snapshot]

	unavailableOfferings *awscache.UnavailableOfferings
	pricingProvider      pricing.Provider
}

func NewDefaultProvider(unavailableOfferingsCache *awscache.UnavailableOfferings, pricingProvider pricing.Provider) *DefaultProvider {
	return &DefaultProvider{
		snapshots:            map[types.UID]weak.Pointer[snapshot]{},
		unavailableOfferings: unavailableOfferingsCache,
		pricingProvider:      pricingProvider,
	}
}

func (p *DefaultProvider) InjectOfferings(
	instanceTypes []*cloudprovider.InstanceType,
	nodeClass *v1.EC2NodeClass,
	allZones sets.Set[string],
	snapshotUUID *types.UID,
) []*cloudprovider.InstanceType {
	subnetZones := lo.SliceToMap(nodeClass.Status.Subnets, func(s v1.Subnet) (string, string) {
		return s.Zone, s.ZoneID
	})
	its := []*cloudprovider.InstanceType{}
	for _, it := range instanceTypes {
		offerings := p.createOfferings(
			it,
			nodeClass,
			allZones,
			subnetZones,
			snapshotUUID,
		)
		for _, of := range offerings {
			InstanceTypeOfferingAvailable.Set(float64(lo.Ternary(of.Available, 1, 0)), map[string]string{
				instanceTypeLabel: it.Name,
				capacityTypeLabel: of.Requirements.Get(karpv1.CapacityTypeLabelKey).Any(),
				zoneLabel:         of.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
			InstanceTypeOfferingPriceEstimate.Set(of.Price, map[string]string{
				instanceTypeLabel: it.Name,
				capacityTypeLabel: of.Requirements.Get(karpv1.CapacityTypeLabelKey).Any(),
				zoneLabel:         of.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
		}

		its = append(its, &cloudprovider.InstanceType{
			Name:         it.Name,
			Requirements: it.Requirements,
			Offerings:    offerings,
			Capacity:     it.Capacity,
			Overhead:     it.Overhead,
		})
	}
	return its
}

//nolint:gocyclo
func (p *DefaultProvider) createOfferings(
	it *cloudprovider.InstanceType,
	nodeClass *v1.EC2NodeClass,
	allZones sets.Set[string],
	subnetZones map[string]string,
	snapshotUUID *types.UID,
) cloudprovider.Offerings {
	itZones := sets.New(it.Requirements.Get(corev1.LabelTopologyZone).Values()...)

	offerings := []cloudprovider.Offering{}
	for zone := range allZones {
		for _, capacityType := range it.Requirements.Get(karpv1.CapacityTypeLabelKey).Values() {
			// Reserved capacity types are constructed separately, skip them for now.
			if capacityType == karpv1.CapacityTypeReserved {
				continue
			}

			isUnavailable := p.unavailableOfferings.IsUnavailable(it.Name, zone, capacityType)
			_, hasSubnetZone := subnetZones[zone]
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
			offering := cloudprovider.Offering{
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityType),
					scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, zone),
					scheduling.NewRequirement(v1.LabelCapacityReservationID, corev1.NodeSelectorOpDoesNotExist),
				),
				Price:     price,
				Available: !isUnavailable && hasPrice && itZones.Has(zone) && hasSubnetZone,
			}
			if id, ok := subnetZones[zone]; ok {
				offering.Requirements.Add(scheduling.NewRequirement(v1.LabelTopologyZoneID, corev1.NodeSelectorOpIn, id))
			}
			offerings = append(offerings, offering)
		}
	}

	for i := range nodeClass.Status.CapacityReservations {
		if nodeClass.Status.CapacityReservations[i].InstanceType != it.Name {
			continue
		}
		reservation := &nodeClass.Status.CapacityReservations[i]

		isUnavailable := p.unavailableOfferings.IsReservationUnavailable(reservation.ID)
		_, hasSubnetZone := subnetZones[reservation.AvailabilityZone]
		price := 0.0
		if odPrice, ok := p.pricingProvider.OnDemandPrice(ec2types.InstanceType(it.Name)); ok {
			// Divide the on-demand price by a sufficiently large constant. This allows us to treat the reservation as "free",
			// while maintaining relative ordering for consolidation. If the pricing details are unavailable for whatever reason,
			// still succeed to create the offering and leave the price at zero. This will break consolidation, but will allow
			// users to utilize the instances they're already paying for.
			price = odPrice / 100_000.0
		}
		offering := cloudprovider.Offering{
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeReserved),
				scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, reservation.AvailabilityZone),
				scheduling.NewRequirement(v1.LabelCapacityReservationID, corev1.NodeSelectorOpIn, reservation.ID),
			),
			Price:     price,
			Available: !isUnavailable && itZones.Has(reservation.AvailabilityZone) && hasSubnetZone,
			// TODO: Retrieve available instance count from an in-memory store accounting for launched and terminated instances
			// in real time, rather than dealing with eventual consistency delays with the NodeClass status.
			ReservationManager: p.createReservationManager(reservation.ID, reservation.AvailableInstanceCount, snapshotUUID),
		}
		if id, ok := subnetZones[reservation.AvailabilityZone]; ok {
			offering.Requirements.Add(scheduling.NewRequirement(v1.LabelTopologyZoneID, corev1.NodeSelectorOpIn, id))
		}
		offerings = append(offerings, offering)
	}
	return cloudprovider.Offerings(offerings)
}

func (p *DefaultProvider) createReservationManager(capacityReservationID string, availableCount int, snapshotUUID *types.UID) reservationManagerAdapter {
	snapshot := p.getSnapshot(lo.FromPtrOr(snapshotUUID, uuid.NewUUID()))
	if value, ok := snapshot.availability[capacityReservationID]; !ok || value < availableCount {
		snapshot.availability[capacityReservationID] = availableCount
	}
	return reservationManagerAdapter{
		snapshot:              snapshot,
		capacityReservationID: capacityReservationID,
	}
}

func (p *DefaultProvider) getSnapshot(snapshotUUID types.UID) *snapshot {
	s := func() *snapshot {
		p.RLock()
		defer p.RUnlock()
		return p.snapshots[snapshotUUID].Value()
	}()
	if s != nil {
		return s
	}

	p.Lock()
	defer p.Unlock()
	// Double check the snaphot entries now that we've obtained the write-lock. This prevents a race where multiple reads
	// occur before the write is committed. Note that this shouldn't occur in practice because a single thread should only
	// ever access a given snapshot.
	if val := p.snapshots[snapshotUUID].Value(); val != nil {
		return val
	}

	s = &snapshot{
		availability: map[string]int{},
		reservations: map[string]sets.Set[string]{},
	}
	// Once the snapshot has been garbage collected, ensure its entry from the snapshot map is removed as well. This
	// ensures the snapshot map doesn't grow unbounded.
	type providerWithToken struct {
		*DefaultProvider
		uuid types.UID
	}
	runtime.AddCleanup(s, func(p providerWithToken) {
		// Since cleanup functions run sequentially in a single goroutine, we don't want to block other cleanup operations
		// on lock acquisition
		go func() {
			p.Lock()
			defer p.Unlock()
			delete(p.snapshots, p.uuid)
		}()
	}, providerWithToken{
		DefaultProvider: p,
		uuid:            snapshotUUID,
	})
	p.snapshots[snapshotUUID] = weak.Make(s)
	return s
}

// snapshot is a point in time representation of offering availability. When tracking offering availability across
// multiple NodePools, the same snapshot should be used to ensure consistency when checking availability wrt
// reservations. This is motivated by on-demand capacity reservations shared between multiple NodePools.
type snapshot struct {
	availability map[string]int              // capacity-reservation-id -> int
	reservations map[string]sets.Set[string] // reservation-id -> set[capacity-reservation-id]
}

type reservationManagerAdapter struct {
	*snapshot
	capacityReservationID string
}

func (os reservationManagerAdapter) Reserve(reservationID string) bool {
	reservations, ok := os.reservations[reservationID]
	if !ok {
		reservations = sets.New[string]()
		os.reservations[reservationID] = reservations
	}
	if reservations.Has(os.capacityReservationID) {
		return true
	}
	if os.availability[os.capacityReservationID] <= 0 {
		return false
	}
	os.availability[os.capacityReservationID] -= 1
	reservations.Insert(os.capacityReservationID)
	return true
}

func (os reservationManagerAdapter) Release(reservationID string) {
	if reservations, ok := os.reservations[reservationID]; ok && reservations.Has(os.capacityReservationID) {
		os.availability[os.capacityReservationID] += 1
		reservations.Delete(os.capacityReservationID)
	}
}
