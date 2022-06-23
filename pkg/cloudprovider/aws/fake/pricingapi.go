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
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/aws/aws-sdk-go/service/pricing/pricingiface"
)

type PricingAPI struct {
	pricingiface.PricingAPI
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

func (p *PricingAPI) GetProductsPagesWithContext(_ aws.Context, inp *pricing.GetProductsInput, fn func(*pricing.GetProductsOutput, bool) bool, opts ...request.Option) error {
	if !p.NextError.IsNil() {
		return p.NextError.Get()
	}
	if !p.GetProductsOutput.IsNil() {
		fn(p.GetProductsOutput.Clone(), false)
		return nil
	}
	// fail if the test doesn't provide specific data which causes our pricing provider to use its static price list
	return errors.New("no pricing data provided")
}

func NewOnDemandPrice(instanceType string, price float64) aws.JSONValue {
	return aws.JSONValue{
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
							"pricePerUnit": map[string]interface{}{"USD": fmt.Sprintf("%f", price)},
						},
					},
				},
			},
		},
	}
}
