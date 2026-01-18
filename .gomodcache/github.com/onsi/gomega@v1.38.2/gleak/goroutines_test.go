package gleak

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("goroutines", func() {

	It("returns all goroutines", func() {
		Expect(Goroutines()).To(ContainElement(
			HaveField("TopFunction", SatisfyAny(
				Equal("testing.(*T).Run"),
				Equal("testing.RunTests")))))
	})

})
