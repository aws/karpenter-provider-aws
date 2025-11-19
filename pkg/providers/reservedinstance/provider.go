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
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	"github.com/aws/karpenter-provider-aws/pkg/aws"
)

const (
	cacheKey = "reserved-instances"
)

// DefaultProvider is the reserved instance provider using the AWS API to get reserved instance information
type DefaultProvider struct {
	ec2   aws.EC2API
	cache *cache.Cache
	cm    *pretty.ChangeMonitor
	mu    sync.Mutex
}

type instanceCounts map[ec2types.InstanceType]map[string]int32

// NewDefaultProvider constructs a new DefaultProvider
func NewDefaultProvider(ec2 aws.EC2API) *DefaultProvider {
	return &DefaultProvider{
		ec2:   ec2,
		cm:    pretty.NewChangeMonitor(),
		cache: cache.New(10*time.Minute, 1*time.Minute),
	}
}

// GetReservedInstances gets all reserved instances and adds them to the cache
func (p *DefaultProvider) GetReservedInstances(ctx context.Context) ([]*ReservedInstance, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cached, ok := p.cache.Get(cacheKey); ok {
		return cached.([]*ReservedInstance), nil
	}

	// 1. Get all active Reserved Instances
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

	// 2. Get all running EC2 instances that could be consuming RIs
	instanceOutput, err := p.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("instance-state-name"),
				Values: []string{string(ec2types.InstanceStateNameRunning)},
			},
			{
				Name:   lo.ToPtr("tenancy"),
				Values: []string{string(ec2types.TenancyDefault)}, // RIs apply to default tenancy
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("describing instances, %w", err)
	}

	// 3. Calculate available RIs and cache the result
	reservedInstances := p.buildReservedInstances(riOutput.ReservedInstances, instanceOutput.Reservations)
	p.cache.SetDefault(cacheKey, reservedInstances)
	if p.cm.HasChanged(cacheKey, reservedInstances) {
		pretty.FriendlyDiff(p.cm.Previous(cacheKey), p.cm.Current(cacheKey))
	}
	return reservedInstances, nil
}

func (p *DefaultProvider) buildReservedInstances(ris []ec2types.ReservedInstances, reservations []ec2types.Reservation) []*ReservedInstance {
	// Group running instances by type and AZ for efficient lookup
	runningInstances := make(instanceCounts)
	for _, res := range reservations {
		for _, inst := range res.Instances {
			if _, ok := runningInstances[inst.InstanceType]; !ok {
				runningInstances[inst.InstanceType] = make(map[string]int32)
			}
			// We only consider Linux instances for RIs
			if inst.Platform != ec2types.PlatformValuesWindows {
				runningInstances[inst.InstanceType][*inst.Placement.AvailabilityZone]++
			}
		}
	}

	var availableRIs []*ReservedInstance
	// Filter out expired RIs and those with dedicated tenancy
	activeRIs := lo.Filter(ris, func(r ec2types.ReservedInstances, _ int) bool {
		return r.End.After(time.Now()) && r.InstanceTenancy == ec2types.TenancyDefault
	})

	for _, ri := range activeRIs {
		runningCount := int32(0)
		if _, ok := runningInstances[ri.InstanceType]; ok {
			runningCount = runningInstances[ri.InstanceType][*ri.AvailabilityZone]
		}

		availableCount := *ri.InstanceCount - runningCount
		if availableCount > 0 {
			availableRIs = append(availableRIs, &ReservedInstance{
				ID:               *ri.ReservedInstancesId,
				InstanceType:     ri.InstanceType,
				InstanceCount:    availableCount, // Only expose the available count
				AvailabilityZone: *ri.AvailabilityZone,
				State:            ri.State,
			})
		}
	}
	return availableRIs
}