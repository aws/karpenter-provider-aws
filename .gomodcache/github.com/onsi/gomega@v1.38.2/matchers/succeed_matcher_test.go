package matchers_test

import (
	"errors"
	"regexp"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

func Erroring() error {
	return errors.New("bam")
}

func NotErroring() error {
	return nil
}

type AnyType struct{}

func Invalid() *AnyType {
	return nil
}

type formattedGomegaError struct {
	message string
}

func (e formattedGomegaError) Error() string {
	return "NOT THIS ERROR"
}

func (e formattedGomegaError) FormattedGomegaError() string {
	return e.message
}

var _ = Describe("Succeed", func() {
	It("should succeed if the function succeeds", func() {
		Expect(NotErroring()).Should(Succeed())
	})

	It("should succeed (in the negated) if the function errored", func() {
		Expect(Erroring()).ShouldNot(Succeed())
	})

	It("should not if passed a non-error", func() {
		success, err := (&SucceedMatcher{}).Match(Invalid())
		Expect(success).Should(BeFalse())
		Expect(err).Should(MatchError("Expected an error-type.  Got:\n    <*matchers_test.AnyType | 0x0>: nil"))
	})

	It("doesn't support non-error type", func() {
		success, err := (&SucceedMatcher{}).Match(AnyType{})
		Expect(success).Should(BeFalse())
		Expect(err).Should(MatchError("Expected an error-type.  Got:\n    <matchers_test.AnyType>: {}"))
	})

	It("doesn't support non-error pointer type", func() {
		success, err := (&SucceedMatcher{}).Match(&AnyType{})
		Expect(success).Should(BeFalse())
		Expect(err).Should(MatchError(MatchRegexp(`Expected an error-type.  Got:\n    <*matchers_test.AnyType | 0x[[:xdigit:]]+>: {}`)))
	})

	It("should not succeed with pointer types that conform to error interface", func() {
		err := &CustomErr{"ohai"}
		Expect(err).ShouldNot(Succeed())
	})

	It("should succeed with nil pointers to types that conform to error interface", func() {
		var err *CustomErr = nil
		Expect(err).Should(Succeed())
	})

	It("builds failure message", func() {
		actual := Succeed().FailureMessage(errors.New("oops"))
		actual = regexp.MustCompile(" 0x.*>").ReplaceAllString(actual, " 0x00000000>")
		Expect(actual).To(Equal("Expected success, but got an error:\n    <*errors.errorString | 0x00000000>: \n    oops\n    {s: \"oops\"}"))
	})

	It("simply returns .Error() for the failure message if the error is an AsyncPolledActualError", func() {
		actual := Succeed().FailureMessage(formattedGomegaError{message: "this is already formatted appropriately"})
		Expect(actual).To(Equal("this is already formatted appropriately"))
	})

	It("operates correctly when paired with an Eventually that receives a Gomega", func() {
		_, file, line, _ := runtime.Caller(0)
		failureMessage := InterceptGomegaFailure(func() {
			Eventually(func(g Gomega) {
				g.Expect(true).To(BeFalse())
			}).WithTimeout(time.Millisecond * 10).Should(Succeed())
		}).Error()
		Ω(failureMessage).Should(HavePrefix("Timed out after"))
		Ω(failureMessage).Should(HaveSuffix("The function passed to Eventually failed at %s:%d with:\nExpected\n    <bool>: true\nto be false", file, line+3))
	})

	It("builds negated failure message", func() {
		actual := Succeed().NegatedFailureMessage(123)
		Expect(actual).To(Equal("Expected failure, but got no error."))
	})
})
