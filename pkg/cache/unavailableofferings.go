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
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/patrickmn/go-cache"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// UnavailableOfferings stores any offerings that return ICE (insufficient capacity errors) when
// attempting to launch the capacity. These offerings are ignored as long as they are in the cache on
// GetInstanceTypes responses
type UnavailableOfferings struct {
	// key: <capacityType>:<instanceType>:<zone>, value: struct{}{}
	cache  *cache.Cache
	SeqNum uint64
}

func NewUnavailableOfferings(ttl time.Duration) *UnavailableOfferings {
	uo := &UnavailableOfferings{
		cache:  cache.New(ttl, UnavailableOfferingsCleanupInterval),
		SeqNum: 0,
	}
	uo.cache.OnEvicted(func(_ string, _ interface{}) {
		atomic.AddUint64(&uo.SeqNum, 1)
	})
	return uo
}

// IsUnavailable returns true if the offering appears in the cache
func (u *UnavailableOfferings) IsUnavailable(instanceType, zone, capacityType string) bool {
	_, found := u.cache.Get(u.key(instanceType, zone, capacityType))
	return found
}

// MarkUnavailable communicates recently observed temporary capacity shortages in the provided offerings
func (u *UnavailableOfferings) MarkUnavailable(ctx context.Context, unavailableReason, instanceType, zone, capacityType string) {
	// even if the key is already in the cache, we still need to call Set to extend the cached entry's TTL
	log.FromContext(ctx).WithValues(
		"reason", unavailableReason,
		"instance-type", instanceType,
		"zone", zone,
		"capacity-type", capacityType,
		"ttl", UnavailableOfferingsTTL).V(1).Info("removing offering from offerings")
	u.cache.SetDefault(u.key(instanceType, zone, capacityType), struct{}{})
	atomic.AddUint64(&u.SeqNum, 1)
}

func (u *UnavailableOfferings) MarkUnavailableForFleetErr(ctx context.Context, fleetErr *ec2.CreateFleetError, capacityType string) {
	instanceType := aws.StringValue(fleetErr.LaunchTemplateAndOverrides.Overrides.InstanceType)
	zone := aws.StringValue(fleetErr.LaunchTemplateAndOverrides.Overrides.AvailabilityZone)
	u.MarkUnavailable(ctx, aws.StringValue(fleetErr.ErrorCode), instanceType, zone, capacityType)
}

func (u *UnavailableOfferings) Delete(instanceType string, zone string, capacityType string) {
	u.cache.Delete(u.key(instanceType, zone, capacityType))
}

func (u *UnavailableOfferings) Flush() {
	u.cache.Flush()
}

// key returns the cache key for all offerings in the cache
func (u *UnavailableOfferings) key(instanceType string, zone string, capacityType string) string {
	return fmt.Sprintf("%s:%s:%s", capacityType, instanceType, zone)
}
