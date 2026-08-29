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

package efadra

import (
	"context"
	"fmt"
	"unique"

	"github.com/samber/lo"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/providers/drametadata"
)

const DriverName = "dra.net"

// PoolName names the simulated device pool. The driver pools per node at runtime, but a template is
// inherently per-instance-type, so a stable name is all the allocator needs.
const PoolName = "network"

// dynamicResources holds the network device DRA metadata keyed by instance type name, built once at
// package init and immutable afterwards, making it safe to share by pointer.
var dynamicResources = buildDynamicResources()

// buildDynamicResources converts the scraped network metadata into one template per instance type,
// with one device per PCI device.
//
// Device names are synthetic. The driver derives its names from each device's PCI address, which the
// scraped metadata does not carry, and a template name only has to be unique: the template decides
// whether an instance type can satisfy a claim, and the node makes the real allocation under its own
// names.
func buildDynamicResources() map[string]cloudprovider.DynamicResources {
	resources := make(map[string]cloudprovider.DynamicResources, len(drametadata.EFAMetadataByInstanceType))
	for instanceType, metadata := range drametadata.EFAMetadataByInstanceType {
		devices := lo.Map(metadata.Devices, func(device drametadata.DRADevice, index int) cloudprovider.Device {
			return cloudprovider.Device{
				Name:       unique.Make(fmt.Sprintf("efa-%d", index)),
				Attributes: drametadata.Attributes(device.Attributes),
			}
		})
		resources[instanceType] = cloudprovider.DynamicResources{
			ResourceSliceTemplates: []*cloudprovider.ResourceSliceTemplate{{
				Driver:  unique.Make(DriverName),
				Pool:    cloudprovider.ResourcePool{Name: unique.Make(PoolName)},
				Devices: devices,
			}},
		}
	}
	return resources
}

// Provider resolves the dranet DRA metadata for a set of instance types.
type Provider interface {
	// ResolveDynamicResources returns the dra.net templates keyed by instance type name. Instance
	// types with no network device metadata are omitted.
	ResolveDynamicResources(ctx context.Context, instanceTypes []*cloudprovider.InstanceType) (map[string]cloudprovider.DynamicResources, error)
}

type DefaultProvider struct{}

func NewDefaultProvider() *DefaultProvider {
	return &DefaultProvider{}
}

func (p *DefaultProvider) ResolveDynamicResources(_ context.Context, instanceTypes []*cloudprovider.InstanceType) (map[string]cloudprovider.DynamicResources, error) {
	resources := map[string]cloudprovider.DynamicResources{}
	for _, it := range instanceTypes {
		if r, ok := dynamicResources[it.Name]; ok {
			resources[it.Name] = r
		}
	}
	return resources, nil
}
