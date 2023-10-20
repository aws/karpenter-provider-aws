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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	awspricing "github.com/aws/aws-sdk-go/service/pricing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/types"
	. "knative.dev/pkg/logging/testing"

	coreoptions "github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	. "github.com/aws/karpenter-core/pkg/test/expectations"

	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/providers/pricing"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var controller *pricing.Controller

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx = settings.ToContext(ctx, test.Settings())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	controller = pricing.NewController(awsEnv.PricingProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx = settings.ToContext(ctx, test.Settings())

	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Pricing", func() {
	DescribeTable(
		"should return correct static data for all partitions",
		func(staticPricing map[string]map[string]float64) {
			for region, prices := range staticPricing {
				provider := pricing.NewProvider(ctx, awsEnv.PricingAPI, awsEnv.EC2API, region)
				for instance, price := range prices {
					val, ok := provider.OnDemandPrice(instance)
					Expect(ok).To(BeTrue())
					Expect(val).To(Equal(price))
				}
			}
		},
		Entry("aws", pricing.InitialOnDemandPricesAWS),
		Entry("aws-us-gov", pricing.InitialOnDemandPricesUSGov),
		Entry("aws-cn", pricing.InitialOnDemandPricesCN),
	)
	It("should return static on-demand data if pricing API fails", func() {
		awsEnv.PricingAPI.NextError.Set(fmt.Errorf("failed"))
		ExpectReconcileFailed(ctx, controller, types.NamespacedName{})
		price, ok := awsEnv.PricingProvider.OnDemandPrice("c5.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically(">", 0))
	})
	It("should return static spot data if EC2 describeSpotPriceHistory API fails", func() {
		awsEnv.PricingAPI.NextError.Set(fmt.Errorf("failed"))
		ExpectReconcileFailed(ctx, controller, types.NamespacedName{})
		price, ok := awsEnv.PricingProvider.SpotPrice("c5.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically(">", 0))
	})
	It("should update on-demand pricing with response from the pricing API", func() {
		// modify our API before creating the pricing provider as it performs an initial update on creation. The pricing
		// API provides on-demand prices, the ec2 API provides spot prices
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectReconcileFailed(ctx, controller, types.NamespacedName{})

		price, ok := awsEnv.PricingProvider.OnDemandPrice("c98.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.20))
		Expect(getPricingEstimateMetricValue("c98.large", ec2.UsageClassTypeOnDemand, "")).To(BeNumerically("==", 1.20))

		price, ok = awsEnv.PricingProvider.OnDemandPrice("c99.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.23))
		Expect(getPricingEstimateMetricValue("c99.large", ec2.UsageClassTypeOnDemand, "")).To(BeNumerically("==", 1.23))
	})
	It("should update spot pricing with response from the pricing API", func() {
		now := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []*ec2.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("c99.large"),
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("c98.large"),
					SpotPrice:        aws.String("1.20"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     aws.String("c99.large"),
					SpotPrice:        aws.String("1.50"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     aws.String("c98.large"),
					SpotPrice:        aws.String("1.10"),
					Timestamp:        &now,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})

		price, ok := awsEnv.PricingProvider.SpotPrice("c98.large", "test-zone-1b")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.10))
		Expect(getPricingEstimateMetricValue("c98.large", ec2.UsageClassTypeSpot, "test-zone-1b")).To(BeNumerically("==", 1.10))

		price, ok = awsEnv.PricingProvider.SpotPrice("c99.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.23))
		Expect(getPricingEstimateMetricValue("c99.large", ec2.UsageClassTypeSpot, "test-zone-1a")).To(BeNumerically("==", 1.23))
	})
	It("should update zonal pricing with data from the spot pricing API", func() {
		now := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []*ec2.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("c99.large"),
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("c98.large"),
					SpotPrice:        aws.String("1.20"),
					Timestamp:        &now,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})

		price, ok := awsEnv.PricingProvider.SpotPrice("c98.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.20))
		Expect(getPricingEstimateMetricValue("c98.large", ec2.UsageClassTypeSpot, "test-zone-1a")).To(BeNumerically("==", 1.20))

		_, ok = awsEnv.PricingProvider.SpotPrice("c98.large", "test-zone-1b")
		Expect(ok).ToNot(BeTrue())
	})
	It("should respond with false if price doesn't exist in zone", func() {
		now := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []*ec2.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("c99.large"),
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &now,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})

		_, ok := awsEnv.PricingProvider.SpotPrice("c99.large", "test-zone-1b")
		Expect(ok).To(BeFalse())
	})
	It("should query for both `Linux/UNIX` and `Linux/UNIX (Amazon VPC)`", func() {
		// If an account supports EC2 classic, then the non-classic instance types have a product
		// description of Linux/UNIX (Amazon VPC)
		// If it doesn't, they have a product description of Linux/UNIX. To work in both cases, we
		// need to search for both values.
		updateStart := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []*ec2.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("c99.large"),
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &updateStart,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
		inp := awsEnv.EC2API.DescribeSpotPriceHistoryInput.Clone()
		Expect(lo.Map(inp.ProductDescriptions, func(x *string, _ int) string { return *x })).
			To(ContainElements("Linux/UNIX", "Linux/UNIX (Amazon VPC)"))
	})
})

func getPricingEstimateMetricValue(instanceType string, capacityType string, zone string) float64 {
	var value *float64
	metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_price_estimate", map[string]string{
		pricing.InstanceTypeLabel: instanceType,
		pricing.CapacityTypeLabel: capacityType,
		pricing.RegionLabel:       fake.DefaultRegion,
		pricing.TopologyLabel:     zone,
	})
	Expect(ok).To(BeTrue())
	value = metric.GetGauge().Value
	Expect(value).To(Not(BeNil()))
	return *value
}
