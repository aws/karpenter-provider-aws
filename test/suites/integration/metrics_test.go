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

package integration_test

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"
)

var _ = Describe("Metrics", func() {
	var instanceTypeCount int
	var availabilityZoneCount int
	var capacityTypeCount int
	BeforeEach(func() {
		azOut, err := env.EC2API.DescribeAvailabilityZones(env.Context, &ec2.DescribeAvailabilityZonesInput{})
		Expect(err).ToNot(HaveOccurred())
		availabilityZoneCount = len(azOut.AvailabilityZones)

		itOut, err := env.EC2API.DescribeInstanceTypes(env.Context, &ec2.DescribeInstanceTypesInput{})
		Expect(err).ToNot(HaveOccurred())
		instanceTypeCount = len(itOut.InstanceTypes)
		capacityTypeCount = 3 // on-demand, spot, and reserved
	})
	It("should expose karpenter_cloudprovider_instance_type_offering_price_estimate metrics", func() {
		env.ExpectCreated(nodeClass, nodePool)
		Eventually(func(g Gomega) {
			defer GinkgoRecover()
			podMetrics := env.ExpectPodMetrics()
			priceMetricCount := lo.CountBy(podMetrics, func(p common.PrometheusMetric) bool {
				return p.Name == "karpenter_cloudprovider_instance_type_offering_price_estimate"
			})
			nonZeroPriceMetricCount := lo.CountBy(podMetrics, func(p common.PrometheusMetric) bool {
				return p.Name == "karpenter_cloudprovider_instance_type_offering_price_estimate" &&
					p.Value > 0
			})
			// We provide a 100 instance type buffer just in case instance types don't have offerings in every zone
			// We provide a 200 instance type buffer for spot since there should be even less availability
			expectedCount := (instanceTypeCount - 100) * availabilityZoneCount * capacityTypeCount
			expectedNonZeroCount := (instanceTypeCount-100)*availabilityZoneCount + (instanceTypeCount-200)*availabilityZoneCount
			g.Expect(priceMetricCount).To(BeNumerically(">", expectedCount))
			g.Expect(nonZeroPriceMetricCount).To(BeNumerically(">", expectedNonZeroCount))
		}, time.Minute, time.Second*5).Should(Succeed())

	})
	It("should expose karpenter_cloudprovider_instance_type_offering_available metrics", func() {
		env.ExpectCreated(nodeClass, nodePool)
		// Availability only has non-zero values for the subnets that we support
		selectedAZCount := len(env.GetSubnets(map[string]string{"karpenter.sh/discovery": env.ClusterName}))

		Eventually(func(g Gomega) {
			defer GinkgoRecover()

			podMetrics := env.ExpectPodMetrics()
			availableMetricCount := lo.CountBy(podMetrics, func(p common.PrometheusMetric) bool {
				return p.Name == "karpenter_cloudprovider_instance_type_offering_available"
			})
			nonZeroAvailableMetricCount := lo.CountBy(podMetrics, func(p common.PrometheusMetric) bool {
				return p.Name == "karpenter_cloudprovider_instance_type_offering_available" &&
					p.Value > 0
			})
			// We provide a 100 instance type buffer just in case instance types don't have offerings in every zone
			// We provide a 200 instance type buffer for spot since there should be even less availability
			expectedCount := (instanceTypeCount - 100) * selectedAZCount * capacityTypeCount
			expectedNonZeroCount := (instanceTypeCount-100)*selectedAZCount + (instanceTypeCount-200)*selectedAZCount
			g.Expect(availableMetricCount).To(BeNumerically(">", expectedCount))
			g.Expect(nonZeroAvailableMetricCount).To(BeNumerically(">", expectedNonZeroCount))
		}, time.Minute, time.Second*5).Should(Succeed())
	})
})
