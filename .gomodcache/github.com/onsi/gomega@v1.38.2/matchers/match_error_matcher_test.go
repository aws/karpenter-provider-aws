package matchers_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

type CustomError struct {
}

func (c CustomError) Error() string {
	return "an error"
}

type ComplexError struct {
	Key string
}

func (t *ComplexError) Error() string {
	return fmt.Sprintf("err: %s", t.Key)
}

var _ = Describe("MatchErrorMatcher", func() {
	Context("When asserting against an error", func() {
		When("passed an error", func() {
			It("should succeed when errors are deeply equal", func() {
				err := errors.New("an error")
				fmtErr := fmt.Errorf("an error")
				customErr := CustomError{}

				Expect(err).Should(MatchError(errors.New("an error")))
				Expect(err).ShouldNot(MatchError(errors.New("another error")))

				Expect(fmtErr).Should(MatchError(errors.New("an error")))
				Expect(customErr).Should(MatchError(CustomError{}))
			})

			It("should succeed when any error in the chain matches the passed error", func() {
				innerErr := errors.New("inner error")
				outerErr := fmt.Errorf("outer error wrapping: %w", innerErr)

				Expect(outerErr).Should(MatchError(innerErr))
			})

			It("uses deep equality with unwrapped errors", func() {
				innerErr := &ComplexError{Key: "abc"}
				outerErr := fmt.Errorf("outer error wrapping: %w", &ComplexError{Key: "abc"})
				Expect(outerErr).To(MatchError(innerErr))
			})
		})

		When("actual an expected are both pointers to an error", func() {
			It("should succeed when errors are deeply equal", func() {
				err := CustomError{}
				Expect(&err).To(MatchError(&err))
			})
		})

		It("should succeed when matching with a string", func() {
			err := errors.New("an error")
			fmtErr := fmt.Errorf("an error")
			customErr := CustomError{}

			Expect(err).Should(MatchError("an error"))
			Expect(err).ShouldNot(MatchError("another error"))

			Expect(fmtErr).Should(MatchError("an error"))
			Expect(customErr).Should(MatchError("an error"))
		})

		When("passed a matcher", func() {
			It("should pass if the matcher passes against the error string", func() {
				err := errors.New("error 123 abc")

				Expect(err).Should(MatchError(MatchRegexp(`\d{3}`)))
			})

			It("should fail if the matcher fails against the error string", func() {
				err := errors.New("no digits")
				Expect(err).ShouldNot(MatchError(MatchRegexp(`\d`)))
			})
		})

		When("passed a function that takes error and returns bool", func() {
			var IsFooError = func(err error) bool {
				return err.Error() == "foo"
			}

			It("requires an additional description", func() {
				_, err := (&MatchErrorMatcher{
					Expected: IsFooError,
				}).Match(errors.New("foo"))
				Expect(err).Should(MatchError("MatchError requires an additional description when passed a function"))
			})

			It("matches iff the function returns true", func() {
				Ω(errors.New("foo")).Should(MatchError(IsFooError, "FooError"))
				Ω(errors.New("fooo")).ShouldNot(MatchError(IsFooError, "FooError"))
			})

			It("uses the error description to construct its message", func() {
				failuresMessages := InterceptGomegaFailures(func() {
					Ω(errors.New("fooo")).Should(MatchError(IsFooError, "FooError"))
				})
				Ω(failuresMessages[0]).Should(ContainSubstring("fooo\n    {s: \"fooo\"}\nto match error function FooError"))

				failuresMessages = InterceptGomegaFailures(func() {
					Ω(errors.New("foo")).ShouldNot(MatchError(IsFooError, "FooError"))
				})
				Ω(failuresMessages[0]).Should(ContainSubstring("foo\n    {s: \"foo\"}\nnot to match error function FooError"))
			})
		})

		It("should fail when passed anything else", func() {
			actualErr := errors.New("an error")
			_, err := (&MatchErrorMatcher{
				Expected: []byte("an error"),
			}).Match(actualErr)
			Expect(err).Should(HaveOccurred())

			_, err = (&MatchErrorMatcher{
				Expected: 3,
			}).Match(actualErr)
			Expect(err).Should(HaveOccurred())

			_, err = (&MatchErrorMatcher{
				Expected: func(e error) {},
			}).Match(actualErr)
			Expect(err).Should(HaveOccurred())

			_, err = (&MatchErrorMatcher{
				Expected: func() bool { return false },
			}).Match(actualErr)
			Expect(err).Should(HaveOccurred())

			_, err = (&MatchErrorMatcher{
				Expected: func() {},
			}).Match(actualErr)
			Expect(err).Should(HaveOccurred())

			_, err = (&MatchErrorMatcher{
				Expected: func(e error, a string) (bool, error) { return false, nil },
			}).Match(actualErr)
			Expect(err).Should(HaveOccurred())
		})
	})

	When("passed nil", func() {
		It("should fail", func() {
			_, err := (&MatchErrorMatcher{
				Expected: "an error",
			}).Match(nil)
			Expect(err).Should(HaveOccurred())
		})
	})

	When("passed a non-error", func() {
		It("should fail", func() {
			_, err := (&MatchErrorMatcher{
				Expected: "an error",
			}).Match("an error")
			Expect(err).Should(HaveOccurred())

			_, err = (&MatchErrorMatcher{
				Expected: "an error",
			}).Match(3)
			Expect(err).Should(HaveOccurred())
		})
	})

	When("passed an error that is also a string", func() {
		It("should use it as an error", func() {
			var e mockErr = "mockErr"

			// this fails if the matcher casts e to a string before comparison
			Expect(e).Should(MatchError(e))
		})
	})

	It("shows failure message", func() {
		failuresMessages := InterceptGomegaFailures(func() {
			Expect(errors.New("foo")).To(MatchError("bar"))
		})
		Expect(failuresMessages[0]).To(ContainSubstring("foo\n    {s: \"foo\"}\nto match error\n    <string>: bar"))
	})

	It("shows negated failure message", func() {
		failuresMessages := InterceptGomegaFailures(func() {
			Expect(errors.New("foo")).ToNot(MatchError("foo"))
		})
		Expect(failuresMessages[0]).To(ContainSubstring("foo\n    {s: \"foo\"}\nnot to match error\n    <string>: foo"))
	})

})

type mockErr string

func (m mockErr) Error() string { return string(m) }
