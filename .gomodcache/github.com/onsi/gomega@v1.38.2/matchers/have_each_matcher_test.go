package matchers_test

import (
	"github.com/onsi/gomega/matchers/internal/miter"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("HaveEach", func() {
	When("passed a supported type", func() {
		Context("and expecting a non-matcher", func() {
			It("should do the right thing", func() {
				Expect([2]int{2, 2}).Should(HaveEach(2))
				Expect([2]int{2, 3}).ShouldNot(HaveEach(3))

				Expect([]int{2, 2}).Should(HaveEach(2))
				Expect([]int{1, 2}).ShouldNot(HaveEach(3))

				Expect(map[string]int{"foo": 2, "bar": 2}).Should(HaveEach(2))
				Expect(map[int]int{3: 3, 4: 2}).ShouldNot(HaveEach(3))

				arr := make([]myCustomType, 2)
				arr[0] = myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}
				arr[1] = myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}
				Expect(arr).Should(HaveEach(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}))
				Expect(arr).ShouldNot(HaveEach(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"b", "c"}}))

				// ...and finaaaaaly, let's eat our own documentation ;)
				Expect([]string{"Foo", "FooBar"}).Should(HaveEach(ContainSubstring("Foo")))
				Expect([]string{"Foo", "FooBar"}).ShouldNot(HaveEach(ContainSubstring("Bar")))
			})
		})

		Context("and expecting a matcher", func() {
			It("should pass each element through the matcher", func() {
				Expect([]int{1, 2, 3}).Should(HaveEach(BeNumerically(">=", 1)))
				Expect([]int{1, 2, 3}).ShouldNot(HaveEach(BeNumerically(">", 1)))
				Expect(map[string]int{"foo": 1, "bar": 2}).Should(HaveEach(BeNumerically(">=", 1)))
				Expect(map[string]int{"foo": 1, "bar": 2}).ShouldNot(HaveEach(BeNumerically(">=", 2)))
			})

			It("should not power through if the matcher ever fails", func() {
				actual := []any{1, 2, "3", 4}
				success, err := (&HaveEachMatcher{Element: BeNumerically(">=", 1)}).Match(actual)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})

			It("should fail if the matcher fails", func() {
				actual := []any{1, 2, "3", "4"}
				success, err := (&HaveEachMatcher{Element: BeNumerically(">=", 1)}).Match(actual)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	When("passed an empty supported type or correctly typed nil", func() {
		It("should error", func() {
			success, err := (&HaveEachMatcher{Element: []int{}}).Match(42)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			var nilSlice []int
			success, err = (&HaveEachMatcher{Element: nilSlice}).Match(1)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			var nilMap map[int]string
			success, err = (&HaveEachMatcher{Element: nilMap}).Match(1)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			// again, we eat our own documentation food here...
			Expect([]int{}).To(Or(BeEmpty(), HaveEach(42)))
			Expect([]int{1}).NotTo(Or(BeEmpty(), HaveEach(42)))
		})
	})

	When("passed an unsupported type", func() {
		It("should error", func() {
			success, err := (&HaveEachMatcher{Element: 0}).Match(0)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&HaveEachMatcher{Element: 0}).Match("abc")
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&HaveEachMatcher{Element: 0}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("iterators", func() {
		BeforeEach(func() {
			if !miter.HasIterators() {
				Skip("iterators not available")
			}
		})

		When("passed an iterator type", func() {
			Context("and expecting a non-matcher", func() {
				It("should do the right thing", func() {
					Expect(fooIter).Should(HaveEach("foo"))
					Expect(fooIter).ShouldNot(HaveEach("bar"))

					Expect(fooIter2).Should(HaveEach("foo"))
					Expect(fooIter2).ShouldNot(HaveEach("bar"))
				})
			})

			Context("and expecting a matcher", func() {
				It("should pass each element through the matcher", func() {
					Expect(universalIter).Should(HaveEach(HaveLen(3)))
					Expect(universalIter).ShouldNot(HaveEach(HaveLen(4)))

					Expect(universalIter2).Should(HaveEach(HaveLen(3)))
					Expect(universalIter2).ShouldNot(HaveEach(HaveLen(4)))
				})

				It("should not power through if the matcher ever fails", func() {
					success, err := (&HaveEachMatcher{Element: BeNumerically(">=", 1)}).Match(universalIter)
					Expect(success).Should(BeFalse())
					Expect(err).Should(HaveOccurred())

					success, err = (&HaveEachMatcher{Element: BeNumerically(">=", 1)}).Match(universalIter2)
					Expect(success).Should(BeFalse())
					Expect(err).Should(HaveOccurred())
				})
			})
		})

		When("passed an iterator yielding nothing or correctly typed nil", func() {
			It("should error", func() {
				success, err := (&HaveEachMatcher{Element: "foo"}).Match(emptyIter)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())

				success, err = (&HaveEachMatcher{Element: "foo"}).Match(emptyIter2)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())

				var nilIter func(func(string) bool)
				success, err = (&HaveEachMatcher{Element: "foo"}).Match(nilIter)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())

				var nilIter2 func(func(int, string) bool)
				success, err = (&HaveEachMatcher{Element: "foo"}).Match(nilIter2)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})
	})
})
