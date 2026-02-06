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

package pricing

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pricing")
}

var _ = Describe(
	"getPricingAPIRegion", func() {
		Context(
			"when regionOverride is empty", func() {
				DescribeTable(
					"should map regions to their pricing API endpoints",
					func(cfgRegion, expectedRegion string) {
						result := getPricingAPIRegion(cfgRegion, "")
						Expect(result).To(Equal(expectedRegion))
					},
					// EU regions
					Entry("EU West 1 maps to eu-central-1", "eu-west-1", "eu-central-1"),
					Entry("EU West 2 maps to eu-central-1", "eu-west-2", "eu-central-1"),
					Entry("EU West 3 maps to eu-central-1", "eu-west-3", "eu-central-1"),
					Entry("EU Central 1 maps to eu-central-1", "eu-central-1", "eu-central-1"),
					Entry("EU Central 2 maps to eu-central-1", "eu-central-2", "eu-central-1"),
					Entry("EU North 1 maps to eu-central-1", "eu-north-1", "eu-central-1"),
					Entry("EU South 1 maps to eu-central-1", "eu-south-1", "eu-central-1"),
					Entry("EU South 2 maps to eu-central-1", "eu-south-2", "eu-central-1"),

					// AP regions
					Entry("AP Southeast 1 maps to ap-south-1", "ap-southeast-1", "ap-south-1"),
					Entry("AP Southeast 2 maps to ap-south-1", "ap-southeast-2", "ap-south-1"),
					Entry("AP Southeast 3 maps to ap-south-1", "ap-southeast-3", "ap-south-1"),
					Entry("AP Southeast 4 maps to ap-south-1", "ap-southeast-4", "ap-south-1"),
					Entry("AP Northeast 1 maps to ap-south-1", "ap-northeast-1", "ap-south-1"),
					Entry("AP Northeast 2 maps to ap-south-1", "ap-northeast-2", "ap-south-1"),
					Entry("AP Northeast 3 maps to ap-south-1", "ap-northeast-3", "ap-south-1"),
					Entry("AP South 1 maps to ap-south-1", "ap-south-1", "ap-south-1"),
					Entry("AP South 2 maps to ap-south-1", "ap-south-2", "ap-south-1"),
					Entry("AP East 1 maps to ap-south-1", "ap-east-1", "ap-south-1"),

					// CN regions
					Entry("CN North 1 maps to cn-northwest-1", "cn-north-1", "cn-northwest-1"),
					Entry("CN Northwest 1 maps to cn-northwest-1", "cn-northwest-1", "cn-northwest-1"),

					// US regions (default fallback)
					Entry("US East 1 maps to us-east-1", "us-east-1", "us-east-1"),
					Entry("US East 2 maps to us-east-1", "us-east-2", "us-east-1"),
					Entry("US West 1 maps to us-east-1", "us-west-1", "us-east-1"),
					Entry("US West 2 maps to us-east-1", "us-west-2", "us-east-1"),

					// Other regions that fall back to us-east-1
					Entry("CA Central 1 maps to us-east-1", "ca-central-1", "us-east-1"),
					Entry("SA East 1 maps to us-east-1", "sa-east-1", "us-east-1"),
					Entry("AF South 1 maps to us-east-1", "af-south-1", "us-east-1"),
					Entry("ME South 1 maps to us-east-1", "me-south-1", "us-east-1"),
					Entry("ME Central 1 maps to us-east-1", "me-central-1", "us-east-1"),
				)
			},
		)

		Context(
			"when regionOverride is provided", func() {
				DescribeTable(
					"should return the regionOverride regardless of source region",
					func(cfgRegion, regionOverride string) {
						result := getPricingAPIRegion(cfgRegion, regionOverride)
						Expect(result).To(Equal(regionOverride))
					},
					Entry("EU region with US override", "eu-west-1", "us-east-1"),
					Entry("AP region with EU override", "ap-southeast-1", "eu-central-1"),
					Entry("CN region with US override", "cn-north-1", "us-east-1"),
					Entry("US region with AP override", "us-west-2", "ap-south-1"),
					Entry("EU region with CN override", "eu-central-1", "cn-northwest-1"),
					Entry("CA region with override", "ca-central-1", "ap-south-1"),
					Entry("SA region with override", "sa-east-1", "eu-central-1"),
				)
			},
		)
	},
)
