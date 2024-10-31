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

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awspricing "github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/samber/lo"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	controllerspricing "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var controller *controllerspricing.Controller

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pricing")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	controller = controllerspricing.NewController(awsEnv.PricingProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())

	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Pricing", func() {
	DescribeTable(
		"should return correct static data for all partitions",
		func(staticPricing map[string]map[ec2types.InstanceType]float64) {
			for region, prices := range staticPricing {
				provider := pricing.NewDefaultProvider(ctx, awsEnv.PricingAPI, awsEnv.EC2API, region)
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
		_ = ExpectSingletonReconcileFailed(ctx, controller)
		price, ok := awsEnv.PricingProvider.OnDemandPrice("c5.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically(">", 0))
	})
	It("should return static spot data if EC2 describeSpotPriceHistory API fails", func() {
		awsEnv.PricingAPI.NextError.Set(fmt.Errorf("failed"))
		_ = ExpectSingletonReconcileFailed(ctx, controller)
		price, ok := awsEnv.PricingProvider.SpotPrice("c5.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically(">", 0))
	})
	It("should update on-demand pricing with response from the pricing API", func() {
		// modify our API before creating the pricing provider as it performs an initial update on creation. The pricing
		// API provides on-demand prices, the ec2 API provides spot prices
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []string{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		_ = ExpectSingletonReconcileFailed(ctx, controller)

		price, ok := awsEnv.PricingProvider.OnDemandPrice("c98.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.20))

		price, ok = awsEnv.PricingProvider.OnDemandPrice("c99.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.23))
	})
	It("should update spot pricing with response from the pricing API", func() {
		now := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []ec2types.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     "c99.large",
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     "c98.large",
					SpotPrice:        aws.String("1.20"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     "c99.large",
					SpotPrice:        aws.String("1.50"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     "c98.large",
					SpotPrice:        aws.String("1.10"),
					Timestamp:        &now,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{

			PriceList: []string{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectSingletonReconciled(ctx, controller)

		price, ok := awsEnv.PricingProvider.SpotPrice("c98.large", "test-zone-1b")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.10))

		price, ok = awsEnv.PricingProvider.SpotPrice("c99.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.23))
	})
	It("should update zonal pricing with data from the spot pricing API", func() {
		now := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []ec2types.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     "c99.large",
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     "c98.large",
					SpotPrice:        aws.String("1.20"),
					Timestamp:        &now,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []string{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectSingletonReconciled(ctx, controller)

		price, ok := awsEnv.PricingProvider.SpotPrice("c98.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.20))

		_, ok = awsEnv.PricingProvider.SpotPrice("c98.large", "test-zone-1b")
		Expect(ok).ToNot(BeTrue())
	})
	It("should respond with false if price doesn't exist in zone", func() {
		now := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []ec2types.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     "c99.large",
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &now,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []string{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectSingletonReconciled(ctx, controller)

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
			SpotPriceHistory: []ec2types.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     "c99.large",
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &updateStart,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []string{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectSingletonReconciled(ctx, controller)
		inp := awsEnv.EC2API.DescribeSpotPriceHistoryInput.Clone()
		Expect(lo.Map(inp.ProductDescriptions, func(x string, _ int) string { return x })).
			To(ContainElements("Linux/UNIX", "Linux/UNIX (Amazon VPC)"))
	})
	It("should return static on-demand data when in isolated-vpc", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			IsolatedVPC: lo.ToPtr(true),
		}))
		now := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []ec2types.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     "c99.large",
					SpotPrice:        aws.String("1.50"),
					Timestamp:        &now,
				},
				{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     "c98.large",
					SpotPrice:        aws.String("1.10"),
					Timestamp:        &now,
				},
			},
		})

		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			// these are incorrect prices which are here to ensure that
			// results from only static pricing are used
			PriceList: []string{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		ExpectSingletonReconciled(ctx, controller)
		price, ok := awsEnv.PricingProvider.OnDemandPrice("c3.2xlarge")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 0.420000))

		price, ok = awsEnv.PricingProvider.SpotPrice("c98.large", "test-zone-1b")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.10))
	})
	It("should update on-demand pricing with response from the pricing API when in the CN partition", func() {
		tmpPricingProvider := pricing.NewDefaultProvider(ctx, awsEnv.PricingAPI, awsEnv.EC2API, "cn-anywhere-1")
		tmpController := controllerspricing.NewController(tmpPricingProvider)

		now := time.Now()
		awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []ec2types.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     "c99.large",
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &now,
				},
			},
		})
		awsEnv.PricingAPI.GetProductsOutput.Set(&awspricing.GetProductsOutput{
			PriceList: []string{
				fake.NewOnDemandPriceWithCurrency("c98.large", 1.20, "CNY"),
				fake.NewOnDemandPriceWithCurrency("c99.large", 1.23, "CNY"),
			},
		})
		ExpectSingletonReconciled(ctx, tmpController)

		price, ok := tmpPricingProvider.OnDemandPrice("c98.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.20))

		price, ok = tmpPricingProvider.OnDemandPrice("c99.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.23))
	})
})
