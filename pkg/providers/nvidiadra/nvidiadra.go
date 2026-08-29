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
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
)

const DriverName = "gpu.nvidia.com"

const PoolName = "gpus"

const (
	AttributeProductName           resourcev1.QualifiedName = "productName"
	AttributeType                  resourcev1.QualifiedName = "type"
	AttributeBrand                 resourcev1.QualifiedName = "brand"
	AttributeArchitecture          resourcev1.QualifiedName = "architecture"
	AttributeCUDAComputeCapability resourcev1.QualifiedName = "cudaComputeCapability"
	AttributeAddressingMode        resourcev1.QualifiedName = "addressingMode"
	AttributePCIBusID              resourcev1.QualifiedName = "resource.kubernetes.io/pciBusID"
	AttributePCIeRoot              resourcev1.QualifiedName = "resource.kubernetes.io/pcieRoot"
	AttributeNUMANode              resourcev1.QualifiedName = "resource.kubernetes.io/numaNode"
	CapacityMemory                 resourcev1.QualifiedName = "memory"
	AttributeDriverVersion         resourcev1.QualifiedName = "driverVersion"
	AttributeCUDADriverVersion     resourcev1.QualifiedName = "cudaDriverVersion"
)

// MIG partition attributes and capacities
const (
	AttributeProfile    resourcev1.QualifiedName = "profile"
	AttributeParentUUID resourcev1.QualifiedName = "parentUUID"

	CapacityMultiprocessors resourcev1.QualifiedName = "multiprocessors"
	CapacityCopyEngines     resourcev1.QualifiedName = "copyEngines"
	CapacityDecoders        resourcev1.QualifiedName = "decoders"
	CapacityEncoders        resourcev1.QualifiedName = "encoders"
	CapacityJPEGEngines     resourcev1.QualifiedName = "jpegEngines"
	CapacityOFAEngines      resourcev1.QualifiedName = "ofaEngines"
	TypeMIG                                          = "mig"
	TypeGPU                                          = "gpu"
)
const (
	counterMemory            = "memory"
	counterMultiprocessors   = "multiprocessors"
	counterCopyEngines       = "copy-engines"
	counterDecoders          = "decoders"
	counterEncoders          = "encoders"
	counterJPEGEngines       = "jpeg-engines"
	counterOFAEngines        = "ofa-engines"
	counterMemorySlicePrefix = "memory-slice-"
)

var boundAttributes = []resourcev1.QualifiedName{AttributeDriverVersion, AttributeCUDADriverVersion}

// dynamicResources holds the whole-GPU DRA metadata, keyed by instance type name.
// partitionedDynamicResources adds the MIG partitions for the GPU models that can advertise them.
var (
	dynamicResources            = buildDynamicResources(false)
	partitionedDynamicResources = buildDynamicResources(true)
)

// buildDynamicResources converts the hard-coded GPU metadata into the templates for each instance
// type: one device per physical GPU named to match the driver's "gpu-<index>" convention, plus the
// AttributeBindings for the runtime-only attributes those devices share.
//
// With withPartitions set, every candidate MIG partition is added alongside the whole GPUs for the
// GPU models whose layout can be advertised (see MIGLayout.ModeTogglable), matching what the driver
// publishes for such a GPU whether or not MIG mode is currently on.
//
// It panics on a malformed memory quantity, failing fast at startup on a bad table entry.
func buildDynamicResources(withPartitions bool) map[string]cloudprovider.DynamicResources {
	resources := make(map[string]cloudprovider.DynamicResources, len(GPUs))
	for instanceType, info := range GPUs {
		memory := resource.MustParse(info.Memory)
		// Only layouts the driver advertises regardless of MIG mode can be published ahead of the node.
		layout, partitioned := MIGLayouts[info.ProductName]
		partitioned = withPartitions && partitioned && layout.ModeTogglable

		productName := resourcev1.DeviceAttribute{StringValue: lo.ToPtr(info.ProductName)}
		deviceType := resourcev1.DeviceAttribute{StringValue: lo.ToPtr(info.Type)}
		brand := resourcev1.DeviceAttribute{StringValue: lo.ToPtr(info.Brand)}
		architecture := resourcev1.DeviceAttribute{StringValue: lo.ToPtr(info.Architecture)}
		cudaComputeCapability := resourcev1.DeviceAttribute{VersionValue: lo.ToPtr(info.CUDAComputeCapability)}
		addressingMode := resourcev1.DeviceAttribute{StringValue: lo.ToPtr(info.AddressingMode)}

		devices := lo.Map(info.Devices, func(device GPUDevice, index int) cloudprovider.Device {
			return cloudprovider.Device{
				Name: unique.Make(fmt.Sprintf("gpu-%d", index)),
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					AttributeProductName:           productName,
					AttributeType:                  deviceType,
					AttributeBrand:                 brand,
					AttributeArchitecture:          architecture,
					AttributeCUDAComputeCapability: cudaComputeCapability,
					AttributeAddressingMode:        addressingMode,
					AttributePCIBusID:              {StringValue: lo.ToPtr(device.PCIBusID)},
					AttributePCIeRoot:              {StringValue: lo.ToPtr(device.PCIeRoot)},
					AttributeNUMANode:              {IntValue: lo.ToPtr(device.NUMANode)},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					CapacityMemory: {Value: memory},
				},
			}
		})
		driver := unique.Make(DriverName)
		pool := cloudprovider.ResourcePool{Name: unique.Make(PoolName)}
		deviceID := func(device cloudprovider.Device, _ int) cloudprovider.DeviceID {
			return cloudprovider.DeviceID{Driver: driver, Pool: pool.Name, Device: device.Name}
		}

		var counterSets []resourcev1.CounterSet
		var bindings []*cloudprovider.AttributeBinding
		if partitioned {
			counterSets = make([]resourcev1.CounterSet, 0, len(info.Devices))
			model := migModelAttributes(info)
			for index, gpu := range info.Devices {
				counterSet := migCounterSetName(index)
				counters := migCounters(layout.CounterSet)
				counterSets = append(counterSets, resourcev1.CounterSet{Name: counterSet, Counters: counters})
				// The whole GPU consumes its entire counter set, so it and any partition on the same GPU
				// can never both be allocated. The driver models it the same way.
				devices[index].ConsumesCounters = []resourcev1.DeviceCounterConsumption{{
					CounterSet: counterSet,
					Counters:   counters,
				}}
				// The driver reports the whole GPU with MIG capacities which is observed to be a bit less.
				devices[index].Capacity = migCapacity(MIGProfile{
					Memory:          layout.CounterSet.Memory,
					Multiprocessors: layout.CounterSet.Multiprocessors,
					CopyEngines:     layout.CounterSet.CopyEngines,
					Decoders:        layout.CounterSet.Decoders,
					Encoders:        layout.CounterSet.Encoders,
					JPEGEngines:     layout.CounterSet.JPEGEngines,
					OFAEngines:      layout.CounterSet.OFAEngines,
				})
				partitions := migPartitions(layout, index, gpu, model, counterSet)

				bindings = append(bindings, &cloudprovider.AttributeBinding{
					Attribute: AttributeParentUUID,
					Devices:   lo.Map(partitions, deviceID),
				})
				devices = append(devices, partitions...)
			}
		}

		// SharedCounters and Devices are mutually exclusive within a single template
		// so the counters get their own template on the same pool.
		templates := make([]*cloudprovider.ResourceSliceTemplate, 0, 2)
		if len(counterSets) > 0 {
			templates = append(templates, &cloudprovider.ResourceSliceTemplate{Driver: driver, Pool: pool, SharedCounters: counterSets})
		}
		templates = append(templates, &cloudprovider.ResourceSliceTemplate{Driver: driver, Pool: pool, Devices: devices})

		// The bound attributes are node-wide, so they cover the whole GPUs and every partition alike.
		deviceIDs := lo.Map(devices, deviceID)
		bindings = append(bindings, lo.Map(boundAttributes, func(attribute resourcev1.QualifiedName, _ int) *cloudprovider.AttributeBinding {
			return &cloudprovider.AttributeBinding{Attribute: attribute, Devices: deviceIDs}
		})...)

		resources[instanceType] = cloudprovider.DynamicResources{
			ResourceSliceTemplates: templates,
			AttributeBindings:      bindings,
		}
	}
	return resources
}

// migModelAttributes are the attributes every partition on an instance type shares, built once so
// all of them can share the pointers.
func migModelAttributes(info *GPUInfo) map[resourcev1.QualifiedName]resourcev1.DeviceAttribute {
	return map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
		AttributeProductName:           {StringValue: lo.ToPtr(info.ProductName)},
		AttributeType:                  {StringValue: lo.ToPtr(TypeMIG)},
		AttributeBrand:                 {StringValue: lo.ToPtr(info.Brand)},
		AttributeArchitecture:          {StringValue: lo.ToPtr(info.Architecture)},
		AttributeCUDAComputeCapability: {VersionValue: lo.ToPtr(info.CUDAComputeCapability)},
		AttributeAddressingMode:        {StringValue: lo.ToPtr(info.AddressingMode)},
	}
}

func migPartitions(
	layout *MIGLayout,
	index int,
	gpu GPUDevice,
	model map[resourcev1.QualifiedName]resourcev1.DeviceAttribute,
	counterSet string,
) []cloudprovider.Device {
	// Topology is the parent GPU's, shared by all of its partitions.
	topology := map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
		AttributePCIBusID: {StringValue: lo.ToPtr(gpu.PCIBusID)},
		AttributePCIeRoot: {StringValue: lo.ToPtr(gpu.PCIeRoot)},
		AttributeNUMANode: {IntValue: lo.ToPtr(gpu.NUMANode)},
	}
	var devices []cloudprovider.Device
	for _, profile := range layout.Profiles {
		profileAttribute := resourcev1.DeviceAttribute{StringValue: lo.ToPtr(profile.Profile)}
		capacity := migCapacity(profile)
		for _, placement := range profile.Placements {
			attributes := make(map[resourcev1.QualifiedName]resourcev1.DeviceAttribute, len(model)+len(topology)+1)
			for name, value := range model {
				attributes[name] = value
			}
			for name, value := range topology {
				attributes[name] = value
			}
			attributes[AttributeProfile] = profileAttribute
			devices = append(devices, cloudprovider.Device{
				Name:       unique.Make(fmt.Sprintf("gpu-%d-%s-%d", index, profile.NamePrefix, placement)),
				Attributes: attributes,
				Capacity:   capacity,
				ConsumesCounters: []resourcev1.DeviceCounterConsumption{{
					CounterSet: counterSet,
					Counters:   migConsumption(profile, placement),
				}},
			})
		}
	}
	return devices
}

// migCounters is the whole-GPU budget that its partitions draw down.
func migCounters(set MIGCounterSet) map[string]resourcev1.Counter {
	counters := map[string]resourcev1.Counter{
		counterMemory:          {Value: resource.MustParse(set.Memory)},
		counterMultiprocessors: counterOf(set.Multiprocessors),
		counterCopyEngines:     counterOf(set.CopyEngines),
		counterDecoders:        counterOf(set.Decoders),
		counterEncoders:        counterOf(set.Encoders),
		counterJPEGEngines:     counterOf(set.JPEGEngines),
		counterOFAEngines:      counterOf(set.OFAEngines),
	}
	// One counter per memory slice, each worth 1. Consuming them is what makes overlapping
	// placements mutually exclusive.
	for slice := int64(0); slice < set.MemorySlices; slice++ {
		counters[migMemorySliceName(slice)] = counterOf(1)
	}
	return counters
}

// migConsumption is what one placement takes from its GPU's counter set.
func migConsumption(profile MIGProfile, placement int64) map[string]resourcev1.Counter {
	counters := map[string]resourcev1.Counter{
		counterMemory:          {Value: resource.MustParse(profile.Memory)},
		counterMultiprocessors: counterOf(profile.Multiprocessors),
		counterCopyEngines:     counterOf(profile.CopyEngines),
		counterDecoders:        counterOf(profile.Decoders),
		counterEncoders:        counterOf(profile.Encoders),
		counterJPEGEngines:     counterOf(profile.JPEGEngines),
		counterOFAEngines:      counterOf(profile.OFAEngines),
	}
	for offset := int64(0); offset < profile.SlicesPerPlacement; offset++ {
		counters[migMemorySliceName(placement+offset)] = counterOf(1)
	}
	return counters
}

func migCapacity(profile MIGProfile) map[resourcev1.QualifiedName]resourcev1.DeviceCapacity {
	return map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
		CapacityMemory:          {Value: resource.MustParse(profile.Memory)},
		CapacityMultiprocessors: capacityOf(profile.Multiprocessors),
		CapacityCopyEngines:     capacityOf(profile.CopyEngines),
		CapacityDecoders:        capacityOf(profile.Decoders),
		CapacityEncoders:        capacityOf(profile.Encoders),
		CapacityJPEGEngines:     capacityOf(profile.JPEGEngines),
		CapacityOFAEngines:      capacityOf(profile.OFAEngines),
	}
}

func migCounterSetName(index int) string {
	return fmt.Sprintf("gpu-%d-counter-set", index)
}

func migMemorySliceName(slice int64) string {
	return fmt.Sprintf("%s%d", counterMemorySlicePrefix, slice)
}

func counterOf(value int64) resourcev1.Counter {
	return resourcev1.Counter{Value: *resource.NewQuantity(value, resource.DecimalSI)}
}

func capacityOf(value int64) resourcev1.DeviceCapacity {
	return resourcev1.DeviceCapacity{Value: *resource.NewQuantity(value, resource.DecimalSI)}
}

type Provider interface {
	// ResolveDynamicResources returns the NVIDIA GPU templates and attribute bindings keyed by
	// instance type name.
	ResolveDynamicResources(ctx context.Context, instanceTypes []*cloudprovider.InstanceType) (map[string]cloudprovider.DynamicResources, error)
}

type DefaultProvider struct{}

func NewDefaultProvider() *DefaultProvider {
	return &DefaultProvider{}
}

func (p *DefaultProvider) ResolveDynamicResources(ctx context.Context, instanceTypes []*cloudprovider.InstanceType) (map[string]cloudprovider.DynamicResources, error) {
	table := dynamicResources
	if opts := options.FromContext(ctx); opts != nil && opts.FeatureGates.NVIDIADynamicMIG {
		table = partitionedDynamicResources
	}
	resources := map[string]cloudprovider.DynamicResources{}
	for _, it := range instanceTypes {
		if r, ok := table[it.Name]; ok {
			resources[it.Name] = r
		}
	}
	return resources, nil
}
