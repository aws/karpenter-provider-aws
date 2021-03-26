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
		When("empty args", func() {
			It("returns empty map", func() {
				Expect(UnionStringMaps()).To(BeEmpty())
			})
		})
		m := map[string]string{
			"a": "b",
			"c": "d",
		}
		When("one arg", func() {
			It("returns the arg", func() {
				Expect(UnionStringMaps(m)).To(Equal(m))
			})
		})
		When("2nd overwrites first", func() {
			overwriter := map[string]string{
				"a": "y",
				"c": "z",
			}
			It("returns the 2nd arg", func() {
				Expect(UnionStringMaps(m, overwriter)).To(Equal(overwriter))
			})
		})
		When("2nd is disjoint", func() {
			disjoiner := map[string]string{
				"d": "y",
				"e": "z",
			}
			It("returns the union", func() {
				union := map[string]string{
					"a": "b",
					"c": "d",
					"d": "y",
					"e": "z",
				}
				Expect(UnionStringMaps(m, disjoiner)).To(Equal(union))
			})
		})

		When("3rd and 2nd collide", func() {
			m2 := map[string]string{
				"d": "y",
				"e": "z",
			}
			m3 := map[string]string{
				"d": "q",
				"e": "z",
			}

			Specify("3rd takes precedence", func() {
				union := map[string]string{
					"a": "b",
					"c": "d",
					"d": "q",
					"e": "z",
				}
				Expect(UnionStringMaps(m, m2, m3)).To(Equal(union))
			})
		})
	})
})
