package gleak

import (
	"reflect"

	"github.com/onsi/gomega/gleak/goroutine"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IgnoringInBacktrace matcher", func() {

	It("returns an error for an invalid actual", func() {
		m := IgnoringInBacktrace("foo.bar")
		Expect(m.Match(nil)).Error().To(MatchError(
			"IgnoringInBacktrace matcher expects a Goroutine or *Goroutine.  Got:\n    <nil>: nil"))
	})

	It("matches", func() {
		type T struct{}
		pkg := reflect.TypeOf(T{}).PkgPath()
		m := IgnoringInBacktrace(pkg + "/goroutine.stacks")
		Expect(m.Match(somefunction())).To(BeTrue())
	})

	It("returns failure messages", func() {
		m := IgnoringInBacktrace("foo.bar")
		Expect(m.FailureMessage(Goroutine{Backtrace: "abc"})).To(MatchRegexp(
			`Expected\n    <goroutine.Goroutine>: {ID: 0, State: "", TopFunction: "", CreatorFunction: "", BornAt: ""}\nto contain "foo.bar" in the goroutine's backtrace`))
		Expect(m.NegatedFailureMessage(Goroutine{Backtrace: "abc"})).To(MatchRegexp(
			`Expected\n    <goroutine.Goroutine>: {ID: 0, State: "", TopFunction: "", CreatorFunction: "", BornAt: ""}\nnot to contain "foo.bar" in the goroutine's backtrace`))
	})

})

func somefunction() Goroutine {
	return goroutine.Current()
}
