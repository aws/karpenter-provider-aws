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
	"math"
	"strings"

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

// SpotInstanceFilter removes all instances with spot offerings which are more expensive than the cheapest compatible
// and available on-demand offering. This ensures we don't launch with a more expensive spot instance for a mixed-launch
// NodeClaim. Note that instance types with available, compatible reserved offerings will not be filtered out.
// NOTE: This filter assumes all provided instance types have compatible and available offerings
func SpotInstanceFilter(requirements scheduling.Requirements) Filter {
	return spotInstanceFilter{
		requirements: requirements,
	}
}

type spotInstanceFilter struct {
	requirements scheduling.Requirements
}

//nolint:gocyclo
func (f spotInstanceFilter) FilterReject(instanceTypes []*cloudprovider.InstanceType) ([]*cloudprovider.InstanceType, []*cloudprovider.InstanceType) {
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

	// Filter out any types where the cheapest spot offering is more expensive than the cheapest on-demand instance type
	// that would have worked. This prevents us from getting a larger, more-expensive spot instance type compared to the
	// cheapest sufficiently large on-demand instance type.
	return lo.FilterReject(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		var hasSpotOffering bool
		for _, o := range it.Offerings.Compatible(f.requirements).Available() {
			// Always include instance types which have compatible, available reserved offerings since they're modeled as free
			if o.CapacityType() == karpv1.CapacityTypeReserved {
				return true
			}
			// If the offering is spot and cheaper than the cheapest on-demand instance type, include the instance type
			if o.CapacityType() == karpv1.CapacityTypeSpot {
				hasSpotOffering = true
				if o.Price <= cheapestOnDemand {
					return true
				}
			}
		}
		return !hasSpotOffering
	})
}

func (spotInstanceFilter) Name() string {
	return "spot-instance-filter"
}
