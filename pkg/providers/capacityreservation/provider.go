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

package capacityreservation

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/utils/clock"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Provider interface {
	List(context.Context, ...v1.CapacityReservationSelectorTerm) ([]*ec2types.CapacityReservation, error)
	GetAvailableInstanceCount(string) int
	MarkLaunched(string)
	MarkTerminated(string)
	MarkUnavailable(...string)
}

type DefaultProvider struct {
	availabilityCache

	ec2api           sdk.EC2API
	clk              clock.Clock
	reservationCache *cache.Cache
	cm               *pretty.ChangeMonitor
}

func NewProvider(ec2api sdk.EC2API, clk clock.Clock, reservationCache, reservationAvailabilityCache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		availabilityCache: availabilityCache{
			cache: reservationAvailabilityCache,
			clk:   clk,
		},
		ec2api:           ec2api,
		clk:              clk,
		reservationCache: reservationCache,
		cm:               pretty.NewChangeMonitor(),
	}
}

func (p *DefaultProvider) List(ctx context.Context, selectorTerms ...v1.CapacityReservationSelectorTerm) ([]*ec2types.CapacityReservation, error) {
	queries := QueriesFromSelectorTerms(selectorTerms...)

	var reservations []*ec2types.CapacityReservation
	var remainingQueries []*Query
	for _, query := range queries {
		if value, ok := p.reservationCache.Get(query.CacheKey()); ok {
			reservations = append(reservations, value.([]*ec2types.CapacityReservation)...)
		} else {
			remainingQueries = append(remainingQueries, query)
		}
	}
	if len(remainingQueries) == 0 {
		return p.filterReservations(reservations), nil
	}

	for _, query := range remainingQueries {
		paginator := ec2.NewDescribeCapacityReservationsPaginator(p.ec2api, query.DescribeCapacityReservationsInput())
		for paginator.HasMorePages() {
			out, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("listing capacity reservations, %w", err)
			}
			queryReservations := lo.ToSlicePtr(out.CapacityReservations)
			p.reservationCache.SetDefault(query.CacheKey(), queryReservations)
			reservations = append(reservations, queryReservations...)
			p.syncAvailability(lo.SliceToMap(queryReservations, func(r *ec2types.CapacityReservation) (string, int) {
				return *r.CapacityReservationId, int(*r.AvailableInstanceCount)
			}))
		}
	}

	return p.filterReservations(reservations), nil
}

// filterReservations removes duplicate and expired reservations
func (p *DefaultProvider) filterReservations(reservations []*ec2types.CapacityReservation) []*ec2types.CapacityReservation {
	return lo.Filter(lo.UniqBy(reservations, func(r *ec2types.CapacityReservation) string {
		return *r.CapacityReservationId
	}), func(r *ec2types.CapacityReservation, _ int) bool {
		if r.EndDate == nil {
			return true
		}
		return r.EndDate.After(p.clk.Now())
	})
}
