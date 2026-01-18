package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("HaveSuffixMatcher", func() {
	When("actual is a string", func() {
		It("should match a string suffix", func() {
			Expect("Ab").Should(HaveSuffix("b"))
			Expect("A").ShouldNot(HaveSuffix("Ab"))
		})
	})

	When("the matcher is called with multiple arguments", func() {
		It("should pass the string and arguments to sprintf", func() {
			Expect("C3PO").Should(HaveSuffix("%dPO", 3))
		})
	})

	When("actual is a stringer", func() {
		It("should call the stringer and match against the returned string", func() {
			Expect(&myStringer{a: "Ab"}).Should(HaveSuffix("b"))
		})
	})

	When("actual is neither a string nor a stringer", func() {
		It("should error", func() {
			success, err := (&HaveSuffixMatcher{Suffix: "2"}).Match(2)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	It("shows failure message", func() {
		failuresMessages := InterceptGomegaFailures(func() {
			Expect("foo").To(HaveSuffix("bar"))
		})
		Expect(failuresMessages[0]).To(Equal("Expected\n    <string>: foo\nto have suffix\n    <string>: bar"))
	})

	It("shows negated failure message", func() {
		failuresMessages := InterceptGomegaFailures(func() {
			Expect("foo").ToNot(HaveSuffix("oo"))
		})
		Expect(failuresMessages[0]).To(Equal("Expected\n    <string>: foo\nnot to have suffix\n    <string>: oo"))
	})
})
