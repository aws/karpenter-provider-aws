package gleak

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("utilities", func() {

	Context("G(oroutine) descriptions", func() {

		It("returns an error for actual <nil>", func() {
			Expect(func() { _, _ = G(nil, "foo") }).NotTo(Panic())
			Expect(G(nil, "foo")).Error().To(MatchError("foo matcher expects a Goroutine or *Goroutine.  Got:\n    <nil>: nil"))
		})

		It("returns an error when passing something that's not a goroutine by any means", func() {
			Expect(func() { _, _ = G("foobar", "foo") }).NotTo(Panic())
			Expect(G("foobar", "foo")).Error().To(MatchError("foo matcher expects a Goroutine or *Goroutine.  Got:\n    <string>: foobar"))
		})

		It("returns a goroutine", func() {
			actual := Goroutine{ID: 42}
			g, err := G(actual, "foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(g.ID).To(Equal(uint64(42)))

			g, err = G(&actual, "foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(g.ID).To(Equal(uint64(42)))
		})

	})

	It("returns a list of Goroutine IDs in textual format", func() {
		Expect(goids(nil)).To(BeEmpty())
		Expect(goids([]Goroutine{
			{ID: 666},
			{ID: 42},
		})).To(Equal("42, 666"))
		Expect(goids([]Goroutine{
			{ID: 42},
		})).To(Equal("42"))
	})

})
