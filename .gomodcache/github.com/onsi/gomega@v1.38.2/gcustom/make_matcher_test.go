package gcustom_test

import (
	"errors"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gcustom"
	"github.com/onsi/gomega/internal"
)

// InstrumentedGomega
type InstrumentedGomega struct {
	G                 *internal.Gomega
	FailureMessage    string
	FailureSkip       []int
	RegisteredHelpers []string
}

func NewInstrumentedGomega() *InstrumentedGomega {
	out := &InstrumentedGomega{}

	out.G = internal.NewGomega(internal.FetchDefaultDurationBundle())
	out.G.Fail = func(message string, skip ...int) {
		out.FailureMessage = message
		out.FailureSkip = skip
	}
	out.G.THelper = func() {
		pc, _, _, _ := runtime.Caller(1)
		f := runtime.FuncForPC(pc)
		funcName := strings.TrimPrefix(f.Name(), "github.com/onsi/gomega/internal.")
		out.RegisteredHelpers = append(out.RegisteredHelpers, funcName)
	}

	return out
}

type someType struct {
	Name string
}

var _ = Describe("MakeMatcher", func() {
	It("generatees a custom matcher that satisfies the GomegaMatcher interface and renders correct failure messages", func() {
		m := gcustom.MakeMatcher(func(a int) (bool, error) {
			if a == 0 {
				return true, nil
			}
			if a == 1 {
				return false, nil
			}
			return false, errors.New("bam")
		}).WithMessage("match")

		Ω(0).Should(m)
		Ω(1).ShouldNot(m)

		ig := NewInstrumentedGomega()
		ig.G.Ω(1).Should(m)
		Ω(ig.FailureMessage).Should(Equal("Expected:\n    <int>: 1\nto match"))

		ig.G.Ω(0).ShouldNot(m)
		Ω(ig.FailureMessage).Should(Equal("Expected:\n    <int>: 0\nnot to match"))

		ig.G.Ω(2).Should(m)
		Ω(ig.FailureMessage).Should(Equal("bam"))

		ig.G.Ω(2).ShouldNot(m)
		Ω(ig.FailureMessage).Should(Equal("bam"))
	})

	Describe("validating and wrapping the MatchFunc", func() {
		DescribeTable("it panics when passed an invalid function", func(f any) {
			Expect(func() {
				gcustom.MakeMatcher(f)
			}).To(PanicWith("MakeMatcher must be passed a function that takes one argument and returns (bool, error)"))
		},
			Entry("a non-function", "foo"),
			Entry("a non-function", 1),
			Entry("a function with no input", func() (bool, error) { return false, nil }),
			Entry("a function with too many inputs", func(a int, b string) (bool, error) { return false, nil }),
			Entry("a function with no outputs", func(a any) {}),
			Entry("a function with insufficient outputs", func(a any) bool { return false }),
			Entry("a function with insufficient outputs", func(a any) error { return nil }),
			Entry("a function with too many outputs", func(a any) (bool, error, string) { return false, nil, "" }),
			Entry("a function with the wrong types of outputs", func(a any) (int, error) { return 1, nil }),
			Entry("a function with the wrong types of outputs", func(a any) (bool, int) { return false, 1 }),
		)

		Context("when the match func accepts any actual", func() {
			It("always passes in the actual, regardless of type", func() {
				var passedIn any
				m := gcustom.MakeMatcher(func(a any) (bool, error) {
					passedIn = a
					return true, nil
				})

				m.Match(1)
				Ω(passedIn).Should(Equal(1))

				m.Match("foo")
				Ω(passedIn).Should(Equal("foo"))

				m.Match(someType{"foo"})
				Ω(passedIn).Should(Equal(someType{"foo"}))

				c := make(chan bool)
				m.Match(c)
				Ω(passedIn).Should(Equal(c))
			})
		})

		Context("when the match func accepts a specific type", func() {
			It("ensure the type matches before calling func", func() {
				var passedIn any
				m := gcustom.MakeMatcher(func(a int) (bool, error) {
					passedIn = a
					return true, nil
				})

				success, err := m.Match(1)
				Ω(success).Should(BeTrue())
				Ω(err).ShouldNot(HaveOccurred())
				Ω(passedIn).Should(Equal(1))

				passedIn = nil
				success, err = m.Match(1.2)
				Ω(success).Should(BeFalse())
				Ω(err).Should(MatchError(ContainSubstring("Matcher expected actual of type <int>.  Got:\n    <float64>: 1.2")))
				Ω(passedIn).Should(BeNil())

				m = gcustom.MakeMatcher(func(a someType) (bool, error) {
					passedIn = a
					return true, nil
				})

				success, err = m.Match(someType{"foo"})
				Ω(success).Should(BeTrue())
				Ω(err).ShouldNot(HaveOccurred())
				Ω(passedIn).Should(Equal(someType{"foo"}))

				passedIn = nil
				success, err = m.Match("foo")
				Ω(success).Should(BeFalse())
				Ω(err).Should(MatchError(ContainSubstring("Matcher expected actual of type <gcustom_test.someType>.  Got:\n    <string>: foo")))
				Ω(passedIn).Should(BeNil())

			})
		})

		Context("when the match func accepts a nil-able type", func() {
			It("ensure nil matches the type", func() {
				var passedIn any
				m := gcustom.MakeMatcher(func(a *someType) (bool, error) {
					passedIn = a
					return true, nil
				})

				success, err := m.Match(nil)
				Ω(success).Should(BeTrue())
				Ω(err).ShouldNot(HaveOccurred())
				Ω(passedIn).Should(BeNil())
			})
		})
	})

	It("calls the matchFunc and returns whatever it returns when Match is called", func() {
		m := gcustom.MakeMatcher(func(a int) (bool, error) {
			if a == 0 {
				return true, nil
			}
			if a == 1 {
				return false, nil
			}
			return false, errors.New("bam")
		})

		Ω(m.Match(0)).Should(BeTrue())
		Ω(m.Match(1)).Should(BeFalse())
		success, err := m.Match(2)
		Ω(success).Should(BeFalse())
		Ω(err).Should(MatchError("bam"))
	})

	Describe("rendering messages", func() {
		var m gcustom.CustomGomegaMatcher
		BeforeEach(func() {
			m = gcustom.MakeMatcher(func(a any) (bool, error) { return false, nil })
		})

		Context("when no message is configured", func() {
			It("renders a simple canned message", func() {
				Ω(m.FailureMessage(3)).Should(Equal("Custom matcher failed for:\n    <int>: 3"))
				Ω(m.NegatedFailureMessage(3)).Should(Equal("Custom matcher succeeded (but was expected to fail) for:\n    <int>: 3"))
			})
		})

		Context("when a simple message is configured", func() {
			It("tacks that message onto the end of a formatted string", func() {
				m = m.WithMessage("have been confabulated")
				Ω(m.FailureMessage(3)).Should(Equal("Expected:\n    <int>: 3\nto have been confabulated"))
				Ω(m.NegatedFailureMessage(3)).Should(Equal("Expected:\n    <int>: 3\nnot to have been confabulated"))

				m = gcustom.MakeMatcher(func(a any) (bool, error) { return false, nil }, "have been confabulated")
				Ω(m.FailureMessage(3)).Should(Equal("Expected:\n    <int>: 3\nto have been confabulated"))
				Ω(m.NegatedFailureMessage(3)).Should(Equal("Expected:\n    <int>: 3\nnot to have been confabulated"))

			})
		})

		Context("when a template is registered", func() {
			It("uses that template", func() {
				m = m.WithTemplate("{{.Failure}} {{.NegatedFailure}} {{.To}} {{.FormattedActual}} {{.Actual.Name}}")
				Ω(m.FailureMessage(someType{"foo"})).Should(Equal("true false to     <gcustom_test.someType>: {Name: \"foo\"} foo"))
				Ω(m.NegatedFailureMessage(someType{"foo"})).Should(Equal("false true not to     <gcustom_test.someType>: {Name: \"foo\"} foo"))

			})
		})

		Context("when a template with custom data is registered", func() {
			It("provides that custom data", func() {
				m = m.WithTemplate("{{.Failure}} {{.NegatedFailure}} {{.To}} {{.FormattedActual}} {{.Actual.Name}} {{.Data}}", 17)

				Ω(m.FailureMessage(someType{"foo"})).Should(Equal("true false to     <gcustom_test.someType>: {Name: \"foo\"} foo 17"))
				Ω(m.NegatedFailureMessage(someType{"foo"})).Should(Equal("false true not to     <gcustom_test.someType>: {Name: \"foo\"} foo 17"))
			})

			It("provides a mechanism for formatting custom data", func() {
				m = m.WithTemplate("{{format .Data}}", 17)

				Ω(m.FailureMessage(0)).Should(Equal("<int>: 17"))
				Ω(m.NegatedFailureMessage(0)).Should(Equal("<int>: 17"))

				m = m.WithTemplate("{{format .Data 1}}", 17)

				Ω(m.FailureMessage(0)).Should(Equal("    <int>: 17"))
				Ω(m.NegatedFailureMessage(0)).Should(Equal("    <int>: 17"))

			})
		})

		Context("when a precompiled template is registered", func() {
			It("uses that template", func() {
				templ, err := gcustom.ParseTemplate("{{.Failure}} {{.NegatedFailure}} {{.To}} {{.FormattedActual}} {{.Actual.Name}} {{format .Data}}")
				Ω(err).ShouldNot(HaveOccurred())

				m = m.WithPrecompiledTemplate(templ, 17)
				Ω(m.FailureMessage(someType{"foo"})).Should(Equal("true false to     <gcustom_test.someType>: {Name: \"foo\"} foo <int>: 17"))
				Ω(m.NegatedFailureMessage(someType{"foo"})).Should(Equal("false true not to     <gcustom_test.someType>: {Name: \"foo\"} foo <int>: 17"))
			})

			It("can also take a template as an argument upon construction", func() {
				templ, err := gcustom.ParseTemplate("{{.To}} {{format .Data}}")
				Ω(err).ShouldNot(HaveOccurred())
				m = gcustom.MakeMatcher(func(a any) (bool, error) { return false, nil }, templ)

				Ω(m.FailureMessage(0)).Should(Equal("to <nil>: nil"))
				Ω(m.NegatedFailureMessage(0)).Should(Equal("not to <nil>: nil"))

				m = m.WithTemplateData(17)
				Ω(m.FailureMessage(0)).Should(Equal("to <int>: 17"))
				Ω(m.NegatedFailureMessage(0)).Should(Equal("not to <int>: 17"))
			})
		})
	})
})
