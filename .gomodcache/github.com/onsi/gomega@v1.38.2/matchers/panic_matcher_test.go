package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("Panic", func() {
	When("passed something that's not a function that takes zero arguments and returns nothing", func() {
		It("should error", func() {
			success, err := (&PanicMatcher{}).Match("foo")
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())

			success, err = (&PanicMatcher{}).Match(nil)
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())

			success, err = (&PanicMatcher{}).Match(func(foo string) {})
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())

			success, err = (&PanicMatcher{}).Match(func() string { return "bar" })
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})

	When("passed a function of the correct type", func() {
		It("should call the function and pass if the function panics", func() {
			Expect(func() { panic("ack!") }).To(Panic())
			Expect(func() {}).NotTo(Panic())
		})
	})

	When("assertion fails", func() {
		It("prints the object passed to panic() when negative", func() {
			failuresMessages := InterceptGomegaFailures(func() {
				Expect(func() { panic("ack!") }).NotTo(Panic())
			})
			Expect(failuresMessages).To(ConsistOf(ContainSubstring("not to panic, but panicked with\n    <string>: ack!")))
		})

		It("prints simple message when positive", func() {
			failuresMessages := InterceptGomegaFailures(func() {
				Expect(func() {}).To(Panic())
			})
			Expect(failuresMessages).To(ConsistOf(MatchRegexp("Expected\n\\s+<func\\(\\)>: .+\nto panic")))
		})
	})
})

var _ = Describe("PanicWith", func() {
	When("a specific panic value is expected", func() {
		matcher := PanicWith("ack!")

		When("no panic occurs", func() {
			actual := func() {}

			It("prints a message that includes the expected value", func() {
				failuresMessages := InterceptGomegaFailures(func() {
					Expect(actual).To(matcher)
				})
				Expect(failuresMessages).To(ConsistOf(
					MatchRegexp("Expected\n\\s+<func\\(\\)>: .+\nto panic with\\s+<string>: ack!"),
				))
			})

			It("passes when negated", func() {
				Expect(actual).NotTo(matcher)
			})
		})

		When("the panic value matches", func() {
			actual := func() { panic("ack!") }

			It("passes", func() {
				Expect(actual).To(matcher)
			})

			It("prints a message that includes the (un)expected value when negated", func() {
				failuresMessages := InterceptGomegaFailures(func() {
					Expect(actual).NotTo(matcher)
				})
				Expect(failuresMessages).To(ConsistOf(
					MatchRegexp("Expected\n\\s+<func\\(\\)>: .+\nnot to panic with\\s+<string>: ack!"),
				))
			})
		})

		When("the panic value does not match", func() {
			actual := func() { panic("unexpected!") }

			It("prints a message that includes both the actual and expected values", func() {
				failuresMessages := InterceptGomegaFailures(func() {
					Expect(actual).To(matcher)
				})
				Expect(failuresMessages).To(ConsistOf(
					MatchRegexp("Expected\n\\s+<func\\(\\)>: .+\nto panic with\\s+<string>: ack!\nbut panicked with\n\\s+<string>: unexpected!"),
				))
			})

			It("passes when negated", func() {
				Expect(actual).NotTo(matcher)
			})
		})
	})

	When("the expected value is actually a matcher", func() {
		matcher := PanicWith(MatchRegexp("ack"))

		When("no panic occurs", func() {
			actual := func() {}

			It("prints a message that includes the expected value", func() {
				failuresMessages := InterceptGomegaFailures(func() {
					Expect(actual).To(matcher)
				})
				Expect(failuresMessages).To(ConsistOf(
					MatchRegexp("Expected\n\\s+<func\\(\\)>: .+\nto panic with a value matching\n.+MatchRegexpMatcher.+ack"),
				))
			})

			It("passes when negated", func() {
				Expect(actual).NotTo(matcher)
			})
		})

		When("the panic value matches", func() {
			actual := func() { panic("ack!") }

			It("passes", func() {
				Expect(actual).To(matcher)
			})

			It("prints a message that includes the (un)expected value when negated", func() {
				failuresMessages := InterceptGomegaFailures(func() {
					Expect(actual).NotTo(matcher)
				})
				Expect(failuresMessages).To(ConsistOf(
					MatchRegexp("Expected\n\\s+<func\\(\\)>: .+\nnot to panic with a value matching\n.+MatchRegexpMatcher.+ack"),
				))
			})
		})

		When("the panic value does not match", func() {
			actual := func() { panic("unexpected!") }

			It("prints a message that includes both the actual and expected values", func() {
				failuresMessages := InterceptGomegaFailures(func() {
					Expect(actual).To(matcher)
				})
				Expect(failuresMessages).To(ConsistOf(
					MatchRegexp("Expected\n\\s+<func\\(\\)>: .+\nto panic with a value matching\n.+MatchRegexpMatcher.+ack.+\nbut panicked with\n\\s+<string>: unexpected!"),
				))
			})

			It("passes when negated", func() {
				Expect(actual).NotTo(matcher)
			})
		})
	})
})
