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
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/utils/clock"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type Query struct {
	ID                    string
	OwnerID               string
	Tags                  map[string]string
	InstanceMatchCriteria string
}

func QueriesFromSelectorTerms(terms ...v1.CapacityReservationSelectorTerm) []*Query {
	queries := []*Query{}
	for i := range terms {
		if id := terms[i].ID; id != "" {
			queries = append(queries, &Query{ID: id})
		}
		if len(terms[i].Tags) != 0 || terms[i].InstanceMatchCriteria != "" {
			queries = append(queries, &Query{
				OwnerID:               terms[i].OwnerID,
				Tags:                  terms[i].Tags,
				InstanceMatchCriteria: terms[i].InstanceMatchCriteria,
			})
		}
	}
	return queries
}

func (q *Query) CacheKey() string {
	return fmt.Sprintf("%d", lo.Must(hashstructure.Hash(q, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets: true,
	})))
}

func (q *Query) DescribeCapacityReservationsInput() *ec2.DescribeCapacityReservationsInput {
	filters := []ec2types.Filter{{
		Name:   lo.ToPtr("state"),
		Values: []string{string(ec2types.CapacityReservationStateActive)},
	}}
	if len(q.ID) != 0 {
		// MaxResults can't be used when listing a capacity reservation by id
		return &ec2.DescribeCapacityReservationsInput{
			Filters:                filters,
			CapacityReservationIds: []string{q.ID},
		}
	}
	if q.OwnerID != "" {
		filters = append(filters, ec2types.Filter{
			Name:   lo.ToPtr("owner-id"),
			Values: []string{q.OwnerID},
		})
	}
	if len(q.Tags) != 0 {
		filters = append(filters, lo.MapToSlice(q.Tags, func(k, v string) ec2types.Filter {
			if v == "*" {
				return ec2types.Filter{
					Name:   lo.ToPtr("tag-key"),
					Values: []string{k},
				}
			}
			return ec2types.Filter{
				Name:   lo.ToPtr(fmt.Sprintf("tag:%s", k)),
				Values: []string{v},
			}
		})...)
	}
	if q.InstanceMatchCriteria != "" {
		filters = append(filters, ec2types.Filter{
			Name:   lo.ToPtr("instance-match-criteria"),
			Values: []string{q.InstanceMatchCriteria},
		})
	}
	return &ec2.DescribeCapacityReservationsInput{
		Filters: filters,
		// MaxResults for DescribeCapacityReservations is capped at 1000
		MaxResults: lo.ToPtr[int32](1000),
	}
}

type availabilityCache struct {
	mu    sync.RWMutex
	cache *cache.Cache
	clk   clock.Clock
}

type availabilityCacheEntry struct {
	count    int
	syncTime time.Time
}

func (c *availabilityCache) syncAvailability(availability map[string]int) {
	now := c.clk.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, count := range availability {
		c.cache.SetDefault(id, &availabilityCacheEntry{
			count:    count,
			syncTime: now,
		})
	}
}

func (c *availabilityCache) MarkLaunched(reservationID string) {
	now := c.clk.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache.Get(reservationID)
	if !ok {
		return
	}
	// Only count the launch if it occurred before the last sync from EC2. In the worst case, this will lead to us
	// overestimating availability if there's an eventual consistency delay with EC2, but we'd rather overestimate than
	// underestimate.
	if entry.(*availabilityCacheEntry).syncTime.After(now) {
		return
	}

	if entry.(*availabilityCacheEntry).count != 0 {
		entry.(*availabilityCacheEntry).count -= 1
	}
}

func (c *availabilityCache) MarkTerminated(reservationID string) {
	// We don't do a time based comparison for CountTerminated because the reservation becomes available some time between
	// the termination call and the instance state transitioning to terminated. This can be a pretty big gap, so a time
	// based comparison would have limited value. In the worst case, this can result in us overestimating the available
	// capacity, but we'd rather overestimate than underestimate.
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache.Get(reservationID)
	if !ok {
		return
	}
	entry.(*availabilityCacheEntry).count += 1
}

func (c *availabilityCache) GetAvailableInstanceCount(reservationID string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.cache.Get(reservationID)
	if !ok {
		return 0
	}
	return entry.(*availabilityCacheEntry).count
}

// TODO: Determine better abstraction for setting availability in tests without reconciling the nodeclass controller
func (c *availabilityCache) SetAvailableInstanceCount(reservationID string, count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.SetDefault(reservationID, &availabilityCacheEntry{
		count:    count,
		syncTime: c.clk.Now(),
	})
}

func (c *availabilityCache) MarkUnavailable(reservationIDs ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range reservationIDs {
		entry, ok := c.cache.Get(id)
		if !ok {
			continue
		}
		entry.(*availabilityCacheEntry).count = 0
	}
}
