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

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	arczonalshiftProvider "github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/compatibility"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

// ReservedCapacityResolver appends reserved capacity offerings for instance types
// that match capacity reservations defined on the NodeClass.
type ReservedCapacityResolver struct {
	PricingProvider             pricing.Provider
	CapacityReservationProvider capacityreservation.Provider
	ZonalshiftProvider          arczonalshiftProvider.Provider
}

func (r *ReservedCapacityResolver) ResolveOfferings(
	ctx context.Context,
	it *cloudprovider.InstanceType,
	offerings cloudprovider.Offerings,
	resolverCtx *OfferingResolverContext,
) cloudprovider.Offerings {
	if !options.FromContext(ctx).FeatureGates.ReservedCapacity {
		return offerings
	}

	itZones := sets.New(it.Requirements.Get(corev1.LabelTopologyZone).Values()...)
	zoneInfo := resolverCtx.NodeClass.ZoneInfo()
	isCompatibleWithNodeClass := compatibility.IsCompatibleWithNodeClass(resolverCtx.InstanceTypeInfo, resolverCtx.NodeClass, resolverCtx.PlacementGroup)

	capacityReservations := resolverCtx.NodeClass.CapacityReservations()
	for i := range capacityReservations {
		if capacityReservations[i].InstanceType != it.Name {
			continue
		}
		reservation := &capacityReservations[i]
		price := 0.0
		if odPrice, ok := r.PricingProvider.OnDemandPrice(ec2types.InstanceType(it.Name)); ok {
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
			isZonalShifted = r.ZonalshiftProvider.IsZonalShifted(ctx, zonalInfo.ZoneID)
		}
		reservationCapacity := r.CapacityReservationProvider.GetAvailableInstanceCount(reservation.ID)
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
	return offerings
}
