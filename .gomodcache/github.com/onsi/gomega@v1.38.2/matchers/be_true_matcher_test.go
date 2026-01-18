package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeTrue and BeTrueBecause", func() {
	It("should handle true and false correctly", func() {
		Expect(true).Should(BeTrue())
		Expect(false).ShouldNot(BeTrue())
	})

	It("should only support booleans", func() {
		success, err := (&BeTrueMatcher{}).Match("foo")
		Expect(success).Should(BeFalse())
		Expect(err).Should(HaveOccurred())
	})

	It("returns the passed in failure message if provided", func() {
		x := 10
		err := InterceptGomegaFailure(func() { Expect(x == 100).Should(BeTrue()) })
		Ω(err.Error()).Should(Equal("Expected\n    <bool>: false\nto be true"))

		err = InterceptGomegaFailure(func() { Expect(x == 100).Should(BeTrueBecause("x should be 100%%")) })
		Ω(err.Error()).Should(Equal("x should be 100%"))

		err = InterceptGomegaFailure(func() { Expect(x == 100).Should(BeTrueBecause("x should be %d%%", 100)) })
		Ω(err.Error()).Should(Equal("x should be 100%"))
	})

	It("prints out a useful message if a negation fails", func() {
		x := 100
		err := InterceptGomegaFailure(func() { Expect(x == 100).ShouldNot(BeTrue()) })
		Ω(err.Error()).Should(Equal("Expected\n    <bool>: true\nnot to be true"))

		err = InterceptGomegaFailure(func() { Expect(x == 100).ShouldNot(BeTrueBecause("x should be 100%%")) })
		Ω(err.Error()).Should(Equal(`Expected not true but got true\nNegation of "x should be 100%" failed`))

		err = InterceptGomegaFailure(func() { Expect(x == 100).ShouldNot(BeTrueBecause("x should be %d%%", 100)) })
		Ω(err.Error()).Should(Equal(`Expected not true but got true\nNegation of "x should be 100%" failed`))
	})
})
