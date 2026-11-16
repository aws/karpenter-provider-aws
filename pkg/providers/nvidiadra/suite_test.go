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

package nvidiadra_test

import (
	"context"
	"fmt"
	"testing"
	"unique"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling/dynamicresources"

	"github.com/aws/karpenter-provider-aws/pkg/providers/drametadata"
	"github.com/aws/karpenter-provider-aws/pkg/providers/nvidiadra"
)

func TestNVIDIADRA(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NVIDIADRA")
}

var _ = Describe("NVIDIA DRA Provider", func() {
	var provider nvidiadra.Provider
	BeforeEach(func() {
		provider = nvidiadra.NewDefaultProvider()
	})
	Describe("DynamicResourcesFor", func() {
		It("should return false for an instance type with no NVIDIA GPU metadata", func() {
			_, ok := nvidiadra.DynamicResourcesFor("m5.large", nil)
			Expect(ok).To(BeFalse())
		})
		It("should return the same unscoped set and bindings the provider resolves", func() {
			mode := lo.Must(nvidiadra.ParseConsumableCapacity("4"))
			for name := range drametadata.GPUMetadataByInstanceType {
				for _, m := range []*nvidiadra.ConsumableCapacityMode{nil, mode} {
					r, ok := nvidiadra.DynamicResourcesFor(name, m)
					Expect(ok).To(BeTrue(), name)
					Expect(r.Set.Requirements).To(BeEmpty(), name)
					resolved := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{{Name: name}}, m)[name]
					Expect(r).To(Equal(resolved), name)
				}
			}
		})
		It("should leave GPUs single-allocation for a nil mode and shareable otherwise", func() {
			plain, _ := nvidiadra.DynamicResourcesFor("g6.12xlarge", nil)
			shared, _ := nvidiadra.DynamicResourcesFor("g6.12xlarge", lo.Must(nvidiadra.ParseConsumableCapacity("4")))
			Expect(plain.Set.ResourceSliceTemplates[0].Devices[0].AllowMultipleAllocations).To(BeFalse())
			Expect(shared.Set.ResourceSliceTemplates[0].Devices[0].AllowMultipleAllocations).To(BeTrue())
			Expect(shared.AttributeBindings).To(Equal(plain.AttributeBindings))
		})
	})
	It("should omit instance types with no GPU metadata", func() {
		resources := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "m5.large"},
		}, nil)
		Expect(resources).To(BeEmpty())
	})
	It("should build one device per GPU from the scraped metadata", func() {
		resources := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "g6.12xlarge"},
		}, nil)
		Expect(resources["g6.12xlarge"].Set.ResourceSliceTemplates).To(HaveLen(1))

		metadata := drametadata.GPUMetadataByInstanceType["g6.12xlarge"]
		template := resources["g6.12xlarge"].Set.ResourceSliceTemplates[0]
		Expect(template.Driver.Value()).To(Equal(nvidiadra.DriverName))
		Expect(template.Devices).To(HaveLen(len(metadata.Devices)))
		for i, device := range template.Devices {
			Expect(device.Name.Value()).To(Equal(fmt.Sprintf("gpu-%d", i)))
			// Guards against every device aliasing the same table entry: pcieRoot is the one attribute
			// that differs between the GPUs on an instance type.
			expected := metadata.Devices[i]
			Expect(device.Attributes).To(HaveLen(len(expected.Attributes)))
			Expect(*device.Attributes["resource.kubernetes.io/pcieRoot"].StringValue).
				To(Equal(*expected.Attributes["resource.kubernetes.io/pcieRoot"].StringValue))
			Expect(*device.Attributes["productName"].StringValue).To(Equal("NVIDIA L4"))
			Expect(*device.Attributes["type"].StringValue).To(Equal("gpu"))
			// cudaComputeCapability is a version, not a string, so it has to land in VersionValue for a
			// semver CEL selector to match it.
			Expect(*device.Attributes["cudaComputeCapability"].VersionValue).To(Equal("8.9.0"))
			Expect(device.Attributes["cudaComputeCapability"].StringValue).To(BeNil())
			Expect(device.Capacity[resourcev1.QualifiedName("memory")].Value.Equal(resource.MustParse("23034Mi"))).To(BeTrue())
		}
	})
	It("should bind the runtime-only attributes across every GPU", func() {
		resources := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "g6.12xlarge"},
		}, nil)

		bindings := resources["g6.12xlarge"].AttributeBindings
		Expect(lo.Map(bindings, func(b *cloudprovider.AttributeBinding, _ int) resourcev1.QualifiedName {
			return b.Attribute
		})).To(ConsistOf(nvidiadra.AttributeDriverVersion, nvidiadra.AttributeCUDADriverVersion))
		for _, binding := range bindings {
			// A binding covering fewer than two devices is ignored by the allocator.
			Expect(len(binding.Devices)).To(BeNumerically(">=", 2))
			Expect(lo.Map(binding.Devices, func(d cloudprovider.DeviceID, _ int) string {
				return d.Device.Value()
			})).To(ConsistOf("gpu-0", "gpu-1", "gpu-2", "gpu-3"))
			Expect(binding.Devices[0].Driver.Value()).To(Equal(nvidiadra.DriverName))
		}
	})
	It("should bind under the fully qualified names a claim's matchAttribute uses", func() {
		// Run the bindings through the allocator's own lookup, keyed by the literal names a ResourceClaim would
		// carry, so a binding the allocator can't match fails here rather than leaving claims unschedulable.
		it := &cloudprovider.InstanceType{Name: "g6.12xlarge"}
		resources := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{it}, nil)["g6.12xlarge"]
		it.DynamicResources = []*cloudprovider.DynamicResources{resources.Set}
		it.AttributeBindings = resources.AttributeBindings
		bindings := dynamicresources.BuildAttributeBindings(map[string][]*cloudprovider.InstanceType{"default": {it}})

		// Pool and device names are internal placeholders that core may rewrite, so take the device IDs from the
		// templates core resolved instead of building them by hand.
		tmpl := it.AllDynamicResources()[0].ResourceSliceTemplates[0]
		Expect(len(tmpl.Devices)).To(BeNumerically(">=", 2))
		deviceIDs := lo.Map(tmpl.Devices, func(d cloudprovider.Device, _ int) dynamicresources.DeviceID {
			return dynamicresources.DeviceID{DeviceID: cloudprovider.DeviceID{Driver: tmpl.Driver, Pool: tmpl.Pool.Name, Device: d.Name}}
		})
		for _, attribute := range []resourcev1.QualifiedName{"gpu.nvidia.com/driverVersion", "gpu.nvidia.com/cudaDriverVersion"} {
			for _, deviceID := range deviceIDs {
				Expect(bindings.HasBindings("default", unique.Make(it.Name), attribute, deviceID)).To(BeTrue(), string(attribute))
				Expect(bindings.Bound("default", unique.Make(it.Name), attribute, deviceIDs[0], deviceID)).To(BeTrue(), string(attribute))
			}
		}
	})
	It("should advertise every scraped instance type with its full device count", func() {
		instanceTypes := lo.MapToSlice(drametadata.GPUMetadataByInstanceType, func(name string, _ *drametadata.DeviceMetadata) *cloudprovider.InstanceType {
			return &cloudprovider.InstanceType{Name: name}
		})
		resources := provider.ResolveDynamicResources(context.Background(), instanceTypes, nil)
		Expect(resources).To(HaveLen(len(drametadata.GPUMetadataByInstanceType)))

		for name, metadata := range drametadata.GPUMetadataByInstanceType {
			// Count and the device list have to agree, or the template understates the node.
			Expect(metadata.Devices).To(HaveLen(metadata.Count), name)
			Expect(resources[name].Set.ResourceSliceTemplates[0].Devices).To(HaveLen(metadata.Count), name)
			// Device names must be unique within a pool.
			names := lo.Map(resources[name].Set.ResourceSliceTemplates[0].Devices, func(d cloudprovider.Device, _ int) string {
				return d.Name.Value()
			})
			Expect(lo.Uniq(names)).To(HaveLen(len(names)), name)
		}
	})
	It("should publish no counter sets, since MIG partitions are out of scope", func() {
		resources := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "g6.48xlarge"},
		}, nil)
		templates := resources["g6.48xlarge"].Set.ResourceSliceTemplates
		Expect(templates).To(HaveLen(1))
		Expect(templates[0].SharedCounters).To(BeEmpty())
		for _, device := range templates[0].Devices {
			Expect(device.ConsumesCounters).To(BeEmpty())
			Expect(*device.Attributes["type"].StringValue).To(Equal("gpu"))
		}
	})
})
