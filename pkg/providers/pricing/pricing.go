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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingtypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

var initialOnDemandPrices = lo.Assign(InitialOnDemandPricesAWS, InitialOnDemandPricesUSGov, InitialOnDemandPricesCN)

type Provider interface {
	LivenessProbe(*http.Request) error
	InstanceTypes() []ec2types.InstanceType
	OnDemandPrice(ec2types.InstanceType) (float64, bool)
	SpotPrice(ec2types.InstanceType, string) (float64, bool)
	UpdateOnDemandPricing(context.Context) error
	UpdateSpotPricing(context.Context) error
}

// DefaultProvider provides actual pricing data to the AWS cloud provider to allow it to make more informed decisions
// regarding which instances to launch.  This is initialized at startup with a periodically updated static price list to
// support running in locations where pricing data is unavailable.  In those cases the static pricing data provides a
// relative ordering that is still more accurate than our previous pricing model.  In the event that a pricing update
// fails, the previous pricing information is retained and used which may be the static initial pricing data if pricing
// updates never succeed.
type DefaultProvider struct {
	ec2     sdk.EC2API
	pricing sdk.PricingAPI
	region  string
	cm      *pretty.ChangeMonitor

	muOnDemand     sync.RWMutex
	onDemandPrices map[ec2types.InstanceType]float64

	muSpot             sync.RWMutex
	spotPrices         map[ec2types.InstanceType]zonal
	spotPricingUpdated bool
}

// zonalPricing is used to capture the per-zone price
// for spot data as well as the default price
// based on on-demand price when the provisioningController first
// comes up
type zonal struct {
	defaultPrice float64 // Used until we get the spot pricing data
	prices       map[string]float64
}

func combineZonalPricing(pricingData ...zonal) zonal {
	z := newZonalPricing(0)
	for _, elem := range pricingData {
		if elem.defaultPrice != 0 {
			z.defaultPrice = elem.defaultPrice
		}
		for zone, price := range elem.prices {
			z.prices[zone] = price
		}
	}
	return z
}

func newZonalPricing(defaultPrice float64) zonal {
	z := zonal{
		prices: map[string]float64{},
	}
	z.defaultPrice = defaultPrice
	return z
}

// NewPricingAPI returns a pricing API configured based on a particular region
func NewAPI(cfg aws.Config) *pricing.Client {
	// pricing API doesn't have an endpoint in all regions
	pricingAPIRegion := "us-east-1"
	if strings.HasPrefix(cfg.Region, "ap-") {
		pricingAPIRegion = "ap-south-1"
	} else if strings.HasPrefix(cfg.Region, "cn-") {
		pricingAPIRegion = "cn-northwest-1"
	} else if strings.HasPrefix(cfg.Region, "eu-") {
		pricingAPIRegion = "eu-central-1"
	}
	//create pricing config using pricing endpoint
	pricingCfg := cfg.Copy()
	pricingCfg.Region = pricingAPIRegion
	return pricing.NewFromConfig(pricingCfg)
}

func NewDefaultProvider(_ context.Context, pricing sdk.PricingAPI, ec2Api sdk.EC2API, region string) *DefaultProvider {
	p := &DefaultProvider{
		region:  region,
		ec2:     ec2Api,
		pricing: pricing,
		cm:      pretty.NewChangeMonitor(),
	}
	// sets the pricing data from the static default state for the provider
	p.Reset()

	return p
}

// InstanceTypes returns the list of all instance types for which either a spot or on-demand price is known.
func (p *DefaultProvider) InstanceTypes() []ec2types.InstanceType {
	p.muOnDemand.RLock()
	p.muSpot.RLock()
	defer p.muOnDemand.RUnlock()
	defer p.muSpot.RUnlock()
	return lo.Union(lo.Keys(p.onDemandPrices), lo.Keys(p.spotPrices))
}

// OnDemandPrice returns the last known on-demand price for a given instance type, returning an error if there is no
// known on-demand pricing for the instance type.
func (p *DefaultProvider) OnDemandPrice(instanceType ec2types.InstanceType) (float64, bool) {
	p.muOnDemand.RLock()
	defer p.muOnDemand.RUnlock()
	price, ok := p.onDemandPrices[instanceType]
	if !ok {
		return 0.0, false
	}
	return price, true
}

// SpotPrice returns the last known spot price for a given instance type and zone, returning an error
// if there is no known spot pricing for that instance type or zone
func (p *DefaultProvider) SpotPrice(instanceType ec2types.InstanceType, zone string) (float64, bool) {
	p.muSpot.RLock()
	defer p.muSpot.RUnlock()
	if val, ok := p.spotPrices[instanceType]; ok {
		if !p.spotPricingUpdated {
			return val.defaultPrice, true
		}
		if price, ok := p.spotPrices[instanceType].prices[zone]; ok {
			return price, true
		}
		return 0.0, false
	}
	return 0.0, false
}

func (p *DefaultProvider) UpdateOnDemandPricing(ctx context.Context) error {
	// standard on-demand instances
	var wg sync.WaitGroup
	var onDemandPrices, onDemandMetalPrices map[ec2types.InstanceType]float64
	var onDemandErr, onDemandMetalErr error

	// if we are in isolated vpc, skip updating on demand pricing
	// as pricing api may not be available
	if options.FromContext(ctx).IsolatedVPC {
		if p.cm.HasChanged("on-demand-prices", nil) {
			log.FromContext(ctx).V(1).Info("running in an isolated VPC, on-demand pricing information will not be updated")
		}
		return nil
	}

	p.muOnDemand.Lock()
	defer p.muOnDemand.Unlock()

	wg.Add(1)
	go func() {
		defer wg.Done()
		onDemandPrices, onDemandErr = p.fetchOnDemandPricing(ctx,
			pricingtypes.Filter{
				Field: aws.String("tenancy"),
				Type:  "TERM_MATCH",
				Value: aws.String("Shared"),
			},
			pricingtypes.Filter{
				Field: aws.String("productFamily"),
				Type:  "TERM_MATCH",
				Value: aws.String("Compute Instance"),
			})
	}()

	// bare metal on-demand prices
	wg.Add(1)
	go func() {
		defer wg.Done()
		onDemandMetalPrices, onDemandMetalErr = p.fetchOnDemandPricing(ctx,
			pricingtypes.Filter{
				Field: aws.String("tenancy"),
				Type:  "TERM_MATCH",
				Value: aws.String("Dedicated"),
			},
			pricingtypes.Filter{
				Field: aws.String("productFamily"),
				Type:  "TERM_MATCH",
				Value: aws.String("Compute Instance (bare metal)"),
			})
	}()

	wg.Wait()

	err := multierr.Append(onDemandErr, onDemandMetalErr)
	if err != nil {
		return fmt.Errorf("retreiving on-demand pricing data, %w", err)
	}

	if len(onDemandPrices) == 0 || len(onDemandMetalPrices) == 0 {
		return fmt.Errorf("no on-demand pricing found")
	}

	// Maintain previously retrieved pricing data
	p.onDemandPrices = lo.Assign(p.onDemandPrices, onDemandPrices, onDemandMetalPrices)
	if p.cm.HasChanged("on-demand-prices", p.onDemandPrices) {
		log.FromContext(ctx).WithValues("instance-type-count", len(p.onDemandPrices)).V(1).Info("updated on-demand pricing")
	}
	return nil
}

func (p *DefaultProvider) fetchOnDemandPricing(ctx context.Context, additionalFilters ...pricingtypes.Filter) (map[ec2types.InstanceType]float64, error) {
	prices := map[ec2types.InstanceType]float64{}
	filters := append([]pricingtypes.Filter{
		{
			Field: aws.String("regionCode"),
			Type:  "TERM_MATCH",
			Value: aws.String(p.region),
		},
		{
			Field: aws.String("serviceCode"),
			Type:  "TERM_MATCH",
			Value: aws.String("AmazonEC2"),
		},
		{
			Field: aws.String("preInstalledSw"),
			Type:  "TERM_MATCH",
			Value: aws.String("NA"),
		},
		{
			Field: aws.String("operatingSystem"),
			Type:  "TERM_MATCH",
			Value: aws.String("Linux"),
		},
		{
			Field: aws.String("capacitystatus"),
			Type:  "TERM_MATCH",
			Value: aws.String("Used"),
		},
		{
			Field: aws.String("marketoption"),
			Type:  "TERM_MATCH",
			Value: aws.String("OnDemand"),
		}},
		additionalFilters...)

	input := &pricing.GetProductsInput{
		Filters:     filters,
		ServiceCode: aws.String("AmazonEC2"),
	}

	paginator := pricing.NewGetProductsPaginator(p.pricing, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)

		if err != nil {
			return nil, fmt.Errorf("getting pricing data, %w", err)
		}
		prices = lo.Assign(prices, p.onDemandPage(ctx, output))
	}

	return prices, nil
}

func (p *DefaultProvider) spotPage(ctx context.Context, output *ec2.DescribeSpotPriceHistoryOutput) map[ec2types.InstanceType]zonal {
	result := map[ec2types.InstanceType]zonal{}
	for _, sph := range output.SpotPriceHistory {
		spotPriceStr := aws.ToString(sph.SpotPrice)
		spotPrice, err := strconv.ParseFloat(spotPriceStr, 64)
		// these errors shouldn't occur, but if pricing API does have an error, we ignore the record
		if err != nil {
			log.FromContext(ctx).V(1).Info(fmt.Sprintf("unable to parse price record %#v", sph))
			continue
		}
		if sph.Timestamp == nil {
			continue
		}
		instanceType := sph.InstanceType
		az := aws.ToString(sph.AvailabilityZone)
		_, ok := result[instanceType]
		if !ok {
			result[instanceType] = zonal{
				prices: map[string]float64{},
			}
		}
		result[instanceType].prices[az] = spotPrice

	}
	return result
}

// turning off cyclo here, it measures as a 12 due to all of the type checks of the pricing data which returns a deeply
// nested map[string]interface{}
// nolint: gocyclo
func (p *DefaultProvider) onDemandPage(ctx context.Context, output *pricing.GetProductsOutput) map[ec2types.InstanceType]float64 {
	// this isn't the full pricing struct, just the portions we care about
	type priceItem struct {
		Product struct {
			Attributes struct {
				InstanceType string
			}
		}
		Terms struct {
			OnDemand map[string]struct {
				PriceDimensions map[string]struct {
					PricePerUnit map[string]string
				}
			}
		}
	}

	result := map[ec2types.InstanceType]float64{}
	currency := "USD"
	if strings.HasPrefix(p.region, "cn-") {
		currency = "CNY"
	}
	for _, outer := range output.PriceList {
		pItem := &priceItem{}
		if err := json.Unmarshal([]byte(outer), pItem); err != nil {
			log.FromContext(ctx).Error(err, "failed unmarshaling pricing data")
		}
		if pItem.Product.Attributes.InstanceType == "" {
			continue
		}
		for _, term := range pItem.Terms.OnDemand {
			for _, v := range term.PriceDimensions {
				price, err := strconv.ParseFloat(v.PricePerUnit[currency], 64)
				if err != nil || price == 0 {
					continue
				}
				result[ec2types.InstanceType(pItem.Product.Attributes.InstanceType)] = price
			}
		}
	}

	return result
}

// nolint: gocyclo
func (p *DefaultProvider) UpdateSpotPricing(ctx context.Context) error {
	prices := map[ec2types.InstanceType]zonal{}

	p.muSpot.Lock()
	defer p.muSpot.Unlock()

	input := &ec2.DescribeSpotPriceHistoryInput{
		ProductDescriptions: []string{
			"Linux/UNIX",
			"Linux/UNIX (Amazon VPC)",
		},
		// get the latest spot price for each instance type
		StartTime: aws.Time(time.Now()),
	}

	paginator := ec2.NewDescribeSpotPriceHistoryPaginator(p.ec2, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("retrieving spot pricing data, %w", err)
		}
		for it, z := range p.spotPage(ctx, output) {
			prices[it] = combineZonalPricing(prices[it], z)
		}
	}
	if len(prices) == 0 {
		return fmt.Errorf("no spot pricing found")
	}
	totalOfferings := 0
	for it, zoneData := range prices {
		// Maintain previously retrieved pricing data
		p.spotPrices[it] = combineZonalPricing(p.spotPrices[it], zoneData)
		totalOfferings += len(zoneData.prices)
	}

	p.spotPricingUpdated = true
	if p.cm.HasChanged("spot-prices", p.spotPrices) {
		log.FromContext(ctx).WithValues(
			"instance-type-count", len(p.spotPrices),
			"offering-count", totalOfferings).V(1).Info("updated spot pricing with instance types and offerings")
	}
	return nil
}

func (p *DefaultProvider) LivenessProbe(_ *http.Request) error {
	// ensure we don't deadlock and nolint for the empty critical section
	p.muOnDemand.Lock()
	p.muSpot.Lock()
	//nolint: staticcheck
	p.muOnDemand.Unlock()
	p.muSpot.Unlock()
	return nil
}

func populateInitialSpotPricing(pricing map[ec2types.InstanceType]float64) map[ec2types.InstanceType]zonal {
	m := map[ec2types.InstanceType]zonal{}
	for it, price := range pricing {
		m[it] = newZonalPricing(price)
	}
	return m
}

func (p *DefaultProvider) Reset() {
	// see if we've got region specific pricing data
	staticPricing, ok := initialOnDemandPrices[p.region]
	if !ok {
		// and if not, fall back to the always available us-east-1
		staticPricing = initialOnDemandPrices["us-east-1"]
	}

	p.onDemandPrices = staticPricing
	// default our spot pricing to the same as the on-demand pricing until a price update
	p.spotPrices = populateInitialSpotPricing(staticPricing)
	p.spotPricingUpdated = false
}
