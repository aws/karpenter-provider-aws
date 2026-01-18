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

package v1_test

import (
	"math"
	"strings"
	"time"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"

	. "sigs.k8s.io/karpenter/pkg/apis/v1"
)

var _ = Describe("Budgets", func() {
	var nodePool *NodePool
	var budgets []Budget
	var fakeClock *clock.FakeClock
	var allKnownDisruptionReasons []DisruptionReason

	BeforeEach(func() {
		// Set the time to the middle of the year of 2000, the best year ever
		fakeClock = clock.NewFakeClock(time.Date(2000, time.June, 15, 12, 30, 30, 0, time.UTC))
		budgets = []Budget{
			{
				Nodes:    "10",
				Schedule: lo.ToPtr("* * * * *"),
				Duration: lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))}),
			},
			{
				Nodes:    "100",
				Schedule: lo.ToPtr("* * * * *"),
				Duration: lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))}),
			},
			{
				Nodes:    "100%",
				Schedule: lo.ToPtr("* * * * *"),
				Duration: lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))}),
			},
			{
				Reasons: []DisruptionReason{
					DisruptionReasonDrifted,
					DisruptionReasonUnderutilized,
				},
				Nodes:    "15",
				Schedule: lo.ToPtr("* * * * *"),
				Duration: lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))}),
			},
			{
				Reasons: []DisruptionReason{
					DisruptionReasonDrifted,
				},
				Nodes:    "5",
				Schedule: lo.ToPtr("* * * * *"),
				Duration: lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))}),
			},
			{
				Reasons: []DisruptionReason{
					DisruptionReasonUnderutilized,
					DisruptionReasonDrifted,
					DisruptionReasonEmpty,
				},
				Nodes:    "0",
				Schedule: lo.ToPtr("@weekly"),
				Duration: lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))}),
			},
		}
		nodePool = &NodePool{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: NodePoolSpec{
				Disruption: Disruption{
					Budgets: budgets,
				},
			},
		}
		allKnownDisruptionReasons = []DisruptionReason{
			DisruptionReasonEmpty,
			DisruptionReasonUnderutilized,
			DisruptionReasonDrifted,
		}
	})

	Context("GetAllowedDisruptionsByReason", func() {
		It("should return 0 for all reasons if a budget is active for all reasons", func() {
			budgets[5].Schedule = lo.ToPtr("* * * * *")
			budgets[5].Duration = lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))})

			for _, reason := range allKnownDisruptionReasons {
				allowedDisruption, err := nodePool.GetAllowedDisruptionsByReason(fakeClock, 100, reason)
				Expect(err).To(BeNil())
				Expect(allowedDisruption).To(Equal(0))
			}
		})

		It("should return MaxInt32 for all reasons when there are no active budgets", func() {
			for i := range budgets {
				budgets[i].Schedule = lo.ToPtr("@yearly")
				budgets[i].Duration = lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))})
			}

			// All budgets should have unbounded disruptions when inactive
			for _, reason := range allKnownDisruptionReasons {
				allowedDisruption, err := nodePool.GetAllowedDisruptionsByReason(fakeClock, 100, reason)
				Expect(err).To(BeNil())
				Expect(allowedDisruption).To(Equal(math.MaxInt32))
			}
		})

		It("should ignore reason-defined budgets when inactive", func() {
			budgets[3].Schedule = lo.ToPtr("@yearly")
			budgets[4].Schedule = lo.ToPtr("@yearly")

			for _, reason := range allKnownDisruptionReasons {
				allowedDisruption, err := nodePool.GetAllowedDisruptionsByReason(fakeClock, 100, reason)
				Expect(err).To(BeNil())
				Expect(allowedDisruption).To(Equal(10))
			}
		})

		It("should return the budget for all disruption reasons when undefined", func() {
			nodePool.Spec.Disruption.Budgets = budgets[:1]
			Expect(len(nodePool.Spec.Disruption.Budgets)).To(Equal(1))
			Expect(len(budgets[0].Reasons)).To(Equal(0))

			for _, reason := range allKnownDisruptionReasons {
				allowedDisruption, err := nodePool.GetAllowedDisruptionsByReason(fakeClock, 100, reason)
				Expect(err).To(BeNil())
				Expect(allowedDisruption).To(Equal(10))
			}
		})

		It("should get the minimum budget for each reason", func() {

			nodePool.Spec.Disruption.Budgets = append(nodePool.Spec.Disruption.Budgets,
				[]Budget{
					{
						Schedule: lo.ToPtr("* * * * *"),
						Nodes:    "4",
						Duration: lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))}),
						Reasons: []DisruptionReason{
							DisruptionReasonEmpty,
						},
					},
				}...)

			emptyAllowedDisruption, err := nodePool.GetAllowedDisruptionsByReason(fakeClock, 100, DisruptionReasonEmpty)
			Expect(err).To(BeNil())
			Expect(emptyAllowedDisruption).To(Equal(4))
			driftedAllowedDisruption, err := nodePool.GetAllowedDisruptionsByReason(fakeClock, 100, DisruptionReasonDrifted)
			Expect(err).To(BeNil())
			Expect(driftedAllowedDisruption).To(Equal(5))
			// The budget where reason == nil overrides the budget with a specified reason
			underutilizedAllowedDisruption, err := nodePool.GetAllowedDisruptionsByReason(fakeClock, 100, DisruptionReasonUnderutilized)
			Expect(err).To(BeNil())
			Expect(underutilizedAllowedDisruption).To(Equal(10))
		})

	})

	Context("AllowedDisruptions", func() {
		It("should return zero values if a schedule is invalid", func() {
			budgets[0].Schedule = lo.ToPtr("@wrongly")
			val, err := budgets[0].GetAllowedDisruptions(fakeClock, 100)
			Expect(err).ToNot(Succeed())
			Expect(val).To(BeNumerically("==", 0))
		})
		It("should return zero values if a nodes value is invalid", func() {
			budgets[0].Nodes = "1000a%"
			val, err := budgets[0].GetAllowedDisruptions(fakeClock, 100)
			Expect(err).ToNot(Succeed())
			Expect(val).To(BeNumerically("==", 0))
		})
		It("should return MaxInt32 when a budget is inactive", func() {
			budgets[0].Schedule = lo.ToPtr("@yearly")
			budgets[0].Duration = lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("1h"))})
			val, err := budgets[0].GetAllowedDisruptions(fakeClock, 100)
			Expect(err).To(Succeed())
			Expect(val).To(BeNumerically("==", math.MaxInt32))
		})
		It("should return the int value when a budget is active", func() {
			val, err := budgets[0].GetAllowedDisruptions(fakeClock, 100)
			Expect(err).To(Succeed())
			Expect(val).To(BeNumerically("==", 10))
		})
		It("should return the string value when a budget is active", func() {
			val, err := budgets[2].GetAllowedDisruptions(fakeClock, 100)
			Expect(err).To(Succeed())
			Expect(val).To(BeNumerically("==", 100))
		})
	})

	Context("IsActive", func() {
		It("should always consider a schedule and time in UTC", func() {
			// Set the time to start of June 2000 in a time zone 1 hour ahead of UTC
			fakeClock = clock.NewFakeClock(time.Date(2000, time.June, 0, 0, 0, 0, 0, time.FixedZone("fake-zone", 3600)))
			budgets[0].Schedule = lo.ToPtr("@daily")
			budgets[0].Duration = lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("30m"))})
			// IsActive should use UTC, not the location of the clock that's inputted.
			active, err := budgets[0].IsActive(fakeClock)
			Expect(err).To(Succeed())
			Expect(active).To(BeFalse())
		})
		It("should return that a schedule is active when schedule and duration are nil", func() {
			budgets[0].Schedule = nil
			budgets[0].Duration = nil
			active, err := budgets[0].IsActive(fakeClock)
			Expect(err).To(Succeed())
			Expect(active).To(BeTrue())
		})
		It("should return that a schedule is active", func() {
			active, err := budgets[0].IsActive(fakeClock)
			Expect(err).To(Succeed())
			Expect(active).To(BeTrue())
		})
		It("should return that a schedule is inactive", func() {
			budgets[0].Schedule = lo.ToPtr("@yearly")
			active, err := budgets[0].IsActive(fakeClock)
			Expect(err).To(Succeed())
			Expect(active).To(BeFalse())
		})
		It("should return that a schedule is active when the schedule hit is in the middle of the duration", func() {
			// Set the date to the start of the year 1000, the best year ever
			fakeClock = clock.NewFakeClock(time.Date(1000, time.January, 1, 12, 0, 0, 0, time.UTC))
			budgets[0].Schedule = lo.ToPtr("@yearly")
			budgets[0].Duration = lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("24h"))})
			active, err := budgets[0].IsActive(fakeClock)
			Expect(err).To(Succeed())
			Expect(active).To(BeTrue())
		})
		It("should return that a schedule is active when the duration is longer than the recurrence", func() {
			// Set the date to the first monday in 2024, the best year ever
			fakeClock = clock.NewFakeClock(time.Date(2024, time.January, 7, 0, 0, 0, 0, time.UTC))
			budgets[0].Schedule = lo.ToPtr("@daily")
			budgets[0].Duration = lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("48h"))})
			active, err := budgets[0].IsActive(fakeClock)
			Expect(err).To(Succeed())
			Expect(active).To(BeTrue())
		})
		It("should return that a schedule is inactive when the schedule hit is after the duration", func() {
			// Set the date to the first monday in 2024, the best year ever
			fakeClock = clock.NewFakeClock(time.Date(2024, time.January, 7, 0, 0, 0, 0, time.UTC))
			budgets[0].Schedule = lo.ToPtr("30 6 * * SUN")
			budgets[0].Duration = lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("6h"))})
			active, err := budgets[0].IsActive(fakeClock)
			Expect(err).To(Succeed())
			Expect(active).ToNot(BeTrue())
		})
		It("should return an error indicating why the cron fails to parse", func() {
			// Set the date to the first monday in 2024, the best year ever
			fakeClock = clock.NewFakeClock(time.Date(2024, time.January, 7, 0, 0, 0, 0, time.UTC))
			budgets[0].Schedule = lo.ToPtr("0 0 * * tue-mon")
			budgets[0].Duration = lo.ToPtr(metav1.Duration{Duration: lo.Must(time.ParseDuration("6h"))})
			_, err := budgets[0].IsActive(fakeClock)
			Expect(err).To(MatchError(ContainSubstring("beginning of range (2) beyond end of range (1)")))
		})
	})
})
