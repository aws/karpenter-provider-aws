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

package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/patrickmn/go-cache"
)

// UnavailableOfferings stores any offerings that return ICE (insufficient capacity errors) when
// attempting to launch the capacity. These offerings are ignored as long as they are in the cache on
// GetInstanceTypes responses
type UnavailableOfferings struct {
	// key: <capacityType>:<instanceType>:<zone>, value: struct{}{}
	offeringCache         *cache.Cache
	offeringCacheSeqNumMu sync.RWMutex
	offeringCacheSeqNum   map[ec2types.InstanceType]uint64

	capacityTypeCache       *cache.Cache
	capacityTypeCacheSeqNum atomic.Uint64

	azCache       *cache.Cache
	azCacheSeqNum atomic.Uint64
}

func NewUnavailableOfferings() *UnavailableOfferings {
	uo := &UnavailableOfferings{
		offeringCache:         cache.New(UnavailableOfferingsTTL, UnavailableOfferingsCleanupInterval),
		offeringCacheSeqNumMu: sync.RWMutex{},
		offeringCacheSeqNum:   map[ec2types.InstanceType]uint64{},

		capacityTypeCache: cache.New(UnavailableOfferingsTTL, UnavailableOfferingsCleanupInterval),
		azCache:           cache.New(UnavailableOfferingsTTL, UnavailableOfferingsCleanupInterval),
	}
	uo.offeringCache.OnEvicted(func(k string, _ interface{}) {
		elems := strings.Split(k, ":")
		if len(elems) != 3 {
			panic("unavailable offerings cache key is not of expected format <capacity-type>:<instance-type>:<zone>")
		}
		uo.offeringCacheSeqNumMu.Lock()
		uo.offeringCacheSeqNum[ec2types.InstanceType(elems[1])]++
		uo.offeringCacheSeqNumMu.Unlock()
	})
	uo.capacityTypeCache.OnEvicted(func(k string, _ interface{}) {
		uo.capacityTypeCacheSeqNum.Add(1)
	})
	uo.azCache.OnEvicted(func(k string, _ interface{}) {
		uo.azCacheSeqNum.Add(1)
	})
	return uo
}

// SeqNum returns a sequence number for an instance type to capture whether the offering cache has changed for the intance type
func (u *UnavailableOfferings) SeqNum(instanceType ec2types.InstanceType) uint64 {
	u.offeringCacheSeqNumMu.RLock()
	defer u.offeringCacheSeqNumMu.RUnlock()

	v := u.offeringCacheSeqNum[instanceType]
	return v + u.capacityTypeCacheSeqNum.Load() + u.azCacheSeqNum.Load()
}

// IsUnavailable returns true if the offering appears in the cache
func (u *UnavailableOfferings) IsUnavailable(instanceType ec2types.InstanceType, zone, capacityType string) bool {
	_, offeringFound := u.offeringCache.Get(u.key(instanceType, zone, capacityType))
	_, capacityTypeFound := u.capacityTypeCache.Get(capacityType)
	_, azFound := u.azCache.Get(zone)
	return offeringFound || capacityTypeFound || azFound
}

// MarkUnavailable communicates recently observed temporary capacity shortages in the provided offerings
func (u *UnavailableOfferings) MarkUnavailable(ctx context.Context, instanceType ec2types.InstanceType, zone, capacityType string, unavailableReason map[string]string) {
	// even if the key is already in the cache, we still need to call Set to extend the cached entry's TTL
	logValues := []interface{}{
		"reason", unavailableReason["reason"],
		"instance-type", instanceType,
		"zone", zone,
		"capacity-type", capacityType,
		"ttl", UnavailableOfferingsTTL,
	}
	// Add fleetID if provided
	key := "fleet-id"
	_, ok := unavailableReason[key]
	if ok {
		logValues = append(logValues, key, unavailableReason[key])
	}
	log.FromContext(ctx).WithValues(logValues...).V(1).Info("removing offering from offerings")
	u.offeringCache.SetDefault(u.key(instanceType, zone, capacityType), struct{}{})
	u.offeringCacheSeqNumMu.Lock()
	u.offeringCacheSeqNum[instanceType]++
	u.offeringCacheSeqNumMu.Unlock()
}

func (u *UnavailableOfferings) MarkCapacityTypeUnavailable(capacityType string) {
	u.capacityTypeCache.SetDefault(capacityType, struct{}{})
	u.capacityTypeCacheSeqNum.Add(1)
}

func (u *UnavailableOfferings) MarkAZUnavailable(zone string) {
	u.azCache.SetDefault(zone, struct{}{})
	u.azCacheSeqNum.Add(1)
}

func (u *UnavailableOfferings) Delete(instanceType ec2types.InstanceType, zone string, capacityType string) {
	u.offeringCache.Delete(u.key(instanceType, zone, capacityType))
}

func (u *UnavailableOfferings) Flush() {
	u.offeringCache.Flush()
	u.capacityTypeCache.Flush()
	u.azCache.Flush()
}

// key returns the cache key for all offerings in the cache
func (u *UnavailableOfferings) key(instanceType ec2types.InstanceType, zone string, capacityType string) string {
	return fmt.Sprintf("%s:%s:%s", capacityType, instanceType, zone)
}
