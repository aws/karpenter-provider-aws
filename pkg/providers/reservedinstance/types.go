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

// package reservedinstance provides types and methods for querying and managing
// EC2 Reserved Instances.
package reservedinstance

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"k8s.io/utils/clock"
)

// ReservedInstance is a struct that defines the parameters for an EC2 Reserved Instance.
type ReservedInstance struct {
	ID               string
	InstanceType     ec2types.InstanceType
	InstanceCount    int32
	AvailabilityZone string
	State            ec2types.ReservedInstanceState
}

type availabilityCache struct {
	mu    sync.RWMutex
	cache *cache.Cache
	clk   clock.Clock
}

type availabilityCacheEntry struct {
	count    int32
	total    int32
	syncTime time.Time
}

func (c *availabilityCache) makeCacheKey(instanceType ec2types.InstanceType, zone string) string {
	return fmt.Sprintf("%s/%s", instanceType, zone)
}

func (c *availabilityCache) decodeCacheKey(key string) (ec2types.InstanceType, string) {
	parts := strings.Split(key, "/")
	return ec2types.InstanceType(parts[0]), parts[1]
}

func (c *availabilityCache) syncAvailability(availability map[string]*availabilityCacheEntry) {
	now := c.clk.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Flush()
	for key, entry := range availability {
		entry.syncTime = now
		c.cache.SetDefault(key, entry)
	}
}

func (c *availabilityCache) MarkLaunched(instanceType ec2types.InstanceType, zone string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.makeCacheKey(instanceType, zone)
	if entry, ok := c.cache.Get(key); ok {
		cacheEntry := entry.(*availabilityCacheEntry)
		if cacheEntry.count > 0 {
			cacheEntry.count--
		}
	}
}

func (c *availabilityCache) MarkTerminated(instanceType ec2types.InstanceType, zone string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.makeCacheKey(instanceType, zone)
	if entry, ok := c.cache.Get(key); ok {
		cacheEntry := entry.(*availabilityCacheEntry)
		if cacheEntry.count < cacheEntry.total {
			cacheEntry.count++
		}
	}
}