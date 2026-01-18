package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/matchers/internal/miter"
)

var _ = Describe("HaveKey", func() {
	var (
		stringKeys map[string]int
		intKeys    map[int]string
		objKeys    map[*myCustomType]string

		customA *myCustomType
		customB *myCustomType
	)
	BeforeEach(func() {
		stringKeys = map[string]int{"foo": 2, "bar": 3}
		intKeys = map[int]string{2: "foo", 3: "bar"}

		customA = &myCustomType{s: "a", n: 2, f: 2.3, arr: []string{"ice", "cream"}}
		customB = &myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"cake"}}
		objKeys = map[*myCustomType]string{customA: "aardvark", customB: "kangaroo"}
	})

	When("passed a map", func() {
		It("should do the right thing", func() {
			Expect(stringKeys).Should(HaveKey("foo"))
			Expect(stringKeys).ShouldNot(HaveKey("baz"))

			Expect(intKeys).Should(HaveKey(2))
			Expect(intKeys).ShouldNot(HaveKey(4))

			Expect(objKeys).Should(HaveKey(customA))
			Expect(objKeys).Should(HaveKey(&myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"cake"}}))
			Expect(objKeys).ShouldNot(HaveKey(&myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"apple", "pie"}}))
		})
	})

	When("passed a correctly typed nil", func() {
		It("should operate successfully on the passed in value", func() {
			var nilMap map[int]string
			Expect(nilMap).ShouldNot(HaveKey("foo"))
		})
	})

	When("the passed in key is actually a matcher", func() {
		It("should pass each element through the matcher", func() {
			Expect(stringKeys).Should(HaveKey(ContainSubstring("oo")))
			Expect(stringKeys).ShouldNot(HaveKey(ContainSubstring("foobar")))
		})

		It("should fail if the matcher ever fails", func() {
			actual := map[int]string{1: "a", 3: "b", 2: "c"}
			success, err := (&HaveKeyMatcher{Key: ContainSubstring("ar")}).Match(actual)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	When("passed something that is not a map", func() {
		It("should error", func() {
			success, err := (&HaveKeyMatcher{Key: "foo"}).Match([]string{"foo"})
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&HaveKeyMatcher{Key: "foo"}).Match(nil)
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
				Expect(universalMapIter2).To(HaveKey("bar"))
				Expect(universalMapIter2).To(HaveKey(HavePrefix("ba")))
				Expect(universalMapIter2).NotTo(HaveKey("barrrrz"))
				Expect(universalMapIter2).NotTo(HaveKey(42))
			})
		})

		When("passed a correctly typed nil", func() {
			It("should operate successfully on the passed in value", func() {
				var nilIter2 func(func(string, int) bool)
				Expect(nilIter2).ShouldNot(HaveKey("foo"))
			})
		})

		When("the passed in key is actually a matcher", func() {
			It("should pass each element through the matcher", func() {
				Expect(universalMapIter2).Should(HaveKey(ContainSubstring("oo")))
				Expect(universalMapIter2).ShouldNot(HaveKey(ContainSubstring("foobar")))
			})

			It("should fail if the matcher ever fails", func() {
				success, err := (&HaveKeyMatcher{Key: ContainSubstring("ar")}).Match(universalIter2)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})

		When("passed something that is not an iter.Seq2", func() {
			It("should error", func() {
				success, err := (&HaveKeyMatcher{Key: "foo"}).Match(universalIter)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})
	})
})
