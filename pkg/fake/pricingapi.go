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

package fake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/pricing"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type PricingAPI struct {
	sdk.PricingAPI
	PricingBehavior
}
type PricingBehavior struct {
	NextError         AtomicError
	GetProductsOutput AtomicPtr[pricing.GetProductsOutput]
}

func (p *PricingAPI) Reset() {
	p.NextError.Reset()
	p.GetProductsOutput.Reset()
}

func (p *PricingAPI) GetProducts(_ context.Context, _ *pricing.GetProductsInput, _ ...func(*pricing.Options)) (*pricing.GetProductsOutput, error) {
	if !p.NextError.IsNil() {
		return &pricing.GetProductsOutput{}, p.NextError.Get()
	}
	if !p.GetProductsOutput.IsNil() {
		return p.GetProductsOutput.Clone(), nil
	}
	// fail if the test doesn't provide specific data which causes our pricing provider to use its static price list
	return &pricing.GetProductsOutput{}, errors.New("no pricing data provided")
}

func NewOnDemandPrice(instanceType string, price float64) string {
	return NewOnDemandPriceWithCurrency(instanceType, price, "USD")

}

func NewOnDemandPriceWithCurrency(instanceType string, price float64, currency string) string {
	data := map[string]interface{}{
		"product": map[string]interface{}{
			"attributes": map[string]interface{}{
				"instanceType": instanceType,
			},
		},
		"terms": map[string]interface{}{
			"OnDemand": map[string]interface{}{
				"JRTCKXETXF.foo": map[string]interface{}{
					"offerTermCode": "JRTCKXETXF",
					"priceDimensions": map[string]interface{}{
						"JRTCKXETXF.foo.bar": map[string]interface{}{
							"pricePerUnit": map[string]interface{}{currency: fmt.Sprintf("%f", price)},
						},
					},
				},
			},
		},
	}
	ondemand, _ := json.Marshal(data)
	return string(ondemand)
}
