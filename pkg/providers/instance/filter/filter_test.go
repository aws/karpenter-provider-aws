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

package filter_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/awslabs/operatorpkg/option"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance/filter"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "FilterTest")
}

var _ = Describe("InstanceFiltersTest", func() {
	Context("CompatibleAvailableFilter", func() {
		It("should filter compatible instances (by requirements)", func() {
			f := filter.CompatibleAvailableFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				corev1.LabelTopologyZone,
				corev1.NodeSelectorOpIn,
				"zone-1a",
			)), corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1000m"),
			})
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
			f := filter.CompatibleAvailableFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				corev1.LabelTopologyZone,
				corev1.NodeSelectorOpIn,
				"zone-1a",
			)), corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1000m"),
			})
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
			f := filter.CompatibleAvailableFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				corev1.LabelTopologyZone,
				corev1.NodeSelectorOpIn,
				"zone-1a",
			)), corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1000m"),
			})
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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

	Context("CapacityReservationTypeFilter", func() {
		DescribeTable(
			"should prioritize the capacity reservation type with the cheapest offering",
			func(selectedType v1.CapacityReservationType) {
				f := filter.CapacityReservationTypeFilter(scheduling.NewRequirements(
					scheduling.NewRequirement(
						karpv1.CapacityTypeLabelKey,
						corev1.NodeSelectorOpIn,
						karpv1.CapacityTypeReserved,
					),
					scheduling.NewRequirement(
						corev1.LabelTopologyZone,
						corev1.NodeSelectorOpIn,
						"zone-1a",
					),
				))

				keptInstanceTypes := []*cloudprovider.InstanceType{
					makeInstanceType(fmt.Sprintf("cheap-instance-%s", string(selectedType)), withOfferings(
						makeOffering(karpv1.CapacityTypeReserved, true, withCapacityReservationType(selectedType), withPrice(5.0), withZone("zone-1a")),
					)),
					makeInstanceType(fmt.Sprintf("expensive-instance-%s", string(selectedType)), withOfferings(
						makeOffering(karpv1.CapacityTypeReserved, true, withCapacityReservationType(selectedType), withPrice(10.0), withZone("zone-1a")),
					)),
				}
				rejectedInstanceTypes := lo.FilterMap(
					v1.CapacityReservationType("").Values(),
					func(t v1.CapacityReservationType, _ int) (*cloudprovider.InstanceType, bool) {
						if t == selectedType {
							return nil, false
						}
						return makeInstanceType(fmt.Sprintf("expensive-instance-%s", string(t)), withOfferings(
							makeOffering(karpv1.CapacityTypeReserved, true, withCapacityReservationType(t), withPrice(10.0), withZone("zone-1a")),
							// Include offerings which are cheaper than the cheapest selected offering, but are unavailable and incompatible
							// respectively to ensure compatible and available offering checks are performed correctly.
							makeOffering(karpv1.CapacityTypeReserved, false, withCapacityReservationType(t), withPrice(1.0), withZone("zone-1a")),
							makeOffering(karpv1.CapacityTypeReserved, true, withCapacityReservationType(t), withPrice(1.0), withZone("zone-1b")),
						)), true
					},
				)

				kept, rejected := f.FilterReject(lo.Flatten([][]*cloudprovider.InstanceType{keptInstanceTypes, rejectedInstanceTypes}))
				expectInstanceTypes(kept, lo.Map(keptInstanceTypes, func(it *cloudprovider.InstanceType, _ int) string { return it.Name })...)
				expectInstanceTypes(rejected, lo.Map(rejectedInstanceTypes, func(it *cloudprovider.InstanceType, _ int) string { return it.Name })...)
			},
			lo.Map(v1.CapacityReservationType("").Values(), func(crt v1.CapacityReservationType, _ int) TableEntry {
				return Entry(fmt.Sprintf("when the type is %q", string(crt)), crt)
			}),
		)
		DescribeTable(
			"should break ties by priority",
			func(selectedType, rejectedType v1.CapacityReservationType) {
				f := filter.CapacityReservationTypeFilter(scheduling.NewRequirements(scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpIn,
					karpv1.CapacityTypeReserved,
				)))
				kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
					makeInstanceType(string(selectedType), withOfferings(
						makeOffering(karpv1.CapacityTypeReserved, true, withPrice(5.0), withCapacityReservationType(selectedType)),
					)),
					makeInstanceType(string(rejectedType), withOfferings(
						makeOffering(karpv1.CapacityTypeReserved, true, withPrice(5.0), withCapacityReservationType(rejectedType)),
					)),
				})
				expectInstanceTypes(kept, string(selectedType))
				expectInstanceTypes(rejected, string(rejectedType))
			},
			func() []TableEntry {
				crts := []v1.CapacityReservationType{
					v1.CapacityReservationTypeDefault,
					v1.CapacityReservationTypeCapacityBlock,
				}
				var entries []TableEntry
				// Iterate over the capacity reservation types in order of priority
				for i := range crts {
					for j := i + 1; j < len(crts); j++ {
						entries = append(entries, Entry(fmt.Sprintf("when the two capacity reservation types are %q and %q", crts[i], crts[j]), crts[i], crts[j]))
					}
				}
				return entries
			}(),
		)
		DescribeTable(
			"should remove offerings which aren't the selected capacity reservation type",
			func(selectedType v1.CapacityReservationType) {
				f := filter.CapacityReservationTypeFilter(scheduling.NewRequirements(scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpIn,
					karpv1.CapacityTypeReserved,
				)))
				kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
					makeInstanceType("pin-instance", withOfferings(
						makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.0), withCapacityReservationType(selectedType)),
					)),
					makeInstanceType("filter-instance", withOfferings(lo.Map(
						v1.CapacityReservationType("").Values(),
						func(t v1.CapacityReservationType, _ int) *cloudprovider.Offering {
							return makeOffering(karpv1.CapacityTypeReserved, true, withPrice(5.0), withCapacityReservationType(t))
						},
					)...)),
				})
				expectInstanceTypes(kept, "pin-instance", "filter-instance")
				Expect(rejected).To(BeEmpty())
				for _, it := range kept {
					Expect(it.Offerings).To(HaveLen(1))
					Expect(it.Offerings[0].CapacityType()).To(Equal(karpv1.CapacityTypeReserved))
					Expect(it.Offerings[0].Requirements.Get(v1.LabelCapacityReservationType).Any()).To(Equal(string(selectedType)))
				}
			},
			lo.Map(v1.CapacityReservationType("").Values(), func(crt v1.CapacityReservationType, _ int) TableEntry {
				return Entry(fmt.Sprintf("when the type is %q", string(crt)), crt)
			}),
		)
		It("should not filter instance types when the nodeclaim is not compatible with capacity type reserved", func() {
			f := filter.CapacityReservationTypeFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				karpv1.CapacityTypeLabelKey,
				corev1.NodeSelectorOpNotIn,
				karpv1.CapacityTypeReserved,
			)))
			instanceTypes := lo.Map(v1.CapacityReservationType("").Values(), func(t v1.CapacityReservationType, _ int) *cloudprovider.InstanceType {
				return makeInstanceType(fmt.Sprintf("%s-instance", string(t)), withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true),
					makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.0), withCapacityReservationType(t)),
				))
			})
			kept, rejected := f.FilterReject(instanceTypes)
			expectInstanceTypes(kept, lo.Map(instanceTypes, func(it *cloudprovider.InstanceType, _ int) string { return it.Name })...)
			Expect(rejected).To(BeEmpty())
		})
	})

	Context("CapacityBlockFilter", func() {
		It("should select the instance type with the cheapest capacity-block offering", func() {
			f := filter.CapacityBlockFilter(scheduling.NewRequirements(scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpExists)))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("cheap-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)),
					makeOffering(karpv1.CapacityTypeReserved, true, withPrice(10.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)),
				)),
				makeInstanceType("expensive-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeReserved, true, withPrice(2.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)),
					makeOffering(karpv1.CapacityTypeReserved, true, withPrice(10.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)),
				)),
			})
			expectInstanceTypes(kept, "cheap-instance")
			expectInstanceTypes(rejected, "expensive-instance")
			Expect(kept[0].Offerings).To(HaveLen(1))
		})
		DescribeTable(
			"OfferingSelection",
			func(expectedReservationID string, offerings ...*cloudprovider.Offering) {
				f := filter.CapacityBlockFilter(
					scheduling.NewRequirements(scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpExists),
						scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpNotIn, "forbidden-zone"),
					))
				kept, _ := f.FilterReject([]*cloudprovider.InstanceType{makeInstanceType("instance", withOfferings(offerings...))})
				Expect(kept[0].Offerings).To(HaveLen(1))
				Expect(kept[0].Offerings[0].ReservationID()).To(Equal(expectedReservationID))
			},
			Entry(
				"should select the cheapest offering",
				"cheapest",
				makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock), withReservationID("cheapest")),
				makeOffering(karpv1.CapacityTypeReserved, true, withPrice(2.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock), withReservationID("expensive")),
			),
			Entry(
				"should not select unavailable offerings",
				"cheapest-available",
				makeOffering(karpv1.CapacityTypeReserved, false, withPrice(1.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock), withReservationID("cheapest")),
				makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.5), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock), withReservationID("cheapest-available")),
				makeOffering(karpv1.CapacityTypeReserved, true, withPrice(2.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock), withReservationID("expensive")),
			),
			Entry(
				"should not select incompatible offerings",
				"cheapest-compatible",
				makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock), withZone("forbidden-zone"), withReservationID("cheapest")),
				makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.5), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock), withReservationID("cheapest-compatible")),
				makeOffering(karpv1.CapacityTypeReserved, true, withPrice(2.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock), withReservationID("expensive")),
			),
		)
		DescribeTable(
			"shouldn't filter instance types when the capacity reservation type is not capacity-block",
			func(crt v1.CapacityReservationType) {
				f := filter.CapacityBlockFilter(scheduling.NewRequirements(scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpExists)))
				kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
					makeInstanceType("cheap-instance", withOfferings(
						makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.0), withCapacityReservationType(crt)),
						makeOffering(karpv1.CapacityTypeReserved, true, withPrice(10.0), withCapacityReservationType(crt)),
					)),
					makeInstanceType("expensive-instance", withOfferings(
						makeOffering(karpv1.CapacityTypeReserved, true, withPrice(2.0), withCapacityReservationType(crt)),
						makeOffering(karpv1.CapacityTypeReserved, true, withPrice(10.0), withCapacityReservationType(crt)),
					)),
				})
				expectInstanceTypes(kept, "cheap-instance", "expensive-instance")
				Expect(rejected).To(BeEmpty())
				for _, it := range kept {
					Expect(it.Offerings).To(HaveLen(2))
				}
			},
			lo.FilterMap(v1.CapacityReservationType("").Values(), func(crt v1.CapacityReservationType, _ int) (TableEntry, bool) {
				return Entry(fmt.Sprintf("when the capacity reservation type is %q", string(crt)), crt), crt != v1.CapacityReservationTypeCapacityBlock
			}),
		)
		It("shouldn't filter instance types when the requirements aren't compatible with capacity type reserved", func() {
			f := filter.CapacityBlockFilter(scheduling.NewRequirements(scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpNotIn, karpv1.CapacityTypeReserved)))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("cheap-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeReserved, true, withPrice(1.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)),
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(1.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)),
				)),
				makeInstanceType("expensive-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeReserved, true, withPrice(2.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)),
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(2.0), withCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)),
				)),
			})
			expectInstanceTypes(kept, "cheap-instance", "expensive-instance")
			Expect(rejected).To(BeEmpty())
			for _, it := range kept {
				Expect(it.Offerings).To(HaveLen(2))
			}
		})
	})

	Context("ReservedOfferingFilter", func() {
		var f filter.Filter
		BeforeEach(func() {
			f = filter.ReservedOfferingFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				karpv1.CapacityTypeLabelKey,
				corev1.NodeSelectorOpExists,
			)))
		})

		It("shouldn't filter instance types if there are no available reserved offerings", func() {
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
			f = filter.ReservedOfferingFilter(scheduling.NewRequirements(scheduling.NewRequirement(
				karpv1.CapacityTypeLabelKey,
				corev1.NodeSelectorOpNotIn,
				karpv1.CapacityTypeReserved,
			)))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
		var f filter.Filter
		BeforeEach(func() {
			f = filter.ExoticInstanceTypeFilter(scheduling.NewRequirements())
		})

		DescribeTable(
			"should reject instance types with exotic resources",
			func(resourceName corev1.ResourceName) {
				kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
				kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
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
			f = filter.ExoticInstanceTypeFilter(scheduling.NewRequirements(scheduling.NewRequirementWithFlexibility(
				corev1.LabelInstanceTypeStable,
				corev1.NodeSelectorOpExists,
				lo.ToPtr(2),
			)))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("generic-instance-type"),
				makeInstanceType("exotic-instance-type", withResource(v1.WellKnownExoticResources.UnsortedList()[0], resource.MustParse("1"))),
			})
			expectInstanceTypes(kept, "generic-instance-type", "exotic-instance-type")
			Expect(rejected).To(BeEmpty())
		})
	})

	Context("SpotOfferingFilter", func() {
		It("should filter out expensive spot offerings while keeping instance types with remaining offerings", func() {
			f := filter.SpotOfferingFilter(scheduling.NewRequirements(
				scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpExists,
				),
				scheduling.NewRequirement(
					"test.karpenter.sh/tag",
					corev1.NodeSelectorOpExists,
				),
			))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
				// On-demand instance should be kept with all offerings
				makeInstanceType("od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(5.0), withTag("kept")), // cheapest od offering
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(10.0), withTag("kept")),
				)),
				// Instance with cheap spot offerings should be kept with all offerings
				makeInstanceType("cheap-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0), withTag("kept")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(2.0), withTag("kept")),
				)),
				// Instance with mixed spot offerings should be kept but expensive spot offering filtered out
				makeInstanceType("mixed-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(1.0), withTag("kept")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withTag("rejected")), // should be filtered
				)),
			})
			expectInstanceTypes(kept, "od-instance", "cheap-spot-instance", "mixed-spot-instance")
			Expect(rejected).To(BeEmpty())
			Expect(lo.Map(kept, func(it *cloudprovider.InstanceType, _ int) int {
				return len(it.Offerings)
			})).To(ConsistOf(2, 2, 1))
			// Check that all kept offerings have tag="kept"
			for _, it := range kept {
				for _, o := range it.Offerings {
					Expect(o.Requirements.Get("test.karpenter.sh/tag").Any()).To(Equal("kept"))
				}
			}
		})
		It("should reject instance types with no remaining offerings after filtering", func() {
			f := filter.SpotOfferingFilter(scheduling.NewRequirements(
				scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpExists,
				),
				scheduling.NewRequirement(
					"test.karpenter.sh/tag",
					corev1.NodeSelectorOpExists,
				),
			))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(5.0), withTag("kept")),
				)),
				// Instance with only expensive spot offerings should be rejected
				makeInstanceType("expensive-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withTag("kept")),
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(15.0), withTag("kept")),
				)),
			})
			expectInstanceTypes(kept, "od-instance")
			expectInstanceTypes(rejected, "expensive-spot-instance")
			Expect(kept[0].Offerings).To(HaveLen(1))
			for _, o := range kept[0].Offerings {
				Expect(o.Requirements.Get("test.karpenter.sh/tag").Any()).To(Equal("kept"))
			}
		})
		It("should not filter when no on-demand offerings exist", func() {
			f := filter.SpotOfferingFilter(scheduling.NewRequirements(
				scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpExists,
				),
				scheduling.NewRequirement(
					"test.karpenter.sh/tag",
					corev1.NodeSelectorOpExists,
				),
			))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withTag("kept")),
				)),
			})
			expectInstanceTypes(kept, "spot-instance")
			Expect(rejected).To(BeEmpty())
			Expect(kept[0].Offerings).To(HaveLen(1))
			for _, o := range kept[0].Offerings {
				Expect(o.Requirements.Get("test.karpenter.sh/tag").Any()).To(Equal("kept"))
			}
		})
		It("should not filter when requirements don't support both spot and on-demand", func() {
			f := filter.SpotOfferingFilter(scheduling.NewRequirements(
				scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpIn,
					karpv1.CapacityTypeSpot,
				),
				scheduling.NewRequirement(
					"test.karpenter.sh/tag",
					corev1.NodeSelectorOpExists,
				),
			))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withTag("kept")),
				)),
			})
			expectInstanceTypes(kept, "spot-instance")
			Expect(rejected).To(BeEmpty())
			Expect(kept[0].Offerings).To(HaveLen(1))
			for _, o := range kept[0].Offerings {
				Expect(o.Requirements.Get("test.karpenter.sh/tag").Any()).To(Equal("kept"))
			}
		})
		It("should not filter when minValues is set", func() {
			f := filter.SpotOfferingFilter(scheduling.NewRequirements(
				scheduling.NewRequirement(
					karpv1.CapacityTypeLabelKey,
					corev1.NodeSelectorOpExists,
				),
				scheduling.NewRequirement(
					"test.karpenter.sh/tag",
					corev1.NodeSelectorOpExists,
				),
				scheduling.NewRequirementWithFlexibility(
					corev1.LabelInstanceTypeStable,
					corev1.NodeSelectorOpExists,
					lo.ToPtr(2),
				),
			))
			kept, rejected := f.FilterReject([]*cloudprovider.InstanceType{
				makeInstanceType("od-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeOnDemand, true, withPrice(5.0), withTag("kept")),
				)),
				makeInstanceType("expensive-spot-instance", withOfferings(
					makeOffering(karpv1.CapacityTypeSpot, true, withPrice(10.0), withTag("kept")),
				)),
			})
			expectInstanceTypes(kept, "od-instance", "expensive-spot-instance")
			Expect(rejected).To(BeEmpty())
			Expect(lo.Map(kept, func(it *cloudprovider.InstanceType, _ int) int {
				return len(it.Offerings)
			})).To(ConsistOf(1, 1))
			for _, it := range kept {
				for _, o := range it.Offerings {
					Expect(o.Requirements.Get("test.karpenter.sh/tag").Any()).To(Equal("kept"))
				}
			}
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
	rand.Shuffle(len(instanceType.Offerings), func(i, j int) {
		instanceType.Offerings[i], instanceType.Offerings[j] = instanceType.Offerings[j], instanceType.Offerings[i]
	})
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

func withCapacityReservationType(crt v1.CapacityReservationType) mockOfferingOptions {
	return func(o *cloudprovider.Offering) {
		if o.Requirements == nil {
			o.Requirements = scheduling.NewRequirements()
		}
		o.Requirements.Add(scheduling.NewRequirement(
			v1.LabelCapacityReservationType,
			corev1.NodeSelectorOpIn,
			string(crt),
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

func withTag(tag string) mockOfferingOptions {
	return func(o *cloudprovider.Offering) {
		if o.Requirements == nil {
			o.Requirements = scheduling.NewRequirements()
		}
		o.Requirements.Add(scheduling.NewRequirement(
			"test.karpenter.sh/tag",
			corev1.NodeSelectorOpIn,
			tag,
		))
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
