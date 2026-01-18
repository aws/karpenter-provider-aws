package option_test

import (
	options "github.com/awslabs/operatorpkg/option"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type Option struct {
	Foo bool
	Bar string
	Baz int
}

func FooOptions(o *Option) {
	o.Foo = true
}
func BarOptions(o *Option) {
	o.Bar = "bar"
}

func BazOptions(baz int) options.Function[Option] {
	return func(o *Option) {
		o.Baz = baz
	}
}

var _ = Describe("Function", func() {
	It("should resolve options", func() {
		Expect(options.Resolve(
			FooOptions,
			BarOptions,
			BazOptions(5),
		)).To(Equal(&Option{
			Foo: true,
			Bar: "bar",
			Baz: 5,
		}))

		Expect(options.Resolve(
			FooOptions,
			BarOptions,
		)).To(Equal(&Option{
			Foo: true,
			Bar: "bar",
		}))

		Expect(options.Resolve[Option]()).To(Equal(&Option{}))
	})
})
