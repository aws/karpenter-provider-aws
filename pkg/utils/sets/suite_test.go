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

	var fullSet, emptySet, setA, setAComplement, setB, setAB sets.Set
	BeforeEach(func() {
		fullSet = sets.NewComplementSet()
		emptySet = sets.NewSet()
		setA = sets.NewSet("A")
		setAComplement = sets.NewComplementSet("A")
		setB = sets.NewSet("B")
		setAB = sets.NewSet("A", "B")
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

	Context("Functional Correctness", func() {

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
		It("Should panic when call A' values", func() {
			Expect(func() { setAComplement.Values() }).To(Panic())
		})
	})
})
