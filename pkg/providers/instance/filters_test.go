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

package instance_test

import (
	"github.com/awslabs/operatorpkg/option"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

var _ = Describe("InstanceFiltersTest", func() {
	Context("CompatibleAvailableFilter", func() {
		It("should filter compatible instances (by requirements)", func() {
			filter := instance.CompatibleAvailableFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				corev1.LabelTopologyZone,
				corev1.NodeSelectorOpIn,
				"zone-1a",
			)), corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1000m"),
			})
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType(
					"compatible-instance",
					withRequirements(scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "zone-1a")),
					withResource(corev1.ResourceCPU, resource.MustParse("2000m")),
					withOfferings(makeOffering(karpv1.CapacityTypeOnDemand, true, withZone("zone-1a"))),
				),
				makeInstanceType(
					"incompatible-instance",
					withRequirements(scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "zone-1b")),
					withResource(corev1.ResourceCPU, resource.MustParse("2000m")),
					withOfferings(makeOffering(karpv1.CapacityTypeOnDemand, true, withZone("zone-1b"))),
				),
			})
			expectInstanceTypes(kept, "compatible-instance")
			expectInstanceTypes(rejected, "incompatible-instance")
		})
		It("should filter compatible instances (by requests)", func() {
			filter := instance.CompatibleAvailableFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				corev1.LabelTopologyZone,
				corev1.NodeSelectorOpIn,
				"zone-1a",
			)), corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1000m"),
			})
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType(
					"compatible-instance",
					withRequirements(scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "zone-1a")),
					withResource(corev1.ResourceCPU, resource.MustParse("2000m")),
					withOfferings(makeOffering(karpv1.CapacityTypeOnDemand, true, withZone("zone-1a"))),
				),
				makeInstanceType(
					"incompatible-instance",
					withRequirements(scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "zone-1a")),
					withResource(corev1.ResourceCPU, resource.MustParse("500m")),
					withOfferings(makeOffering(karpv1.CapacityTypeOnDemand, true, withZone("zone-1a"))),
				),
			})
			expectInstanceTypes(kept, "compatible-instance")
			expectInstanceTypes(rejected, "incompatible-instance")
		})
		It("should filter available instances", func() {
			filter := instance.CompatibleAvailableFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				corev1.LabelTopologyZone,
				corev1.NodeSelectorOpIn,
				"zone-1a",
			)), corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1000m"),
			})
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType(
					"available-instance",
					withRequirements(scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "zone-1a")),
					withResource(corev1.ResourceCPU, resource.MustParse("2000m")),
					withOfferings(
						makeOffering(karpv1.CapacityTypeOnDemand, true, withZone("zone-1a")),
					),
				),
				makeInstanceType(
					"unavailable-instance",
					withRequirements(scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "zone-1a")),
					withResource(corev1.ResourceCPU, resource.MustParse("2000m")),
					withOfferings(
						makeOffering(karpv1.CapacityTypeOnDemand, false, withZone("zone-1a")),
					),
				),
			})
			expectInstanceTypes(kept, "available-instance")
			expectInstanceTypes(rejected, "unavailable-instance")
		})
	})
	Context("ReservedOfferingFilter", func() {
		var filter instance.Filter
		BeforeEach(func() {
			filter = instance.ReservedOfferingFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				karpv1.CapacityTypeLabelKey,
				corev1.NodeSelectorOpExists,
			)))
		})

		It("shouldn't filter instance types if there are no available reserved offerings", func() {
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("non-reserved-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true),
					makeOffering(karpv1.CapacityTypeSpot, true),
				)),
				makeInstanceType("reserved-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true),
					makeOffering(karpv1.CapacityTypeSpot, true),
					makeOffering(karpv1.CapacityTypeReserved, false),
				)),
			})
			expectInstanceTypes(kept, "non-reserved-instance", "reserved-instance")
			Expect(rejected).To(BeEmpty())
		})
		It("should only include one reserved offering per instance pool", func() {
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("non-reserved-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true),
					makeOffering(karpv1.CapacityTypeSpot, true),
				)),
				makeInstanceType("reserved-instance-a", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true),
					makeOffering(karpv1.CapacityTypeSpot, true),
					// NOTE: Being a little hacky by using the reservation ID as an indicator for if the reservation should be kept.
					// This makes an assumption that the reservation ID is not used by the underlying filter - this isn't actually
					// modeling the same reservation in multiple zones
					makeOffering(karpv1.CapacityTypeReserved, true, withZone("1"), withReservationID("kept"), withReservationCapacity(5)),
					makeOffering(karpv1.CapacityTypeReserved, true, withZone("2"), withReservationID("kept"), withReservationCapacity(6)),
					makeOffering(karpv1.CapacityTypeReserved, true, withZone("2"), withReservationID("rejected"), withReservationCapacity(5)),
				)),
				makeInstanceType("reserved-instance-b", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true),
					makeOffering(karpv1.CapacityTypeSpot, true),
					// We should keep the offering with less capacity since the one with greater capacity is unavailable
					makeOffering(karpv1.CapacityTypeReserved, true, withZone("1"), withReservationID("kept"), withReservationCapacity(1)),
					makeOffering(karpv1.CapacityTypeReserved, false, withZone("1"), withReservationID("rejected"), withReservationCapacity(2)),
				)),
			})
			expectInstanceTypes(kept, "reserved-instance-a", "reserved-instance-b")
			expectInstanceTypes(rejected, "non-reserved-instance")
			Expect(len(kept[0].Offerings)).To(Equal(2))
			for _, it := range kept {
				for _, o := range it.Offerings {
					Expect(o.ReservationID()).To(Equal("kept"))
				}
			}
		})
		It("shouldn't filter instance types if the requirements are not compatible with reserved offerings", func() {
			filter = instance.ReservedOfferingFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				karpv1.CapacityTypeLabelKey,
				corev1.NodeSelectorOpNotIn,
				karpv1.CapacityTypeReserved,
			)))
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("non-reserved-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true),
					makeOffering(karpv1.CapacityTypeSpot, true),
				)),
				makeInstanceType("reserved-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true),
					makeOffering(karpv1.CapacityTypeSpot, true),
					makeOffering(karpv1.CapacityTypeReserved, true, withZone("1")),
				)),
			})
			expectInstanceTypes(kept, "non-reserved-instance", "reserved-instance")
			Expect(rejected).To(BeEmpty())
			Expect(lo.Map(kept, func(it *cloudprovider.InstanceType, _ int) int {
				return len(it.Offerings)
			})).To(ConsistOf(2, 3))
		})
	})
	Context("ExoticInstanceFilter", func() {
		var filter instance.Filter
		BeforeEach(func() {
			filter = instance.ExoticInstanceTypeFilter(scheduling.NewRequirements())
		})

		DescribeTable(
			"should reject instance types with exotic resources",
			func(resourceName corev1.ResourceName) {
				kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
					makeInstanceType("generic-instance-type"),
					makeInstanceType("exotic-instance-type", withResource(resourceName, resource.MustParse("1"))),
				})
				expectInstanceTypes(kept, "generic-instance-type")
				expectInstanceTypes(rejected, "exotic-instance-type")
			},
			lo.Map(v1.WellKnownExoticResources.UnsortedList(), func(resource corev1.ResourceName, _ int) TableEntry {
				return Entry(string(resource), resource)
			}),
		)
		DescribeTable(
			"should not reject instance types with normal resources",
			func(resourceName corev1.ResourceName) {
				kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
					makeInstanceType("generic-instance-type", withResource(resourceName, resource.MustParse("1"))),
					makeInstanceType("exotic-instance-type", withResource(v1.WellKnownExoticResources.UnsortedList()[0], resource.MustParse("1"))),
				})
				expectInstanceTypes(kept, "generic-instance-type")
				expectInstanceTypes(rejected, "exotic-instance-type")
			},
			lo.Map(v1.WellKnownResources.Difference(v1.WellKnownExoticResources).UnsortedList(), func(resource corev1.ResourceName, _ int) TableEntry {
				return Entry(string(resource), resource)
			}),
		)
		It("should reject metal instance types", func() {
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("generic-instance-type"),
				makeInstanceType("generic-instance-type-metal", withRequirements(scheduling.NewRequirement(
					v1.LabelInstanceSize,
					corev1.NodeSelectorOpIn,
					"metal",
				))),
			})
			expectInstanceTypes(kept, "generic-instance-type")
			expectInstanceTypes(rejected, "generic-instance-type-metal")
		})
		It("should include metal instance types and those with exotic resources if there are no other options", func() {
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("exotic-instance-type", withResource(v1.WellKnownExoticResources.UnsortedList()[0], resource.MustParse("1"))),
				makeInstanceType("generic-instance-type-metal", withRequirements(scheduling.NewRequirement(
					v1.LabelInstanceSize,
					corev1.NodeSelectorOpIn,
					"metal",
				))),
			})
			expectInstanceTypes(kept, "generic-instance-type-metal", "exotic-instance-type")
			Expect(rejected).To(BeEmpty())
		})
		It("should include instance types with exotic resources if minValues is set", func() {
			filter = instance.ExoticInstanceTypeFilter(scheduling.NewRequirements(scheduling.NewRequirementWithFlexibility(
				corev1.LabelInstanceTypeStable,
				corev1.NodeSelectorOpExists,
				lo.ToPtr(2),
			)))
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("generic-instance-type"),
				makeInstanceType("exotic-instance-type", withResource(v1.WellKnownExoticResources.UnsortedList()[0], resource.MustParse("1"))),
			})
			expectInstanceTypes(kept, "generic-instance-type", "exotic-instance-type")
			Expect(rejected).To(BeEmpty())
		})
	})
	Context("SpotInstanceFilter", func() {
		It("should reject spot instances whose cheapest offering is more expensive than the cheapest on-demand offering", func() {
			filter := instance.SpotInstanceFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				karpv1.CapacityTypeLabelKey,
				corev1.NodeSelectorOpExists,
			)))
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				// Include an expensive on-demand offering to ensure we're comparing against the cheapest. On-demand instances
				// should always be kept.
				makeInstanceType("expensive-od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(15.0)),
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(15.0)),
				)),
				// On-demand instances should always be kept
				makeInstanceType("od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(5.0)), // cheapest od offering
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(10.0)),
				)),
				// Instance should be kept because all offerings are cheaper than the cheapest od instance
				makeInstanceType("cheap-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0)),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(2.0)),
				)),
				// Instance should be kept since it contains a single cheaper offering
				makeInstanceType("mixed-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0)),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0)),
				)),
				// Instance should be rejected because although it has a cheaper offering, that offering is not available
				makeInstanceType("mixed-unavailable-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, false, withPrice(1.0)),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0)),
				)),
				// Instance should be rejected because all offerings are more expensive than the cheapest on-demand offering
				makeInstanceType("expensive-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0)),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0)),
				)),
			})
			expectInstanceTypes(kept, "expensive-od-instance", "od-instance", "cheap-spot-instance", "mixed-spot-instance")
			expectInstanceTypes(rejected, "mixed-unavailable-spot-instance", "expensive-spot-instance")
		})
		It("should not reject spot instances whose cheapest offering is more expensive than the cheapest on-demand offering if it is only compatible with spot", func() {
			filter := instance.SpotInstanceFilter(scheduling.NewRequirements(
				scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpExists,
				),
				scheduling.NewRequirement(
					corev1.LabelTopologyZone,
					corev1.NodeSelectorOpIn,
					"zone-1a",
					"zone-1b",
				),
			))

			keptInstanceTypes := []*cloudprovider.InstanceType{
				// Include an expensive on-demand offering to ensure we're comparing against the cheapest. On-demand instances
				// should always be kept.
				makeInstanceType("expensive-od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(15.0), withZone("zone-1a")),
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(15.0), withZone("zone-1b")),
				)),
				// On-demand instances should always be kept
				makeInstanceType("od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(5.0), withZone("zone-1a")), // cheapest od offering
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(10.0), withZone("zone-1b")),
				)),
				// Instance should be kept because all offerings are cheaper than the cheapest od instance
				makeInstanceType("cheap-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0), withZone("zone-1a")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(2.0), withZone("zone-1b")),
				)),
				// Instance should be kept since it contains a single cheaper offering
				makeInstanceType("mixed-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0), withZone("zone-1a")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withZone("zone-1b")),
				)),
				// Instance should be kept since it contains an offering in a compatible zone cheaper than the cheapest od offering
				makeInstanceType("mixed-compatible-available-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0), withZone("zone-1a")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withZone("zone-1c")),
				)),
				// Instance should be kept since it contains a reserved offering
				makeInstanceType("reserved-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withZone("zone-1a")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withZone("zone-1b")),
					makeOffering(karpv1.CapacityTypeReserved, true, withZone("zone-1b")),
				)),
			}
			rejectedInstanceTypes := []*cloudprovider.InstanceType{
				// Instance should be rejected because although it has a cheaper offering, that offering is not available
				makeInstanceType("mixed-unavailable-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, false, withPrice(1.0), withZone("zone-1a")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withZone("zone-1b")),
				)),
				// Instance should be rejected since it does not contain an offering in a compatible zone cheaper than the
				// cheapest od offering
				makeInstanceType("mixed-compatible-unavailable-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withZone("zone-1a")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0), withZone("zone-1c")),
				)),
				// Instance should be rejected because all offerings are more expensive than the cheapest on-demand offering
				makeInstanceType("expensive-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withZone("zone-1a")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withZone("zone-1b")),
				)),
			}

			kept, rejected := filter.FilterReject(lo.Flatten([][]*cloudprovider.InstanceType{keptInstanceTypes, rejectedInstanceTypes}))
			expectInstanceTypes(kept, lo.Map(keptInstanceTypes, func(it *cloudprovider.InstanceType, _ int) string { return it.Name })...)
			expectInstanceTypes(rejected, lo.Map(rejectedInstanceTypes, func(it *cloudprovider.InstanceType, _ int) string { return it.Name })...)
		})
		It("should not reject instances if the nodeclaim is only compatible with spot", func() {
			filter := instance.SpotInstanceFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				karpv1.CapacityTypeLabelKey,
				corev1.NodeSelectorOpIn,
				karpv1.CapacityTypeSpot,
			)))
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(5.0)),
				)),
				makeInstanceType("cheap-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0)),
				)),
				makeInstanceType("expensive-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0)),
				)),
			})
			expectInstanceTypes(kept, "od-instance", "cheap-spot-instance", "expensive-spot-instance")
			Expect(rejected).To(BeEmpty())
		})
		It("should not reject instances if minValues is set", func() {
			filter := instance.SpotInstanceFilter(scheduling.NewRequirements(
				scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpExists,
				),
				scheduling.NewRequirementWithFlexibility(
					corev1.LabelInstanceTypeStable,
					corev1.NodeSelectorOpExists,
					lo.ToPtr(2),
				),
			))
			kept, rejected := filter.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(5.0)),
				)),
				makeInstanceType("cheap-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0)),
				)),
				makeInstanceType("expensive-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0)),
				)),
			})
			expectInstanceTypes(kept, "od-instance", "cheap-spot-instance", "expensive-spot-instance")
			Expect(rejected).To(BeEmpty())
		})
	})
})

func expectInstanceTypes(instanceTypes []*cloudprovider.InstanceType, names ...string) {
	GinkgoHelper()
	resolvedNames := lo.Map(instanceTypes, func(it *cloudprovider.InstanceType, _ int) string {
		return it.Name
	})
	Expect(resolvedNames).To(ConsistOf(lo.Map(names, func(n string, _ int) any { return n })...))
}

type mockInstanceTypeOptions = option.Function[cloudprovider.InstanceType]

func withRequirements(reqs ...*scheduling.Requirement) mockInstanceTypeOptions {
	return func(it *cloudprovider.InstanceType) {
		if it.Requirements == nil {
			it.Requirements = scheduling.NewRequirements()
		}
		it.Requirements.Add(reqs...)
	}
}

func withResource(name corev1.ResourceName, quantity resource.Quantity) mockInstanceTypeOptions {
	return func(it *cloudprovider.InstanceType) {
		if it.Capacity == nil {
			it.Capacity = corev1.ResourceList{}
		}
		it.Capacity[name] = quantity
	}
}

func withOfferings(offerings ...*cloudprovider.Offering) mockInstanceTypeOptions {
	return func(it *cloudprovider.InstanceType) {
		it.Offerings = offerings
	}
}

func makeInstanceType(name string, opts ...mockInstanceTypeOptions) *cloudprovider.InstanceType {
	instanceType := option.Resolve(opts...)
	instanceType.Name = name
	instanceType.Overhead = &cloudprovider.InstanceTypeOverhead{
		KubeReserved:      corev1.ResourceList{},
		SystemReserved:    corev1.ResourceList{},
		EvictionThreshold: corev1.ResourceList{},
	}
	return instanceType
}

type mockOfferingOptions = option.Function[cloudprovider.Offering]

func withReservationCapacity(capacity int) mockOfferingOptions {
	return func(o *cloudprovider.Offering) {
		o.ReservationCapacity = capacity
	}
}

func withReservationID(id string) mockOfferingOptions {
	return func(o *cloudprovider.Offering) {
		if o.Requirements == nil {
			o.Requirements = scheduling.NewRequirements()
		}
		o.Requirements.Add(scheduling.NewRequirement(
			v1.LabelCapacityReservationID,
			corev1.NodeSelectorOpIn,
			id,
		))
	}
}

func withZone(zone string) mockOfferingOptions {
	return func(o *cloudprovider.Offering) {
		if o.Requirements == nil {
			o.Requirements = scheduling.NewRequirements()
		}
		o.Requirements.Add(scheduling.NewRequirement(
			corev1.LabelTopologyZone,
			corev1.NodeSelectorOpIn,
			zone,
		))
	}
}

func withPrice(price float64) mockOfferingOptions {
	return func(o *cloudprovider.Offering) {
		o.Price = price
	}
}

func makeOffering(capacityType string, available bool, opts ...mockOfferingOptions) *cloudprovider.Offering {
	offering := option.Resolve(opts...)
	if offering.Requirements == nil {
		offering.Requirements = scheduling.NewRequirements()
	}
	offering.Requirements.Add(scheduling.NewRequirement(
		karpv1.CapacityTypeLabelKey,
		corev1.NodeSelectorOpIn,
		capacityType,
	))
	offering.Available = available
	return offering
}
