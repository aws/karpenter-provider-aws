package matchers_test

import (
	"errors"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

type wrapError struct {
	msg string
	err error
}

func (e wrapError) Error() string {
	return e.msg
}

func (e wrapError) Unwrap() error {
	return e.err
}

var _ = Describe("BeComparableTo", func() {
	When("asserting that nil is comparable to nil", func() {
		It("should error", func() {
			success, err := (&BeComparableToMatcher{Expected: nil}).Match(nil)

			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("When asserting on nil", func() {
		It("should do the right thing", func() {
			Expect("foo").ShouldNot(BeComparableTo(nil))
			Expect(nil).ShouldNot(BeComparableTo(3))
			Expect([]int{1, 2}).ShouldNot(BeComparableTo(nil))
		})
	})

	Context("When asserting time with different location ", func() {
		var t1, t2, t3 time.Time

		BeforeEach(func() {
			t1 = time.Time{}
			t2 = time.Time{}.Local()
			t3 = t1.Add(time.Second)
		})

		It("should do the right thing", func() {
			Expect(t1).Should(BeComparableTo(t2))
			Expect(t1).ShouldNot(BeComparableTo(t3))
		})
	})

	Context("When struct contain unexported fields", func() {
		type structWithUnexportedFields struct {
			unexported string
			Exported   string
		}

		var s1, s2 structWithUnexportedFields

		BeforeEach(func() {
			s1 = structWithUnexportedFields{unexported: "unexported", Exported: "Exported"}
			s2 = structWithUnexportedFields{unexported: "unexported", Exported: "Exported"}
		})

		It("should get match err", func() {
			success, err := (&BeComparableToMatcher{Expected: s1}).Match(s2)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})

		It("should do the right thing", func() {
			Expect(s1).Should(BeComparableTo(s2, cmpopts.IgnoreUnexported(structWithUnexportedFields{})))
		})
	})

	Context("When compare error", func() {
		var err1, err2 error

		It("not equal", func() {
			err1 = errors.New("error")
			err2 = errors.New("error")
			Expect(err1).ShouldNot(BeComparableTo(err2, cmpopts.EquateErrors()))
		})

		It("equal if err1 is err2", func() {
			err1 = errors.New("error")
			err2 = &wrapError{
				msg: "some error",
				err: err1,
			}

			Expect(err1).Should(BeComparableTo(err2, cmpopts.EquateErrors()))
		})
	})

	Context("When asserting equal between objects", func() {
		Context("with no additional cmp.Options", func() {
			It("should do the right thing", func() {
				Expect(5).Should(BeComparableTo(5))
				Expect(5.0).Should(BeComparableTo(5.0))

				Expect(5).ShouldNot(BeComparableTo("5"))
				Expect(5).ShouldNot(BeComparableTo(5.0))
				Expect(5).ShouldNot(BeComparableTo(3))

				Expect("5").Should(BeComparableTo("5"))
				Expect([]int{1, 2}).Should(BeComparableTo([]int{1, 2}))
				Expect([]int{1, 2}).ShouldNot(BeComparableTo([]int{2, 1}))
				Expect([]byte{'f', 'o', 'o'}).Should(BeComparableTo([]byte{'f', 'o', 'o'}))
				Expect([]byte{'f', 'o', 'o'}).ShouldNot(BeComparableTo([]byte{'b', 'a', 'r'}))
				Expect(map[string]string{"a": "b", "c": "d"}).Should(BeComparableTo(map[string]string{"a": "b", "c": "d"}))
				Expect(map[string]string{"a": "b", "c": "d"}).ShouldNot(BeComparableTo(map[string]string{"a": "b", "c": "e"}))
			})
		})

		Context("with custom cmp.Options", func() {
			It("should do the right thing", func() {
				Expect(myCustomType{s: "abc", n: 3, f: 2.0, arr: []string{"a", "b"}}).Should(BeComparableTo(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}, cmpopts.IgnoreUnexported(myCustomType{})))

				Expect(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}).Should(BeComparableTo(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}, cmp.AllowUnexported(myCustomType{})))
				Expect(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}).ShouldNot(BeComparableTo(myCustomType{s: "bar", n: 3, f: 2.0, arr: []string{"a", "b"}}, cmp.AllowUnexported(myCustomType{})))
				Expect(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}).ShouldNot(BeComparableTo(myCustomType{s: "foo", n: 2, f: 2.0, arr: []string{"a", "b"}}, cmp.AllowUnexported(myCustomType{})))
				Expect(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}).ShouldNot(BeComparableTo(myCustomType{s: "foo", n: 3, f: 3.0, arr: []string{"a", "b"}}, cmp.AllowUnexported(myCustomType{})))
				Expect(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}).ShouldNot(BeComparableTo(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b", "c"}}, cmp.AllowUnexported(myCustomType{})))
			})

			type structWithUnexportedFields struct {
				unexported string
				Exported   string
			}

			It("should produce failure message according to passed cmp.Option", func() {
				actual := structWithUnexportedFields{unexported: "xxx", Exported: "exported field value"}
				expectedEqual := structWithUnexportedFields{unexported: "yyy", Exported: "exported field value"}
				matcherWithEqual := BeComparableTo(expectedEqual, cmpopts.IgnoreUnexported(structWithUnexportedFields{}))

				Expect(matcherWithEqual.FailureMessage(actual)).To(BeEquivalentTo("Expected object to be comparable, diff: "))

				expectedDifferent := structWithUnexportedFields{unexported: "xxx", Exported: "other value"}
				matcherWithDifference := BeComparableTo(expectedDifferent, cmpopts.IgnoreUnexported(structWithUnexportedFields{}))
				Expect(matcherWithDifference.FailureMessage(actual)).To(ContainSubstring("1 ignored field"))
				Expect(matcherWithDifference.FailureMessage(actual)).To(ContainSubstring("Exported: \"other value\""))
				Expect(matcherWithDifference.FailureMessage(actual)).To(ContainSubstring("Exported: \"exported field value\""))
			})
		})
	})
})
