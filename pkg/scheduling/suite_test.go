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

package scheduling_test

import (
	"math"
	"testing"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/scheduling"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

func TestScheduling(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scheduling")
}

var _ = Describe("Scheduling", func() {
	Context("Compatibility", func() {
		It("should normalize aliased labels", func() {
			requirements := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelFailureDomainBetaZone, v1.NodeSelectorOpIn, "test"))
			Expect(requirements.Get(v1.LabelTopologyZone).Has("test")).To(BeTrue())
		})
		It("should ignore labels in IgnoredLabels", func() {
			for label := range v1alpha5.IgnoredLabels {
				requirements := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelFailureDomainBetaZone, v1.NodeSelectorOpIn, "test"))
				Expect(requirements.Has(label)).To(BeFalse())
			}
		})
		It("A should be compatible to B, <In, In> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "test", "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <In, In> operaton, no overlap", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "test", "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "bar"))
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <In, NotIn> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "test", "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <In, NotIn> operator, cancel out", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <In, Exists> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "test", "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <In, DoesNotExist> operator, conflicting", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "test", "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <In, Empty> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "foo"))
			B := scheduling.NewRequirements()
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <NotIn, In> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "test", "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <NotIn, In> operator, cancel out", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "foo"))
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <NotIn, NotIn> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "test", "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <NotIn, Exists> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "test", "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <NotIn, DoesNotExist> operator, conflicting", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "test", "foo"))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <NotIn, Empty> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			B := scheduling.NewRequirements()
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Exists, In> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Exists, NotIn> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Exists, Exists> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <Exists, DoesNotExist> operaton, conflicting", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <Exists, Empty> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			B := scheduling.NewRequirements()
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <DoesNotExist, In> operator, conflicting", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "foo"))
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <DoesNotExist, NotIn> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <DoesNotExists, Exists> operator, conflicting", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <DoesNotExist, DoesNotExists> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <DoesNotExist, Empty> operator", func() {
			A := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			B := scheduling.NewRequirements()
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Empty, In> operator, indirectional", func() {
			A := scheduling.NewRequirements()
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Empty, NotIn> operator", func() {
			A := scheduling.NewRequirements()
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpNotIn, "foo"))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Empty, Exists> operator, conflicting", func() {
			A := scheduling.NewRequirements()
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpExists))
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Empty, DoesNotExist> operator", func() {
			A := scheduling.NewRequirements()
			B := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpDoesNotExist))
			Expect(A.Compatible(B)).To(Succeed())
		})
	})
})

var _ = Describe("Requirement", func() {
	var fullSet, emptySet, setA, setAComplement, setB, setAB *scheduling.Requirement
	BeforeEach(func() {
		fullSet = scheduling.NewRequirement("key", v1.NodeSelectorOpExists)
		emptySet = scheduling.NewRequirement("key", v1.NodeSelectorOpDoesNotExist)
		setA = scheduling.NewRequirement("key", v1.NodeSelectorOpIn, "A")
		setB = scheduling.NewRequirement("key", v1.NodeSelectorOpIn, "B")
		setAB = scheduling.NewRequirement("key", v1.NodeSelectorOpIn, "A", "B")
		setAComplement = scheduling.NewRequirement("key", v1.NodeSelectorOpNotIn, "A")
		// upperBoundedSet = scheduling.NewRequirement("key", v1.NodeSelectorOpLt, "B")
		// lowerBoundedSet = scheduling.NewRequirement("key", v1.NodeSelectorOpGt, "A")
	})
	Context("Intersection", func() {
		It("A should not intersect with B", func() {
			Expect(setA.Intersection(setB)).To(Equal(emptySet))
		})
		It("A should not intersect with its own complement", func() {
			Expect(setA.Intersection(setAComplement)).To(Equal(emptySet))
		})
		It("A should intersect with AB", func() {
			Expect(setA.Intersection(setAB)).To(Equal(setA))
		})
		It("A intersect with full set should return itself", func() {
			Expect(setA.Intersection(fullSet)).To(Equal(setA))
		})
		It("A intersect with empty set should return empty", func() {
			Expect(setA.Intersection(emptySet)).To(Equal(emptySet))
		})
	})
	Context("Len", func() {
		It("size of AB should be 2", func() {
			Expect(setAB.Len()).To(Equal(2))
		})
		It("size of empty set should be 0", func() {
			Expect(emptySet.Len()).To(Equal(0))
		})
		It("size of full set should be MAX", func() {
			Expect(fullSet.Len()).To(Equal(math.MaxInt64))
		})
		It("A complement should not have A", func() {
			Expect(setAComplement.Has("A")).To(BeFalse())
		})
		It("A should not have B", func() {
			Expect(setA.Has("B")).To(BeFalse())
		})
	})
})
