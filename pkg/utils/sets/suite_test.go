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

package sets_test

import (
	"math"
	"testing"

	"github.com/aws/karpenter/pkg/utils/sets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sets Suite")
}

var _ = Describe("Set", func() {

	var fullSet, emptySet, setA, setAComplement, setB, setAB *sets.Set
	BeforeEach(func() {
		fullSet = sets.NewSet(true)
		emptySet = sets.NewSet(false)
		setA = sets.NewSet(false, "A")
		setAComplement = sets.NewSet(true, "A")
		setB = sets.NewSet(false, "B")
		setAB = sets.NewSet(false, "A", "B")
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

	Context("Different", func() {
		It("A differece B should be A", func() {
			Expect(setA.Difference(setB)).To(Equal(setA))
		})
		It("A difference with its own complement should be A", func() {
			Expect(setA.Difference(setAComplement)).To(Equal(setA))
		})
		It("AB difference B should be A", func() {
			Expect(setAB.Difference(setB)).To(Equal(setA))
		})
		It("A difference with full set should return empty", func() {
			Expect(setA.Difference(fullSet)).To(Equal(emptySet))
		})
		It("A difference with empty set should return itself", func() {
			Expect(setA.Difference(emptySet)).To(Equal(setA))
		})
	})

	Context("Union", func() {
		It("A union B should be AB", func() {
			Expect(setA.Union(setB)).To(Equal(setAB))
		})
		It("A union with its own complement should return full set", func() {
			Expect(setA.Union(setAComplement)).To(Equal(fullSet))
		})
		It("AB union with B should return AB", func() {
			Expect(setAB.Union(setB)).To(Equal(setAB))
		})
		It("A union with full set should return fullSet", func() {
			Expect(setA.Union(fullSet)).To(Equal(fullSet))
		})
		It("A union with empty set should return A", func() {
			Expect(setA.Union(emptySet)).To(Equal(setA))
		})
	})
	Context("Functional Correctness", func() {
		It("A should equal A", func() {
			Expect(setA.Equal(setA)).To(BeTrue())
		})
		It("A should not equal B", func() {
			Expect(setA.Equal(setB)).To(BeFalse())
		})
		It("A should not equal AB", func() {
			Expect(setA.Equal(setAB)).To(BeFalse())
		})
		It("A should not equal A complement", func() {
			Expect(setA.Equal(setAComplement)).To(BeFalse())
		})

		It("size of A should be 1", func() {
			Expect(setA.Len()).To(Equal(1))
		})
		It("size of AB should be 2", func() {
			Expect(setAB.Len()).To(Equal(2))
		})
		It("size of empty set should be 0", func() {
			Expect(emptySet.Len()).To(Equal(0))
		})
		It("size of full set should be MAX", func() {
			Expect(fullSet.Len()).To(Equal(math.MaxInt64))
		})
		It("A should have A", func() {
			Expect(setA.Has("A")).To(BeTrue())
		})
		It("A complement should not have A", func() {
			Expect(setAComplement.Has("A")).To(BeFalse())
		})
		It("A should not have B", func() {
			Expect(setA.Has("B")).To(BeFalse())
		})
		It("A should have either A or B", func() {
			Expect(setA.HasAny("A", "B")).To(BeTrue())
		})
		It("A complement should have either A or B", func() {
			Expect(setAComplement.HasAny("A", "B")).To(BeTrue())
		})
		It("A should not have either C or D", func() {
			Expect(setA.HasAny("C", "D")).To(BeFalse())
		})
		It("A should not have both A and B", func() {
			Expect(setA.HasAll("A", "B")).To(BeFalse())
		})
		It("A complement should not have both A and B", func() {
			Expect(setAComplement.HasAll("A", "B")).To(BeFalse())
		})
		It("AB should have both A and B", func() {
			Expect(setAB.HasAll("A", "B")).To(BeTrue())
		})
		It("A Insert B should return AB", func() {
			Expect(setA.Insert("B")).To(Equal(setAB))
		})
		It("A complement Insert A should return full set", func() {
			Expect(setAComplement.Insert("A")).To(Equal(fullSet))
		})
		It("A should be able to call values", func() {
			_, err := setA.Values()
			Expect(err).To(Succeed())
		})
		It("A should have values A", func() {
			values, _ := setA.Values()
			Expect(values).To(Equal([]string{"A"}))
		})
	})
})
