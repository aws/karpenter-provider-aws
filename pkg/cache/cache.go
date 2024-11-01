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
	// InstanceTypesAndZonesTTL is the time before we refresh instance types and zones at EC2
	InstanceTypesAndZonesTTL = 5 * time.Minute
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
)

const (
	// DefaultCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	DefaultCleanupInterval = time.Minute
	// UnavailableOfferingsCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	// We drop the cleanup interval down for the ICE cache to get quicker reactivity to offerings
	// that become available after they get evicted from the cache
	UnavailableOfferingsCleanupInterval = time.Second * 10
)
