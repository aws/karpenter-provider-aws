// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package strategy

import (
	"context"
	"fmt"
	"math"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

type Strategy interface {
	GetScore(instanceType, capacityType, availabilityZone string) float64
}

type LowestPrice struct {
	pricingProvider pricing.Provider
}

func NewLowestPrice(pricingAPI sdk.PricingAPI, ec2API sdk.EC2API, region string) *LowestPrice {
	pricingProvider := pricing.NewDefaultProvider(pricingAPI, ec2API, region, false)
	lo.Must0(pricingProvider.UpdateOnDemandPricing(context.Background()))
	lo.Must0(pricingProvider.UpdateSpotPricing(context.Background()))
	return &LowestPrice{
		pricingProvider: pricingProvider,
	}
}

func (p *LowestPrice) GetScore(instanceType, capacityType, availabilityZone string) float64 {
	switch capacityType {
	case v1.CapacityTypeSpot:
		if score, ok := p.pricingProvider.SpotPrice(ec2types.InstanceType(instanceType), availabilityZone); ok {
			return score
		}
		return math.MaxFloat64
	case v1.CapacityTypeOnDemand:
		if score, ok := p.pricingProvider.OnDemandPrice(ec2types.InstanceType(instanceType)); ok {
			return score
		}
		return math.MaxFloat64
	default:
		panic(fmt.Sprintf("Unsupported capacity type: %s", capacityType))
	}
}
