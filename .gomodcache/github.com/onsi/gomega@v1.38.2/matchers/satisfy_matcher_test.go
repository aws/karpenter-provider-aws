package matchers_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SatisfyMatcher", func() {

	var isEven = func(x int) bool { return x%2 == 0 }

	Context("Panic if predicate is invalid", func() {
		panicsWithPredicate := func(predicate any) {
			Expect(func() { Satisfy(predicate) }).WithOffset(1).To(Panic())
		}
		It("nil", func() {
			panicsWithPredicate(nil)
		})
		Context("Invalid number of args, but correct return value count", func() {
			It("zero", func() {
				panicsWithPredicate(func() int { return 5 })
			})
			It("two", func() {
				panicsWithPredicate(func(i, j int) int { return 5 })
			})
		})
		Context("Invalid return types, but correct number of arguments", func() {
			It("zero", func() {
				panicsWithPredicate(func(i int) {})
			})
			It("two", func() {
				panicsWithPredicate(func(i int) (int, int) { return 5, 6 })
			})
			It("invalid type", func() {
				panicsWithPredicate(func(i int) string { return "" })
			})
		})
	})

	When("the actual value is incompatible", func() {
		It("fails to pass int to func(string)", func() {
			actual, predicate := int(0), func(string) bool { return false }
			success, err := Satisfy(predicate).Match(actual)
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expects 'string'"))
			Expect(err.Error()).To(ContainSubstring("have 'int'"))
		})

		It("fails to pass string to func(interface)", func() {
			actual, predicate := "bang", func(error) bool { return false }
			success, err := Satisfy(predicate).Match(actual)
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expects 'error'"))
			Expect(err.Error()).To(ContainSubstring("have 'string'"))
		})

		It("fails to pass nil interface to func(int)", func() {
			actual, predicate := error(nil), func(int) bool { return false }
			success, err := Satisfy(predicate).Match(actual)
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expects 'int'"))
			Expect(err.Error()).To(ContainSubstring("have '<nil>'"))
		})

		It("fails to pass nil interface to func(pointer)", func() {
			actual, predicate := error(nil), func(*string) bool { return false }
			success, err := Satisfy(predicate).Match(actual)
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expects '*string'"))
			Expect(err.Error()).To(ContainSubstring("have '<nil>'"))
		})
	})

	It("works with positive cases", func() {
		Expect(2).To(Satisfy(isEven))

		// transform expects interface
		takesError := func(error) bool { return true }
		Expect(nil).To(Satisfy(takesError), "handles nil actual values")
		Expect(errors.New("abc")).To(Satisfy(takesError))
	})

	It("works with negative cases", func() {
		Expect(1).ToNot(Satisfy(isEven))
	})

	Context("failure messages", func() {
		When("match fails", func() {
			It("gives a descriptive message", func() {
				m := Satisfy(isEven)
				Expect(m.Match(1)).To(BeFalse())
				Expect(m.FailureMessage(1)).To(ContainSubstring("Expected\n    <int>: 1\nto satisfy predicate\n    <func(int) bool>: "))
			})
		})

		When("match succeeds, but expected it to fail", func() {
			It("gives a descriptive message", func() {
				m := Not(Satisfy(isEven))
				Expect(m.Match(2)).To(BeFalse())
				Expect(m.FailureMessage(2)).To(ContainSubstring("Expected\n    <int>: 2\nto not satisfy predicate\n    <func(int) bool>: "))
			})
		})

		Context("actual value is incompatible with predicate's argument type", func() {
			It("gracefully fails", func() {
				m := Satisfy(isEven)
				result, err := m.Match("hi") // give it a string but predicate expects int; doesn't panic
				Expect(result).To(BeFalse())
				Expect(err).To(MatchError("predicate expects 'int' but we have 'string'"))
			})
		})
	})
})
