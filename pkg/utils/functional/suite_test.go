/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

		Specify("2nd arg overrides 1st", func() {
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
	Context("IntersectStringSlice", func() {
		var nilset []string
		empty := []string{}
		universe := []string{"a", "b", "c"}
		subset := []string{"a", "b"}
		overlap := []string{"a", "b", "d"}
		disjoint := []string{"d", "e"}
		duplicates := []string{"a", "a"}
		Specify("nil set", func() {
			Expect(IntersectStringSlice()).To(BeNil())
			Expect(IntersectStringSlice(nilset)).To(BeNil())
			Expect(IntersectStringSlice(nilset, nilset)).To(BeNil())
			Expect(IntersectStringSlice(nilset, universe)).To(ConsistOf(universe))
			Expect(IntersectStringSlice(universe, nilset)).To(ConsistOf(universe))
			Expect(IntersectStringSlice(universe, nilset, nilset)).To(ConsistOf(universe))
		})
		Specify("empty set", func() {
			Expect(IntersectStringSlice(empty, nilset)).To(And(BeEmpty(), Not(BeNil())))
			Expect(IntersectStringSlice(nilset, empty)).To(And(BeEmpty(), Not(BeNil())))
			Expect(IntersectStringSlice(universe, empty)).To(And(BeEmpty(), Not(BeNil())))
			Expect(IntersectStringSlice(universe, universe, empty)).To(And(BeEmpty(), Not(BeNil())))
		})
		Specify("intersect", func() {
			Expect(IntersectStringSlice(universe, subset)).To(ConsistOf(subset))
			Expect(IntersectStringSlice(subset, universe)).To(ConsistOf(subset))
			Expect(IntersectStringSlice(universe, overlap)).To(ConsistOf(subset))
			Expect(IntersectStringSlice(overlap, universe)).To(ConsistOf(subset))
			Expect(IntersectStringSlice(universe, disjoint)).To(And(BeEmpty(), Not(BeNil())))
			Expect(IntersectStringSlice(disjoint, universe)).To(And(BeEmpty(), Not(BeNil())))
			Expect(IntersectStringSlice(overlap, disjoint, universe)).To(And(BeEmpty(), Not(BeNil())))
		})
		Specify("duplicates", func() {
			Expect(IntersectStringSlice(duplicates)).To(ConsistOf("a"))
			Expect(IntersectStringSlice(duplicates, nilset)).To(ConsistOf("a"))
			Expect(IntersectStringSlice(duplicates, universe)).To(ConsistOf("a"))
			Expect(IntersectStringSlice(duplicates, universe, subset)).To(ConsistOf("a"))
		})
	})
})
