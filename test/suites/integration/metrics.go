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

package integration

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"
)

const metricsTimeout = 3 * time.Minute

func registerMetrics() bool {
	return Describe("Metrics", func() {
		var instanceTypeCount int
		var availabilityZones sets.Set[string]
		var capacityTypeCount int
		// offeredZones maps each instance type Karpenter discovers to the zones EC2 offers it in
		var offeredZones map[ec2types.InstanceType]sets.Set[string]
		offeredPairs := func(zones sets.Set[string]) int {
			return lo.SumBy(lo.Values(offeredZones), func(z sets.Set[string]) int { return z.Intersection(zones).Len() })
		}
		BeforeEach(func() {
			azOut, err := Env.EC2API.DescribeAvailabilityZones(Env.Context, &ec2.DescribeAvailabilityZonesInput{})
			Expect(err).ToNot(HaveOccurred())
			availabilityZones = sets.New(lo.Map(azOut.AvailabilityZones, func(az ec2types.AvailabilityZone, _ int) string {
				return lo.FromPtr(az.ZoneName)
			})...)

			// Same filters Karpenter uses to discover instance types
			offeredZones = map[ec2types.InstanceType]sets.Set[string]{}
			itPaginator := ec2.NewDescribeInstanceTypesPaginator(Env.EC2API, &ec2.DescribeInstanceTypesInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("supported-virtualization-type"), Values: []string{"hvm"}},
					{Name: aws.String("processor-info.supported-architecture"), Values: []string{"x86_64", "arm64"}},
				},
				MaxResults: aws.Int32(100),
			})
			for itPaginator.HasMorePages() {
				page, err := itPaginator.NextPage(Env.Context)
				Expect(err).ToNot(HaveOccurred())
				for _, info := range page.InstanceTypes {
					offeredZones[info.InstanceType] = sets.New[string]()
				}
			}
			instanceTypeCount = len(offeredZones)
			Expect(instanceTypeCount).To(BeNumerically(">", 200))

			offeringPaginator := ec2.NewDescribeInstanceTypeOfferingsPaginator(Env.EC2API, &ec2.DescribeInstanceTypeOfferingsInput{
				LocationType: ec2types.LocationTypeAvailabilityZone,
				MaxResults:   aws.Int32(1000),
			})
			for offeringPaginator.HasMorePages() {
				page, err := offeringPaginator.NextPage(Env.Context)
				Expect(err).ToNot(HaveOccurred())
				for _, offering := range page.InstanceTypeOfferings {
					if zones, ok := offeredZones[offering.InstanceType]; ok {
						zones.Insert(lo.FromPtr(offering.Location))
					}
				}
			}
			capacityTypeCount = 3 // on-demand, spot, and reserved
		})
		It("should expose karpenter_cloudprovider_instance_type_offering_price_estimate metrics", func() {
			Env.ExpectCreated(NodeClass, NodePool)
			availabilityZoneCount := availabilityZones.Len()
			Eventually(func(g Gomega) {
				defer GinkgoRecover()
				podMetrics := Env.ExpectPodMetrics()
				priceMetricCount := lo.CountBy(podMetrics, func(p common.PrometheusMetric) bool {
					return p.Name == "karpenter_cloudprovider_instance_type_offering_price_estimate"
				})
				nonZeroPriceMetricCount := lo.CountBy(podMetrics, func(p common.PrometheusMetric) bool {
					return p.Name == "karpenter_cloudprovider_instance_type_offering_price_estimate" &&
						p.Value > 0
				})
				// On-demand prices are regional, spot prices exist only where EC2 offers the type.
				// Buffers: 100 on-demand, 200 spot per zone.
				expectedCount := (instanceTypeCount - 100) * availabilityZoneCount * capacityTypeCount
				expectedNonZeroCount := (instanceTypeCount-100)*availabilityZoneCount + offeredPairs(availabilityZones) - 200*availabilityZoneCount
				g.Expect(priceMetricCount).To(BeNumerically(">", expectedCount))
				g.Expect(nonZeroPriceMetricCount).To(BeNumerically(">", expectedNonZeroCount))
			}, metricsTimeout, time.Second*5).Should(Succeed())

		})
		It("should expose karpenter_cloudprovider_instance_type_offering_available metrics", func() {
			Env.ExpectCreated(NodeClass, NodePool)
			// Availability only has non-zero values for the subnets that we support
			selectedAZs := sets.New(lo.Keys(Env.DiscoveredSubnets())...)
			selectedAZCount := selectedAZs.Len()

			Eventually(func(g Gomega) {
				defer GinkgoRecover()

				podMetrics := Env.ExpectPodMetrics()
				availableMetricCount := lo.CountBy(podMetrics, func(p common.PrometheusMetric) bool {
					return p.Name == "karpenter_cloudprovider_instance_type_offering_available"
				})
				nonZeroAvailableMetricCount := lo.CountBy(podMetrics, func(p common.PrometheusMetric) bool {
					return p.Name == "karpenter_cloudprovider_instance_type_offering_available" &&
						p.Value > 0
				})
				// Offerings are only available where EC2 offers the type in a selected zone.
				// Buffers: 100 on-demand, 200 spot per zone.
				expectedCount := (instanceTypeCount - 100) * selectedAZCount * capacityTypeCount
				expectedNonZeroCount := offeredPairs(selectedAZs) - 100*selectedAZCount + offeredPairs(selectedAZs) - 200*selectedAZCount
				g.Expect(availableMetricCount).To(BeNumerically(">", expectedCount))
				g.Expect(nonZeroAvailableMetricCount).To(BeNumerically(">", expectedNonZeroCount))
			}, metricsTimeout, time.Second*5).Should(Succeed())
		})
	})
}
