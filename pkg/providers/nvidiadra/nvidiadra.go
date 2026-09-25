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
	"context"
	"fmt"
	"unique"

	"github.com/samber/lo"
	resourcev1 "k8s.io/api/resource/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/providers/drametadata"
)

// DriverName is the NVIDIA GPU DRA driver name, matching the ResourceSlices it publishes.
const DriverName = "gpu.nvidia.com"

// PoolName names the simulated device pool. The driver pools per node at runtime, but a template is
// inherently per-instance-type, so a stable name is all the allocator needs.
const PoolName = "gpus"

// Attributes the driver only resolves at runtime, so they're declared as AttributeBindings rather
// than given a value. Note the allocator ignores bindings covering fewer than two devices, so these
// do nothing for a single-GPU instance type.
const (
	AttributeDriverVersion     resourcev1.QualifiedName = "driverVersion"
	AttributeCUDADriverVersion resourcev1.QualifiedName = "cudaDriverVersion"
)

// Capacities the driver publishes. Memory is always present; shares appears only when the driver runs
// with an integer consumableShares value.
const (
	CapacityMemory resourcev1.QualifiedName = "memory"
	CapacityShares resourcev1.QualifiedName = "shares"
)

var boundAttributes = []resourcev1.QualifiedName{AttributeDriverVersion, AttributeCUDADriverVersion}

// dynamicResources holds the GPU DRA metadata keyed by instance type name, built once at package init
// and immutable afterwards, making it safe to share by pointer. Only the capacities and the shareable
// flag depend on the EC2NodeClass annotation, so this is everything else.
var dynamicResources = buildDynamicResources()

// buildDynamicResources converts the scraped GPU metadata into one template per instance type, with one
// device per physical GPU named to match the driver's "gpu-<index>" convention, plus the
// AttributeBindings for the runtime-only attributes those devices share.
func buildDynamicResources() map[string]cloudprovider.DynamicResources {
	resources := make(map[string]cloudprovider.DynamicResources, len(drametadata.GPUMetadataByInstanceType))
	for instanceType, metadata := range drametadata.GPUMetadataByInstanceType {
		driver := unique.Make(DriverName)
		pool := cloudprovider.ResourcePool{Name: unique.Make(PoolName)}

		devices := lo.Map(metadata.Devices, func(device drametadata.DRADevice, index int) cloudprovider.Device {
			return cloudprovider.Device{
				Name:       unique.Make(fmt.Sprintf("gpu-%d", index)),
				Attributes: device.Attributes,
				Capacity:   device.Capacity,
			}
		})
		deviceIDs := lo.Map(devices, func(device cloudprovider.Device, _ int) cloudprovider.DeviceID {
			return cloudprovider.DeviceID{Driver: driver, Pool: pool.Name, Device: device.Name}
		})

		resources[instanceType] = cloudprovider.DynamicResources{
			ResourceSliceTemplates: []*cloudprovider.ResourceSliceTemplate{{
				Driver:  driver,
				Pool:    pool,
				Devices: devices,
			}},
			// The bound attributes are node-wide, so one binding covers every GPU.
			AttributeBindings: lo.Map(boundAttributes, func(attribute resourcev1.QualifiedName, _ int) *cloudprovider.AttributeBinding {
				return &cloudprovider.AttributeBinding{Attribute: attribute, Devices: deviceIDs}
			}),
		}
	}
	return resources
}

// withConsumableCapacity marks every GPU allocatable to multiple claims and gives its capacities the
// driver's request policies. The base is shared, so the device list is rebuilt rather than written to.
// The attribute bindings carry over untouched: device identity does not depend on the mode.
func withConsumableCapacity(base cloudprovider.DynamicResources, mode *ConsumableCapacityMode) cloudprovider.DynamicResources {
	template := base.ResourceSliceTemplates[0]
	return cloudprovider.DynamicResources{
		ResourceSliceTemplates: []*cloudprovider.ResourceSliceTemplate{{
			Driver: template.Driver,
			Pool:   template.Pool,
			Devices: lo.Map(template.Devices, func(device cloudprovider.Device, _ int) cloudprovider.Device {
				// Every sharing mode is denominated in the GPU's memory, so a device without it can't be
				// given a request policy. Leave it single-allocation rather than advertising sharing the
				// driver won't back, which would over-pack the node.
				if _, ok := device.Capacity[CapacityMemory]; !ok {
					return device
				}
				device.Capacity = capacityFor(device.Capacity, mode)
				device.AllowMultipleAllocations = true
				return device
			}),
		}},
		AttributeBindings: base.AttributeBindings,
	}
}

// Provider resolves the NVIDIA GPU DRA metadata for a set of instance types.
type Provider interface {
	// ResolveDynamicResources returns the NVIDIA GPU templates and attribute bindings keyed by
	// instance type name. Instance types with no NVIDIA GPU metadata are omitted.
	ResolveDynamicResources(ctx context.Context, instanceTypes []*cloudprovider.InstanceType, mode *ConsumableCapacityMode) map[string]cloudprovider.DynamicResources
}

type DefaultProvider struct{}

func NewDefaultProvider() *DefaultProvider {
	return &DefaultProvider{}
}

func (p *DefaultProvider) ResolveDynamicResources(_ context.Context, instanceTypes []*cloudprovider.InstanceType, mode *ConsumableCapacityMode) map[string]cloudprovider.DynamicResources {
	resources := map[string]cloudprovider.DynamicResources{}
	for _, it := range instanceTypes {
		base, ok := dynamicResources[it.Name]
		if !ok {
			continue
		}
		if mode == nil {
			resources[it.Name] = base
			continue
		}
		resources[it.Name] = withConsumableCapacity(base, mode)
	}
	return resources
}
