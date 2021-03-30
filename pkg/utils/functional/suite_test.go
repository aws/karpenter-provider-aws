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
		empty := make(map[string]string)

		Specify("no args returns empty", func() {
			Expect(UnionStringMaps()).To(BeEmpty())
		})

		Specify("all empty returns empty", func() {
			Expect(UnionStringMaps(empty, empty, empty, empty)).To(BeEmpty())
		})

		Context("non-empty args", func() {
			m := map[string]string{
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
			m4 := map[string]string{
				"d": "q",
				"e": "z",
			}

			Specify("one arg returns the arg", func() {
				Expect(UnionStringMaps(m)).To(Equal(m))
			})

			Specify("2nd overrrides first", func() {
				Expect(UnionStringMaps(m, overwriter)).To(Equal(overwriter))
			})

			Specify("returns the union when disjoint", func() {
				union := map[string]string{
					"a": "b",
					"c": "d",
					"d": "y",
					"e": "z",
				}
				Expect(UnionStringMaps(m, disjoiner)).To(Equal(union))
			})

			Specify("final arg takes precedence", func() {
				union := map[string]string{
					"a": "b",
					"c": "d",
					"d": "q",
					"e": "z",
				}
				Expect(UnionStringMaps(m, disjoiner, empty, m4)).To(Equal(union))
			})
		})
	})
})
