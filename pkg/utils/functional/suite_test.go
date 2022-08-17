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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional Suite")
}

var _ = Describe("Functional", func() {
	Context("SplitCommaSeparatedString", func() {
		// No commas in input should produce identical output (single value)
		Specify("no commas in string", func() {
			input := "foo"
			expected := []string{input}
			Expect(SplitCommaSeparatedString(input)).To(Equal(expected))
		})
		// Multiple elements in input, no extraneous whitespace
		Specify("multiple elements without whitespace", func() {
			expected := []string{"a", "b"}
			Expect(SplitCommaSeparatedString("a,b")).To(Equal(expected))
		})
		// Multiple elements in input, lots of extraneous whitespace
		Specify("multiple elements with whitespace", func() {
			expected := []string{"a", "b"}
			Expect(SplitCommaSeparatedString(" a\t ,\n\t b  \n\t  ")).To(Equal(expected))
		})
	})
})
