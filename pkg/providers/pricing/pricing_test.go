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

package pricing_test

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

var _ = Describe("API", func() {
	DescribeTable("should select the endpoint region from the cluster region",
		func(clusterRegion, endpointRegion string) {
			Expect(pricing.NewAPI(aws.Config{Region: clusterRegion}).Options().Region).To(Equal(endpointRegion))
		},
		Entry("default", "us-west-2", "us-east-1"),
		Entry("Asia Pacific", "ap-southeast-2", "ap-south-1"),
		Entry("China", "cn-north-1", "cn-northwest-1"),
		Entry("Europe", "eu-west-1", "eu-central-1"),
	)

	It("should use the endpoint region override without mutating the AWS config", func() {
		cfg := aws.Config{Region: "ap-southeast-2"}

		Expect(pricing.NewAPIWithEndpointRegion(cfg, "us-east-1").Options().Region).To(Equal("us-east-1"))
		Expect(cfg.Region).To(Equal("ap-southeast-2"))
	})
})
