package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/matchers/internal/miter"
)

var _ = Describe("HaveKeyWithValue", func() {
	var (
		stringKeys map[string]int
		intKeys    map[int]string
		objKeys    map[*myCustomType]*myCustomType

		customA *myCustomType
		customB *myCustomType
	)
	BeforeEach(func() {
		stringKeys = map[string]int{"foo": 2, "bar": 3}
		intKeys = map[int]string{2: "foo", 3: "bar"}

		customA = &myCustomType{s: "a", n: 2, f: 2.3, arr: []string{"ice", "cream"}}
		customB = &myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"cake"}}
		objKeys = map[*myCustomType]*myCustomType{customA: customA, customB: customA}
	})

	When("passed a map", func() {
		It("should do the right thing", func() {
			Expect(stringKeys).Should(HaveKeyWithValue("foo", 2))
			Expect(stringKeys).ShouldNot(HaveKeyWithValue("foo", 1))
			Expect(stringKeys).ShouldNot(HaveKeyWithValue("baz", 2))
			Expect(stringKeys).ShouldNot(HaveKeyWithValue("baz", 1))

			Expect(intKeys).Should(HaveKeyWithValue(2, "foo"))
			Expect(intKeys).ShouldNot(HaveKeyWithValue(4, "foo"))
			Expect(intKeys).ShouldNot(HaveKeyWithValue(2, "baz"))

			Expect(objKeys).Should(HaveKeyWithValue(customA, customA))
			Expect(objKeys).Should(HaveKeyWithValue(&myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"cake"}}, &myCustomType{s: "a", n: 2, f: 2.3, arr: []string{"ice", "cream"}}))
			Expect(objKeys).ShouldNot(HaveKeyWithValue(&myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"apple", "pie"}}, customA))
		})
	})

	When("passed a correctly typed nil", func() {
		It("should operate successfully on the passed in value", func() {
			var nilMap map[int]string
			Expect(nilMap).ShouldNot(HaveKeyWithValue("foo", "bar"))
		})
	})

	When("the passed in key or value is actually a matcher", func() {
		It("should pass each element through the matcher", func() {
			Expect(stringKeys).Should(HaveKeyWithValue(ContainSubstring("oo"), 2))
			Expect(intKeys).Should(HaveKeyWithValue(2, ContainSubstring("oo")))
			Expect(stringKeys).ShouldNot(HaveKeyWithValue(ContainSubstring("foobar"), 2))
		})

		It("should fail if the matcher ever fails", func() {
			actual := map[int]string{1: "a", 3: "b", 2: "c"}
			success, err := (&HaveKeyWithValueMatcher{Key: ContainSubstring("ar"), Value: 2}).Match(actual)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			otherActual := map[string]int{"a": 1, "b": 2, "c": 3}
			success, err = (&HaveKeyWithValueMatcher{Key: "a", Value: ContainSubstring("1")}).Match(otherActual)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	When("passed something that is not a map", func() {
		It("should error", func() {
			success, err := (&HaveKeyWithValueMatcher{Key: "foo", Value: "bar"}).Match([]string{"foo"})
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&HaveKeyWithValueMatcher{Key: "foo", Value: "bar"}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("iterators", func() {
		BeforeEach(func() {
			if !miter.HasIterators() {
				Skip("iterators not available")
			}
		})

		When("passed an iter.Seq2", func() {
			It("should do the right thing", func() {
				Expect(universalMapIter2).Should(HaveKeyWithValue("foo", 0))
				Expect(universalMapIter2).ShouldNot(HaveKeyWithValue("foo", 1))
				Expect(universalMapIter2).ShouldNot(HaveKeyWithValue("baz", 2))
				Expect(universalMapIter2).ShouldNot(HaveKeyWithValue("baz", 1))

				Expect(universalMapIter2).Should(HaveKeyWithValue("bar", 42))
				Expect(universalMapIter2).Should(HaveKeyWithValue("baz", 666))

				Expect(universalMapIter2).ShouldNot(HaveKeyWithValue("bar", "abc"))
				Expect(universalMapIter2).ShouldNot(HaveKeyWithValue(555, "abc"))
			})
		})

		When("passed a correctly typed nil", func() {
			It("should operate successfully on the passed in value", func() {
				var nilIter2 func(func(string, int) bool)
				Expect(nilIter2).ShouldNot(HaveKeyWithValue("foo", 0))
			})
		})

		When("the passed in key or value is actually a matcher", func() {
			It("should pass each element through the matcher", func() {
				Expect(universalMapIter2).Should(HaveKeyWithValue(ContainSubstring("oo"), BeNumerically("<", 1)))
				Expect(universalMapIter2).Should(HaveKeyWithValue(ContainSubstring("foo"), 0))
			})

			It("should fail if the matcher ever fails", func() {
				success, err := (&HaveKeyWithValueMatcher{Key: "bar", Value: ContainSubstring("argh")}).Match(universalMapIter2)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())

				success, err = (&HaveKeyWithValueMatcher{Key: "foo", Value: ContainSubstring("1")}).Match(universalMapIter2)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})

		When("passed something that is not an iter.Seq2", func() {
			It("should error", func() {
				success, err := (&HaveKeyWithValueMatcher{Key: "foo", Value: "bar"}).Match(universalIter)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})
	})
})
