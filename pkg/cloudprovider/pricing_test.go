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

package cloudprovider

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/pricing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/fake"
)

var _ = Describe("Pricing", func() {
	BeforeEach(func() {
		fakeEC2API.Reset()
		fakePricingAPI.Reset()
	})
	It("should return static on-demand data if pricing API fails", func() {
		fakePricingAPI.NextError.Set(fmt.Errorf("failed"))
		p := NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
		price, ok := p.OnDemandPrice("c5.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically(">", 0))
	})
	It("should return static spot data if EC2 describeSpotPriceHistory API fails", func() {
		fakePricingAPI.NextError.Set(fmt.Errorf("failed"))
		p := NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
		price, ok := p.SpotPrice("c5.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically(">", 0))
	})
	It("should update on-demand pricing with response from the pricing API", func() {
		// modify our API before creating the pricing provider as it performs an initial update on creation. The pricing
		// API provides on-demand prices, the ec2 API provides spot prices
		fakePricingAPI.GetProductsOutput.Set(&pricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		updateStart := time.Now()
		p := NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
		Eventually(func() bool { return p.OnDemandLastUpdated().After(updateStart) }).Should(BeTrue())

		price, ok := p.OnDemandPrice("c98.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.20))

		price, ok = p.OnDemandPrice("c99.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.23))
	})
	It("should update spot pricing with response from the pricing API", func() {
		now := time.Now()
		fakeEC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
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
		fakePricingAPI.GetProductsOutput.Set(&pricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		updateStart := time.Now()
		p := NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
		Eventually(func() bool { return p.SpotLastUpdated().After(updateStart) }).Should(BeTrue())

		price, ok := p.SpotPrice("c98.large", "test-zone-1b")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.10))

		price, ok = p.SpotPrice("c99.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.23))
	})
	It("should update zonal pricing with data from the spot pricing API", func() {
		now := time.Now()
		fakeEC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
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
		fakePricingAPI.GetProductsOutput.Set(&pricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		updateStart := time.Now()
		p := NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
		Eventually(func() bool { return p.SpotLastUpdated().After(updateStart) }).Should(BeTrue())

		price, ok := p.SpotPrice("c98.large", "test-zone-1a")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically("==", 1.20))

		_, ok = p.SpotPrice("c98.large", "test-zone-1b")
		Expect(ok).ToNot(BeTrue())
	})
	It("should respond with false if price doesn't exist in zone", func() {
		now := time.Now()
		fakeEC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []*ec2.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("c99.large"),
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &now,
				},
			},
		})
		fakePricingAPI.GetProductsOutput.Set(&pricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		updateStart := time.Now()
		p := NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
		Eventually(func() bool { return p.SpotLastUpdated().After(updateStart) }).Should(BeTrue())

		_, ok := p.SpotPrice("c99.large", "test-zone-1b")
		Expect(ok).To(BeFalse())
	})
	It("should query for both `Linux/UNIX` and `Linux/UNIX (Amazon VPC)`", func() {
		// If an account supports EC2 classic, then the non-classic instance types have a product
		// description of Linux/UNIX (Amazon VPC)
		// If it doesn't, they have a product description of Linux/UNIX. To work in both cases, we
		// need to search for both values.
		updateStart := time.Now()
		fakeEC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
			SpotPriceHistory: []*ec2.SpotPrice{
				{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("c99.large"),
					SpotPrice:        aws.String("1.23"),
					Timestamp:        &updateStart,
				},
			},
		})
		fakePricingAPI.GetProductsOutput.Set(&pricing.GetProductsOutput{
			PriceList: []aws.JSONValue{
				fake.NewOnDemandPrice("c98.large", 1.20),
				fake.NewOnDemandPrice("c99.large", 1.23),
			},
		})
		p := NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
		Eventually(func() bool { return p.OnDemandLastUpdated().After(updateStart) }, 5*time.Second).Should(BeTrue())
		inp := fakeEC2API.DescribeSpotPriceHistoryInput.Clone()
		Expect(lo.Map(inp.ProductDescriptions, func(x *string, _ int) string { return *x })).
			To(ContainElements("Linux/UNIX", "Linux/UNIX (Amazon VPC)"))
	})
})
