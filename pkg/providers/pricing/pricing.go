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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/aws/aws-sdk-go/service/pricing/pricingiface"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"knative.dev/pkg/logging"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

var initialOnDemandPrices = lo.Assign(InitialOnDemandPricesAWS, InitialOnDemandPricesUSGov, InitialOnDemandPricesCN)

// Provider provides actual pricing data to the AWS cloud provider to allow it to make more informed decisions
// regarding which instances to launch.  This is initialized at startup with a periodically updated static price list to
// support running in locations where pricing data is unavailable.  In those cases the static pricing data provides a
// relative ordering that is still more accurate than our previous pricing model.  In the event that a pricing update
// fails, the previous pricing information is retained and used which may be the static initial pricing data if pricing
// updates never succeed.
type Provider struct {
	ec2     ec2iface.EC2API
	pricing pricingiface.PricingAPI
	region  string
	cm      *pretty.ChangeMonitor

	muOnDemand     sync.RWMutex
	onDemandPrices map[string]float64

	muSpot             sync.RWMutex
	spotPrices         map[string]zonal
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

func newZonalPricing(defaultPrice float64) zonal {
	z := zonal{
		prices: map[string]float64{},
	}
	z.defaultPrice = defaultPrice
	return z
}

// NewPricingAPI returns a pricing API configured based on a particular region
func NewAPI(sess *session.Session, region string) pricingiface.PricingAPI {
	if sess == nil {
		return nil
	}
	// pricing API doesn't have an endpoint in all regions
	pricingAPIRegion := "us-east-1"
	if strings.HasPrefix(region, "ap-") {
		pricingAPIRegion = "ap-south-1"
	} else if strings.HasPrefix(region, "cn-") {
		pricingAPIRegion = "cn-northwest-1"
	} else if strings.HasPrefix(region, "eu-") {
		pricingAPIRegion = "eu-central-1"
	}
	return pricing.New(sess, &aws.Config{Region: aws.String(pricingAPIRegion)})
}

func NewProvider(_ context.Context, pricing pricingiface.PricingAPI, ec2Api ec2iface.EC2API, region string) *Provider {
	p := &Provider{
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
func (p *Provider) InstanceTypes() []string {
	p.muOnDemand.RLock()
	p.muSpot.RLock()
	defer p.muOnDemand.RUnlock()
	defer p.muSpot.RUnlock()
	return lo.Union(lo.Keys(p.onDemandPrices), lo.Keys(p.spotPrices))
}

// OnDemandPrice returns the last known on-demand price for a given instance type, returning an error if there is no
// known on-demand pricing for the instance type.
func (p *Provider) OnDemandPrice(instanceType string) (float64, bool) {
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
func (p *Provider) SpotPrice(instanceType string, zone string) (float64, bool) {
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

func (p *Provider) UpdateOnDemandPricing(ctx context.Context) error {
	// standard on-demand instances
	var wg sync.WaitGroup
	var onDemandPrices, onDemandMetalPrices map[string]float64
	var onDemandErr, onDemandMetalErr error

	// if we are in isolated vpc, skip updating on demand pricing
	// as pricing api may not be available
	if options.FromContext(ctx).IsolatedVPC {
		if p.cm.HasChanged("on-demand-prices", nil) {
			logging.FromContext(ctx).Debug("running in an isolated VPC, on-demand pricing information will not be updated")
		}
		return nil
	}

	p.muOnDemand.Lock()
	defer p.muOnDemand.Unlock()

	wg.Add(1)
	go func() {
		defer wg.Done()
		onDemandPrices, onDemandErr = p.fetchOnDemandPricing(ctx,
			&pricing.Filter{
				Field: aws.String("tenancy"),
				Type:  aws.String("TERM_MATCH"),
				Value: aws.String("Shared"),
			},
			&pricing.Filter{
				Field: aws.String("productFamily"),
				Type:  aws.String("TERM_MATCH"),
				Value: aws.String("Compute Instance"),
			})
	}()

	// bare metal on-demand prices
	wg.Add(1)
	go func() {
		defer wg.Done()
		onDemandMetalPrices, onDemandMetalErr = p.fetchOnDemandPricing(ctx,
			&pricing.Filter{
				Field: aws.String("tenancy"),
				Type:  aws.String("TERM_MATCH"),
				Value: aws.String("Dedicated"),
			},
			&pricing.Filter{
				Field: aws.String("productFamily"),
				Type:  aws.String("TERM_MATCH"),
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

	p.onDemandPrices = lo.Assign(onDemandPrices, onDemandMetalPrices)
	if p.cm.HasChanged("on-demand-prices", p.onDemandPrices) {
		logging.FromContext(ctx).With("instance-type-count", len(p.onDemandPrices)).Debugf("updated on-demand pricing")
	}
	return nil
}

func (p *Provider) fetchOnDemandPricing(ctx context.Context, additionalFilters ...*pricing.Filter) (map[string]float64, error) {
	prices := map[string]float64{}
	filters := append([]*pricing.Filter{
		{
			Field: aws.String("regionCode"),
			Type:  aws.String("TERM_MATCH"),
			Value: aws.String(p.region),
		},
		{
			Field: aws.String("serviceCode"),
			Type:  aws.String("TERM_MATCH"),
			Value: aws.String("AmazonEC2"),
		},
		{
			Field: aws.String("preInstalledSw"),
			Type:  aws.String("TERM_MATCH"),
			Value: aws.String("NA"),
		},
		{
			Field: aws.String("operatingSystem"),
			Type:  aws.String("TERM_MATCH"),
			Value: aws.String("Linux"),
		},
		{
			Field: aws.String("capacitystatus"),
			Type:  aws.String("TERM_MATCH"),
			Value: aws.String("Used"),
		},
		{
			Field: aws.String("marketoption"),
			Type:  aws.String("TERM_MATCH"),
			Value: aws.String("OnDemand"),
		}},
		additionalFilters...)

	err := p.pricing.GetProductsPagesWithContext(
		ctx,
		&pricing.GetProductsInput{
			Filters:     filters,
			ServiceCode: aws.String("AmazonEC2"),
		},
		p.onDemandPage(ctx, prices),
	)
	if err != nil {
		return nil, err
	}

	return prices, nil
}

func (p *Provider) spotPage(ctx context.Context, prices map[string]map[string]float64) func(output *ec2.DescribeSpotPriceHistoryOutput, b bool) bool {
	return func(output *ec2.DescribeSpotPriceHistoryOutput, b bool) bool {
		for _, sph := range output.SpotPriceHistory {
			spotPriceStr := aws.StringValue(sph.SpotPrice)
			spotPrice, err := strconv.ParseFloat(spotPriceStr, 64)
			// these errors shouldn't occur, but if pricing API does have an error, we ignore the record
			if err != nil {
				logging.FromContext(ctx).Debugf("unable to parse price record %#v", sph)
				continue
			}
			if sph.Timestamp == nil {
				continue
			}
			instanceType := aws.StringValue(sph.InstanceType)
			az := aws.StringValue(sph.AvailabilityZone)
			_, ok := prices[instanceType]
			if !ok {
				prices[instanceType] = map[string]float64{}
			}
			prices[instanceType][az] = spotPrice
		}
		return true
	}
}

// turning off cyclo here, it measures as a 12 due to all of the type checks of the pricing data which returns a deeply
// nested map[string]interface{}
// nolint: gocyclo
func (p *Provider) onDemandPage(ctx context.Context, prices map[string]float64) func(output *pricing.GetProductsOutput, b bool) bool {
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

	return func(output *pricing.GetProductsOutput, b bool) bool {
		currency := "USD"
		if strings.HasPrefix(p.region, "cn-") {
			currency = "CNY"
		}
		for _, outer := range output.PriceList {
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			if err := enc.Encode(outer); err != nil {
				logging.FromContext(ctx).Errorf("encoding %s", err)
			}
			dec := json.NewDecoder(&buf)
			var pItem priceItem
			if err := dec.Decode(&pItem); err != nil {
				logging.FromContext(ctx).Errorf("decoding %s", err)
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
					prices[pItem.Product.Attributes.InstanceType] = price
				}
			}
		}
		return true
	}
}

// nolint: gocyclo
func (p *Provider) UpdateSpotPricing(ctx context.Context) error {
	prices := map[string]map[string]float64{}

	p.muSpot.Lock()
	defer p.muSpot.Unlock()
	err := p.ec2.DescribeSpotPriceHistoryPagesWithContext(
		ctx,
		&ec2.DescribeSpotPriceHistoryInput{
			ProductDescriptions: []*string{
				aws.String("Linux/UNIX"),
				aws.String("Linux/UNIX (Amazon VPC)"),
			},
			// get the latest spot price for each instance type
			StartTime: aws.Time(time.Now()),
		},
		p.spotPage(ctx, prices),
	)

	if err != nil {
		return fmt.Errorf("retrieving spot pricing data, %w", err)
	}
	if len(prices) == 0 {
		return fmt.Errorf("no spot pricing found")
	}

	totalOfferings := 0
	for it, zoneData := range prices {
		if _, ok := p.spotPrices[it]; !ok {
			p.spotPrices[it] = newZonalPricing(0)
		}
		for zone, price := range zoneData {
			p.spotPrices[it].prices[zone] = price
		}
		totalOfferings += len(zoneData)
	}

	p.spotPricingUpdated = true
	if p.cm.HasChanged("spot-prices", p.spotPrices) {
		logging.FromContext(ctx).With(
			"instance-type-count", len(p.onDemandPrices),
			"offering-count", totalOfferings).Debugf("updated spot pricing with instance types and offerings")
	}
	return nil
}

func (p *Provider) LivenessProbe(_ *http.Request) error {
	// ensure we don't deadlock and nolint for the empty critical section
	p.muOnDemand.Lock()
	p.muSpot.Lock()
	//nolint: staticcheck
	p.muOnDemand.Unlock()
	p.muSpot.Unlock()
	return nil
}

func populateInitialSpotPricing(pricing map[string]float64) map[string]zonal {
	m := map[string]zonal{}
	for it, price := range pricing {
		m[it] = newZonalPricing(price)
	}
	return m
}

func (p *Provider) Reset() {
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
