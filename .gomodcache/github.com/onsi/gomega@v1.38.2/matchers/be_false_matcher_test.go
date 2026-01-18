package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeFalse and BeFalseBecause", func() {
	It("should handle true and false correctly", func() {
		Expect(true).ShouldNot(BeFalse())
		Expect(false).Should(BeFalse())
	})

	It("should only support booleans", func() {
		success, err := (&BeFalseMatcher{}).Match("foo")
		Expect(success).Should(BeFalse())
		Expect(err).Should(HaveOccurred())
	})

	It("returns the passed in failure message if provided", func() {
		x := 100
		err := InterceptGomegaFailure(func() { Expect(x == 100).Should(BeFalse()) })
		Ω(err.Error()).Should(Equal("Expected\n    <bool>: true\nto be false"))

		err = InterceptGomegaFailure(func() { Expect(x == 100).Should(BeFalseBecause("x should not be 100%%")) })
		Ω(err.Error()).Should(Equal("x should not be 100%"))

		err = InterceptGomegaFailure(func() { Expect(x == 100).Should(BeFalseBecause("x should not be %d%%", 100)) })
		Ω(err.Error()).Should(Equal("x should not be 100%"))
	})

	It("prints out a useful message if a negation fails", func() {
		x := 10
		err := InterceptGomegaFailure(func() { Expect(x == 100).ShouldNot(BeFalse()) })
		Ω(err.Error()).Should(Equal("Expected\n    <bool>: false\nnot to be false"))

		err = InterceptGomegaFailure(func() { Expect(x == 100).ShouldNot(BeFalseBecause("x should not be 100%%")) })
		Ω(err.Error()).Should(Equal(`Expected not false but got false\nNegation of "x should not be 100%" failed`))

		err = InterceptGomegaFailure(func() { Expect(x == 100).ShouldNot(BeFalseBecause("x should not be %d%%", 100)) })
		Ω(err.Error()).Should(Equal(`Expected not false but got false\nNegation of "x should not be 100%" failed`))
	})
})
