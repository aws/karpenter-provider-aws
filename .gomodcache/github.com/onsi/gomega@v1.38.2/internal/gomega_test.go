package internal_test

import (
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/internal"
)

var _ = Describe("Gomega", func() {
	It("is mostly tested in assertion_test and async_assertion_test", func() {

	})
	Describe("when initialized", func() {
		var g *internal.Gomega

		BeforeEach(func() {
			g = internal.NewGomega(internal.DurationBundle{})
			Ω(g.Fail).Should(BeNil())
			Ω(g.THelper).Should(BeNil())
		})

		It("should be registered as unconfigured", func() {
			Ω(g.IsConfigured()).Should(BeFalse())
		})

		Context("when configured with a fail handler", func() {
			It("registers the fail handler and a no-op helper", func() {
				var capturedMessage string
				g.ConfigureWithFailHandler(func(message string, skip ...int) {
					capturedMessage = message
				})
				Ω(g.IsConfigured()).Should(BeTrue())

				g.Fail("hi bob")
				Ω(capturedMessage).Should(Equal("hi bob"))
				Ω(g.THelper).ShouldNot(Panic())
			})
		})

		Context("when configured with a T", func() {
			It("registers a fail handler an the T's helper", func() {
				fake := &FakeGomegaTestingT{}
				g.ConfigureWithT(fake)
				Ω(g.IsConfigured()).Should(BeTrue())

				g.Fail("hi bob")
				Ω(fake.CalledHelper).Should(BeTrue())
				Ω(fake.CalledFatalf).Should(Equal("\nhi bob"))

				fake.CalledHelper = false
				g.THelper()
				Ω(fake.CalledHelper).Should(BeTrue())
			})
		})
	})

	Describe("Offset", func() {
		It("computes the correct offsets", func() {
			doubleNested := func(g Gomega, eventually bool) {
				func() {
					if eventually {
						g.Eventually(true, "10ms", "5ms").WithOffset(2).Should(BeFalse())
					} else {
						g.Expect(true).WithOffset(2).To(BeFalse())
					}
				}()
			}

			reportedFile, reportedLine := "", 0
			_, thisFile, anchorLine, _ := runtime.Caller(0)    // 0
			g := NewGomega(func(message string, skip ...int) { // 1
				_, reportedFile, reportedLine, _ = runtime.Caller(skip[0] + 1) // 2
			}) // 3
			g.Expect(true).To(BeFalse())                        // *4*
			Ω(reportedFile).Should(Equal(thisFile))             // 5
			Ω(reportedLine - anchorLine).Should(Equal(4))       // 6
			doubleNested(g, false)                              // *7*
			Ω(reportedFile).Should(Equal(thisFile))             // 8
			Ω(reportedLine - anchorLine).Should(Equal(7))       // 9
			g.Eventually(true, "10ms", "5ms").Should(BeFalse()) // *10*
			Ω(reportedFile).Should(Equal(thisFile))             // 11
			Ω(reportedLine - anchorLine).Should(Equal(10))      // 12
			doubleNested(g, true)                               // *13*
			Ω(reportedFile).Should(Equal(thisFile))             // 14
			Ω(reportedLine - anchorLine).Should(Equal(13))      // 15
		})
	})
})
