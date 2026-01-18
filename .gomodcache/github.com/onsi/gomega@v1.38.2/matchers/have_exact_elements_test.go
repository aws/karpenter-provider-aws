package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/matchers/internal/miter"
)

var _ = Describe("HaveExactElements", func() {
	Context("with a slice", func() {
		It("should do the right thing", func() {
			Expect([]string{"foo", "bar"}).Should(HaveExactElements("foo", "bar"))
			Expect([]string{"foo", "bar"}).ShouldNot(HaveExactElements("foo"))
			Expect([]string{"foo", "bar"}).ShouldNot(HaveExactElements("foo", "bar", "baz"))
			Expect([]string{"foo", "bar"}).ShouldNot(HaveExactElements("bar", "foo"))
		})

		It("should work with arbitrary types, including nil", func() {
			Expect([]any{"foo", nil, "bar", 17, true, []string{"hi", "there"}}).Should(HaveExactElements("foo", nil, "bar", 17, true, []string{"hi", "there"}))
		})
	})
	Context("with an array", func() {
		It("should do the right thing", func() {
			Expect([2]string{"foo", "bar"}).Should(HaveExactElements("foo", "bar"))
			Expect([2]string{"foo", "bar"}).ShouldNot(HaveExactElements("foo"))
			Expect([2]string{"foo", "bar"}).ShouldNot(HaveExactElements("foo", "bar", "baz"))
			Expect([2]string{"foo", "bar"}).ShouldNot(HaveExactElements("bar", "foo"))
		})
	})
	Context("with map", func() {
		It("should error", func() {
			failures := InterceptGomegaFailures(func() {
				Expect(map[int]string{1: "foo"}).Should(HaveExactElements("foo"))
			})

			Expect(failures).Should(HaveLen(1))
		})
	})
	Context("with anything else", func() {
		It("should error", func() {
			failures := InterceptGomegaFailures(func() {
				Expect("foo").Should(HaveExactElements("f", "o", "o"))
			})

			Expect(failures).Should(HaveLen(1))
		})
	})

	When("passed matchers", func() {
		It("should pass if matcher pass", func() {
			Expect([]string{"foo", "bar", "baz"}).Should(HaveExactElements("foo", MatchRegexp("^ba"), MatchRegexp("az$")))
			Expect([]string{"foo", "bar", "baz"}).ShouldNot(HaveExactElements("foo", MatchRegexp("az$"), MatchRegexp("^ba")))
			Expect([]string{"foo", "bar", "baz"}).ShouldNot(HaveExactElements("foo", MatchRegexp("az$")))
			Expect([]string{"foo", "bar", "baz"}).ShouldNot(HaveExactElements("foo", MatchRegexp("az$"), "baz", "bac"))
		})

		When("a matcher errors", func() {
			It("should soldier on", func() {
				Expect([]string{"foo", "bar", "baz"}).ShouldNot(HaveExactElements(BeFalse(), "bar", "baz"))
				Expect([]any{"foo", "bar", false}).Should(HaveExactElements(ContainSubstring("foo"), "bar", BeFalse()))
			})

			It("should include the error message, not the failure message", func() {
				failures := InterceptGomegaFailures(func() {
					Expect([]string{"foo", "bar", "baz"}).Should(HaveExactElements("foo", BeFalse(), "bar"))
				})
				Ω(failures[0]).ShouldNot(ContainSubstring("to be false"))
				Ω(failures[0]).Should(ContainSubstring("1: Expected a boolean.  Got:\n    <string>: bar"))
			})
		})
	})

	When("passed exactly one argument, and that argument is a slice", func() {
		It("should match against the elements of that arguments", func() {
			Expect([]string{"foo", "bar", "baz"}).Should(HaveExactElements([]string{"foo", "bar", "baz"}))
			Expect([]string{"foo", "bar", "baz"}).ShouldNot(HaveExactElements([]string{"foo", "bar"}))
		})
	})

	When("passed nil", func() {
		It("should fail correctly", func() {
			failures := InterceptGomegaFailures(func() {
				var expected []any
				Expect([]string{"one"}).Should(HaveExactElements(expected...))
			})
			Expect(failures).Should(HaveLen(1))
		})
	})

	Describe("Failure Message", func() {
		When("actual contains extra elements", func() {
			It("should print the starting index of the extra elements", func() {
				failures := InterceptGomegaFailures(func() {
					Expect([]int{1, 2}).Should(HaveExactElements(1))
				})

				expected := "Expected\n.*\\[1, 2\\]\nto have exact elements with\n.*\\[1\\]\nthe extra elements start from index 1"
				Expect(failures).To(ConsistOf(MatchRegexp(expected)))
			})
		})

		When("actual misses an element", func() {
			It("should print the starting index of missing element", func() {
				failures := InterceptGomegaFailures(func() {
					Expect([]int{1}).Should(HaveExactElements(1, 2))
				})

				expected := "Expected\n.*\\[1\\]\nto have exact elements with\n.*\\[1, 2\\]\nthe missing elements start from index 1"
				Expect(failures).To(ConsistOf(MatchRegexp(expected)))
			})
		})

		When("actual have mismatched elements", func() {
			It("should print the index, expected element, and actual element", func() {
				failures := InterceptGomegaFailures(func() {
					Expect([]int{1, 2}).Should(HaveExactElements(2, 1))
				})

				expected := `Expected
.*\[1, 2\]
to have exact elements with
.*\[2, 1\]
the mismatch indexes were:
0: Expected
    <int>: 1
to equal
    <int>: 2
1: Expected
    <int>: 2
to equal
    <int>: 1`
				Expect(failures[0]).To(MatchRegexp(expected))
			})
		})
	})

	When("matcher instance is reused", func() {
		// This is a regression test for https://github.com/onsi/gomega/issues/647.
		// Matcher instance may be reused, if placed inside ContainElement() or other collection matchers.
		It("should work properly", func() {
			matchSingleFalse := HaveExactElements(Equal(false))
			Expect([]bool{true}).ShouldNot(matchSingleFalse)
			Expect([]bool{false}).Should(matchSingleFalse)
		})
	})

	Context("iterators", func() {
		BeforeEach(func() {
			if !miter.HasIterators() {
				Skip("iterators not available")
			}
		})

		Context("with an iter.Seq", func() {
			It("should do the right thing", func() {
				Expect(universalIter).Should(HaveExactElements("foo", "bar", "baz"))
				Expect(universalIter).ShouldNot(HaveExactElements("foo"))
				Expect(universalIter).ShouldNot(HaveExactElements("foo", "bar", "baz", "argh"))
				Expect(universalIter).ShouldNot(HaveExactElements("foo", "bar"))

				var nilIter func(func(string) bool)
				Expect(nilIter).Should(HaveExactElements())
			})
		})

		Context("with an iter.Seq2", func() {
			It("should error", func() {
				failures := InterceptGomegaFailures(func() {
					Expect(universalIter2).Should(HaveExactElements("foo"))
				})

				Expect(failures).Should(HaveLen(1))
			})
		})

		When("passed matchers", func() {
			It("should pass if matcher pass", func() {
				Expect(universalIter).Should(HaveExactElements("foo", MatchRegexp("^ba"), MatchRegexp("az$")))
				Expect(universalIter).ShouldNot(HaveExactElements("foo", MatchRegexp("az$"), MatchRegexp("^ba")))
				Expect(universalIter).ShouldNot(HaveExactElements("foo", MatchRegexp("az$")))
				Expect(universalIter).ShouldNot(HaveExactElements("foo", MatchRegexp("az$"), "baz", "bac"))
			})

			When("a matcher errors", func() {
				It("should soldier on", func() {
					Expect(universalIter).ShouldNot(HaveExactElements(BeFalse(), "bar", "baz"))
					poly := []any{"foo", "bar", false}
					polyIter := func(yield func(any) bool) {
						for _, v := range poly {
							if !yield(v) {
								return
							}
						}
					}
					Expect(polyIter).Should(HaveExactElements(ContainSubstring("foo"), "bar", BeFalse()))
				})

				It("should include the error message, not the failure message", func() {
					failures := InterceptGomegaFailures(func() {
						Expect(universalIter).Should(HaveExactElements("foo", BeFalse(), "bar"))
					})
					Ω(failures[0]).ShouldNot(ContainSubstring("to be false"))
					Ω(failures[0]).Should(ContainSubstring("1: Expected a boolean.  Got:\n    <string>: bar"))
				})
			})
		})
		When("passed exactly one argument, and that argument is a slice", func() {
			It("should match against the elements of that arguments", func() {
				Expect(universalIter).Should(HaveExactElements([]string{"foo", "bar", "baz"}))
				Expect(universalIter).ShouldNot(HaveExactElements([]string{"foo", "bar"}))
			})
		})

		When("passed nil", func() {
			It("should fail correctly", func() {
				failures := InterceptGomegaFailures(func() {
					var expected []any
					Expect(universalIter).Should(HaveExactElements(expected...))
				})
				Expect(failures).Should(HaveLen(1))
			})
		})

		Describe("Failure Message", func() {
			When("actual contains extra elements", func() {
				It("should print the starting index of the extra elements", func() {
					failures := InterceptGomegaFailures(func() {
						Expect(universalIter).Should(HaveExactElements("foo"))
					})

					expected := "Expected\n.*<func\\(func\\(string\\) bool\\)>:.*\nto have exact elements with\n.*\\[\"foo\"\\]\nthe extra elements start from index 1"
					Expect(failures).To(ConsistOf(MatchRegexp(expected)))
				})
			})

			When("actual misses an element", func() {
				It("should print the starting index of missing element", func() {
					failures := InterceptGomegaFailures(func() {
						Expect(universalIter).Should(HaveExactElements("foo", "bar", "baz", "argh"))
					})

					expected := "Expected\n.*<func\\(func\\(string\\) bool\\)>:.*\nto have exact elements with\n.*\\[\"foo\", \"bar\", \"baz\", \"argh\"\\]\nthe missing elements start from index 3"
					Expect(failures).To(ConsistOf(MatchRegexp(expected)))
				})
			})
		})

		When("actual have mismatched elements", func() {
			It("should print the index, expected element, and actual element", func() {
				failures := InterceptGomegaFailures(func() {
					Expect(universalIter).Should(HaveExactElements("bar", "baz", "foo"))
				})

				expected := `Expected
.*<func\(func\(string\) bool\)>:.*
to have exact elements with
.*\["bar", "baz", "foo"\]
the mismatch indexes were:
0: Expected
    <string>: foo
to equal
    <string>: bar
1: Expected
    <string>: bar
to equal
    <string>: baz
2: Expected
    <string>: baz
to equal
    <string>: foo`
				Expect(failures[0]).To(MatchRegexp(expected))
			})
		})

		When("matcher instance is reused", func() {
			// This is a regression test for https://github.com/onsi/gomega/issues/647.
			// Matcher instance may be reused, if placed inside ContainElement() or other collection matchers.
			It("should work properly", func() {
				matchSingleFalse := HaveExactElements(Equal(false))
				allOf := func(a []bool) func(func(bool) bool) {
					return func(yield func(bool) bool) {
						for _, b := range a {
							if !yield(b) {
								return
							}
						}
					}
				}
				Expect(allOf([]bool{true})).ShouldNot(matchSingleFalse)
				Expect(allOf([]bool{false})).Should(matchSingleFalse)
			})
		})
	})
})
