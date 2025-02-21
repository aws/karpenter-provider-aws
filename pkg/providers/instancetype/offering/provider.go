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
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

type Provider interface {
	InjectOfferings(context.Context, []*cloudprovider.InstanceType, *v1.EC2NodeClass, []string) []*cloudprovider.InstanceType
}

type DefaultProvider struct {
	unavailableOfferings        *awscache.UnavailableOfferings
	pricingProvider             pricing.Provider
	capacityReservationProvider capacityreservation.Provider
}

func NewDefaultProvider(unavailableOfferingsCache *awscache.UnavailableOfferings, pricingProvider pricing.Provider) *DefaultProvider {
	return &DefaultProvider{
		unavailableOfferings: unavailableOfferingsCache,
		pricingProvider:      pricingProvider,
	}
}

func (p *DefaultProvider) InjectOfferings(
	instanceTypes []*cloudprovider.InstanceType,
	nodeClass *v1.EC2NodeClass,
	allZones sets.Set[string],
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
) cloudprovider.Offerings {
	itZones := sets.New(it.Requirements.Get(corev1.LabelTopologyZone).Values()...)

	var offerings []*cloudprovider.Offering
	for zone := range allZones {
		for _, capacityType := range it.Requirements.Get(karpv1.CapacityTypeLabelKey).Values() {
			// Reserved capacity types are constructed separately
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
			offering := &cloudprovider.Offering{
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityType),
					scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, zone),
					scheduling.NewRequirement(cloudprovider.ReservationIDLabel, corev1.NodeSelectorOpDoesNotExist),
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

		_, hasSubnetZone := subnetZones[reservation.AvailabilityZone]
		price := 0.0
		if odPrice, ok := p.pricingProvider.OnDemandPrice(ec2types.InstanceType(it.Name)); ok {
			// Divide the on-demand price by a sufficiently large constant. This allows us to treat the reservation as "free",
			// while maintaining relative ordering for consolidation. If the pricing details are unavailable for whatever reason,
			// still succeed to create the offering and leave the price at zero. This will break consolidation, but will allow
			// users to utilize the instances they're already paying for.
			price = odPrice / 10_000_000.0
		}
		reservationCapacity := p.capacityReservationProvider.GetAvailableInstanceCount(reservation.ID)
		offering := &cloudprovider.Offering{
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeReserved),
				scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, reservation.AvailabilityZone),
				scheduling.NewRequirement(cloudprovider.ReservationIDLabel, corev1.NodeSelectorOpIn, reservation.ID),
			),
			Price:               price,
			Available:           reservationCapacity != 0 && itZones.Has(reservation.AvailabilityZone) && hasSubnetZone,
			ReservationCapacity: reservationCapacity,
		}
		if id, ok := subnetZones[reservation.AvailabilityZone]; ok {
			offering.Requirements.Add(scheduling.NewRequirement(v1.LabelTopologyZoneID, corev1.NodeSelectorOpIn, id))
		}
		offerings = append(offerings, offering)
	}
	return offerings
}
