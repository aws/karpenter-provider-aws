package matchers_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveExistingField", func() {

	var book Book
	BeforeEach(func() {
		book = Book{
			Title: "Les Miserables",
			Author: person{
				FirstName: "Victor",
				LastName:  "Hugo",
				DOB:       time.Date(1802, 2, 26, 0, 0, 0, 0, time.UTC),
			},
			Pages: 2783,
			Sequel: &Book{
				Title: "Les Miserables 2",
			},
		}
	})

	DescribeTable("traversing the struct works",
		func(field string) {
			Ω(book).Should(HaveExistingField(field))
		},
		Entry("Top-level field", "Title"),
		Entry("Nested field", "Author.FirstName"),
		Entry("Top-level method", "AuthorName()"),
		Entry("Nested method", "Author.DOB.Year()"),
		Entry("Traversing past a method", "AbbreviatedAuthor().FirstName"),
		Entry("Traversing a pointer", "Sequel.Title"),
	)

	DescribeTable("negation works",
		func(field string) {
			Ω(book).ShouldNot(HaveExistingField(field))
		},
		Entry("Top-level field", "Class"),
		Entry("Nested field", "Author.Class"),
		Entry("Top-level method", "ClassName()"),
		Entry("Nested method", "Author.DOB.BOT()"),
		Entry("Traversing past a method", "AbbreviatedAuthor().LastButOneName"),
		Entry("Traversing a pointer", "Sequel.Titles"),
	)

	It("errors appropriately", func() {
		success, err := HaveExistingField("Pages.Count").Match(book)
		Ω(success).Should(BeFalse())
		Ω(err.Error()).Should(Equal("HaveExistingField encountered:\n    <int>: 2783\nWhich is not a struct."))

		success, err = HaveExistingField("Prequel.Title").Match(book)
		Ω(success).Should(BeFalse())
		Ω(err.Error()).Should(ContainSubstring("HaveExistingField encountered nil while dereferencing a pointer of type *matchers_test.Book."))

		success, err = HaveExistingField("HasArg()").Match(book)
		Ω(success).Should(BeFalse())
		Ω(err.Error()).Should(ContainSubstring("HaveExistingField found an invalid method named 'HasArg()' in struct of type matchers_test.Book.\nMethods must take no arguments and return exactly one value."))
	})

	It("renders failure messages", func() {
		matcher := HaveExistingField("Turtle")
		success, err := matcher.Match(book)
		Ω(success).Should(BeFalse())
		Ω(err).ShouldNot(HaveOccurred())

		msg := matcher.FailureMessage(book)
		Ω(msg).Should(MatchRegexp(`(?s)Expected\n\s+<matchers_test\.Book>: .*\nto have field 'Turtle'`))

		matcher = HaveExistingField("Title")
		success, err = matcher.Match(book)
		Ω(success).Should(BeTrue())
		Ω(err).ShouldNot(HaveOccurred())

		msg = matcher.NegatedFailureMessage(book)
		Ω(msg).Should(MatchRegexp(`(?s)Expected\n\s+<matchers_test\.Book>: .*\nnot to have field 'Title'`))
	})

})
