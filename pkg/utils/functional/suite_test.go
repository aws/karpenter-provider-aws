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
	Context("UnionStringMaps", func() {
		empty := map[string]string{}
		original := map[string]string{
			"a": "b",
			"c": "d",
		}
		overwriter := map[string]string{
			"a": "y",
			"c": "z",
		}
		disjoiner := map[string]string{
			"d": "y",
			"e": "z",
		}
		uberwriter := map[string]string{
			"d": "q",
			"e": "z",
		}

		Specify("no args returns empty", func() {
			Expect(UnionStringMaps()).To(BeEmpty())
		})

		Specify("multiple empty returns empty", func() {
			Expect(UnionStringMaps(empty, empty, empty, empty)).To(BeEmpty())
		})

		Specify("one arg returns the arg", func() {
			Expect(UnionStringMaps(original)).To(Equal(original))
		})

		Specify("2nd arg overrrides 1st", func() {
			Expect(UnionStringMaps(original, overwriter)).To(Equal(overwriter))
		})

		Specify("returns union when disjoint", func() {
			expected := map[string]string{
				"a": "b",
				"c": "d",
				"d": "y",
				"e": "z",
			}
			Expect(UnionStringMaps(original, disjoiner)).To(Equal(expected))
		})

		Specify("final arg takes precedence", func() {
			expected := map[string]string{
				"a": "b",
				"c": "d",
				"d": "q",
				"e": "z",
			}
			Expect(UnionStringMaps(original, disjoiner, empty, uberwriter)).To(Equal(expected))
		})
	})
})
