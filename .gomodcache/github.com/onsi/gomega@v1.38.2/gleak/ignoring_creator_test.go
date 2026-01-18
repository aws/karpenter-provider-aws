package gleak

import (
	"reflect"

	"github.com/onsi/gomega/gleak/goroutine"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func creator() Goroutine {
	ch := make(chan Goroutine)
	go func() {
		ch <- goroutine.Current()
	}()
	return <-ch
}

var _ = Describe("IgnoringCreator matcher", func() {

	It("returns an error for an invalid actual", func() {
		m := IgnoringCreator("foo.bar")
		Expect(m.Match(nil)).Error().To(MatchError("IgnoringCreator matcher expects a Goroutine or *Goroutine.  Got:\n    <nil>: nil"))
	})

	It("matches a creator function by full name", func() {
		type T struct{}
		pkg := reflect.TypeOf(T{}).PkgPath()
		ignore := pkg + ".creator"
		m := IgnoringCreator(ignore)
		g := creator()
		Expect(m.Match(g)).To(BeTrue(), "creator: %v\ntried to ignore: %s",
			g.String(), ignore)
		Expect(m.Match(goroutine.Current())).To(BeFalse())
	})

	It("matches a toplevel function by prefix", func() {
		type T struct{}
		pkg := reflect.TypeOf(T{}).PkgPath()
		m := IgnoringCreator(pkg + "...")
		g := creator()
		Expect(m.Match(g)).To(BeTrue(), "creator %v", g.String())
		Expect(m.Match(goroutine.Current())).To(BeFalse())
		Expect(m.Match(Goroutine{
			TopFunction: "spanish.inquisition",
		})).To(BeFalse())
	})

	It("returns failure messages", func() {
		m := IgnoringCreator("foo.bar")
		Expect(m.FailureMessage(Goroutine{ID: 42, TopFunction: "foo"})).To(Equal(
			"Expected\n    <goroutine.Goroutine>: {ID: 42, State: \"\", TopFunction: \"foo\", CreatorFunction: \"\", BornAt: \"\"}\nto be created by \"foo.bar\""))
		Expect(m.NegatedFailureMessage(Goroutine{ID: 42, TopFunction: "foo"})).To(Equal(
			"Expected\n    <goroutine.Goroutine>: {ID: 42, State: \"\", TopFunction: \"foo\", CreatorFunction: \"\", BornAt: \"\"}\nnot to be created by \"foo.bar\""))

		m = IgnoringCreator("foo...")
		Expect(m.FailureMessage(Goroutine{ID: 42, TopFunction: "foo"})).To(Equal(
			"Expected\n    <goroutine.Goroutine>: {ID: 42, State: \"\", TopFunction: \"foo\", CreatorFunction: \"\", BornAt: \"\"}\nto be created by a function with prefix \"foo.\""))
	})

})
