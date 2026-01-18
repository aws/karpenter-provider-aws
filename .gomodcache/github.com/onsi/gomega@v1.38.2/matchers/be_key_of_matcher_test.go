package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeKeyOf", func() {
	When("passed a map", func() {
		It("should do the right thing", func() {
			Expect(2).Should(BeKeyOf(map[int]bool{1: true, 2: false}))
			Expect(3).ShouldNot(BeKeyOf(map[int]bool{1: true, 2: false}))

			var mone map[int]bool
			Expect(42).ShouldNot(BeKeyOf(mone))

			two := 2
			Expect(&two).Should(BeKeyOf(map[*int]bool{&two: true, nil: false}))
		})
	})

	When("passed a correctly typed nil", func() {
		It("should operate successfully on the passed in value", func() {
			two := 2
			Expect((*int)(nil)).Should(BeKeyOf(map[*int]bool{&two: true, nil: false}))

			one := 1
			Expect((*int)(nil)).ShouldNot(BeKeyOf(map[*int]bool{&two: true, &one: false}))
		})
	})

	When("passed an unsupported type", func() {
		It("should error", func() {
			success, err := (&BeKeyOfMatcher{Map: []any{0}}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeKeyOfMatcher{Map: nil}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	It("builds failure message", func() {
		actual := BeKeyOf(map[int]bool{1: true, 2: false}).FailureMessage(42)
		Expect(actual).To(MatchRegexp("Expected\n    <int>: 42\nto be a key of\n    <\\[\\]bool | len:2, cap:2>: \\[(true, false)|(false, true)\\]"))
	})

	It("builds negated failure message", func() {
		actual := BeKeyOf(map[int]bool{1: true, 2: false}).NegatedFailureMessage(42)
		Expect(actual).To(MatchRegexp("Expected\n    <int>: 42\nnot to be a key of\n    <\\[\\]bool | len:2, cap:2>: \\[(true, false)|(false, true)\\]"))
	})

})
