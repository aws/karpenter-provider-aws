package gleak

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IgnoringGoroutines matcher", func() {

	It("returns an error for an invalid actual", func() {
		m := IgnoringGoroutines(Goroutines())
		Expect(m.Match(nil)).Error().To(MatchError(
			"IgnoringGoroutines matcher expects a Goroutine or *Goroutine.  Got:\n    <nil>: nil"))
	})

	It("matches", func() {
		gs := Goroutines()
		me := gs[0]
		m := IgnoringGoroutines(gs)
		Expect(m.Match(me)).To(BeTrue())
		Expect(m.Match(gs[1])).To(BeTrue())
		Expect(m.Match(Goroutine{})).To(BeFalse())
	})

	It("returns failure messages", func() {
		m := IgnoringGoroutines(Goroutines())
		Expect(m.FailureMessage(Goroutine{})).To(MatchRegexp(
			`Expected\n    <goroutine.Goroutine>: {ID: 0, State: "", TopFunction: "", CreatorFunction: "", BornAt: ""}\nto be contained in the list of expected goroutine IDs\n    <\[\]uint64 | len:\d+, cap:\d+>: [.*]`))
		Expect(m.NegatedFailureMessage(Goroutine{})).To(MatchRegexp(
			`Expected\n    <goroutine.Goroutine>: {ID: 0, State: "", TopFunction: "", CreatorFunction: "", BornAt: ""}\nnot to be contained in the list of expected goroutine IDs\n    <\[\]uint64 | len:\d+, cap:\d+>: [.*]`))
	})

})
