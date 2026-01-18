package internal_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type quickMatcher struct {
	matchFunc  func(actual any) (bool, error)
	oracleFunc func(actual any) bool
}

func (q quickMatcher) Match(actual any) (bool, error) {
	return q.matchFunc(actual)
}

func (q quickMatcher) FailureMessage(actual any) (message string) {
	return fmt.Sprintf("QM failure message: %v", actual)
}

func (q quickMatcher) NegatedFailureMessage(actual any) (message string) {
	return fmt.Sprintf("QM negated failure message: %v", actual)
}

func (q quickMatcher) MatchMayChangeInTheFuture(actual any) bool {
	if q.oracleFunc == nil {
		return true
	}
	return q.oracleFunc(actual)
}

func QuickMatcher(matchFunc func(actual any) (bool, error)) OmegaMatcher {
	return quickMatcher{matchFunc, nil}
}

func QuickMatcherWithOracle(matchFunc func(actual any) (bool, error), oracleFunc func(actual any) bool) OmegaMatcher {
	return quickMatcher{matchFunc, oracleFunc}
}

type FakeGinkgoSpecContext struct {
	Attached  func() string
	Cancelled bool
}

func (f *FakeGinkgoSpecContext) AttachProgressReporter(v func() string) func() {
	f.Attached = v
	return func() { f.Cancelled = true }
}

var _ = Describe("Asynchronous Assertions", func() {
	var ig *InstrumentedGomega
	BeforeEach(func() {
		ig = NewInstrumentedGomega()
	})

	Describe("Basic Eventually support", func() {
		Context("the positive case", func() {
			It("polls the function and matcher until a match occurs", func() {
				counter := 0
				ig.G.Eventually(func() string {
					counter++
					if counter > 5 {
						return MATCH
					}
					return NO_MATCH
				}).Should(SpecMatch())
				Ω(counter).Should(Equal(6))
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("continues polling even if the matcher errors", func() {
				counter := 0
				ig.G.Eventually(func() string {
					counter++
					if counter > 5 {
						return MATCH
					}
					return ERR_MATCH
				}).Should(SpecMatch())
				Ω(counter).Should(Equal(6))
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("times out eventually if the assertion doesn't match in time", func() {
				counter := 0
				ig.G.Eventually(func() string {
					counter++
					if counter > 100 {
						return MATCH
					}
					return NO_MATCH
				}).WithTimeout(200 * time.Millisecond).WithPolling(20 * time.Millisecond).Should(SpecMatch())
				Ω(counter).Should(BeNumerically(">", 2))
				Ω(counter).Should(BeNumerically("<", 20))
				Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("positive: no match"))
				Ω(ig.FailureSkip).Should(Equal([]int{3}))
			})

			It("maps Within() correctly to timeout and polling intervals", func() {
				counter := 0
				ig.G.Eventually(func() bool {
					counter++
					return false
				}).WithTimeout(0).WithPolling(20 * time.Millisecond).Within(200 * time.Millisecond).Should(BeTrue())
				Ω(counter).Should(BeNumerically(">", 2))
				Ω(counter).Should(BeNumerically("<", 20))

				counter = 0
				ig.G.Eventually(func() bool {
					counter++
					return false
				}).WithTimeout(0).WithPolling(0). // first zero intervals, then set them
									Within(200 * time.Millisecond).ProbeEvery(20 * time.Millisecond).
									Should(BeTrue())
				Ω(counter).Should(BeNumerically(">", 2))
				Ω(counter).Should(BeNumerically("<", 20))
			})
		})

		Context("the negative case", func() {
			It("polls the function and matcher until a match does not occur", func() {
				counter := 0
				ig.G.Eventually(func() string {
					counter++
					if counter > 5 {
						return NO_MATCH
					}
					return MATCH
				}).ShouldNot(SpecMatch())
				Ω(counter).Should(Equal(6))
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("continues polling when the matcher errors - an error does not count as a successful non-match", func() {
				counter := 0
				ig.G.Eventually(func() string {
					counter++
					if counter > 5 {
						return NO_MATCH
					}
					return ERR_MATCH
				}).ShouldNot(SpecMatch())
				Ω(counter).Should(Equal(6))
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("times out eventually if the assertion doesn't match in time", func() {
				counter := 0
				ig.G.Eventually(func() string {
					counter++
					if counter > 100 {
						return NO_MATCH
					}
					return MATCH
				}).WithTimeout(200 * time.Millisecond).WithPolling(20 * time.Millisecond).ShouldNot(SpecMatch())
				Ω(counter).Should(BeNumerically(">", 2))
				Ω(counter).Should(BeNumerically("<", 20))
				Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("negative: match"))
				Ω(ig.FailureSkip).Should(Equal([]int{3}))
			})
		})

		Context("when a failure occurs", func() {
			It("registers the appropriate helper functions", func() {
				ig.G.Eventually(NO_MATCH).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(SpecMatch())
				Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("positive: no match"))
				Ω(ig.FailureSkip).Should(Equal([]int{3}))
				Ω(ig.RegisteredHelpers).Should(ContainElement("(*AsyncAssertion).Should"))
				Ω(ig.RegisteredHelpers).Should(ContainElement("(*AsyncAssertion).match"))
			})

			It("renders the matcher's error if an error occurred", func() {
				ig.G.Eventually(ERR_MATCH).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(SpecMatch())
				Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("The matcher passed to Eventually returned the following error:"))
				Ω(ig.FailureMessage).Should(ContainSubstring("spec matcher error"))
			})

			It("renders the optional description", func() {
				ig.G.Eventually(NO_MATCH).WithTimeout(50*time.Millisecond).WithPolling(10*time.Millisecond).Should(SpecMatch(), "boop")
				Ω(ig.FailureMessage).Should(ContainSubstring("boop"))
			})

			It("formats and renders the optional description when there are multiple arguments", func() {
				ig.G.Eventually(NO_MATCH).WithTimeout(50*time.Millisecond).WithPolling(10*time.Millisecond).Should(SpecMatch(), "boop %d", 17)
				Ω(ig.FailureMessage).Should(ContainSubstring("boop 17"))
			})

			It("calls the optional description if it is a function", func() {
				ig.G.Eventually(NO_MATCH).WithTimeout(50*time.Millisecond).WithPolling(10*time.Millisecond).Should(SpecMatch(), func() string { return "boop" })
				Ω(ig.FailureMessage).Should(ContainSubstring("boop"))
			})
		})

		Context("with a passed-in context", func() {
			Context("when the passed-in context is cancelled", func() {
				It("stops and returns a failure", func() {
					ctx, cancel := context.WithCancel(context.Background())
					counter := 0
					ig.G.Eventually(func() string {
						counter++
						if counter == 2 {
							cancel()
						} else if counter == 10 {
							return MATCH
						}
						return NO_MATCH
					}, time.Hour, ctx).Should(SpecMatch())
					Ω(ig.FailureMessage).Should(ContainSubstring("Context was cancelled after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("positive: no match"))
				})

				It("can also be configured via WithContext()", func() {
					ctx, cancel := context.WithCancel(context.Background())
					counter := 0
					ig.G.Eventually(func() string {
						counter++
						if counter == 2 {
							cancel()
						} else if counter == 10 {
							return MATCH
						}
						return NO_MATCH
					}, time.Hour).WithContext(ctx).Should(SpecMatch())
					Ω(ig.FailureMessage).Should(ContainSubstring("Context was cancelled after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("positive: no match"))
				})

				It("can also be configured with the context up front", func() {
					ctx, cancel := context.WithCancel(context.Background())
					counter := 0
					ig.G.Eventually(ctx, func() string {
						counter++
						if counter == 2 {
							cancel()
						} else if counter == 10 {
							return MATCH
						}
						return NO_MATCH
					}, time.Hour).Should(SpecMatch())
					Ω(ig.FailureMessage).Should(ContainSubstring("Context was cancelled after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("positive: no match"))
				})

				It("treats a leading context as an actual, even if valid durations are passed in", func() {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					Eventually(ctx).Should(Equal(ctx))
					Eventually(ctx, 0.1).Should(Equal(ctx))
				})

				It("counts as a failure for Consistently", func() {
					ctx, cancel := context.WithCancel(context.Background())
					counter := 0
					ig.G.Consistently(func() string {
						counter++
						if counter == 2 {
							cancel()
						} else if counter == 10 {
							return NO_MATCH
						}
						return MATCH
					}, time.Hour).WithContext(ctx).Should(SpecMatch())
					Ω(ig.FailureMessage).Should(ContainSubstring("Context was cancelled after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("There is no failure as the matcher passed to Consistently has not yet failed"))
				})

				It("includes the cancel cause if provided", func() {
					ctx, cancel := context.WithCancelCause(context.Background())
					counter := 0
					ig.G.Eventually(func() string {
						counter++
						if counter == 2 {
							cancel(fmt.Errorf("kaboom"))
						} else if counter == 10 {
							return MATCH
						}
						return NO_MATCH
					}, time.Hour, ctx).Should(SpecMatch())
					Ω(ig.FailureMessage).Should(ContainSubstring("Context was cancelled (cause: kaboom) after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("positive: no match"))
				})
			})

			Context("when the passed-in context is a Ginkgo SpecContext that can take a progress reporter attachment", func() {
				It("attaches a progress reporter context that allows it to report on demand", func() {
					fakeSpecContext := &FakeGinkgoSpecContext{}
					var message string
					ctx := context.WithValue(context.Background(), "GINKGO_SPEC_CONTEXT", fakeSpecContext)
					ig.G.Eventually(func() string {
						if fakeSpecContext.Attached != nil {
							message = fakeSpecContext.Attached()
						}
						return NO_MATCH
					}).WithTimeout(time.Millisecond * 20).WithContext(ctx).Should(Equal(MATCH))

					Ω(message).Should(Equal("Expected\n    <string>: no match\nto equal\n    <string>: match"))
					Ω(fakeSpecContext.Cancelled).Should(BeTrue())
				})

				Context("when used with consistently", func() {
					It("returns a useful message that does not invoke the matcher's failure handlers", func() {
						fakeSpecContext := &FakeGinkgoSpecContext{}
						var message string
						ctx := context.WithValue(context.Background(), "GINKGO_SPEC_CONTEXT", fakeSpecContext)
						ig.G.Consistently(func() error {
							if fakeSpecContext.Attached != nil {
								message = fakeSpecContext.Attached()
							}
							return nil
						}).WithTimeout(time.Millisecond * 20).WithContext(ctx).ShouldNot(HaveOccurred())

						Ω(message).Should(Equal("There is no failure as the matcher passed to Consistently has not yet failed"))
						Ω(fakeSpecContext.Cancelled).Should(BeTrue())
					})
				})
			})

			Describe("the interaction between the context and the timeout", func() {
				It("only relies on context cancellation when no explicit timeout is specified", func() {
					ig.G.SetDefaultEventuallyTimeout(time.Millisecond * 10)
					ig.G.SetDefaultEventuallyPollingInterval(time.Millisecond * 40)
					t := time.Now()
					ctx, cancel := context.WithCancel(context.Background())
					iterations := 0
					ig.G.Eventually(func() string {
						iterations += 1
						if time.Since(t) > time.Millisecond*200 {
							cancel()
						}
						return "A"
					}).WithContext(ctx).Should(Equal("B"))
					Ω(time.Since(t)).Should(BeNumerically("~", time.Millisecond*200, time.Millisecond*100))
					Ω(iterations).Should(BeNumerically("~", 200/40, 2))
					Ω(ig.FailureMessage).Should(ContainSubstring("Context was cancelled after"))
				})

				It("uses the default timeout if the user explicitly opts into EnforceDefaultTimeoutsWhenUsingContexts()", func() {
					ig.G.SetDefaultEventuallyTimeout(time.Millisecond * 100)
					ig.G.SetDefaultEventuallyPollingInterval(time.Millisecond * 10)
					ig.G.EnforceDefaultTimeoutsWhenUsingContexts()
					t := time.Now()
					ctx, cancel := context.WithCancel(context.Background())
					iterations := 0
					ig.G.Eventually(func() string {
						iterations += 1
						if time.Since(t) > time.Millisecond*1000 {
							cancel()
						}
						return "A"
					}).WithContext(ctx).Should(Equal("B"))
					Ω(time.Since(t)).Should(BeNumerically("~", time.Millisecond*100, time.Millisecond*50))
					Ω(iterations).Should(BeNumerically("~", 100/10, 2))
					Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
					Ω(ctx.Err()).Should(BeNil())
				})

				It("uses the explicit timeout when it is provided", func() {
					t := time.Now()
					ctx, cancel := context.WithCancel(context.Background())
					iterations := 0
					ig.G.Eventually(func() string {
						iterations += 1
						if time.Since(t) > time.Millisecond*200 {
							cancel()
						}
						return "A"
					}).WithContext(ctx).WithTimeout(time.Millisecond * 80).ProbeEvery(time.Millisecond * 40).Should(Equal("B"))
					Ω(time.Since(t)).Should(BeNumerically("~", time.Millisecond*80, time.Millisecond*40))
					Ω(iterations).Should(BeNumerically("~", 80/40, 2))
					Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
				})
			})
		})
	})

	Describe("Basic Consistently support", func() {
		Context("the positive case", func() {
			It("polls the function and matcher ensuring a match occurs consistently", func() {
				counter := 0
				ig.G.Consistently(func() string {
					counter++
					return MATCH
				}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(SpecMatch())
				Ω(counter).Should(BeNumerically(">", 1))
				Ω(counter).Should(BeNumerically("<", 7))
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("fails if the matcher ever errors", func() {
				counter := 0
				ig.G.Consistently(func() string {
					counter++
					if counter == 3 {
						return ERR_MATCH
					}
					return MATCH
				}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(SpecMatch())
				Ω(counter).Should(Equal(3))
				Ω(ig.FailureMessage).Should(ContainSubstring("Failed after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("The matcher passed to Consistently returned the following error:"))
				Ω(ig.FailureMessage).Should(ContainSubstring("spec matcher error"))
			})

			It("fails if the matcher doesn't match at any point", func() {
				counter := 0
				ig.G.Consistently(func() string {
					counter++
					if counter == 3 {
						return NO_MATCH
					}
					return MATCH
				}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(SpecMatch())
				Ω(counter).Should(Equal(3))
				Ω(ig.FailureMessage).Should(ContainSubstring("Failed after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("positive: no match"))
			})
		})

		Context("the negative case", func() {
			It("polls the function and matcher ensuring a match never occurs", func() {
				counter := 0
				ig.G.Consistently(func() string {
					counter++
					return NO_MATCH
				}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(SpecMatch())
				Ω(counter).Should(BeNumerically(">", 1))
				Ω(counter).Should(BeNumerically("<", 7))
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("fails if the matcher ever errors", func() {
				counter := 0
				ig.G.Consistently(func() string {
					counter++
					if counter == 3 {
						return ERR_MATCH
					}
					return NO_MATCH
				}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(SpecMatch())
				Ω(counter).Should(Equal(3))
				Ω(ig.FailureMessage).Should(ContainSubstring("Failed after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("spec matcher error"))
			})

			It("fails if the matcher matches at any point", func() {
				counter := 0
				ig.G.Consistently(func() string {
					counter++
					if counter == 3 {
						return MATCH
					}
					return NO_MATCH
				}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(SpecMatch())
				Ω(counter).Should(Equal(3))
				Ω(ig.FailureMessage).Should(ContainSubstring("Failed after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("negative: match"))
			})
		})

		Context("when a failure occurs", func() {
			It("registers the appropriate helper functions", func() {
				ig.G.Consistently(NO_MATCH).Should(SpecMatch())
				Ω(ig.FailureMessage).Should(ContainSubstring("Failed after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("positive: no match"))
				Ω(ig.FailureSkip).Should(Equal([]int{3}))
				Ω(ig.RegisteredHelpers).Should(ContainElement("(*AsyncAssertion).Should"))
				Ω(ig.RegisteredHelpers).Should(ContainElement("(*AsyncAssertion).match"))
			})

			It("renders the matcher's error if an error occurred", func() {
				ig.G.Consistently(ERR_MATCH).Should(SpecMatch())
				Ω(ig.FailureMessage).Should(ContainSubstring("Failed after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("The matcher passed to Consistently returned the following error:"))
				Ω(ig.FailureMessage).Should(ContainSubstring("spec matcher error"))
			})

			It("renders the optional description", func() {
				ig.G.Consistently(NO_MATCH).Should(SpecMatch(), "boop")
				Ω(ig.FailureMessage).Should(ContainSubstring("boop"))
			})

			It("formats and renders the optional description when there are multiple arguments", func() {
				ig.G.Consistently(NO_MATCH).Should(SpecMatch(), "boop %d", 17)
				Ω(ig.FailureMessage).Should(ContainSubstring("boop 17"))
			})

			It("calls the optional description if it is a function", func() {
				ig.G.Consistently(NO_MATCH).Should(SpecMatch(), func() string { return "boop" })
				Ω(ig.FailureMessage).Should(ContainSubstring("boop"))
			})
		})

		Context("with a passed-in context", func() {
			Context("when the passed-in context is cancelled", func() {
				It("counts as a failure for Consistently", func() {
					ctx, cancel := context.WithCancel(context.Background())
					counter := 0
					ig.G.Consistently(func() string {
						counter++
						if counter == 2 {
							cancel()
						} else if counter == 10 {
							return NO_MATCH
						}
						return MATCH
					}, time.Hour).WithContext(ctx).Should(SpecMatch())
					Ω(ig.FailureMessage).Should(ContainSubstring("Context was cancelled after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("There is no failure as the matcher passed to Consistently has not yet failed"))
				})
			})

			Describe("the interaction between the context and the timeout", func() {
				It("only always uses the default interval even if not explicit duration is provided", func() {
					ig.G.SetDefaultConsistentlyDuration(time.Millisecond * 200)
					ig.G.SetDefaultConsistentlyPollingInterval(time.Millisecond * 40)
					t := time.Now()
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					iterations := 0
					ig.G.Consistently(func() string {
						iterations += 1
						return "A"
					}).WithContext(ctx).Should(Equal("A"))
					Ω(time.Since(t)).Should(BeNumerically("~", time.Millisecond*200, time.Millisecond*100))
					Ω(iterations).Should(BeNumerically("~", 200/40, 2))
					Ω(ig.FailureMessage).Should(BeZero())
				})
			})
		})
	})

	Describe("the passed-in actual", func() {
		type Foo struct{ Bar string }

		Context("when passed a value", func() {
			It("(eventually) continuously checks on the value until a match occurs", func() {
				c := make(chan bool)
				go func() {
					time.Sleep(100 * time.Millisecond)
					close(c)
				}()
				ig.G.Eventually(c).WithTimeout(1 * time.Second).WithPolling(10 * time.Millisecond).Should(BeClosed())
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("(consistently) continuously checks on the value ensuring a match always occurs", func() {
				c := make(chan bool)
				close(c)
				ig.G.Consistently(c).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(BeClosed())
				Ω(ig.FailureMessage).Should(BeZero())
			})
		})

		Context("when passed a function that takes no arguments and returns one value", func() {
			It("(eventually) polls the function until the returned value satisfies the matcher", func() {
				counter := 0
				ig.G.Eventually(func() int {
					counter += 1
					return counter
				}).WithTimeout(1 * time.Second).WithPolling(10 * time.Millisecond).Should(BeNumerically(">", 5))
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("(consistently) polls the function ensuring the returned value satisfies the matcher", func() {
				counter := 0
				ig.G.Consistently(func() int {
					counter += 1
					return counter
				}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 20))
				Ω(counter).Should(BeNumerically(">", 2))
				Ω(ig.FailureMessage).Should(BeZero())
			})

			It("works when the function returns nil", func() {
				counter := 0
				ig.G.Eventually(func() error {
					counter += 1
					if counter > 5 {
						return nil
					}
					return errors.New("oops")
				}).WithTimeout(1 * time.Second).WithPolling(10 * time.Millisecond).Should(BeNil())
				Ω(ig.FailureMessage).Should(BeZero())
			})
		})

		Context("when passed a function that takes no arguments and returns multiple values", func() {
			Context("with Eventually", func() {
				It("polls the function until the first returned value satisfies the matcher _and_ all additional values are zero", func() {
					counter, s, f, err := 0, "hi", Foo{Bar: "hi"}, errors.New("hi")
					ig.G.Eventually(func() (int, string, Foo, error) {
						switch counter += 1; counter {
						case 2:
							s = ""
						case 3:
							f = Foo{}
						case 4:
							err = nil
						}
						return counter, s, f, err
					}).WithTimeout(1 * time.Second).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 100))
					Ω(ig.FailureMessage).Should(BeZero())
					Ω(counter).Should(Equal(4))
				})

				It("reports on the non-zero value if it times out", func() {
					ig.G.Eventually(func() (int, string, Foo, error) {
						return 1, "", Foo{Bar: "hi"}, nil
					}).WithTimeout(30 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 100))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually had an unexpected non-nil/non-zero return value at index 2:"))
					Ω(ig.FailureMessage).Should(ContainSubstring(`<internal_test.Foo>: {Bar: "hi"}`))
				})

				It("has a meaningful message if all the return values are zero except the final return value, and it is an error", func() {
					ig.G.Eventually(func() (int, string, Foo, error) {
						return 1, "", Foo{}, errors.New("welp!")
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 100))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually returned the following error:"))
					Ω(ig.FailureMessage).Should(ContainSubstring("welp!"))
				})

				Context("when making a ShouldNot assertion", func() {
					It("doesn't succeed until the matcher is (not) satisfied with the first returned value _and_ all additional values are zero", func() {
						counter, s, f, err := 0, "hi", Foo{Bar: "hi"}, errors.New("hi")
						ig.G.Eventually(func() (int, string, Foo, error) {
							switch counter += 1; counter {
							case 2:
								s = ""
							case 3:
								f = Foo{}
							case 4:
								err = nil
							}
							return counter, s, f, err
						}).WithTimeout(1 * time.Second).WithPolling(10 * time.Millisecond).ShouldNot(BeNumerically("<", 0))
						Ω(ig.FailureMessage).Should(BeZero())
						Ω(counter).Should(Equal(4))
					})
				})
			})

			Context("with Consistently", func() {
				It("polls the function and succeeds if all the values are zero and the matcher is consistently satisfied", func() {
					var err error
					counter, s, f := 0, "", Foo{}
					ig.G.Consistently(func() (int, string, Foo, error) {
						counter += 1
						return counter, s, f, err
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 100))
					Ω(ig.FailureMessage).Should(BeZero())
					Ω(counter).Should(BeNumerically(">", 2))
				})

				It("polls the function and fails any of the values are non-zero", func() {
					var err error
					counter, s, f := 0, "", Foo{}
					ig.G.Consistently(func() (int, string, Foo, error) {
						counter += 1
						if counter == 3 {
							f = Foo{Bar: "welp"}
						}
						return counter, s, f, err
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 100))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Consistently had an unexpected non-nil/non-zero return value at index 2:"))
					Ω(ig.FailureMessage).Should(ContainSubstring(`<internal_test.Foo>: {Bar: "welp"}`))
					Ω(counter).Should(Equal(3))
				})

				Context("when making a ShouldNot assertion", func() {
					It("succeeds if all additional values are zero", func() {
						var err error
						counter, s, f := 0, "", Foo{}
						ig.G.Consistently(func() (int, string, Foo, error) {
							counter += 1
							return counter, s, f, err
						}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(BeNumerically(">", 100))
						Ω(ig.FailureMessage).Should(BeZero())
						Ω(counter).Should(BeNumerically(">", 2))
					})

					It("fails if any additional values are ever non-zero", func() {
						var err error
						counter, s, f := 0, "", Foo{}
						ig.G.Consistently(func() (int, string, Foo, error) {
							counter += 1
							if counter == 3 {
								s = "welp"
							}
							return counter, s, f, err
						}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(BeNumerically(">", 100))
						Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Consistently had an unexpected non-nil/non-zero return value at index 1:"))
						Ω(ig.FailureMessage).Should(ContainSubstring(`<string>: welp`))
						Ω(counter).Should(Equal(3))
					})
				})
			})
		})

		Context("when passed a function that takes a Gomega argument and returns values", func() {
			Context("with Eventually", func() {
				It("passes in a Gomega and passes if the matcher matches, all extra values are zero, and there are no failed assertions", func() {
					counter, s, f, err := 0, "hi", Foo{Bar: "hi"}, errors.New("hi")
					ig.G.Eventually(func(g Gomega) (int, string, Foo, error) {
						switch counter += 1; counter {
						case 2:
							s = ""
						case 3:
							f = Foo{}
						case 4:
							err = nil
						}
						if counter == 5 {
							g.Expect(true).To(BeTrue())
						} else {
							g.Expect(false).To(BeTrue())
							panic("boom") //never see since the expectation stops execution
						}
						return counter, s, f, err
					}).WithTimeout(1 * time.Second).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 100))
					Ω(ig.FailureMessage).Should(BeZero())
					Ω(counter).Should(Equal(5))
				})

				It("times out if assertions in the function never succeed and reports on the error", func() {
					_, file, line, _ := runtime.Caller(0)
					ig.G.Eventually(func(g Gomega) int {
						g.Expect(false).To(BeTrue())
						return 10
					}).WithTimeout(30 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(Equal(10))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually failed at %s:%d with:", file, line+2))
					Ω(ig.FailureMessage).Should(ContainSubstring("Expected\n    <bool>: false\nto be true"))
				})

				It("forwards panics", func() {
					Ω(func() {
						ig.G.Eventually(func(g Gomega) int {
							g.Expect(true).To(BeTrue())
							panic("boom")
						}).WithTimeout(30 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(Equal(10))
					}).Should(PanicWith("boom"))
					Ω(ig.FailureMessage).Should(BeEmpty())
				})

				It("correctly handles the case (in concert with Ginkgo) when an assertion fails in a goroutine", func() {
					count := 0
					ig.G.Eventually(func(g Gomega) {
						c := make(chan any)
						go func() {
							defer GinkgoRecover()
							defer close(c)
							count += 1
							g.Expect(count).To(Equal(3)) //panics!
						}()
						<-c
					}).WithTimeout(30 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(Succeed())
					Ω(count).Should(Equal(3))
				})

				Context("when making a ShouldNot assertion", func() {
					It("doesn't succeed until all extra values are zero, there are no failed assertions, and the matcher is (not) satisfied", func() {
						counter, s, f, err := 0, "hi", Foo{Bar: "hi"}, errors.New("hi")
						ig.G.Eventually(func(g Gomega) (int, string, Foo, error) {
							switch counter += 1; counter {
							case 2:
								s = ""
							case 3:
								f = Foo{}
							case 4:
								err = nil
							}
							if counter == 5 {
								g.Expect(true).To(BeTrue())
							} else {
								g.Expect(false).To(BeTrue())
								panic("boom") //never see since the expectation stops execution
							}
							return counter, s, f, err
						}).WithTimeout(1 * time.Second).WithPolling(10 * time.Millisecond).ShouldNot(BeNumerically("<", 0))
						Ω(ig.FailureMessage).Should(BeZero())
						Ω(counter).Should(Equal(5))
					})
				})

				It("fails if an assertion is never satisfied", func() {
					_, file, line, _ := runtime.Caller(0)
					ig.G.Eventually(func(g Gomega) int {
						g.Expect(false).To(BeTrue())
						return 9
					}).WithTimeout(30 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(Equal(10))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually failed at %s:%d with:", file, line+2))
					Ω(ig.FailureMessage).Should(ContainSubstring("Expected\n    <bool>: false\nto be true"))
				})

				It("shows the state of the last match if there was a non-failing function at some point", func() {
					counter := 0
					_, file, line, _ := runtime.Caller(0)
					ig.G.Eventually(func(g Gomega) int {
						counter += 1
						g.Expect(counter).To(BeNumerically("<", 3))
						return counter
					}).WithTimeout(100 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(Equal(10))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually failed at %s:%d with:\nExpected\n    <int>: ", file, line+3))
					Ω(ig.FailureMessage).Should(ContainSubstring("to be <\n    <int>: 3"))
					Ω(ig.FailureMessage).Should(ContainSubstring("At one point, however, the function did return successfully.\nYet, Eventually failed because the matcher was not satisfied:\nExpected\n    <int>: 2\nto equal\n    <int>: 10"))
				})
			})

			Context("with Consistently", func() {
				It("passes in a Gomega and passes if the matcher matches, all extra values are zero, and there are no failed assertions", func() {
					var err error
					counter, s, f := 0, "", Foo{}
					ig.G.Consistently(func(g Gomega) (int, string, Foo, error) {
						counter += 1
						g.Expect(true).To(BeTrue())
						return counter, s, f, err
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 100))
					Ω(ig.FailureMessage).Should(BeZero())
					Ω(counter).Should(BeNumerically(">", 2))
				})

				It("fails if the passed-in gomega ever hits a failure", func() {
					var err error
					counter, s, f := 0, "", Foo{}
					_, file, line, _ := runtime.Caller(0)
					ig.G.Consistently(func(g Gomega) (int, string, Foo, error) {
						counter += 1
						g.Expect(true).To(BeTrue())
						if counter == 3 {
							g.Expect(false).To(BeTrue())
							panic("boom") //never see this
						}
						return counter, s, f, err
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(BeNumerically("<", 100))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Consistently failed at %s:%d with:", file, line+5))
					Ω(ig.FailureMessage).Should(ContainSubstring("Expected\n    <bool>: false\nto be true"))
					Ω(counter).Should(Equal(3))
				})

				It("forwards panics", func() {
					Ω(func() {
						ig.G.Consistently(func(g Gomega) int {
							g.Expect(true).To(BeTrue())
							panic("boom")
						}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(Equal(10))
					}).Should(PanicWith("boom"))
					Ω(ig.FailureMessage).Should(BeEmpty())
				})

				Context("when making a ShouldNot assertion", func() {
					It("succeeds if any interior assertions always pass", func() {
						ig.G.Consistently(func(g Gomega) int {
							g.Expect(true).To(BeTrue())
							return 9
						}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(Equal(10))
						Ω(ig.FailureMessage).Should(BeEmpty())
					})

					It("fails if any interior assertions ever fail", func() {
						counter := 0
						_, file, line, _ := runtime.Caller(0)
						ig.G.Consistently(func(g Gomega) int {
							g.Expect(true).To(BeTrue())
							counter += 1
							if counter == 3 {
								g.Expect(false).To(BeTrue())
								panic("boom") //never see this
							}
							return 9
						}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(Equal(10))
						Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Consistently failed at %s:%d with:", file, line+5))
						Ω(ig.FailureMessage).Should(ContainSubstring("Expected\n    <bool>: false\nto be true"))
					})
				})
			})
		})

		Context("when passed a function that takes a Gomega argument and returns nothing", func() {
			Context("with Eventually", func() {
				It("returns the first failed assertion as an error and so should Succeed() if the callback ever runs without issue", func() {
					counter := 0
					ig.G.Eventually(func(g Gomega) {
						counter += 1
						if counter < 5 {
							g.Expect(false).To(BeTrue())
							g.Expect("bloop").To(Equal("blarp"))
						}
					}).WithTimeout(1 * time.Second).WithPolling(10 * time.Millisecond).Should(Succeed())
					Ω(counter).Should(Equal(5))
					Ω(ig.FailureMessage).Should(BeZero())
				})

				It("returns the first failed assertion as an error and so should timeout if the callback always fails", func() {
					counter := 0
					ig.G.Eventually(func(g Gomega) {
						counter += 1
						if counter < 5000 {
							g.Expect(false).To(BeTrue())
							g.Expect("bloop").To(Equal("blarp"))
						}
					}).WithTimeout(100 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(Succeed())
					Ω(counter).Should(BeNumerically(">", 1))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually failed at"))
					Ω(ig.FailureMessage).Should(ContainSubstring("<bool>: false"))
					Ω(ig.FailureMessage).Should(ContainSubstring("to be true"))
					Ω(ig.FailureMessage).ShouldNot(ContainSubstring("bloop"))
				})

				It("returns the first failed assertion as an error and should satisfy ShouldNot(Succeed) eventually", func() {
					counter := 0
					ig.G.Eventually(func(g Gomega) {
						counter += 1
						if counter > 5 {
							g.Expect(false).To(BeTrue())
							g.Expect("bloop").To(Equal("blarp"))
						}
					}).WithTimeout(100 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(Succeed())
					Ω(counter).Should(Equal(6))
					Ω(ig.FailureMessage).Should(BeZero())
				})

				It("should fail to ShouldNot(Succeed) eventually if an error never occurs", func() {
					ig.G.Eventually(func(g Gomega) {
						g.Expect(true).To(BeTrue())
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(Succeed())
					Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("Expected failure, but got no error."))
				})
			})

			Context("with Consistently", func() {
				It("returns the first failed assertion as an error and so should Succeed() if the callback always runs without issue", func() {
					counter := 0
					ig.G.Consistently(func(g Gomega) {
						counter += 1
						g.Expect(true).To(BeTrue())
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(Succeed())
					Ω(counter).Should(BeNumerically(">", 2))
					Ω(ig.FailureMessage).Should(BeZero())
				})

				It("returns the first failed assertion as an error and so should fail if the callback ever fails", func() {
					counter := 0
					ig.G.Consistently(func(g Gomega) {
						counter += 1
						g.Expect(true).To(BeTrue())
						if counter == 3 {
							g.Expect(false).To(BeTrue())
							g.Expect("bloop").To(Equal("blarp"))
						}
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).Should(Succeed())
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Consistently failed at"))
					Ω(ig.FailureMessage).Should(ContainSubstring("<bool>: false"))
					Ω(ig.FailureMessage).Should(ContainSubstring("to be true"))
					Ω(ig.FailureMessage).ShouldNot(ContainSubstring("bloop"))
					Ω(counter).Should(Equal(3))
				})

				It("returns the first failed assertion as an error and should satisfy ShouldNot(Succeed) consistently if an error always occur", func() {
					counter := 0
					ig.G.Consistently(func(g Gomega) {
						counter += 1
						g.Expect(true).To(BeFalse())
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(Succeed())
					Ω(counter).Should(BeNumerically(">", 2))
					Ω(ig.FailureMessage).Should(BeZero())
				})

				It("should fail to satisfy ShouldNot(Succeed) consistently if an error ever does not occur", func() {
					counter := 0
					ig.G.Consistently(func(g Gomega) {
						counter += 1
						if counter == 3 {
							g.Expect(true).To(BeTrue())
						} else {
							g.Expect(false).To(BeTrue())
						}
					}).WithTimeout(50 * time.Millisecond).WithPolling(10 * time.Millisecond).ShouldNot(Succeed())
					Ω(ig.FailureMessage).Should(ContainSubstring("Failed after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("Expected failure, but got no error."))
					Ω(counter).Should(Equal(3))
				})
			})
		})

		Context("when passed a function that takes a context", func() {
			It("forwards its own configured context", func() {
				ctx := context.WithValue(context.Background(), "key", "value")
				Eventually(func(ctx context.Context) string {
					return ctx.Value("key").(string)
				}).WithContext(ctx).Should(Equal("value"))
			})

			It("forwards its own configured context _and_ a Gomega if requested", func() {
				ctx := context.WithValue(context.Background(), "key", "value")
				Eventually(func(g Gomega, ctx context.Context) {
					g.Expect(ctx.Value("key").(string)).To(Equal("schmalue"))
				}).WithContext(ctx).Should(MatchError(ContainSubstring("Expected\n    <string>: value\nto equal\n    <string>: schmalue")))
			})

			Context("when the assertion does not have an attached context", func() {
				It("errors", func() {
					ig.G.Eventually(func(ctx context.Context) string {
						return ctx.Value("key").(string)
					}).Should(Equal("value"))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually requested a context.Context, but no context has been provided.  Please pass one in using Eventually().WithContext()."))
					Ω(ig.FailureSkip).Should(Equal([]int{2}))
				})
			})
		})

		Context("when passed a function that takes additional arguments", func() {
			Context("with just arguments", func() {
				It("forwards those arguments along", func() {
					Eventually(func(a int, b string) string {
						return fmt.Sprintf("%d - %s", a, b)
					}).WithArguments(10, "four").Should(Equal("10 - four"))

					Eventually(func(a int, b string, c ...int) string {
						return fmt.Sprintf("%d - %s (%d%d%d)", a, b, c[0], c[1], c[2])
					}).WithArguments(10, "four", 5, 1, 0).Should(Equal("10 - four (510)"))
				})
			})

			Context("with a Gomega argument as well", func() {
				It("can also forward arguments alongside a Gomega", func() {
					Eventually(func(g Gomega, a int, b int) {
						g.Expect(a).To(Equal(b))
					}).WithArguments(10, 3).ShouldNot(Succeed())
					Eventually(func(g Gomega, a int, b int) {
						g.Expect(a).To(Equal(b))
					}).WithArguments(3, 3).Should(Succeed())
				})
			})

			Context("with a context argument as well", func() {
				It("can also forward arguments alongside a context", func() {
					ctx := context.WithValue(context.Background(), "key", "value")
					Eventually(func(ctx context.Context, animal string) string {
						return ctx.Value("key").(string) + " " + animal
					}).WithArguments("pony").WithContext(ctx).Should(Equal("value pony"))
				})
			})

			Context("with Gomega and context arguments", func() {
				It("forwards arguments alongside both", func() {
					ctx := context.WithValue(context.Background(), "key", "I have")
					f := func(g Gomega, ctx context.Context, count int, zoo ...string) {
						sentence := fmt.Sprintf("%s %d animals: %s", ctx.Value("key"), count, strings.Join(zoo, ", "))
						g.Expect(sentence).To(Equal("I have 3 animals: dog, cat, pony"))
					}

					Eventually(f).WithArguments(3, "dog", "cat", "pony").WithContext(ctx).Should(Succeed())
					Eventually(f).WithArguments(2, "dog", "cat").WithContext(ctx).Should(MatchError(ContainSubstring("Expected\n    <string>: I have 2 animals: dog, cat\nto equal\n    <string>: I have 3 animals: dog, cat, pony")))
				})
			})

			Context("with a context that is in the argument list", func() {
				It("does not forward the configured context", func() {
					ctxA := context.WithValue(context.Background(), "key", "A")
					ctxB := context.WithValue(context.Background(), "key", "B")

					Eventually(func(ctx context.Context, a string) string {
						return ctx.Value("key").(string) + " " + a
					}).WithContext(ctxA).WithArguments(ctxB, "C").Should(Equal("B C"))
				})
			})

			Context("and an incorrect number of arguments is provided", func() {
				It("errors", func() {
					ig.G.Eventually(func(a int) string {
						return ""
					}).Should(Equal("foo"))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually has signature func(int) string takes 1 arguments but 0 have been provided.  Please use Eventually().WithArguments() to pass the correct set of arguments."))

					ig.G.Eventually(func(a int, b int) string {
						return ""
					}).WithArguments(1).Should(Equal("foo"))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually has signature func(int, int) string takes 2 arguments but 1 has been provided.  Please use Eventually().WithArguments() to pass the correct set of arguments."))

					ig.G.Eventually(func(a int, b int) string {
						return ""
					}).WithArguments(1, 2, 3).Should(Equal("foo"))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually has signature func(int, int) string takes 2 arguments but 3 have been provided.  Please use Eventually().WithArguments() to pass the correct set of arguments."))

					ig.G.Eventually(func(g Gomega, a int, b int) string {
						return ""
					}).WithArguments(1, 2, 3).Should(Equal("foo"))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually has signature func(types.Gomega, int, int) string takes 3 arguments but 4 have been provided.  Please use Eventually().WithArguments() to pass the correct set of arguments."))

					ig.G.Eventually(func(a int, b int, c ...int) string {
						return ""
					}).WithArguments(1).Should(Equal("foo"))
					Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually has signature func(int, int, ...int) string takes 3 arguments but 1 has been provided.  Please use Eventually().WithArguments() to pass the correct set of arguments."))

				})
			})
		})

		Describe("when passed an invalid function", func() {
			It("errors with a failure", func() {
				ig.G.Eventually(func() {}).Should(Equal("foo"))
				Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually had an invalid signature of func()"))
				Ω(ig.FailureSkip).Should(Equal([]int{2}))

				ig.G.Consistently(func(ctx context.Context) {}).Should(Equal("foo"))
				Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Consistently had an invalid signature of func(context.Context)"))
				Ω(ig.FailureSkip).Should(Equal([]int{2}))

				ig.G.Eventually(func(ctx context.Context, g Gomega) {}).Should(Equal("foo"))
				Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually had an invalid signature of func(context.Context, types.Gomega)"))
				Ω(ig.FailureSkip).Should(Equal([]int{2}))

				ig = NewInstrumentedGomega()
				ig.G.Eventually(func(foo string) {}).Should(Equal("foo"))
				Ω(ig.FailureMessage).Should(ContainSubstring("The function passed to Eventually had an invalid signature of func(string)"))
				Ω(ig.FailureSkip).Should(Equal([]int{2}))
			})
		})
	})

	Describe("Stopping Early", func() {
		Describe("when using OracleMatchers", func() {
			It("stops and gives up with an appropriate failure message if the OracleMatcher says things can't change", func() {
				c := make(chan bool)
				close(c)

				t := time.Now()
				ig.G.Eventually(c).WithTimeout(100*time.Millisecond).WithPolling(10*time.Millisecond).Should(Receive(), "Receive is an OracleMatcher that gives up if the channel is closed")
				Ω(time.Since(t)).Should(BeNumerically("<", 90*time.Millisecond))
				Ω(ig.FailureMessage).Should(ContainSubstring("No future change is possible."))
				Ω(ig.FailureMessage).Should(ContainSubstring("The channel is closed."))
			})

			It("never gives up if actual is a function", func() {
				c := make(chan bool)
				close(c)

				t := time.Now()
				ig.G.Eventually(func() chan bool { return c }).WithTimeout(100*time.Millisecond).WithPolling(10*time.Millisecond).Should(Receive(), "Receive is an OracleMatcher that gives up if the channel is closed")
				Ω(time.Since(t)).Should(BeNumerically(">=", 90*time.Millisecond))
				Ω(ig.FailureMessage).ShouldNot(ContainSubstring("No future change is possible."))
				Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
			})

			It("exits early and passes when used with consistently", func() {
				i := 0
				order := []string{}
				Consistently(nil).Should(QuickMatcherWithOracle(
					func(_ any) (bool, error) {
						order = append(order, fmt.Sprintf("match %d", i))
						i += 1
						if i > 4 {
							return false, nil
						}
						return true, nil
					},
					func(_ any) bool {
						order = append(order, fmt.Sprintf("oracle %d", i))
						if i == 3 {
							return false
						}
						return true
					},
				))
				Ω(i).Should(Equal(4))
				Ω(order).Should(Equal([]string{
					"oracle 0",
					"match 0",
					"oracle 1",
					"match 1",
					"oracle 2",
					"match 2",
					"oracle 3",
					"match 3",
				}))
			})
		})

		Describe("The StopTrying signal - when sent by actual", func() {
			var i int
			BeforeEach(func() {
				i = 0
			})

			Context("when returned as an additional error argument", func() {
				It("stops trying and prints out the error", func() {
					ig.G.Eventually(func() (int, error) {
						i += 1
						if i < 3 {
							return i, nil
						}
						return 0, StopTrying("bam")
					}).Should(Equal(3))
					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})

				It("fails, even if the match were to happen to succeed", func() {
					ig.G.Eventually(func() (int, error) {
						i += 1
						if i < 3 {
							return i, nil
						}
						return i, StopTrying("bam")
					}).Should(Equal(3))
					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})

				It("fails, even if the match were to happen to succeed and the user uses Succeed", func() {
					ig.G.Eventually(func() (int, error) {
						i += 1
						if i < 3 {
							return i, nil
						}
						return i, StopTrying("bam").Successfully()
					}).Should(Equal(3))
					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying (and ignoring call to Successfully(), as it is only relevant with Consistently) after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})
			})

			Context("when returned as the sole actual", func() {
				It("stops trying and prints out the error", func() {
					ig.G.Eventually(func() error {
						i += 1
						if i < 3 {
							return errors.New("boom")
						}
						return StopTrying("bam")
					}).Should(Succeed())
					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})
			})

			Context("when triggered via StopTrying.Now()", func() {
				It("stops trying and prints out the error", func() {
					ig.G.Eventually(func() int {
						i += 1
						if i < 3 {
							return i
						}
						StopTrying("bam").Now()
						return 0
					}).Should(Equal(3))
					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})

				It("works when used in conjunction with a Gomega and/or context", func() {
					ctx := context.WithValue(context.Background(), "key", "A")
					ig.G.Eventually(func(g Gomega, ctx context.Context, expected string) {
						i += 1
						if i < 3 {
							g.Expect(ctx.Value("key")).To(Equal(expected))
						}
						StopTrying("Out of tries").Now()
					}).WithContext(ctx).WithArguments("B").Should(Succeed())
					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("Out of tries"))
				})

				It("still allows regular panics to get through", func() {
					defer func() {
						e := recover()
						Ω(e).Should(Equal("welp"))
					}()
					Eventually(func() string {
						panic("welp")
					}).Should(Equal("A"))
				})
			})

			Context("when used with consistently", func() {
				It("signifies a failure", func() {
					ig.G.Consistently(func() (int, error) {
						i += 1
						if i >= 3 {
							return i, StopTrying("bam")
						}
						return i, nil
					}).Should(BeNumerically("<", 10))
					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})

				It("signifies success when called Successfully", func() {
					Consistently(func() (int, error) {
						i += 1
						if i >= 3 {
							return i, StopTrying("bam").Successfully()
						}
						return i, nil
					}).Should(BeNumerically("<", 10))
					Ω(i).Should(Equal(3))
				})
			})

			Context("when StopTrying has attachments", func() {
				It("formats them nicely", func() {
					type widget struct {
						Name               string
						DefronculatorCount int
					}
					type sprocket struct {
						Type     string
						Duration time.Duration
					}

					ig.G.Eventually(func() int {
						StopTrying("bam").Wrap(errors.New("boom")).
							Attach("widget", widget{"bob", 17}).
							Attach("sprocket", sprocket{"james", time.Second}).
							Now()
						return 0
					}).Should(Equal(1))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring(`bam: boom
widget:
    <internal_test.widget>: {
        Name: "bob",
        DefronculatorCount: 17,
    }
sprocket:
    <internal_test.sprocket>: {Type: "james", Duration: 1000000000}`))
				})
			})

			Context("when wrapped by an outer error", func() {
				It("still signals as StopTrying - but the outer-error is rendered, along with any attachments", func() {
					ig.G.Eventually(func() error {
						i += 1
						return fmt.Errorf("wizz: %w", StopTrying("bam").Wrap(errors.New("boom")).
							Attach("widget", "bob").
							Attach("sprocket", 17))
					}).Should(Succeed())
					Ω(i).Should(Equal(1))
					Ω(ig.FailureMessage).ShouldNot(ContainSubstring("The function passed to"))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring(`wizz: bam: boom
widget:
    <string>: bob
sprocket:
    <int>: 17`))

				})
			})

			Context("when a non-PollingSignalError is in play", func() {
				It("also includes the format.Object representation", func() {
					ig.G.Eventually(func() (int, error) {
						return 0, fmt.Errorf("bam")
					}).WithTimeout(10 * time.Millisecond).Should(Equal(1))
					Ω(ig.FailureMessage).Should(ContainSubstring(`{s: "bam"}`))
				})
			})
		})

		Describe("The StopTrying signal - when sent by the matcher", func() {
			var i int
			BeforeEach(func() {
				i = 0
			})

			Context("when returned as the error", func() {
				It("stops retrying", func() {
					ig.G.Eventually(nil).Should(QuickMatcher(func(_ any) (bool, error) {
						i += 1
						if i < 3 {
							return false, nil
						}
						return false, StopTrying("bam")
					}))

					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})

				It("fails regardless of the matchers value", func() {
					ig.G.Eventually(nil).Should(QuickMatcher(func(_ any) (bool, error) {
						i += 1
						if i < 3 {
							return false, nil
						}
						return true, StopTrying("bam")
					}))

					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})
			})

			Context("when thrown with .Now()", func() {
				It("stops retrying", func() {
					ig.G.Eventually(nil).Should(QuickMatcher(func(_ any) (bool, error) {
						i += 1
						if i < 3 {
							return false, nil
						}
						StopTrying("bam").Now()
						return false, nil
					}))

					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))
				})
			})

			Context("when used with consistently", func() {
				It("always signifies a failure", func() {
					ig.G.Consistently(nil).Should(QuickMatcher(func(_ any) (bool, error) {
						i += 1
						if i < 3 {
							return true, nil
						}
						return true, StopTrying("bam")
					}))

					Ω(i).Should(Equal(3))
					Ω(ig.FailureMessage).Should(ContainSubstring("Told to stop trying after"))
					Ω(ig.FailureMessage).Should(ContainSubstring("bam"))

				})
			})

			It("forwards any non-signaling panics", func() {
				defer func() {
					e := recover()
					Ω(e).Should(Equal("welp"))
				}()
				Eventually(nil).Should(QuickMatcher(func(actual any) (bool, error) {
					panic("welp")
				}))
			})
		})
	})

	Describe("dynamically adjusting the polling interval", func() {
		var i int
		var times []time.Duration
		var t time.Time

		BeforeEach(func() {
			i = 0
			times = []time.Duration{}
			t = time.Now()
		})

		Context("and the assertion eventually succeeds", func() {
			It("adjusts the timing of the next iteration", func() {
				Eventually(func() error {
					times = append(times, time.Since(t))
					t = time.Now()
					i += 1
					if i < 3 {
						return errors.New("stay on target")
					}
					if i == 3 {
						return TryAgainAfter(time.Millisecond * 200)
					}
					if i == 4 {
						return errors.New("you've switched off your targeting computer")
					}
					if i == 5 {
						TryAgainAfter(time.Millisecond * 100).Now()
					}
					if i == 6 {
						return errors.New("stay on target")
					}
					return nil
				}).ProbeEvery(time.Millisecond * 10).Should(Succeed())
				Ω(i).Should(Equal(7))
				Ω(times).Should(HaveLen(7))
				Ω(times[0]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[1]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[2]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[3]).Should(BeNumerically("~", time.Millisecond*200, time.Millisecond*200))
				Ω(times[4]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[5]).Should(BeNumerically("~", time.Millisecond*100, time.Millisecond*100))
				Ω(times[6]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
			})
		})

		Context("and the assertion timesout while waiting", func() {
			It("fails with a timeout and emits the try again after error", func() {
				ig.G.Eventually(func() (int, error) {
					times = append(times, time.Since(t))
					t = time.Now()
					i += 1
					if i < 3 {
						return i, nil
					}
					if i == 3 {
						return i, TryAgainAfter(time.Second * 10).Wrap(errors.New("bam"))
					}
					return i, nil
				}).ProbeEvery(time.Millisecond * 10).WithTimeout(time.Millisecond * 300).Should(Equal(4))
				Ω(i).Should(Equal(3))
				Ω(times).Should(HaveLen(3))
				Ω(times[0]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[1]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[2]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))

				Ω(ig.FailureMessage).Should(ContainSubstring("Timed out after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("told to try again after 10s: bam"))
				Ω(ig.FailureMessage).Should(ContainSubstring("At one point, however, the function did return successfully.\nYet, Eventually failed because the matcher was not satisfied:\nExpected\n    <int>: 2\nto equal\n    <int>: 4"))
			})
		})

		Context("when used with Consistently", func() {
			It("doesn't immediately count as a failure and adjusts the timing of the next iteration", func() {
				Consistently(func() (int, error) {
					times = append(times, time.Since(t))
					t = time.Now()
					i += 1
					if i == 3 {
						return i, TryAgainAfter(time.Millisecond * 200)
					}
					return i, nil
				}).ProbeEvery(time.Millisecond * 10).WithTimeout(time.Millisecond * 500).Should(BeNumerically("<", 1000))
				Ω(times[0]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[1]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[2]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[3]).Should(BeNumerically("~", time.Millisecond*200, time.Millisecond*200))
				Ω(times[4]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
			})

			It("doesn't count as a failure if a timeout occurs during the try again after window", func() {
				ig.G.Consistently(func() (int, error) {
					times = append(times, time.Since(t))
					t = time.Now()
					i += 1
					if i == 3 {
						return i, TryAgainAfter(time.Second * 10).Wrap(errors.New("bam"))
					}
					return i, nil
				}).ProbeEvery(time.Millisecond * 10).WithTimeout(time.Millisecond * 300).Should(BeNumerically("<", 1000))
				Ω(times[0]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[1]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(times[2]).Should(BeNumerically("~", time.Millisecond*10, time.Millisecond*10))
				Ω(ig.FailureMessage).Should(ContainSubstring("Timed out while waiting on TryAgainAfter after"))
				Ω(ig.FailureMessage).Should(ContainSubstring("told to try again after 10s: bam"))
			})
		})
	})

	Describe("reporting on failures in the presence of either matcher errors or actual errors", func() {
		When("there is no actual error or matcher error", func() {
			It("simply emits the correct matcher failure message", func() {
				ig.G.Eventually(func() (int, error) {
					return 5, nil
				}).WithTimeout(time.Millisecond*10).Should(QuickMatcher(func(actual any) (bool, error) {
					return false, nil
				}), "My Description")
				Ω(ig.FailureMessage).Should(HaveSuffix("My Description\nQM failure message: 5"))

				ig.G.Eventually(func() (int, error) {
					return 5, nil
				}).WithTimeout(time.Millisecond*10).ShouldNot(QuickMatcher(func(actual any) (bool, error) {
					return true, nil
				}), "My Description")
				Ω(ig.FailureMessage).Should(HaveSuffix("My Description\nQM negated failure message: 5"))
			})
		})

		When("there is no actual error, but there is a matcher error", func() {
			It("emits the matcher error", func() {
				ig.G.Eventually(func() (int, error) {
					return 5, nil
				}).WithTimeout(time.Millisecond*10).Should(QuickMatcher(func(actual any) (bool, error) {
					return false, fmt.Errorf("matcher-error")
				}), "My Description")
				Ω(ig.FailureMessage).Should(ContainSubstring("My Description\nThe matcher passed to Eventually returned the following error:\n    <*errors.errorString"))
				Ω(ig.FailureMessage).Should(ContainSubstring("matcher-error"))
			})

			When("the matcher error is a StopTrying with attachments", func() {
				It("emits the error along with its attachments", func() {
					ig.G.Eventually(func() (int, error) {
						return 5, nil
					}).WithTimeout(time.Millisecond*10).Should(QuickMatcher(func(actual any) (bool, error) {
						return false, StopTrying("stop-trying").Attach("now, please", 17)
					}), "My Description")
					Ω(ig.FailureMessage).Should(HavePrefix("Told to stop trying"))
					Ω(ig.FailureMessage).Should(HaveSuffix("My Description\nstop-trying\nnow, please:\n    <int>: 17"))
				})
			})
		})

		When("there is an actual error", func() {
			When("it never manages to get an actual", func() {
				It("simply emits the actual error", func() {
					ig.G.Eventually(func() (int, error) {
						return 0, fmt.Errorf("actual-err")
					}).WithTimeout(time.Millisecond*10).Should(QuickMatcher(func(actual any) (bool, error) {
						return true, nil
					}), "My Description")
					Ω(ig.FailureMessage).Should(ContainSubstring("My Description\nThe function passed to Eventually returned the following error:\n    <*errors.errorString"))
					Ω(ig.FailureMessage).Should(ContainSubstring("actual-err"))
				})
			})

			When("the actual error is because there was a non-nil/non-zero return value", func() {
				It("emites a clear message about the non-nil/non-zero return value", func() {
					ig.G.Eventually(func() (int, int, error) {
						return 0, 1, nil
					}).WithTimeout(time.Millisecond*10).Should(QuickMatcher(func(actual any) (bool, error) {
						return true, nil
					}), "My Description")
					Ω(ig.FailureMessage).Should(ContainSubstring("My Description\nThe function passed to Eventually had an unexpected non-nil/non-zero return value at index 1:\n    <int>: 1"))
				})
			})

			When("the actual error is because there was an assertion failure in the function, and there are return values", func() {
				It("emits a clear message about the error having occurred", func() {
					_, file, line, _ := runtime.Caller(0)
					ig.G.Eventually(func(g Gomega) int {
						g.Expect(true).To(BeFalse())
						return 1
					}).WithTimeout(time.Millisecond*10).Should(QuickMatcher(func(actual any) (bool, error) {
						return true, nil
					}), "My Description")
					Ω(ig.FailureMessage).Should(HaveSuffix("My Description\nThe function passed to Eventually failed at %s:%d with:\nExpected\n    <bool>: true\nto be false\n", file, line+2))
				})
			})

			When("the actual error is because there was an assertion failure in the function, and there are no return values", func() {
				It("emits a clear message about the error having occurred", func() {
					_, file, line, _ := runtime.Caller(0)
					ig.G.Eventually(func(g Gomega) {
						g.Expect(true).To(BeFalse())
					}).WithTimeout(time.Millisecond*10).Should(Succeed(), "My Description")
					Ω(ig.FailureMessage).Should(HaveSuffix("My Description\nThe function passed to Eventually failed at %s:%d with:\nExpected\n    <bool>: true\nto be false", file, line+2))
				})
			})

			When("it did manage to get an actual", func() {
				When("that actual generates a matcher error", func() {
					It("emits the actual error, and then emits the matcher error", func() {
						counter := 0
						ig.G.Eventually(func() (int, error) {
							counter += 1
							if counter > 3 {
								return counter, fmt.Errorf("actual-err")
							} else {
								return counter, nil
							}
						}).WithTimeout(time.Millisecond*100).Should(QuickMatcher(func(actual any) (bool, error) {
							if actual.(int) == 3 {
								return true, fmt.Errorf("matcher-err")
							}
							return false, nil
						}), "My Description")
						Ω(ig.FailureMessage).Should(ContainSubstring("My Description\nThe function passed to Eventually returned the following error:\n    <*errors.errorString"))
						Ω(ig.FailureMessage).Should(ContainSubstring("actual-err"))
						Ω(ig.FailureMessage).Should(ContainSubstring("At one point, however, the function did return successfully.\nYet, Eventually failed because the matcher returned the following error:"))
						Ω(ig.FailureMessage).Should(ContainSubstring("matcher-err"))
					})
				})

				When("that actual simply didn't match", func() {
					It("emits the matcher's failure message", func() {
						counter := 0
						ig.G.Eventually(func() (int, error) {
							counter += 1
							if counter > 3 {
								return counter, fmt.Errorf("actual-err")
							} else {
								return counter, nil
							}
						}).WithTimeout(time.Millisecond*100).Should(QuickMatcher(func(actual any) (bool, error) {
							actualInt := actual.(int)
							return actualInt > 3, nil
						}), "My Description")
						Ω(ig.FailureMessage).Should(ContainSubstring("My Description\nThe function passed to Eventually returned the following error:\n    <*errors.errorString"))
						Ω(ig.FailureMessage).Should(ContainSubstring("actual-err"))
						Ω(ig.FailureMessage).Should(ContainSubstring("At one point, however, the function did return successfully.\nYet, Eventually failed because the matcher was not satisfied:\nQM failure message: 3"))

					})
				})
			})
		})
	})

	When("vetting optional description parameters", func() {
		It("panics when Gomega matcher is at the beginning of optional description parameters", func() {
			ig := NewInstrumentedGomega()
			for _, expectator := range []string{
				"Should", "ShouldNot",
			} {
				Expect(func() {
					eventually := ig.G.Eventually(42) // sic!
					meth := reflect.ValueOf(eventually).MethodByName(expectator)
					Expect(meth.IsValid()).To(BeTrue())
					meth.Call([]reflect.Value{
						reflect.ValueOf(HaveLen(1)),
						reflect.ValueOf(ContainElement(42)),
					})
				}).To(PanicWith(MatchRegexp("Asynchronous assertion has a GomegaMatcher as the first element of optionalDescription")))
			}
		})

		It("accepts Gomega matchers in optional description parameters after the first", func() {
			Expect(func() {
				ig := NewInstrumentedGomega()
				ig.G.Eventually(42).Should(HaveLen(1), "foo", ContainElement(42))
			}).NotTo(Panic())
		})
	})

	Context("eventual nil-ism", func() { // issue #555
		It("doesn't panic on nil actual", func() {
			ig := NewInstrumentedGomega()
			Expect(func() {
				ig.G.Eventually(nil).Should(BeNil())
			}).NotTo(Panic())
		})

		It("doesn't panic on function returning nil error", func() {
			ig := NewInstrumentedGomega()
			Expect(func() {
				ig.G.Eventually(func() error { return nil }).Should(BeNil())
			}).NotTo(Panic())
		})
	})

	When("using MustPassRepeatedly", func() {
		It("errors when using on Consistently", func() {
			ig.G.Consistently(func(g Gomega) {}).MustPassRepeatedly(2).Should(Succeed())
			Ω(ig.FailureMessage).Should(ContainSubstring("Invalid use of MustPassRepeatedly with Consistently it can only be used with Eventually"))
			Ω(ig.FailureSkip).Should(Equal([]int{2}))
		})
		It("errors when using with 0", func() {
			ig.G.Eventually(func(g Gomega) {}).MustPassRepeatedly(0).Should(Succeed())
			Ω(ig.FailureMessage).Should(ContainSubstring("Invalid use of MustPassRepeatedly with Eventually parameter can't be < 1"))
			Ω(ig.FailureSkip).Should(Equal([]int{2}))
		})

		It("should wait 2 success before success", func() {
			counter := 0
			ig.G.Eventually(func() bool {
				counter++
				return counter > 5
			}).MustPassRepeatedly(2).Should(BeTrue())
			Ω(counter).Should(Equal(7))
			Ω(ig.FailureMessage).Should(BeZero())
		})

		It("should fail if it never succeeds twice in a row", func() {
			counter := 0
			ig.G.Eventually(func() int {
				counter++
				return counter % 2
			}).WithTimeout(200 * time.Millisecond).WithPolling(20 * time.Millisecond).MustPassRepeatedly(2).Should(Equal(1))
			Ω(counter).Should(Equal(10))
			Ω(ig.FailureMessage).ShouldNot(BeZero())
		})

		It("TryAgainAfter doesn't restore count", func() {
			counter := 0
			ig.G.Eventually(func() (bool, error) {
				counter++
				if counter == 5 {
					return false, TryAgainAfter(time.Millisecond * 200)
				}
				return counter >= 4, nil
			}).MustPassRepeatedly(3).Should(BeTrue())
			Ω(counter).Should(Equal(7))
			Ω(ig.FailureMessage).Should(BeZero())
		})

	})
})
