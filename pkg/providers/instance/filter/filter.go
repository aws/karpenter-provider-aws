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

package filter

import (
	"fmt"
	"math"
	"strings"

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

type Filter interface {
	FilterReject(instanceTypes []*cloudprovider.InstanceType) (kept []*cloudprovider.InstanceType, rejected []*cloudprovider.InstanceType)
	Name() string
}

// CompatibleAvailableFilter removes instance types which do not have any compatible, available offerings. Other filters
// should not be used without first using this filter.
func CompatibleAvailableFilter(requirements scheduling.Requirements, requests corev1.ResourceList) Filter {
	return compatibleAvailableFilter{
		requirements: requirements,
		requests:     requests,
	}
}

type compatibleAvailableFilter struct {
	requirements scheduling.Requirements
	requests     corev1.ResourceList
}

func (f compatibleAvailableFilter) FilterReject(instanceTypes []*cloudprovider.InstanceType) ([]*cloudprovider.InstanceType, []*cloudprovider.InstanceType) {
	return lo.FilterReject(instanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
		if !f.requirements.IsCompatible(i.Requirements, scheduling.AllowUndefinedWellKnownLabels) {
			return false
		}
		if !resources.Fits(f.requests, i.Allocatable()) {
			return false
		}
		if len(i.Offerings.Compatible(f.requirements).Available()) == 0 {
			return false
		}
		return true
	})
}

func (compatibleAvailableFilter) Name() string {
	return "compatible-available-filter"
}

// CapacityReservationTypeFilter creates a Filter which ensures there aren't instance types with offerings from multiple
// capacity reservation types. This addresses a CreateFleet limitation, where we can only specify a single market type
// (i.e. "on-demand" or "capacity-block").
func CapacityReservationTypeFilter(requirements scheduling.Requirements) Filter {
	return capacityReservationTypeFilter{
		requirements: requirements,
	}
}

type capacityReservationTypeFilter struct {
	requirements scheduling.Requirements
}

func (f capacityReservationTypeFilter) FilterReject(instanceTypes []*cloudprovider.InstanceType) ([]*cloudprovider.InstanceType, []*cloudprovider.InstanceType) {
	if !f.requirements.Get(karpv1.CapacityTypeLabelKey).Has(karpv1.CapacityTypeReserved) {
		return instanceTypes, nil
	}

	// Select the partition with the cheapest instance type
	selectedPartition := lo.MinBy(f.Partition(instanceTypes), func(i, j *capacityReservationTypePartition) bool {
		if i.cheapestPrice != j.cheapestPrice {
			return i.cheapestPrice < j.cheapestPrice
		}
		priorities := map[v1.CapacityReservationType]int{
			v1.CapacityReservationTypeDefault:       0,
			v1.CapacityReservationTypeCapacityBlock: 1,
		}
		return priorities[i.capacityReservationType] < priorities[j.capacityReservationType]
	})
	if len(selectedPartition.instanceTypes) == 0 {
		return instanceTypes, nil
	}

	// Remove offerings which do not belong to the selected partition for the selected instance types
	for _, it := range selectedPartition.instanceTypes {
		it.Offerings = lo.Filter(it.Offerings, func(o *cloudprovider.Offering, _ int) bool {
			if o.CapacityType() != karpv1.CapacityTypeReserved {
				return false
			}
			if o.Requirements.Get(v1.LabelCapacityReservationType).Any() != string(selectedPartition.capacityReservationType) {
				return false
			}
			return true
		})
	}
	return lo.Values(selectedPartition.instanceTypes), lo.Reject(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		_, ok := selectedPartition.instanceTypes[it.Name]
		return ok
	})
}

func (f capacityReservationTypeFilter) Name() string {
	return "capacity-reservation-type-filter"
}

type capacityReservationTypePartition struct {
	capacityReservationType v1.CapacityReservationType
	cheapestPrice           float64
	instanceTypes           map[string]*cloudprovider.InstanceType
}

func (f capacityReservationTypeFilter) Partition(instanceTypes []*cloudprovider.InstanceType) []*capacityReservationTypePartition {
	partitions := map[v1.CapacityReservationType]*capacityReservationTypePartition{}
	for _, t := range v1.CapacityReservationType("").Values() {
		partitions[t] = &capacityReservationTypePartition{
			capacityReservationType: t,
			cheapestPrice:           math.MaxFloat64,
			instanceTypes:           map[string]*cloudprovider.InstanceType{},
		}
	}
	for _, it := range instanceTypes {
		for _, o := range it.Offerings.Available().Compatible(f.requirements) {
			if o.CapacityType() != karpv1.CapacityTypeReserved {
				continue
			}
			t := v1.CapacityReservationType(o.Requirements.Get(v1.LabelCapacityReservationType).Any())
			p, ok := partitions[t]
			if !ok {
				// SAFETY: Valid reservation types are enforced during capacity reservation construction in the NodeClass
				// controller. An invalid value indicates a user manually edited their NodeClass' status, breaking an invariant.
				lo.Must0(serrors.Wrap(
					fmt.Errorf("failed to partition capacity reservations, invalid capacity reservation type"),
					"type", string(t),
					"valid-types", lo.Map(v1.CapacityReservationType("").Values(), func(crt v1.CapacityReservationType, _ int) string { return string(crt) }),
				))
			}
			if p.cheapestPrice > o.Price {
				p.cheapestPrice = o.Price
			}
			p.instanceTypes[it.Name] = it
		}
	}
	return lo.Values(partitions)
}

// CapacityBlockFilter creates a filter which selects the instance type with the cheapest capacity block offering if the
// provided requirements are for a reserved launch and the provided instance types have a capacity block offering. This
// filter is required because CreateFleet does not accept multiple capacity blocks in a given request.
// Ref: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-fleet-launch-instances-capacity-blocks-walkthrough.html
func CapacityBlockFilter(requirements scheduling.Requirements) Filter {
	return capacityBlockFilter{
		requirements: requirements,
	}
}

type capacityBlockFilter struct {
	requirements scheduling.Requirements
}

//nolint:gocyclo
func (f capacityBlockFilter) FilterReject(instanceTypes []*cloudprovider.InstanceType) ([]*cloudprovider.InstanceType, []*cloudprovider.InstanceType) {
	if !f.shouldFilter(instanceTypes) {
		return instanceTypes, nil
	}
	var selectedInstanceType *cloudprovider.InstanceType
	for _, it := range instanceTypes {
		var selectedOffering *cloudprovider.Offering
		for _, o := range it.Offerings {
			if o.CapacityType() != karpv1.CapacityTypeReserved {
				continue
			}
			if !o.Available || !f.requirements.IsCompatible(o.Requirements, scheduling.AllowUndefinedWellKnownLabels) {
				continue
			}
			if o.Requirements.Get(v1.LabelCapacityReservationType).Any() != string(v1.CapacityReservationTypeCapacityBlock) {
				continue
			}
			if selectedOffering == nil || selectedOffering.Price > o.Price {
				selectedOffering = o
			}
		}
		if selectedOffering != nil && (selectedInstanceType == nil || selectedInstanceType.Offerings[0].Price > selectedOffering.Price) {
			// WARNING: It is only safe to mutate the slice containing the offerings, not the offerings themselves. The
			// individual offerings are cached, but not the slice storing them. This helps keep the launch path simple, but
			// changes to the caching strategy employed by the InstanceType provider could result in unexpected behavior.
			it.Offerings = []*cloudprovider.Offering{selectedOffering}
			selectedInstanceType = it
		}
	}
	return []*cloudprovider.InstanceType{selectedInstanceType}, lo.Reject(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		return it.Name == selectedInstanceType.Name
	})
}

func (f capacityBlockFilter) shouldFilter(instanceTypes []*cloudprovider.InstanceType) bool {
	if !f.requirements.Get(karpv1.CapacityTypeLabelKey).Has(karpv1.CapacityTypeReserved) {
		return false
	}
	for _, it := range instanceTypes {
		for _, o := range it.Offerings {
			if !o.Requirements.Has(v1.LabelCapacityReservationType) {
				continue
			}
			if o.Requirements.Get(v1.LabelCapacityReservationType).Any() == string(v1.CapacityReservationTypeCapacityBlock) {
				return true
			} else {
				return false
			}
		}
	}
	return false
}

func (f capacityBlockFilter) Name() string {
	return "capacity-block-filter"
}

// ReservedOfferingFilter creates a Filter which ensures there's only a single reserved offering per zone. This
// addresses a limitation of the CreateFleet API, which limits calls to specifying a single offering per pool. If there
// are multiple offerings in the same pool, the offering with the greatest capacity will be selected.
func ReservedOfferingFilter(requirements scheduling.Requirements) Filter {
	return reservedOfferingFilter{
		requirements: requirements,
	}
}

type reservedOfferingFilter struct {
	requirements scheduling.Requirements
}

func (f reservedOfferingFilter) FilterReject(instanceTypes []*cloudprovider.InstanceType) ([]*cloudprovider.InstanceType, []*cloudprovider.InstanceType) {
	if !f.requirements.Get(karpv1.CapacityTypeLabelKey).Has(karpv1.CapacityTypeReserved) {
		return instanceTypes, nil
	}

	var remaining, rejected []*cloudprovider.InstanceType
	for _, it := range instanceTypes {
		zonalOfferings := map[string]*cloudprovider.Offering{}
		for _, o := range it.Offerings.Available().Compatible(f.requirements) {
			if o.CapacityType() != karpv1.CapacityTypeReserved {
				continue
			}
			if current, ok := zonalOfferings[o.Zone()]; !ok || o.ReservationCapacity > current.ReservationCapacity {
				zonalOfferings[o.Zone()] = o
			}
		}
		if len(zonalOfferings) == 0 {
			rejected = append(rejected, it)
			continue
		}
		// WARNING: It is only safe to mutate the slice containing the offerings, not the offerings themselves. The individual
		// offerings are cached, but not the slice storing them. This helps keep the launch path simple, but changes to the
		// caching strategy employed by the InstanceType provider could result in unexpected behavior.
		it.Offerings = lo.Values(zonalOfferings)
		remaining = append(remaining, it)
	}
	if len(remaining) == 0 {
		return instanceTypes, nil
	}
	return remaining, rejected
}

func (reservedOfferingFilter) Name() string {
	return "reserved-offering-filter"
}

// ExoticInstanceTypeFilter will remove instances with GPUs and accelerators, along with metal instances, if doing so
// doesn't filter out all instance types. This ensures Karpenter only launches these instances if the NodeClaim
// explicitly requests them or all other compatible instance types are unavailable.
func ExoticInstanceTypeFilter(requirements scheduling.Requirements) Filter {
	return exoticInstanceFilter{
		requirements: requirements,
	}
}

type exoticInstanceFilter struct {
	requirements scheduling.Requirements
}

func (f exoticInstanceFilter) FilterReject(instanceTypes []*cloudprovider.InstanceType) ([]*cloudprovider.InstanceType, []*cloudprovider.InstanceType) {
	if f.requirements.HasMinValues() {
		return instanceTypes, nil
	}

	genericInstanceTypes, exoticInstanceTypes := lo.FilterReject(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		if lo.ContainsBy(it.Requirements.Get(v1.LabelInstanceSize).Values(), func(size string) bool {
			return strings.Contains(size, "metal")
		}) {
			return false
		}
		for _, resource := range []corev1.ResourceName{
			v1.ResourceAWSNeuron,
			v1.ResourceAWSNeuronCore,
			v1.ResourceAMDGPU,
			v1.ResourceNVIDIAGPU,
			v1.ResourceHabanaGaudi,
		} {
			if !resources.IsZero(it.Capacity[resource]) {
				return false
			}
		}
		return true
	})
	// If there are no available, compatible reserved instance types Karpenter should fallback to exotic instance types
	if len(genericInstanceTypes) == 0 {
		return instanceTypes, nil
	}
	return genericInstanceTypes, exoticInstanceTypes
}

func (exoticInstanceFilter) Name() string {
	return "exotic-instance-filter"
}

// SpotOfferingFilter removes spot offerings that are more expensive than the cheapest compatible and available
// on-demand offering. This ensures we don't launch with a more expensive spot instance for a mixed-launch NodeClaim.
// NOTE: This filter assumes all provided instance types have compatible and available offerings
func SpotOfferingFilter(requirements scheduling.Requirements) Filter {
	return spotOfferingFilter{
		requirements: requirements,
	}
}

type spotOfferingFilter struct {
	requirements scheduling.Requirements
}

//nolint:gocyclo
func (f spotOfferingFilter) FilterReject(instanceTypes []*cloudprovider.InstanceType) ([]*cloudprovider.InstanceType, []*cloudprovider.InstanceType) {
	if f.requirements.HasMinValues() {
		return instanceTypes, nil
	}
	if req := f.requirements.Get(karpv1.CapacityTypeLabelKey); !req.Has(karpv1.CapacityTypeOnDemand) || !req.Has(karpv1.CapacityTypeSpot) {
		return instanceTypes, nil
	}

	cheapestOnDemand := math.MaxFloat64
	hasSpotOfferings := false
	hasODOfferings := false
	for _, it := range instanceTypes {
		for _, o := range it.Offerings.Compatible(f.requirements).Available() {
			if ct := o.Requirements.Get(karpv1.CapacityTypeLabelKey).Any(); ct == karpv1.CapacityTypeOnDemand {
				hasODOfferings = true
				if o.Price < cheapestOnDemand {
					cheapestOnDemand = o.Price
				}
			} else if ct == karpv1.CapacityTypeSpot {
				hasSpotOfferings = true
			}
		}
	}
	if !hasODOfferings || !hasSpotOfferings {
		return instanceTypes, nil
	}

	var remaining []*cloudprovider.InstanceType
	for _, it := range instanceTypes {
		filteredOfferings := lo.Filter(it.Offerings, func(o *cloudprovider.Offering, _ int) bool {
			if o.CapacityType() == karpv1.CapacityTypeSpot && o.Price > cheapestOnDemand {
				return false
			}
			return true
		})
		if len(filteredOfferings) > 0 {
			// WARNING: It is only safe to mutate the slice containing the offerings, not the offerings themselves. The individual
			// offerings are cached, but not the slice storing them. This helps keep the launch path simple, but changes to the
			// caching strategy employed by the InstanceType provider could result in unexpected behavior.
			it.Offerings = filteredOfferings
			remaining = append(remaining, it)
		}
	}
	return remaining, lo.Reject(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		return lo.Contains(remaining, it)
	})
}

func (spotOfferingFilter) Name() string {
	return "spot-offering-filter"
}
