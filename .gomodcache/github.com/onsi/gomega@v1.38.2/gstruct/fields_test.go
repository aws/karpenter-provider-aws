package gstruct_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Struct", func() {
	allFields := struct{ A, B string }{"a", "b"}
	missingFields := struct{ A string }{"a"}
	extraFields := struct{ A, B, C, D string }{"a", "b", "c", "d"}
	extraUnexportedFields := struct{ A, B, c, d string }{"a", "b", "c", "d"}
	emptyFields := struct{ A, B string }{}

	It("should strictly match all fields", func() {
		m := MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).ShouldNot(m, "should fail with missing fields")
		Expect(extraFields).ShouldNot(m, "should fail with extra fields")
		Expect(extraUnexportedFields).ShouldNot(m, "should fail with extra unexported fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")

		m = MatchAllFields(Fields{
			"A": Equal("a"),
			"B": Equal("fail"),
		})
		Expect(allFields).ShouldNot(m, "should run nested matchers")
	})

	It("should handle empty structs", func() {
		m := MatchAllFields(Fields{})
		Expect(struct{}{}).Should(m, "should handle empty structs")
		Expect(allFields).ShouldNot(m, "should fail with extra fields")
	})

	It("should ignore missing fields", func() {
		m := MatchFields(IgnoreMissing, Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).Should(m, "should ignore missing fields")
		Expect(extraFields).ShouldNot(m, "should fail with extra fields")
		Expect(extraUnexportedFields).ShouldNot(m, "should fail extra unexported fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")
	})

	It("should ignore extra fields", func() {
		m := MatchFields(IgnoreExtras, Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).ShouldNot(m, "should fail with missing fields")
		Expect(extraFields).Should(m, "should ignore extra fields")
		Expect(extraUnexportedFields).Should(m, "should ignore unexported extra fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")
	})

	It("should ignore unexported extra fields", func() {
		m := MatchFields(IgnoreUnexportedExtras, Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).ShouldNot(m, "should fail with missing fields")
		Expect(extraFields).ShouldNot(m, "should fail with exported extra fields")
		Expect(extraUnexportedFields).Should(m, "should ignore unexported extra fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")
	})

	It("should ignore ignored fields", func() {
		m := MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(extraFields).ShouldNot(m, "should fail with exported extra fields")

		m = MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
			"C": Ignore(),
		})
		Expect(extraFields).ShouldNot(m, "should fail with exported extra fields partially ignored")

		m = MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
			"C": Ignore(),
			"D": Ignore(),
		})
		Expect(extraFields).Should(m, "should match with all remaining fields ignored")
	})

	It("should ignore ignored unexported fields", func() {
		m := MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(extraUnexportedFields).ShouldNot(m, "should fail with exported extra fields")

		m = MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
			"c": Ignore(),
		})
		Expect(extraUnexportedFields).ShouldNot(m, "should fail with exported extra fields partially ignored")

		m = MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
			"c": Ignore(),
			"d": Ignore(),
		})
		Expect(extraUnexportedFields).Should(m, "should match with all remaining fields ignored")

		m = MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
			"c": Ignore(),
			"d": Reject(),
		})
		Expect(extraUnexportedFields).ShouldNot(m, "should fail if we used Reject() on an unexported field")
	})

	It("should ignore missing and extra fields", func() {
		m := MatchFields(IgnoreMissing|IgnoreExtras, Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).Should(m, "should ignore missing fields")
		Expect(extraFields).Should(m, "should ignore extra fields")
		Expect(extraUnexportedFields).Should(m, "should ignore unexported extra fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")

		m = MatchFields(IgnoreMissing|IgnoreExtras, Fields{
			"A": Equal("a"),
			"B": Equal("fail"),
		})
		Expect(allFields).ShouldNot(m, "should run nested matchers")
	})

	It("should produce sensible error messages", func() {
		m := MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})

		actual := struct{ A, C string }{A: "b", C: "c"}

		//Because the order of the constituent errors can't be guaranteed,
		//we do a number of checks to make sure everything's included
		m.Match(actual)
		Expect(m.FailureMessage(actual)).Should(HavePrefix(
			"Expected\n    <string>: \nto match fields: {\n",
		))
		Expect(m.FailureMessage(actual)).Should(ContainSubstring(
			".A:\n	Expected\n	    <string>: b\n	to equal\n	    <string>: a\n",
		))
		Expect(m.FailureMessage(actual)).Should(ContainSubstring(
			"missing expected field B\n",
		))
		Expect(m.FailureMessage(actual)).Should(ContainSubstring(
			".C:\n	unexpected field C: {A:b C:c}",
		))
	})
})
