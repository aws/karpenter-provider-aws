package internal_test

import (
	"errors"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/internal"
)

func getGlobalDurationBundle() internal.DurationBundle {
	return Default.(*internal.Gomega).DurationBundle
}

func setGlobalDurationBundle(bundle internal.DurationBundle) {
	SetDefaultEventuallyTimeout(bundle.EventuallyTimeout)
	SetDefaultEventuallyPollingInterval(bundle.EventuallyPollingInterval)
	SetDefaultConsistentlyDuration(bundle.ConsistentlyDuration)
	SetDefaultConsistentlyPollingInterval(bundle.ConsistentlyPollingInterval)
	if bundle.EnforceDefaultTimeoutsWhenUsingContexts {
		EnforceDefaultTimeoutsWhenUsingContexts()
	} else {
		DisableDefaultTimeoutsWhenUsingContext()
	}
}

var _ = Describe("Gomega DSL", func() {
	var globalDurationBundle internal.DurationBundle

	BeforeEach(func() {
		globalDurationBundle = getGlobalDurationBundle()
	})

	AfterEach(func() {
		RegisterFailHandler(Fail)
		setGlobalDurationBundle(globalDurationBundle)
	})

	Describe("The Default, global, Gomega", func() {
		It("exists", func() {
			Ω(Default).ShouldNot(BeNil())
		})

		It("is wired up via the global DSL", func() {
			counter := 0
			Eventually(func() int {
				counter += 1
				return counter
			}).Should(Equal(5))
			Ω(counter).Should(Equal(5))
		})
	})

	Describe("NewGomega", func() {
		It("creates and configures a new Gomega, using the global duration bundle", func() {
			bundle := internal.DurationBundle{
				EventuallyTimeout:                       time.Minute,
				EventuallyPollingInterval:               2 * time.Minute,
				ConsistentlyDuration:                    3 * time.Minute,
				ConsistentlyPollingInterval:             4 * time.Minute,
				EnforceDefaultTimeoutsWhenUsingContexts: true,
			}
			setGlobalDurationBundle(bundle)

			var calledWith string
			g := NewGomega(func(message string, skip ...int) {
				calledWith = message
			})

			gAsStruct := g.(*internal.Gomega)
			Ω(gAsStruct.DurationBundle).Should(Equal(bundle))

			g.Ω(true).Should(BeFalse())
			Ω(calledWith).Should(Equal("Expected\n    <bool>: true\nto be false"))
		})
	})

	Describe("NewWithT", func() {
		It("creates and configure a new Gomega with the passed-in T, using the global duration bundle", func() {
			bundle := internal.DurationBundle{
				EventuallyTimeout:                       time.Minute,
				EventuallyPollingInterval:               2 * time.Minute,
				ConsistentlyDuration:                    3 * time.Minute,
				ConsistentlyPollingInterval:             4 * time.Minute,
				EnforceDefaultTimeoutsWhenUsingContexts: true,
			}
			setGlobalDurationBundle(bundle)

			fakeT := &FakeGomegaTestingT{}
			g := NewWithT(fakeT)

			Ω(g.DurationBundle).Should(Equal(bundle))

			g.Ω(true).Should(BeFalse())
			Ω(fakeT.CalledFatalf).Should(Equal("\nExpected\n    <bool>: true\nto be false"))
			Ω(fakeT.CalledHelper).Should(BeTrue())
		})
	})

	Describe("RegisterFailHandler", func() {
		It("overrides the global fail handler", func() {
			var calledWith string
			RegisterFailHandler(func(message string, skip ...int) {
				calledWith = message
			})

			Ω(true).Should(BeFalse())

			RegisterFailHandler(Fail)
			Ω(calledWith).Should(Equal("Expected\n    <bool>: true\nto be false"))
		})
	})

	Describe("RegisterTestingT", func() {
		It("overrides the global fail handler", func() {
			fakeT := &FakeGomegaTestingT{}
			RegisterTestingT(fakeT)

			Ω(true).Should(BeFalse())
			RegisterFailHandler(Fail)
			Ω(fakeT.CalledFatalf).Should(Equal("\nExpected\n    <bool>: true\nto be false"))
			Ω(fakeT.CalledHelper).Should(BeTrue())
		})
	})

	Describe("InterceptGomegaFailures", func() {
		Context("when no failures occur", func() {
			It("returns an empty array", func() {
				Expect(InterceptGomegaFailures(func() {
					Expect("hi").To(Equal("hi"))
				})).To(BeEmpty())
			})
		})

		Context("when failures occur", func() {
			It("does not stop execution and returns all the failures as strings", func() {
				Expect(InterceptGomegaFailures(func() {
					Expect("hi").To(Equal("bye"))
					Expect(3).To(Equal(2))
				})).To(Equal([]string{
					"Expected\n    <string>: hi\nto equal\n    <string>: bye",
					"Expected\n    <int>: 3\nto equal\n    <int>: 2",
				}))

			})
		})
	})

	Describe("InterceptGomegaFailure", func() {
		Context("when no failures occur", func() {
			It("returns nil", func() {
				Expect(InterceptGomegaFailure(func() {
					Expect("hi").To(Equal("hi"))
				})).To(BeNil())
			})
		})

		Context("when failures occur", func() {
			It("returns the first failure and stops execution", func() {
				gotThere := false
				Expect(InterceptGomegaFailure(func() {
					Expect("hi").To(Equal("bye"))
					gotThere = true
					Expect(3).To(Equal(2))
				})).To(Equal(errors.New("Expected\n    <string>: hi\nto equal\n    <string>: bye")))
				Expect(gotThere).To(BeFalse())
			})
		})

		Context("when the function panics", func() {
			It("panics", func() {
				Expect(func() {
					InterceptGomegaFailure(func() {
						panic("boom")
					})
				}).To(PanicWith("boom"))
			})
		})
	})

	Context("Making an assertion without a registered fail handler", func() {
		It("should panic", func() {
			defer func() {
				e := recover()
				RegisterFailHandler(Fail)
				if e == nil {
					Fail("expected a panic to have occurred")
				}
			}()

			RegisterFailHandler(nil)
			Expect(true).Should(BeTrue())
		})
	})

	Describe("specifying default durations globally", func() {
		It("should update the durations on the Default gomega", func() {
			bundle := internal.DurationBundle{
				EventuallyTimeout:                       time.Minute,
				EventuallyPollingInterval:               2 * time.Minute,
				ConsistentlyDuration:                    3 * time.Minute,
				ConsistentlyPollingInterval:             4 * time.Minute,
				EnforceDefaultTimeoutsWhenUsingContexts: true,
			}

			SetDefaultEventuallyTimeout(bundle.EventuallyTimeout)
			SetDefaultEventuallyPollingInterval(bundle.EventuallyPollingInterval)
			SetDefaultConsistentlyDuration(bundle.ConsistentlyDuration)
			SetDefaultConsistentlyPollingInterval(bundle.ConsistentlyPollingInterval)
			EnforceDefaultTimeoutsWhenUsingContexts()

			Ω(Default.(*internal.Gomega).DurationBundle).Should(Equal(bundle))
		})
	})

	Describe("Offsets", func() {
		AfterEach(func() {
			RegisterFailHandler(Fail)
		})

		It("computes the correct offsets", func() {
			doubleNested := func(eventually bool) {
				func() {
					if eventually {
						Eventually(true, "10ms", "5ms").WithOffset(2).Should(BeFalse())
					} else {
						Expect(true).WithOffset(2).To(BeFalse())
					}
				}()
			}

			reportedFile, reportedLine := "", 0
			captureLocation := func(message string, skip ...int) {
				_, reportedFile, reportedLine, _ = runtime.Caller(skip[0] + 1)
			}

			_, thisFile, anchorLine, _ := runtime.Caller(0)   // 0
			RegisterFailHandler(captureLocation)              // 1
			Expect(true).To(BeFalse())                        // *2*
			RegisterFailHandler(Fail)                         // 3
			Ω(reportedFile).Should(Equal(thisFile))           // 4
			Ω(reportedLine - anchorLine).Should(Equal(2))     // 5
			RegisterFailHandler(captureLocation)              // 6
			doubleNested(false)                               // *7*
			RegisterFailHandler(Fail)                         // 8
			Ω(reportedFile).Should(Equal(thisFile))           // 9
			Ω(reportedLine - anchorLine).Should(Equal(7))     // 10
			RegisterFailHandler(captureLocation)              // 11
			Eventually(true, "10ms", "5ms").Should(BeFalse()) // *12*
			RegisterFailHandler(Fail)                         // 13
			Ω(reportedFile).Should(Equal(thisFile))           // 14
			Ω(reportedLine - anchorLine).Should(Equal(12))    // 15
			RegisterFailHandler(captureLocation)              // 16
			doubleNested(true)                                // *17*
			RegisterFailHandler(Fail)                         // 18
			Ω(reportedFile).Should(Equal(thisFile))           // 19
			Ω(reportedLine - anchorLine).Should(Equal(17))    // 20
		})
	})
})
