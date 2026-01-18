package matchers_test

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type I interface {
	M()
}

type S struct {
	V int
}

func (s S) M() {}

var _ = Describe("HaveValue", func() {

	It("should fail when passed nil", func() {
		var p *struct{}
		m := HaveValue(BeNil())
		Expect(m.Match(p)).Error().To(MatchError(MatchRegexp("not to be <nil>$")))
	})

	It("should fail when passed nil indirectly", func() {
		var p *struct{}
		m := HaveValue(BeNil())
		Expect(m.Match(&p)).Error().To(MatchError(MatchRegexp("not to be <nil>$")))
	})

	It("should use the matcher's failure message", func() {
		m := HaveValue(Equal(42))
		Expect(m.Match(666)).To(BeFalse())
		Expect(m.FailureMessage(nil)).To(Equal("Expected\n    <int>: 666\nto equal\n    <int>: 42"))
		Expect(m.NegatedFailureMessage(nil)).To(Equal("Expected\n    <int>: 666\nnot to equal\n    <int>: 42"))
	})

	It("should unwrap the value pointed to, even repeatedly", func() {
		i := 1
		Expect(&i).To(HaveValue(Equal(1)))
		Expect(&i).NotTo(HaveValue(Equal(2)))

		pi := &i
		Expect(pi).To(HaveValue(Equal(1)))
		Expect(pi).NotTo(HaveValue(Equal(2)))

		Expect(&pi).To(HaveValue(Equal(1)))
		Expect(&pi).NotTo(HaveValue(Equal(2)))
	})

	It("shouldn't endlessly star-gaze", func() {
		dave := "It's full of stars!"
		stargazer := reflect.ValueOf(dave)
		for stars := 1; stars <= 31; stars++ {
			p := reflect.New(stargazer.Type())
			p.Elem().Set(stargazer)
			stargazer = p
		}
		m := HaveValue(Equal(dave))
		Expect(m.Match(stargazer.Interface())).Error().To(
			MatchError(MatchRegexp(`too many indirections`)))
		Expect(m.Match(stargazer.Elem().Interface())).To(BeTrue())
	})

	It("should unwrap the value of an interface", func() {
		var i I = &S{V: 42}
		Expect(i).To(HaveValue(Equal(S{V: 42})))
		Expect(i).NotTo(HaveValue(Equal(S{})))
	})

})
