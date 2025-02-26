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
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/utils/clock"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
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
	sync.Mutex

	ec2api           sdk.EC2API
	clk              clock.Clock
	reservationCache *cache.Cache
	cm               *pretty.ChangeMonitor
}

func NewProvider(
	ec2api sdk.EC2API,
	clk clock.Clock,
	reservationCache, reservationAvailabilityCache *cache.Cache,
) *DefaultProvider {
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
	// Take a write lock over the entire List operation to ensure minimize duplicate DescribeCapacityReservation calls
	p.Lock()
	defer p.Unlock()

	var reservations []*ec2types.CapacityReservation
	queries := QueriesFromSelectorTerms(selectorTerms...)
	reservations, queries = p.resolveCachedQueries(queries...)
	if len(queries) == 0 {
		return p.filterReservations(reservations), nil
	}
	for _, q := range queries {
		paginator := ec2.NewDescribeCapacityReservationsPaginator(p.ec2api, q.DescribeCapacityReservationsInput())
		var queryReservations []*ec2types.CapacityReservation
		for paginator.HasMorePages() {
			out, err := paginator.NextPage(ctx)
			if err != nil {
				if awserrors.IsNotFound(err) {
					// Note: we only receive this error when requesting a single ID, in which case we will only ever get a single page.
					// Replacing this with a continue will result in an infinite loop as HasMorePages will always return true.
					break
				}
				return nil, fmt.Errorf("listing capacity reservations, %w", err)
			}
			queryReservations = append(queryReservations, lo.ToSlicePtr(out.CapacityReservations)...)
		}
		p.syncAvailability(lo.SliceToMap(queryReservations, func(r *ec2types.CapacityReservation) (string, int) {
			return *r.CapacityReservationId, int(*r.AvailableInstanceCount)
		}))
		p.reservationCache.SetDefault(q.CacheKey(), queryReservations)
		reservations = append(reservations, queryReservations...)
	}
	return p.filterReservations(reservations), nil
}

func (p *DefaultProvider) resolveCachedQueries(queries ...*Query) (reservations []*ec2types.CapacityReservation, remainingQueries []*Query) {
	for _, q := range queries {
		if value, ok := p.reservationCache.Get(q.CacheKey()); ok {
			reservations = append(reservations, value.([]*ec2types.CapacityReservation)...)
		} else {
			remainingQueries = append(remainingQueries, q)
		}
	}
	return reservations, remainingQueries
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
