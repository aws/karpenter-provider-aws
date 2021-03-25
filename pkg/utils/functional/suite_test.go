package functional

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional Suite")
}

var _ = Describe("Functional", func() {
	// var (
	// 	emptyInt32 []int32
	// 	//		oneInt32   []int32
	// 	//longInt32  []int32
	// )

	// Context("With empty slice", func() {
	// 	Specify("should return empty slice", func() {
	// 		//Expect(functional.GreaterThanInt32(emptyInt32, 23)).To(BeEmpty())
	// 		Expect(GreaterThanInt32(emptyInt32, 23)).To(BeEmpty())
	// 		Expect(LessThanInt32(emptyInt32, 23)).To(BeEmpty())
	// 		alwaysTrue := func(_, _ int32) bool { return true }
	// 		Expect(FilterInt32(emptyInt32, 23, alwaysTrue)).To(BeEmpty())
	// 	})
	// })

	// Context("MergeInto", func() {
	// 	It("does nothing with empty", func() {
	// 		dest := struct {
	// 			a int
	// 			b int
	// 		}{a: 2, b: 3}
	// 		orig := dest
	// 		MergeInto(dest)
	// 		Expect(dest).To(Equal(orig))
	// 	})

	// 	It("does nothing with copy", func() {
	// 		dest := struct {
	// 			a int
	// 			b int
	// 		}{a: 2, b: 3}
	// 		orig := dest
	// 		same := dest
	// 		MergeInto(&dest, &same)
	// 		Expect(dest).To(Equal(orig))
	// 	})

	// })
})
