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

package version_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
)

var _ = Describe("SupportsDefaultBind", func() {
	DescribeTable("should return correct result for version",
		func(versionStr string, expected bool) {
			result := version.SupportsDefaultBind(versionStr)
			Expect(result).To(Equal(expected))
		},
		Entry("1.45.0", "1.45.0", false),
		Entry("1.46.0", "1.46.0", true),
		Entry("1.47.0", "1.47.0", true),
		Entry("2.0.0", "2.0.0", true),
		Entry("v1.45.0", "v1.45.0", false),
		Entry("v1.46.0", "v1.46.0", true),
		Entry("1.9.0", "1.9.0", false),
		Entry("latest", "latest", true),
		Entry("invalid", "invalid", false),
		Entry("1", "1", false),
		Entry("empty", "", false),
		Entry("1.46", "1.46", false),
		Entry("abc.def.ghi", "abc.def.ghi", false),
	)
})
