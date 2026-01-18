package gleak

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IgnoringTopFunction matcher", func() {

	It("returns an error for an invalid actual", func() {
		m := IgnoringTopFunction("foo.bar")
		Expect(m.Match(nil)).Error().To(MatchError("IgnoringTopFunction matcher expects a Goroutine or *Goroutine.  Got:\n    <nil>: nil"))
	})

	It("matches a toplevel function by full name", func() {
		m := IgnoringTopFunction("foo.bar")
		Expect(m.Match(Goroutine{
			TopFunction: "foo.bar",
		})).To(BeTrue())
		Expect(m.Match(Goroutine{
			TopFunction: "main.main",
		})).To(BeFalse())

		m = IgnoringTopFunction("foo.bar(*implementation[...]).baz")
		Expect(m.Match(Goroutine{
			TopFunction: "foo.bar(*implementation[...]).baz",
		})).To(BeTrue())
		Expect(m.Match(Goroutine{
			TopFunction: "main.main",
		})).To(BeFalse())
	})

	It("matches a toplevel function by prefix", func() {
		m := IgnoringTopFunction("foo...")
		Expect(m.Match(Goroutine{
			TopFunction: "foo.bar",
		})).To(BeTrue())
		Expect(m.Match(Goroutine{
			TopFunction: "foo",
		})).To(BeFalse())
		Expect(m.Match(Goroutine{
			TopFunction: "spanish.inquisition",
		})).To(BeFalse())

		m = IgnoringTopFunction("foo.bar(*implementation[...])...")
		Expect(m.Match(Goroutine{
			TopFunction: "foo.bar(*implementation[...]).baz",
		})).To(BeTrue())
		Expect(m.Match(Goroutine{
			TopFunction: "foo",
		})).To(BeFalse())
		Expect(m.Match(Goroutine{
			TopFunction: "spanish.inquisition",
		})).To(BeFalse())
	})

	It("matches a toplevel function by name and state prefix", func() {
		m := IgnoringTopFunction("foo.bar [worried]")
		Expect(m.Match(Goroutine{
			TopFunction: "foo.bar",
			State:       "worried, stalled",
		})).To(BeTrue())
		Expect(m.Match(Goroutine{
			TopFunction: "foo.bar",
			State:       "uneasy, anxious",
		})).To(BeFalse())

		m = IgnoringTopFunction("foo.bar(*implementation[...]) [worried]")
		Expect(m.Match(Goroutine{
			TopFunction: "foo.bar(*implementation[...])",
			State:       "worried, stalled",
		})).To(BeTrue())
		Expect(m.Match(Goroutine{
			TopFunction: "foo.bar(*implementation[...])",
			State:       "uneasy, anxious",
		})).To(BeFalse())
	})

	It("returns failure messages", func() {
		m := IgnoringTopFunction("foo.bar")
		Expect(m.FailureMessage(Goroutine{ID: 42, TopFunction: "foo"})).To(Equal(
			"Expected\n    <goroutine.Goroutine>: {ID: 42, State: \"\", TopFunction: \"foo\", CreatorFunction: \"\", BornAt: \"\"}\nto have the topmost function \"foo.bar\""))
		Expect(m.NegatedFailureMessage(Goroutine{ID: 42, TopFunction: "foo"})).To(Equal(
			"Expected\n    <goroutine.Goroutine>: {ID: 42, State: \"\", TopFunction: \"foo\", CreatorFunction: \"\", BornAt: \"\"}\nnot to have the topmost function \"foo.bar\""))

		m = IgnoringTopFunction("foo.bar [worried]")
		Expect(m.FailureMessage(Goroutine{ID: 42, TopFunction: "foo"})).To(Equal(
			"Expected\n    <goroutine.Goroutine>: {ID: 42, State: \"\", TopFunction: \"foo\", CreatorFunction: \"\", BornAt: \"\"}\nto have the topmost function \"foo.bar\" and the state \"worried\""))

		m = IgnoringTopFunction("foo...")
		Expect(m.FailureMessage(Goroutine{ID: 42, TopFunction: "foo"})).To(Equal(
			"Expected\n    <goroutine.Goroutine>: {ID: 42, State: \"\", TopFunction: \"foo\", CreatorFunction: \"\", BornAt: \"\"}\nto have the prefix \"foo.\" for its topmost function"))
	})

})
