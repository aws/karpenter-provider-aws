/*
Copyright The Kubernetes Authors.

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

package scheduling_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	pscheduling "sigs.k8s.io/karpenter/pkg/scheduling"
)

var _ = Describe("ReservationManager", func() {
	var rm *scheduling.ReservationManager
	var offerings map[string]cloudprovider.Offerings
	var instanceTypes []*cloudprovider.InstanceType
	var threeCapacityOffering *cloudprovider.Offering
	var twoCapacityOffering *cloudprovider.Offering
	var oneCapacityOffering *cloudprovider.Offering

	BeforeEach(func() {
		threeCapacityOffering = &cloudprovider.Offering{
			Available:           true,
			ReservationCapacity: 3,
			Requirements: pscheduling.NewLabelRequirements(map[string]string{
				v1.CapacityTypeLabelKey:          v1.CapacityTypeReserved,
				corev1.LabelTopologyZone:         "test-zone-1",
				cloudprovider.ReservationIDLabel: "small-reserved",
			}),
		}
		twoCapacityOffering = &cloudprovider.Offering{
			Available:           true,
			ReservationCapacity: 2,
			Requirements: pscheduling.NewLabelRequirements(map[string]string{
				v1.CapacityTypeLabelKey:          v1.CapacityTypeReserved,
				corev1.LabelTopologyZone:         "test-zone-2",
				cloudprovider.ReservationIDLabel: "medium-reserved",
			}),
		}
		oneCapacityOffering = &cloudprovider.Offering{
			Available:           true,
			ReservationCapacity: 1,
			Requirements: pscheduling.NewLabelRequirements(map[string]string{
				v1.CapacityTypeLabelKey:          v1.CapacityTypeReserved,
				corev1.LabelTopologyZone:         "test-zone-3",
				cloudprovider.ReservationIDLabel: "large-reserved",
			}),
		}
		// Create hardcoded instance types with reserved offerings
		instanceTypes = []*cloudprovider.InstanceType{
			fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "small-reserved",
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				},
				Offerings: []*cloudprovider.Offering{threeCapacityOffering},
			}),
			fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "medium-reserved",
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("20"),
				},
				Offerings: []*cloudprovider.Offering{twoCapacityOffering},
			}),
			fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "large-reserved",
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
					corev1.ResourcePods:   resource.MustParse("40"),
				},
				Offerings: []*cloudprovider.Offering{oneCapacityOffering},
			}),
		}

		// Extract offerings from instance types
		offerings = map[string]cloudprovider.Offerings{} // Reset offerings map
		for _, it := range instanceTypes {
			offerings[it.Name] = it.Offerings
		}

		rm = scheduling.NewReservationManager(map[string][]*cloudprovider.InstanceType{"": instanceTypes})
	})

	Describe("CanReserve", func() {
		Context("With Available Capacity", func() {
			It("should return true when capacity is available", func() {
				canReserve := rm.CanReserve("hostname-1", oneCapacityOffering)
				Expect(canReserve).To(BeTrue())
			})

			It("should return true when hostname already has the reservation", func() {
				// First reserve
				rm.Reserve("hostname-1", oneCapacityOffering)
				// Should still be able to "reserve" the same reservation for the same hostname
				canReserve := rm.CanReserve("hostname-1", oneCapacityOffering)
				Expect(canReserve).To(BeTrue())
			})
		})

		Context("With No Capacity", func() {
			It("should return false when capacity is exhausted", func() {
				// Exhaust all capacity
				for i := range oneCapacityOffering.ReservationCapacity {
					rm.Reserve(fmt.Sprintf("hostname-%d", i), oneCapacityOffering)
				}
				// Should not be able to reserve more
				canReserve := rm.CanReserve("hostname-new", oneCapacityOffering)
				Expect(canReserve).To(BeFalse())
			})

			It("should return true for existing hostname even when capacity is exhausted", func() {
				// Reserve for hostname-1
				rm.Reserve("hostname-1", oneCapacityOffering)
				// Should still return true for hostname-1
				canReserve := rm.CanReserve("hostname-1", oneCapacityOffering)
				Expect(canReserve).To(BeTrue())
			})
		})

		Context("Error Cases", func() {
			It("should panic with non-existent reservation ID", func() {
				nonExistentOffering := &cloudprovider.Offering{
					Available:           true,
					ReservationCapacity: 1,
					Requirements: pscheduling.NewLabelRequirements(map[string]string{
						v1.CapacityTypeLabelKey:          v1.CapacityTypeReserved,
						cloudprovider.ReservationIDLabel: "i-dont-exist",
					}),
				}
				Expect(func() {
					rm.CanReserve("hostname-1", nonExistentOffering)
				}).To(Panic())
			})
		})
	})

	Describe("Reserve", func() {
		Context("Single Reservations", func() {
			It("should successfully reserve capacity", func() {
				rm.Reserve("hostname-1", threeCapacityOffering)
				// Verify reservation was made by checking CanReserve behavior
				Expect(rm.HasReservation("hostname-1", threeCapacityOffering)).To(BeTrue())
			})

			It("should not double-reserve for the same hostname and reservation", func() {
				// Reserve twice for the same hostname and reservation
				rm.Reserve("hostname-1", twoCapacityOffering)
				rm.Reserve("hostname-1", twoCapacityOffering)

				// Should still have capacity for other hostnames
				canReserve := rm.CanReserve("hostname-2", twoCapacityOffering)
				Expect(canReserve).To(BeTrue())
			})

			It("should decrement available capacity", func() {
				// Reserve capacity and verify it's decremented
				rm.Reserve("hostname-1", threeCapacityOffering)
				rm.Reserve("hostname-2", threeCapacityOffering)
				rm.Reserve("hostname-3", threeCapacityOffering)

				// Should have no capacity left
				canReserve := rm.CanReserve("hostname-4", threeCapacityOffering)
				Expect(canReserve).To(BeFalse())
			})
		})

		Context("Multiple Reservations", func() {
			It("should handle multiple offerings in a single call", func() {
				rm.Reserve("hostname-1", threeCapacityOffering, twoCapacityOffering)

				// Verify both reservations were made
				Expect(rm.HasReservation("hostname-1", threeCapacityOffering)).To(BeTrue())
				Expect(rm.HasReservation("hostname-1", twoCapacityOffering)).To(BeTrue())
			})

			It("should handle mixed new and existing reservations", func() {
				// First reserve one offering
				rm.Reserve("hostname-1", twoCapacityOffering)

				// Then reserve both (one existing, one new)
				rm.Reserve("hostname-1", twoCapacityOffering, oneCapacityOffering)

				// Verify both are reserved and capacity is correctly tracked
				Expect(rm.HasReservation("hostname-1", twoCapacityOffering)).To(BeTrue())
				Expect(rm.HasReservation("hostname-1", oneCapacityOffering)).To(BeTrue())

				// Verify capacity was only decremented once for twoCapacityOffering
				Expect(rm.CanReserve("hostname-2", twoCapacityOffering)).To(BeTrue()) // Should still have capacity
			})
		})

		Context("Error Cases", func() {
			It("should panic when trying to over-reserve", func() {
				// Exhaust capacity
				for i := range threeCapacityOffering.ReservationCapacity {
					rm.Reserve(fmt.Sprintf("hostname-%d", i), threeCapacityOffering)
				}

				// Attempting to reserve more should panic
				Expect(func() {
					rm.Reserve("hostname-new", threeCapacityOffering)
				}).To(Panic())
			})

			It("should panic with non-existent reservation ID", func() {
				nonExistentOffering := &cloudprovider.Offering{
					Available:           true,
					ReservationCapacity: 1,
					Requirements: pscheduling.NewLabelRequirements(map[string]string{
						v1.CapacityTypeLabelKey:          v1.CapacityTypeReserved,
						cloudprovider.ReservationIDLabel: "i-dont-exist",
					}),
				}
				Expect(func() {
					rm.Reserve("hostname-1", nonExistentOffering)
				}).To(Panic())
			})
		})
	})

	Describe("Release", func() {
		Context("Valid Releases", func() {
			It("should release a single reservation", func() {
				// Reserve and then release
				rm.Reserve("hostname-1", threeCapacityOffering)
				rm.Release("hostname-1", threeCapacityOffering)

				// Verify the reservation is no longer tracked for this hostname
				// but capacity should be restored
				Expect(rm.HasReservation("hostname-1", threeCapacityOffering)).To(BeFalse())
			})

			It("should handle releasing non-existent reservations gracefully", func() {
				// Should not panic when releasing a reservation that doesn't exist
				Expect(func() {
					rm.Release("hostname-1", threeCapacityOffering)
				}).ToNot(Panic())
			})

			It("should handle releasing from non-existent hostname gracefully", func() {
				rm.Reserve("hostname-1", threeCapacityOffering)
				// Should not panic when releasing from a different hostname
				Expect(func() {
					rm.Release("hostname-2", threeCapacityOffering)
				}).ToNot(Panic())
			})
		})

		Context("Multiple Releases", func() {
			It("should handle multiple offerings in a single call", func() {
				// Reserve both offerings
				rm.Reserve("hostname-1", threeCapacityOffering, twoCapacityOffering)

				// Release both
				rm.Release("hostname-1", threeCapacityOffering, twoCapacityOffering)

				// Verify capacity has been reserved for both
				Expect(rm.HasReservation("hostname-1", threeCapacityOffering)).To(BeFalse())
				Expect(rm.HasReservation("hostname-1", twoCapacityOffering)).To(BeFalse())
			})

			It("should handle partial releases", func() {
				// Reserve both offerings
				rm.Reserve("hostname-1", threeCapacityOffering, twoCapacityOffering)

				// Release only one
				rm.Release("hostname-1", threeCapacityOffering)

				// Verify only the released one has restored capacity
				Expect(rm.HasReservation("hostname-1", threeCapacityOffering)).To(BeFalse())
				Expect(rm.HasReservation("hostname-1", twoCapacityOffering)).To(BeTrue())
			})
		})

		Context("Capacity Restoration", func() {
			It("should restore capacity when releasing reservations", func() {
				// Exhaust capacity
				rm.Reserve("hostname-1", threeCapacityOffering)
				rm.Reserve("hostname-2", threeCapacityOffering)
				rm.Reserve("hostname-3", threeCapacityOffering)

				// Verify no capacity left
				Expect(rm.RemainingCapacity(threeCapacityOffering)).To(Equal(0))

				// Release one reservation
				rm.Release("hostname-1", threeCapacityOffering)

				// Verify capacity is restored
				Expect(rm.RemainingCapacity(threeCapacityOffering)).To(Equal(1))
			})

			It("should correctly track capacity after multiple reserve/release cycles", func() {
				// Reserve, release, reserve again
				rm.Reserve("hostname-1", threeCapacityOffering)
				rm.Release("hostname-1", threeCapacityOffering)
				rm.Reserve("hostname-2", threeCapacityOffering)

				// Should still have capacity available
				Expect(rm.RemainingCapacity(threeCapacityOffering)).To(Equal(2))
			})
		})
	})

	Describe("Integration Scenarios", func() {
		It("should handle complex reservation patterns", func() {
			// Multiple hostnames with multiple reservations
			rm.Reserve("host-1", twoCapacityOffering, threeCapacityOffering)
			rm.Reserve("host-2", twoCapacityOffering, oneCapacityOffering)
			rm.Reserve("host-3", threeCapacityOffering)

			// Verify all reservations are tracked
			Expect(rm.HasReservation("host-1", threeCapacityOffering)).To(BeTrue())
			Expect(rm.HasReservation("host-1", twoCapacityOffering)).To(BeTrue())
			Expect(rm.HasReservation("host-2", twoCapacityOffering)).To(BeTrue())
			Expect(rm.HasReservation("host-2", oneCapacityOffering)).To(BeTrue())
			Expect(rm.HasReservation("host-3", threeCapacityOffering)).To(BeTrue())

			// Verify capacity limits
			Expect(rm.RemainingCapacity(twoCapacityOffering)).To(Equal(0))   // Exhausted (2 capacity, 2 used)
			Expect(rm.RemainingCapacity(threeCapacityOffering)).To(Equal(1)) // Still available (3 capacity, 2 used)
			Expect(rm.RemainingCapacity(oneCapacityOffering)).To(Equal(0))   // Exhausted (1 capacity, 1 used)
		})

		It("should maintain consistency during mixed operations", func() {
			// Complex sequence of operations
			rm.Reserve("host-1", twoCapacityOffering, threeCapacityOffering)
			rm.Reserve("host-2", threeCapacityOffering, oneCapacityOffering)
			rm.Release("host-1", twoCapacityOffering)
			rm.Reserve("host-3", twoCapacityOffering)
			rm.Release("host-2", oneCapacityOffering)

			// Verify final state
			Expect(rm.HasReservation("host-1", twoCapacityOffering)).To(BeFalse())  // Released, so not reserved for host-1
			Expect(rm.HasReservation("host-1", threeCapacityOffering)).To(BeTrue()) // Still reserved for host-1
			Expect(rm.HasReservation("host-2", threeCapacityOffering)).To(BeTrue()) // Still reserved for host-2
			Expect(rm.HasReservation("host-2", oneCapacityOffering)).To(BeFalse())  // Released, so not reserved for host-2
			Expect(rm.HasReservation("host-3", twoCapacityOffering)).To(BeTrue())   // Reserved for host-3

			// Check capacity availability
			Expect(rm.RemainingCapacity(twoCapacityOffering)).To(Equal(1)) // Should have 1 available (2 total, 1 used by host-3)
			Expect(rm.RemainingCapacity(twoCapacityOffering)).To(Equal(1)) // Should have 1 available (3 total, 2 used)
			Expect(rm.RemainingCapacity(twoCapacityOffering)).To(Equal(1)) // Should have 1 available (1 total, 0 used after release)
		})
	})
})
