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

package utils_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

var _ = Describe("FilterSetHash", func() {
	f := func(name string, values ...string) ec2types.Filter {
		return ec2types.Filter{Name: aws.String(name), Values: values}
	}
	set := func(filters ...ec2types.Filter) []ec2types.Filter { return filters }

	DescribeTable("hashes filter sets that select the same resources alike",
		func(a, b []ec2types.Filter) {
			Expect(utils.FilterSetHash(a)).To(Equal(utils.FilterSetHash(b)))
		},
		Entry("filter and value order", set(f("tag:a", "1", "2"), f("tag:b", "3")), set(f("tag:b", "3"), f("tag:a", "2", "1"))),
		Entry("duplicate values", set(f("subnet-id", "s-1", "s-2", "s-2")), set(f("subnet-id", "s-1", "s-2"))),
		Entry("duplicate filters", set(f("tag:a", "1"), f("tag:a", "1")), set(f("tag:a", "1"))),
		// Duplicates must be recognised regardless of value order, otherwise the outer
		// SlicesAsSets XOR cancels them and the set aliases the empty filter set
		Entry("duplicate filters with reordered values", set(f("tag:a", "1", "2"), f("tag:a", "2", "1")), set(f("tag:a", "1", "2"))),
		Entry("nil and empty values", set(ec2types.Filter{Name: aws.String("tag:a")}), set(f("tag:a"))),
		Entry("nil and empty filter sets", []ec2types.Filter(nil), []ec2types.Filter{}),
	)

	// SlicesAsSets XORs set elements, so without the dedup in FilterSetHash duplicated
	// values or filters would cancel and alias distinct filter sets (#8619)
	DescribeTable("hashes filter sets that select different resources differently",
		func(sets ...[]ec2types.Filter) {
			Expect(lo.Uniq(lo.Map(sets, func(s []ec2types.Filter, _ int) uint64 { return utils.FilterSetHash(s) }))).To(HaveLen(len(sets)))
		},
		Entry("differ only by a duplicated value",
			set(f("subnet-id", "s-1", "s-1")), set(f("subnet-id", "s-2", "s-2")), set(f("subnet-id")), set(f("subnet-id", "s-1", "s-2", "s-2"))),
		Entry("AND (one filter set) vs OR (separate filter sets)",
			set(f("tag:a", "1"), f("tag:b", "2")), set(f("tag:a", "1")), set(f("tag:b", "2"))),
		Entry("filter values", set(f("tag:a", "1")), set(f("tag:a", "2"))),
		Entry("values swapped between filter names", set(f("tag:a", "1"), f("tag:b", "2")), set(f("tag:a", "2"), f("tag:b", "1"))),
		Entry("tag-key and tag filters swapping values", set(f("tag-key", "a"), f("tag:b", "x")), set(f("tag-key", "x"), f("tag:b", "a"))),
		// Same-name filters with different values are ANDed, so neither may be dropped as a duplicate
		Entry("one filter with two values (OR) vs two same-name filters (AND) vs either alone",
			set(f("tag:a", "1", "2")), set(f("tag:a", "1"), f("tag:a", "2")), set(f("tag:a", "1")), set(f("tag:a", "2"))),
		Entry("filters differing only by name", set(f("tag:a", "1"), f("tag:a", "2")), set(f("tag:b", "1"), f("tag:b", "2"))),
		Entry("subnet-id vs tag with the same value", set(f("subnet-id", "x")), set(f("tag:Name", "x"))),
		Entry("empty filter set vs a filter with no values vs duplicate filters",
			[]ec2types.Filter(nil), set(f("tag:a")), set(f("tag:a", "1", "2"), f("tag:a", "2", "1"))),
	)
})
