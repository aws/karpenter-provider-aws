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

package scheduling

import (
	"math"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ = Describe("Requirement", func() {
	exists := NewRequirement("key", v1.NodeSelectorOpExists)
	doesNotExist := NewRequirement("key", v1.NodeSelectorOpDoesNotExist)
	inA := NewRequirement("key", v1.NodeSelectorOpIn, "A")
	inB := NewRequirement("key", v1.NodeSelectorOpIn, "B")
	inAB := NewRequirement("key", v1.NodeSelectorOpIn, "A", "B")
	notInA := NewRequirement("key", v1.NodeSelectorOpNotIn, "A")
	in1 := NewRequirement("key", v1.NodeSelectorOpIn, "1")
	in9 := NewRequirement("key", v1.NodeSelectorOpIn, "9")
	in19 := NewRequirement("key", v1.NodeSelectorOpIn, "1", "9")
	notIn12 := NewRequirement("key", v1.NodeSelectorOpNotIn, "1", "2")
	greaterThan1 := NewRequirement("key", v1.NodeSelectorOpGt, "1")
	greaterThan9 := NewRequirement("key", v1.NodeSelectorOpGt, "9")
	lessThan1 := NewRequirement("key", v1.NodeSelectorOpLt, "1")
	lessThan9 := NewRequirement("key", v1.NodeSelectorOpLt, "9")

	Context("NewRequirements", func() {
		It("should normalize labels", func() {
			nodeSelector := map[string]string{
				v1.LabelFailureDomainBetaZone:   "test",
				v1.LabelFailureDomainBetaRegion: "test",
				"beta.kubernetes.io/arch":       "test",
				"beta.kubernetes.io/os":         "test",
				v1.LabelInstanceType:            "test",
			}
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
			})
			for _, r := range []Requirements{
				NewLabelRequirements(nodeSelector),
				NewNodeSelectorRequirements(requirements...),
				NewPodRequirements(&v1.Pod{
					Spec: v1.PodSpec{
						NodeSelector: nodeSelector,
						Affinity: &v1.Affinity{
							NodeAffinity: &v1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution:  &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{{MatchExpressions: requirements}}},
								PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{{Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: requirements}}},
							},
						},
					},
				}),
			} {
				Expect(r.Keys().List()).To(ConsistOf(
					v1.LabelArchStable,
					v1.LabelOSStable,
					v1.LabelInstanceTypeStable,
					v1.LabelTopologyRegion,
					v1.LabelTopologyZone,
				))
			}
		})
	})

	Context("Intersection", func() {
		It("should intersect sets", func() {
			Expect(exists.Intersection(exists)).To(Equal(exists))
			Expect(exists.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(exists.Intersection(inA)).To(Equal(inA))
			Expect(exists.Intersection(inB)).To(Equal(inB))
			Expect(exists.Intersection(inAB)).To(Equal(inAB))
			Expect(exists.Intersection(notInA)).To(Equal(notInA))
			Expect(exists.Intersection(in1)).To(Equal(in1))
			Expect(exists.Intersection(in9)).To(Equal(in9))
			Expect(exists.Intersection(in19)).To(Equal(in19))
			Expect(exists.Intersection(notIn12)).To(Equal(notIn12))
			Expect(exists.Intersection(greaterThan1)).To(Equal(greaterThan1))
			Expect(exists.Intersection(greaterThan9)).To(Equal(greaterThan9))
			Expect(exists.Intersection(lessThan1)).To(Equal(lessThan1))
			Expect(exists.Intersection(lessThan9)).To(Equal(lessThan9))

			Expect(doesNotExist.Intersection(exists)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(inA)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(inB)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(inAB)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(notInA)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(in1)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(in9)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(in19)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(notIn12)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(greaterThan1)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(doesNotExist.Intersection(lessThan9)).To(Equal(doesNotExist))

			Expect(inA.Intersection(exists)).To(Equal(inA))
			Expect(inA.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(inA.Intersection(inA)).To(Equal(inA))
			Expect(inA.Intersection(inB)).To(Equal(doesNotExist))
			Expect(inA.Intersection(inAB)).To(Equal(inA))
			Expect(inA.Intersection(notInA)).To(Equal(doesNotExist))
			Expect(inA.Intersection(in1)).To(Equal(doesNotExist))
			Expect(inA.Intersection(in9)).To(Equal(doesNotExist))
			Expect(inA.Intersection(in19)).To(Equal(doesNotExist))
			Expect(inA.Intersection(notIn12)).To(Equal(inA))
			Expect(inA.Intersection(greaterThan1)).To(Equal(doesNotExist))
			Expect(inA.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(inA.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(inA.Intersection(lessThan9)).To(Equal(doesNotExist))

			Expect(inB.Intersection(exists)).To(Equal(inB))
			Expect(inB.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(inB.Intersection(inA)).To(Equal(doesNotExist))
			Expect(inB.Intersection(inB)).To(Equal(inB))
			Expect(inB.Intersection(inAB)).To(Equal(inB))
			Expect(inB.Intersection(notInA)).To(Equal(inB))
			Expect(inB.Intersection(in1)).To(Equal(doesNotExist))
			Expect(inB.Intersection(in9)).To(Equal(doesNotExist))
			Expect(inB.Intersection(in19)).To(Equal(doesNotExist))
			Expect(inB.Intersection(notIn12)).To(Equal(inB))
			Expect(inB.Intersection(greaterThan1)).To(Equal(doesNotExist))
			Expect(inB.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(inB.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(inB.Intersection(lessThan9)).To(Equal(doesNotExist))

			Expect(inAB.Intersection(exists)).To(Equal(inAB))
			Expect(inAB.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(inAB.Intersection(inA)).To(Equal(inA))
			Expect(inAB.Intersection(inB)).To(Equal(inB))
			Expect(inAB.Intersection(inAB)).To(Equal(inAB))
			Expect(inAB.Intersection(notInA)).To(Equal(inB))
			Expect(inAB.Intersection(in1)).To(Equal(doesNotExist))
			Expect(inAB.Intersection(in9)).To(Equal(doesNotExist))
			Expect(inAB.Intersection(in19)).To(Equal(doesNotExist))
			Expect(inAB.Intersection(notIn12)).To(Equal(inAB))
			Expect(inAB.Intersection(greaterThan1)).To(Equal(doesNotExist))
			Expect(inAB.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(inAB.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(inAB.Intersection(lessThan9)).To(Equal(doesNotExist))

			Expect(notInA.Intersection(exists)).To(Equal(notInA))
			Expect(notInA.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(notInA.Intersection(inA)).To(Equal(doesNotExist))
			Expect(notInA.Intersection(inB)).To(Equal(inB))
			Expect(notInA.Intersection(inAB)).To(Equal(inB))
			Expect(notInA.Intersection(notInA)).To(Equal(notInA))
			Expect(notInA.Intersection(in1)).To(Equal(in1))
			Expect(notInA.Intersection(in9)).To(Equal(in9))
			Expect(notInA.Intersection(in19)).To(Equal(in19))
			Expect(notInA.Intersection(notIn12)).To(Equal(&Requirement{Key: "key", complement: true, values: sets.NewString("A", "1", "2")}))
			Expect(notInA.Intersection(greaterThan1)).To(Equal(greaterThan1))
			Expect(notInA.Intersection(greaterThan9)).To(Equal(greaterThan9))
			Expect(notInA.Intersection(lessThan1)).To(Equal(lessThan1))
			Expect(notInA.Intersection(lessThan9)).To(Equal(lessThan9))

			Expect(in1.Intersection(exists)).To(Equal(in1))
			Expect(in1.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(in1.Intersection(inA)).To(Equal(doesNotExist))
			Expect(in1.Intersection(inB)).To(Equal(doesNotExist))
			Expect(in1.Intersection(inAB)).To(Equal(doesNotExist))
			Expect(in1.Intersection(notInA)).To(Equal(in1))
			Expect(in1.Intersection(in1)).To(Equal(in1))
			Expect(in1.Intersection(in9)).To(Equal(doesNotExist))
			Expect(in1.Intersection(in19)).To(Equal(in1))
			Expect(in1.Intersection(notIn12)).To(Equal(doesNotExist))
			Expect(in1.Intersection(greaterThan1)).To(Equal(doesNotExist))
			Expect(in1.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(in1.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(in1.Intersection(lessThan9)).To(Equal(in1))

			Expect(in9.Intersection(exists)).To(Equal(in9))
			Expect(in9.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(in9.Intersection(inA)).To(Equal(doesNotExist))
			Expect(in9.Intersection(inB)).To(Equal(doesNotExist))
			Expect(in9.Intersection(inAB)).To(Equal(doesNotExist))
			Expect(in9.Intersection(notInA)).To(Equal(in9))
			Expect(in9.Intersection(in1)).To(Equal(doesNotExist))
			Expect(in9.Intersection(in9)).To(Equal(in9))
			Expect(in9.Intersection(in19)).To(Equal(in9))
			Expect(in9.Intersection(notIn12)).To(Equal(in9))
			Expect(in9.Intersection(greaterThan1)).To(Equal(in9))
			Expect(in9.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(in9.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(in9.Intersection(lessThan9)).To(Equal(doesNotExist))

			Expect(in19.Intersection(exists)).To(Equal(in19))
			Expect(in19.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(in19.Intersection(inA)).To(Equal(doesNotExist))
			Expect(in19.Intersection(inB)).To(Equal(doesNotExist))
			Expect(in19.Intersection(inAB)).To(Equal(doesNotExist))
			Expect(in19.Intersection(notInA)).To(Equal(in19))
			Expect(in19.Intersection(in1)).To(Equal(in1))
			Expect(in19.Intersection(in9)).To(Equal(in9))
			Expect(in19.Intersection(in19)).To(Equal(in19))
			Expect(in19.Intersection(notIn12)).To(Equal(in9))
			Expect(in19.Intersection(greaterThan1)).To(Equal(in9))
			Expect(in19.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(in19.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(in19.Intersection(lessThan9)).To(Equal(in1))

			Expect(notIn12.Intersection(exists)).To(Equal(notIn12))
			Expect(notIn12.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(notIn12.Intersection(inA)).To(Equal(inA))
			Expect(notIn12.Intersection(inB)).To(Equal(inB))
			Expect(notIn12.Intersection(inAB)).To(Equal(inAB))
			Expect(notIn12.Intersection(notInA)).To(Equal(&Requirement{Key: "key", complement: true, values: sets.NewString("A", "1", "2")}))
			Expect(notIn12.Intersection(in1)).To(Equal(doesNotExist))
			Expect(notIn12.Intersection(in9)).To(Equal(in9))
			Expect(notIn12.Intersection(in19)).To(Equal(in9))
			Expect(notIn12.Intersection(notIn12)).To(Equal(notIn12))
			Expect(notIn12.Intersection(greaterThan1)).To(Equal(&Requirement{Key: "key", complement: true, greaterThan: greaterThan1.greaterThan, values: sets.NewString("2")}))
			Expect(notIn12.Intersection(greaterThan9)).To(Equal(&Requirement{Key: "key", complement: true, greaterThan: greaterThan9.greaterThan, values: sets.NewString()}))
			Expect(notIn12.Intersection(lessThan1)).To(Equal(&Requirement{Key: "key", complement: true, lessThan: lessThan1.lessThan, values: sets.NewString()}))
			Expect(notIn12.Intersection(lessThan9)).To(Equal(&Requirement{Key: "key", complement: true, lessThan: lessThan9.lessThan, values: sets.NewString("1", "2")}))

			Expect(greaterThan1.Intersection(exists)).To(Equal(greaterThan1))
			Expect(greaterThan1.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(greaterThan1.Intersection(inA)).To(Equal(doesNotExist))
			Expect(greaterThan1.Intersection(inB)).To(Equal(doesNotExist))
			Expect(greaterThan1.Intersection(inAB)).To(Equal(doesNotExist))
			Expect(greaterThan1.Intersection(notInA)).To(Equal(greaterThan1))
			Expect(greaterThan1.Intersection(in1)).To(Equal(doesNotExist))
			Expect(greaterThan1.Intersection(in9)).To(Equal(in9))
			Expect(greaterThan1.Intersection(in19)).To(Equal(in9))
			Expect(greaterThan1.Intersection(notIn12)).To(Equal(&Requirement{Key: "key", complement: true, greaterThan: greaterThan1.greaterThan, values: sets.NewString("2")}))
			Expect(greaterThan1.Intersection(greaterThan1)).To(Equal(greaterThan1))
			Expect(greaterThan1.Intersection(greaterThan9)).To(Equal(greaterThan9))
			Expect(greaterThan1.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(greaterThan1.Intersection(lessThan9)).To(Equal(&Requirement{Key: "key", complement: true, greaterThan: greaterThan1.greaterThan, lessThan: lessThan9.lessThan, values: sets.NewString()}))

			Expect(greaterThan9.Intersection(exists)).To(Equal(greaterThan9))
			Expect(greaterThan9.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(greaterThan9.Intersection(inA)).To(Equal(doesNotExist))
			Expect(greaterThan9.Intersection(inB)).To(Equal(doesNotExist))
			Expect(greaterThan9.Intersection(inAB)).To(Equal(doesNotExist))
			Expect(greaterThan9.Intersection(notInA)).To(Equal(greaterThan9))
			Expect(greaterThan9.Intersection(in1)).To(Equal(doesNotExist))
			Expect(greaterThan9.Intersection(in9)).To(Equal(doesNotExist))
			Expect(greaterThan9.Intersection(in19)).To(Equal(doesNotExist))
			Expect(greaterThan9.Intersection(notIn12)).To(Equal(greaterThan9))
			Expect(greaterThan9.Intersection(greaterThan1)).To(Equal(greaterThan9))
			Expect(greaterThan9.Intersection(greaterThan9)).To(Equal(greaterThan9))
			Expect(greaterThan9.Intersection(lessThan1)).To(Equal(doesNotExist))
			Expect(greaterThan9.Intersection(lessThan9)).To(Equal(doesNotExist))

			Expect(lessThan1.Intersection(exists)).To(Equal(lessThan1))
			Expect(lessThan1.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(inA)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(inB)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(inAB)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(notInA)).To(Equal(lessThan1))
			Expect(lessThan1.Intersection(in1)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(in9)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(in19)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(notIn12)).To(Equal(lessThan1))
			Expect(lessThan1.Intersection(greaterThan1)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(lessThan1.Intersection(lessThan1)).To(Equal(lessThan1))
			Expect(lessThan1.Intersection(lessThan9)).To(Equal(lessThan1))

			Expect(lessThan9.Intersection(exists)).To(Equal(lessThan9))
			Expect(lessThan9.Intersection(doesNotExist)).To(Equal(doesNotExist))
			Expect(lessThan9.Intersection(inA)).To(Equal(doesNotExist))
			Expect(lessThan9.Intersection(inB)).To(Equal(doesNotExist))
			Expect(lessThan9.Intersection(inAB)).To(Equal(doesNotExist))
			Expect(lessThan9.Intersection(notInA)).To(Equal(lessThan9))
			Expect(lessThan9.Intersection(in1)).To(Equal(in1))
			Expect(lessThan9.Intersection(in9)).To(Equal(doesNotExist))
			Expect(lessThan9.Intersection(in19)).To(Equal(in1))
			Expect(lessThan9.Intersection(notIn12)).To(Equal(&Requirement{Key: "key", complement: true, lessThan: lessThan9.lessThan, values: sets.NewString("1", "2")}))
			Expect(lessThan9.Intersection(greaterThan1)).To(Equal(&Requirement{Key: "key", complement: true, greaterThan: greaterThan1.greaterThan, lessThan: lessThan9.lessThan, values: sets.NewString()}))
			Expect(lessThan9.Intersection(greaterThan9)).To(Equal(doesNotExist))
			Expect(lessThan9.Intersection(lessThan1)).To(Equal(lessThan1))
			Expect(lessThan9.Intersection(lessThan9)).To(Equal(lessThan9))
		})
	})
	Context("Has", func() {
		It("should have the right values", func() {
			Expect(exists.Has("A")).To(BeTrue())
			Expect(doesNotExist.Has("A")).To(BeFalse())
			Expect(inA.Has("A")).To(BeTrue())
			Expect(inB.Has("A")).To(BeFalse())
			Expect(inAB.Has("A")).To(BeTrue())
			Expect(notInA.Has("A")).To(BeFalse())
			Expect(in1.Has("A")).To(BeFalse())
			Expect(in9.Has("A")).To(BeFalse())
			Expect(in19.Has("A")).To(BeFalse())
			Expect(notIn12.Has("A")).To(BeTrue())
			Expect(greaterThan1.Has("A")).To(BeFalse())
			Expect(greaterThan9.Has("A")).To(BeFalse())
			Expect(lessThan1.Has("A")).To(BeFalse())
			Expect(lessThan9.Has("A")).To(BeFalse())

			Expect(exists.Has("B")).To(BeTrue())
			Expect(doesNotExist.Has("B")).To(BeFalse())
			Expect(inA.Has("B")).To(BeFalse())
			Expect(inB.Has("B")).To(BeTrue())
			Expect(inAB.Has("B")).To(BeTrue())
			Expect(notInA.Has("B")).To(BeTrue())
			Expect(in1.Has("B")).To(BeFalse())
			Expect(in9.Has("B")).To(BeFalse())
			Expect(in19.Has("B")).To(BeFalse())
			Expect(notIn12.Has("B")).To(BeTrue())
			Expect(greaterThan1.Has("B")).To(BeFalse())
			Expect(greaterThan9.Has("B")).To(BeFalse())
			Expect(lessThan1.Has("B")).To(BeFalse())
			Expect(lessThan9.Has("B")).To(BeFalse())

			Expect(exists.Has("1")).To(BeTrue())
			Expect(doesNotExist.Has("1")).To(BeFalse())
			Expect(inA.Has("1")).To(BeFalse())
			Expect(inB.Has("1")).To(BeFalse())
			Expect(inAB.Has("1")).To(BeFalse())
			Expect(notInA.Has("1")).To(BeTrue())
			Expect(in1.Has("1")).To(BeTrue())
			Expect(in9.Has("1")).To(BeFalse())
			Expect(in19.Has("1")).To(BeTrue())
			Expect(notIn12.Has("1")).To(BeFalse())
			Expect(greaterThan1.Has("1")).To(BeFalse())
			Expect(greaterThan9.Has("1")).To(BeFalse())
			Expect(lessThan1.Has("1")).To(BeFalse())
			Expect(lessThan9.Has("1")).To(BeTrue())

			Expect(exists.Has("2")).To(BeTrue())
			Expect(doesNotExist.Has("2")).To(BeFalse())
			Expect(inA.Has("2")).To(BeFalse())
			Expect(inB.Has("2")).To(BeFalse())
			Expect(inAB.Has("2")).To(BeFalse())
			Expect(notInA.Has("2")).To(BeTrue())
			Expect(in1.Has("2")).To(BeFalse())
			Expect(in9.Has("2")).To(BeFalse())
			Expect(in19.Has("2")).To(BeFalse())
			Expect(notIn12.Has("2")).To(BeFalse())
			Expect(greaterThan1.Has("2")).To(BeTrue())
			Expect(greaterThan9.Has("2")).To(BeFalse())
			Expect(lessThan1.Has("2")).To(BeFalse())
			Expect(lessThan9.Has("2")).To(BeTrue())

			Expect(exists.Has("9")).To(BeTrue())
			Expect(doesNotExist.Has("9")).To(BeFalse())
			Expect(inA.Has("9")).To(BeFalse())
			Expect(inB.Has("9")).To(BeFalse())
			Expect(inAB.Has("9")).To(BeFalse())
			Expect(notInA.Has("9")).To(BeTrue())
			Expect(in1.Has("9")).To(BeFalse())
			Expect(in9.Has("9")).To(BeTrue())
			Expect(in19.Has("9")).To(BeTrue())
			Expect(notIn12.Has("9")).To(BeTrue())
			Expect(greaterThan1.Has("9")).To(BeTrue())
			Expect(greaterThan9.Has("9")).To(BeFalse())
			Expect(lessThan1.Has("9")).To(BeFalse())
			Expect(lessThan9.Has("9")).To(BeFalse())
		})
	})
	Context("Operator", func() {
		It("should return the right operator", func() {
			Expect(exists.Operator()).To(Equal(v1.NodeSelectorOpExists))
			Expect(doesNotExist.Operator()).To(Equal(v1.NodeSelectorOpDoesNotExist))
			Expect(inA.Operator()).To(Equal(v1.NodeSelectorOpIn))
			Expect(inB.Operator()).To(Equal(v1.NodeSelectorOpIn))
			Expect(inAB.Operator()).To(Equal(v1.NodeSelectorOpIn))
			Expect(notInA.Operator()).To(Equal(v1.NodeSelectorOpNotIn))
			Expect(in1.Operator()).To(Equal(v1.NodeSelectorOpIn))
			Expect(in9.Operator()).To(Equal(v1.NodeSelectorOpIn))
			Expect(in19.Operator()).To(Equal(v1.NodeSelectorOpIn))
			Expect(notIn12.Operator()).To(Equal(v1.NodeSelectorOpNotIn))
			Expect(greaterThan1.Operator()).To(Equal(v1.NodeSelectorOpExists))
			Expect(greaterThan9.Operator()).To(Equal(v1.NodeSelectorOpExists))
			Expect(lessThan1.Operator()).To(Equal(v1.NodeSelectorOpExists))
			Expect(lessThan9.Operator()).To(Equal(v1.NodeSelectorOpExists))
		})
	})
	Context("Len", func() {
		It("should have the correct length", func() {
			Expect(exists.Len()).To(Equal(math.MaxInt64))
			Expect(doesNotExist.Len()).To(Equal(0))
			Expect(inA.Len()).To(Equal(1))
			Expect(inB.Len()).To(Equal(1))
			Expect(inAB.Len()).To(Equal(2))
			Expect(notInA.Len()).To(Equal(math.MaxInt64 - 1))
			Expect(in1.Len()).To(Equal(1))
			Expect(in9.Len()).To(Equal(1))
			Expect(in19.Len()).To(Equal(2))
			Expect(notIn12.Len()).To(Equal(math.MaxInt64 - 2))
			Expect(greaterThan1.Len()).To(Equal(math.MaxInt64))
			Expect(greaterThan9.Len()).To(Equal(math.MaxInt64))
			Expect(lessThan1.Len()).To(Equal(math.MaxInt64))
			Expect(lessThan9.Len()).To(Equal(math.MaxInt64))
		})
	})

	Context("Any", func() {
		It("should return any", func() {
			Expect(exists.Any()).ToNot(BeEmpty())
			Expect(doesNotExist.Any()).To(BeEmpty())
			Expect(inA.Any()).To(Equal("A"))
			Expect(inB.Any()).To(Equal("B"))
			Expect(inAB.Any()).To(Or(Equal("A"), Equal("B")))
			Expect(notInA.Any()).ToNot(Or(BeEmpty(), Equal("A")))
			Expect(in1.Any()).To(Equal("1"))
			Expect(in9.Any()).To(Equal("9"))
			Expect(in19.Any()).To(Or(Equal("1"), Equal("9")))
			Expect(notIn12.Any()).ToNot(Or(BeEmpty(), Equal("1"), Equal("2")))
			Expect(strconv.Atoi(greaterThan1.Any())).To(BeNumerically(">=", 1))
			Expect(strconv.Atoi(greaterThan9.Any())).To(And(BeNumerically(">=", 9), BeNumerically("<", math.MaxInt64)))
			Expect(lessThan1.Any()).To(Equal("0"))
			Expect(strconv.Atoi(lessThan9.Any())).To(And(BeNumerically(">=", 0), BeNumerically("<", 9)))
		})
	})

	Context("String", func() {
		It("should print the right string", func() {
			Expect(exists.String()).To(Equal("key Exists"))
			Expect(doesNotExist.String()).To(Equal("key DoesNotExist"))
			Expect(inA.String()).To(Equal("key In [A]"))
			Expect(inB.String()).To(Equal("key In [B]"))
			Expect(inAB.String()).To(Equal("key In [A B]"))
			Expect(notInA.String()).To(Equal("key NotIn [A]"))
			Expect(in1.String()).To(Equal("key In [1]"))
			Expect(in9.String()).To(Equal("key In [9]"))
			Expect(in19.String()).To(Equal("key In [1 9]"))
			Expect(notIn12.String()).To(Equal("key NotIn [1 2]"))
			Expect(greaterThan1.String()).To(Equal("key Exists >1"))
			Expect(greaterThan9.String()).To(Equal("key Exists >9"))
			Expect(lessThan1.String()).To(Equal("key Exists <1"))
			Expect(lessThan9.String()).To(Equal("key Exists <9"))
			Expect(greaterThan1.Intersection(lessThan9).String()).To(Equal("key Exists >1 <9"))
			Expect(greaterThan9.Intersection(lessThan1).String()).To(Equal("key DoesNotExist"))
		})
	})
})
