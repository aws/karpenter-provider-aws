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

import "time"

const (
	// DefaultTTL restricts QPS to AWS APIs to this interval for verifying setup
	// resources. This value represents the maximum eventual consistency between
	// AWS actual state and the controller's ability to provision those
	// resources. Cache hits enable faster provisioning and reduced API load on
	// AWS APIs, which can have a serious impact on performance and scalability.
	// DO NOT CHANGE THIS VALUE WITHOUT DUE CONSIDERATION
	DefaultTTL = time.Minute
	// UnavailableOfferingsTTL is the time before offerings that were marked as unavailable
	// are removed from the cache and are available for launch again
	UnavailableOfferingsTTL = 3 * time.Minute
	// CapacityReservationAvailabilityTTL is the time we will persist cached capacity availability. Nominally, this is
	// updated every minute, but we want to persist the data longer in the event of an EC2 API outage. 24 hours was the
	// compormise made for API outage reseliency and gargage collecting entries for orphaned reservations.
	CapacityReservationAvailabilityTTL = 24 * time.Hour
	// InstanceTypesZonesAndOfferingsTTL is the time before we refresh instance types, zones, and offerings at EC2
	InstanceTypesZonesAndOfferingsTTL = 5 * time.Minute
	// InstanceProfileTTL is the time before we refresh checking instance profile existence at IAM
	InstanceProfileTTL = 15 * time.Minute
	// AvailableIPAddressTTL is time to drop AvailableIPAddress data if it is not updated within the TTL
	AvailableIPAddressTTL = 5 * time.Minute
	// AvailableIPAddressTTL is time to drop AssociatePublicIPAddressTTL data if it is not updated within the TTL
	AssociatePublicIPAddressTTL = 5 * time.Minute
	// SSMGetParametersByPathTTL is the time to drop SSM Parameters by path data. This only queries EKS Optimized AMI
	// releases, so we should expect this to be updated relatively infrequently.
	SSMCacheTTL = 24 * time.Hour
	// DiscoveredCapacityCacheTTL is the time to drop discovered resource capacity data per-instance type
	// if it is not updated by a node creation event or refreshed during controller reconciliation
	DiscoveredCapacityCacheTTL = 60 * 24 * time.Hour
	// ValidationTTL is time to check authorization errors with validation controller
	ValidationTTL = 30 * time.Minute
	// RecreationTTL is the duration to suppress instance profile recreation for the same role to avoid duplicates
	RecreationTTL = 1 * time.Minute
	// ProtectedProfilesTTL is the duration to keep profiles as protected before nodeclass garbagecollector considers deletion
	ProtectedProfilesTTL = 1 * time.Hour
	// ReservedInstancePriceTTL is the time before we refresh reserved instance price data
	ReservedInstancePriceTTL = 60 * time.Minute
)

const (
	// DefaultCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	DefaultCleanupInterval = time.Minute
	// UnavailableOfferingsCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	// We drop the cleanup interval down for the ICE cache to get quicker reactivity to offerings
	// that become available after they get evicted from the cache
	UnavailableOfferingsCleanupInterval = time.Second * 10
)
