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

package reservedinstance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/utils/clock"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	"github.com/aws/karpenter-provider-aws/pkg/aws"
)

const (
	cacheKey = "reserved-instances"
)

// Provider is an interface for getting reserved instance data.
type Provider interface {
	// GetReservedInstances returns all reserved instances for a given set of queries.
	GetReservedInstances(context.Context) ([]*ReservedInstance, error)
	// MarkLaunched decrements the count of available instances for a given instance type and availability zone.
	MarkLaunched(ec2types.InstanceType, string)
	// MarkTerminated increments the count of available instances for a given instance type and availability zone.
	MarkTerminated(ec2types.InstanceType, string)
}

// DefaultProvider is the reserved instance provider using the AWS API to get reserved instance information
type DefaultProvider struct {
	ec2   sdk.EC2API
	cache *availabilityCache
	cm    *pretty.ChangeMonitor
	mu    sync.Mutex
}

type instanceCounts map[ec2types.InstanceType]map[string]int32

// NewDefaultProvider constructs a new DefaultProvider
func NewDefaultProvider(ec2 sdk.EC2API, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2: ec2,
		cm:  pretty.NewChangeMonitor(),
		cache: &availabilityCache{
			cache: cache,
			clk:   clock.RealClock{},
		},
	}
}

// GetReservedInstances gets all reserved instances and adds them to the cache
func (p *DefaultProvider) GetReservedInstances(ctx context.Context) ([]*ReservedInstance, error) {
	// return from cache if it's already populated
	reservedInstances := p.buildReservedInstancesFromCache()
	if len(reservedInstances) > 0 {
		return reservedInstances, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// After we acquire the lock, we need to re-validate that the cache is empty. It's possible that another
	// thread acquired the lock and populated the cache while we were waiting.
	reservedInstances = p.buildReservedInstancesFromCache()
	if len(reservedInstances) > 0 {
		return reservedInstances, nil
	}

	riOutput, err := p.ec2.DescribeReservedInstances(ctx, &ec2.DescribeReservedInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("state"),
				Values: []string{string(ec2types.ReservedInstanceStateActive)},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("describing reserved instances, %w", err)
	}

	instanceOutput, err := p.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("instance-state-name"),
				Values: []string{string(ec2types.InstanceStateNameRunning)},
			},
			{
				Name:   lo.ToPtr("tenancy"),
				Values: []string{string(ec2types.TenancyDefault)},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("describing instances, %w", err)
	}

	p.updateCache(riOutput.ReservedInstances, instanceOutput.Reservations)

	reservedInstances = p.buildReservedInstancesFromCache()
	if p.cm.HasChanged(cacheKey, reservedInstances) {
		pretty.FriendlyDiff(p.cm.Previous(cacheKey), p.cm.Current(cacheKey))
	}
	return reservedInstances, nil
}

func (p *DefaultProvider) MarkLaunched(instanceType ec2types.InstanceType, zone string) {
	p.cache.MarkLaunched(instanceType, zone)
}

func (p *DefaultProvider) MarkTerminated(instanceType ec2types.InstanceType, zone string) {
	p.cache.MarkTerminated(instanceType, zone)
}

func (p *DefaultProvider) updateCache(ris []ec2types.ReservedInstances, reservations []ec2types.Reservation) {
	runningInstances := make(instanceCounts)
	for _, res := range reservations {
		for _, inst := range res.Instances {
			if _, ok := runningInstances[inst.InstanceType]; !ok {
				runningInstances[inst.InstanceType] = make(map[string]int32)
			}
			if inst.Platform != ec2types.PlatformValuesWindows {
				runningInstances[inst.InstanceType][*inst.Placement.AvailabilityZone]++
			}
		}
	}

	purchasedRIs := make(instanceCounts)
	activeRIs := lo.Filter(ris, func(r ec2types.ReservedInstances, _ int) bool {
		return r.End.After(time.Now()) && r.InstanceTenancy == ec2types.TenancyDefault
	})
	for _, ri := range activeRIs {
		if _, ok := purchasedRIs[ri.InstanceType]; !ok {
			purchasedRIs[ri.InstanceType] = make(map[string]int32)
		}
		purchasedRIs[ri.InstanceType][*ri.AvailabilityZone] += *ri.InstanceCount
	}

	availability := make(map[string]*availabilityCacheEntry)
	for instType, zones := range purchasedRIs {
		for zone, count := range zones {
			runningCount := int32(0)
			if _, ok := runningInstances[instType]; ok {
				runningCount = runningInstances[instType][zone]
			}
			availableCount := count - runningCount
			key := p.cache.makeCacheKey(instType, zone)
			availability[key] = &availabilityCacheEntry{
				count: lo.Max(0, availableCount),
				total: count,
			}
		}
	}
	p.cache.syncAvailability(availability)
}

func (p *DefaultProvider) buildReservedInstancesFromCache() []*ReservedInstance {
	var reservedInstances []*ReservedInstance
	for key, item := range p.cache.cache.Items() {
		entry := item.Object.(*availabilityCacheEntry)
		if entry.count > 0 {
			instanceType, zone := p.cache.decodeCacheKey(key)
			reservedInstances = append(reservedInstances, &ReservedInstance{
				ID:               fmt.Sprintf("ri-%s-%s", instanceType, zone),
				InstanceType:     instanceType,
				InstanceCount:    entry.count,
				AvailabilityZone: zone,
				State:            ec2types.ReservedInstanceStateActive,
			})
		}
	}
	return reservedInstances
}