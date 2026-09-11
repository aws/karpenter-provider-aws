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
	"strings"
	"unique"

	"github.com/samber/lo"
	resourcev1 "k8s.io/api/resource/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

const DriverName = "dra.net"

const PoolName = "network"

const (
	AttributePCIAddress   resourcev1.QualifiedName = "dra.net/pciAddress"
	AttributePCIDevice    resourcev1.QualifiedName = "dra.net/pciDevice"
	AttributePCISubsystem resourcev1.QualifiedName = "dra.net/pciSubsystem"
	AttributePCIVendor    resourcev1.QualifiedName = "dra.net/pciVendor"
	AttributeRDMA         resourcev1.QualifiedName = "dra.net/rdma"
	AttributeNUMANode     resourcev1.QualifiedName = "dra.net/numaNode"
	AttributeType         resourcev1.QualifiedName = "dra.net/type"
	AttributePCIeRoot     resourcev1.QualifiedName = "resource.kubernetes.io/pcieRoot"
)

var dynamicResources = buildDynamicResources()

// DeviceName returns the driver's device name for a PCI address, e.g. "pci-0000-4f-00-0".
func DeviceName(pciAddress string) string {
	return "pci-" + strings.NewReplacer(":", "-", ".", "-").Replace(pciAddress)
}

// buildDynamicResources converts the hard-coded network metadata into one template per instance type,
// with one device per PCI device.
func buildDynamicResources() map[string]cloudprovider.DynamicResources {
	resources := make(map[string]cloudprovider.DynamicResources, len(Networks))
	for instanceType, info := range Networks {
		pciVendor := resourcev1.DeviceAttribute{StringValue: lo.ToPtr(info.PCIVendor)}

		devices := lo.Map(info.Devices, func(device NetworkDevice, _ int) cloudprovider.Device {
			attributes := map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
				AttributePCIAddress:   {StringValue: lo.ToPtr(device.PCIAddress)},
				AttributePCIDevice:    {StringValue: lo.ToPtr(device.PCIDevice)},
				AttributePCISubsystem: {StringValue: lo.ToPtr(device.PCISubsystem)},
				AttributePCIVendor:    pciVendor,
				AttributeRDMA:         {BoolValue: lo.ToPtr(device.RDMA)},
				AttributeNUMANode:     {IntValue: lo.ToPtr(device.NUMANode)},
				AttributePCIeRoot:     {StringValue: lo.ToPtr(device.PCIeRoot)},
			}
			// The driver only publishes type for devices backed by a network interface.
			if device.Type != "" {
				attributes[AttributeType] = resourcev1.DeviceAttribute{StringValue: lo.ToPtr(device.Type)}
			}
			return cloudprovider.Device{
				Name:       unique.Make(DeviceName(device.PCIAddress)),
				Attributes: attributes,
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
