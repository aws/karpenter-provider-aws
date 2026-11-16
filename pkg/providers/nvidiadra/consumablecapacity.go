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

package nvidiadra

import (
	"fmt"
	"maps"
	"strconv"

	"github.com/samber/lo"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type ConsumableCapacityMode struct {
	Shares            int64
	Memory            bool
	FullMemoryDefault bool
}

const (
	ConsumableCapacityDisabled  = "disabled"
	ConsumableCapacityMemory    = "memory"
	ConsumableCapacityUnlimited = "unlimited"
)

// ParseConsumableCapacity resolves an EC2NodeClass annotation value into a mode. An empty value means
// the annotation is absent, which is the same as disabled. It accepts exactly the driver's value space.
func ParseConsumableCapacity(value string) (*ConsumableCapacityMode, error) {
	switch value {
	case "", ConsumableCapacityDisabled:
		return nil, nil
	case ConsumableCapacityMemory:
		return &ConsumableCapacityMode{Memory: true, FullMemoryDefault: true}, nil
	case ConsumableCapacityUnlimited:
		return &ConsumableCapacityMode{Memory: true}, nil
	}
	shares, err := strconv.ParseInt(value, 10, 64)
	if err != nil || shares < 1 {
		return nil, fmt.Errorf("%q is not one of %q, %q, %q, or a positive integer share count", value,
			ConsumableCapacityDisabled, ConsumableCapacityMemory, ConsumableCapacityUnlimited)
	}
	return &ConsumableCapacityMode{Shares: shares}, nil
}

// capacityFor returns the device's capacities with the driver's request policies applied. The scraped
// map is shared by every caller, so this builds its own rather than writing to it. The caller has
// already established that the device carries a memory capacity.
func capacityFor(scraped map[resourcev1.QualifiedName]resourcev1.DeviceCapacity, mode *ConsumableCapacityMode) map[resourcev1.QualifiedName]resourcev1.DeviceCapacity {
	memory := scraped[CapacityMemory]
	capacities := make(map[resourcev1.QualifiedName]resourcev1.DeviceCapacity, len(scraped)+1)
	maps.Copy(capacities, scraped)

	memoryDefault, memoryMin := resource.MustParse("0"), resource.MustParse("0")
	if mode.FullMemoryDefault {
		memoryDefault, memoryMin = memory.Value, resource.MustParse("1Mi")
	}
	capacities[CapacityMemory] = resourcev1.DeviceCapacity{
		Value: memory.Value,
		RequestPolicy: &resourcev1.CapacityRequestPolicy{
			Default: &memoryDefault,
			ValidRange: &resourcev1.CapacityRequestPolicyRange{
				Min:  &memoryMin,
				Max:  &memory.Value,
				Step: lo.ToPtr(resource.MustParse("1Mi")),
			},
		},
	}

	// An integer mode publishes a shares dimension alongside the zero-defaulted memory one.
	if mode.Shares > 0 {
		capacities[CapacityShares] = resourcev1.DeviceCapacity{
			Value: *resource.NewQuantity(mode.Shares, resource.DecimalSI),
			RequestPolicy: &resourcev1.CapacityRequestPolicy{
				Default: resource.NewQuantity(1, resource.DecimalSI),
				ValidRange: &resourcev1.CapacityRequestPolicyRange{
					Min:  resource.NewQuantity(1, resource.DecimalSI),
					Max:  resource.NewQuantity(mode.Shares, resource.DecimalSI),
					Step: resource.NewQuantity(1, resource.DecimalSI),
				},
			},
		}
	}
	return capacities
}
