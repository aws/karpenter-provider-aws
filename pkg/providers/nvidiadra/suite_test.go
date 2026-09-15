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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

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
	It("should omit instance types with no GPU metadata", func() {
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "m5.large"},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(resources).To(BeEmpty())
	})
	It("should build one device per GPU from the scraped metadata", func() {
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "g6.12xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(resources["g6.12xlarge"].ResourceSliceTemplates).To(HaveLen(1))

		metadata := drametadata.GPUMetadataByInstanceType["g6.12xlarge"]
		template := resources["g6.12xlarge"].ResourceSliceTemplates[0]
		Expect(template.Driver.Value()).To(Equal(nvidiadra.DriverName))
		Expect(template.Devices).To(HaveLen(len(metadata.Devices)))
		for i, device := range template.Devices {
			Expect(device.Name.Value()).To(Equal(fmt.Sprintf("gpu-%d", i)))
			// Guards against every device aliasing the same table entry: pcieRoot is the one attribute
			// that differs between the GPUs on an instance type.
			expected := metadata.Devices[i]
			Expect(device.Attributes).To(HaveLen(len(expected.Attributes)))
			Expect(*device.Attributes["resource.kubernetes.io/pcieRoot"].StringValue).
				To(Equal(*expected.Attributes["resource.kubernetes.io/pcieRoot"].String))
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
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "g6.12xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())

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
	It("should advertise every scraped instance type with its full device count", func() {
		instanceTypes := lo.MapToSlice(drametadata.GPUMetadataByInstanceType, func(name string, _ *drametadata.DeviceMetadata) *cloudprovider.InstanceType {
			return &cloudprovider.InstanceType{Name: name}
		})
		resources, err := provider.ResolveDynamicResources(context.Background(), instanceTypes)
		Expect(err).ToNot(HaveOccurred())
		Expect(resources).To(HaveLen(len(drametadata.GPUMetadataByInstanceType)))

		for name, metadata := range drametadata.GPUMetadataByInstanceType {
			// Count and the device list have to agree, or the template understates the node.
			Expect(metadata.Devices).To(HaveLen(metadata.Count), name)
			Expect(resources[name].ResourceSliceTemplates[0].Devices).To(HaveLen(metadata.Count), name)
			// Device names must be unique within a pool.
			names := lo.Map(resources[name].ResourceSliceTemplates[0].Devices, func(d cloudprovider.Device, _ int) string {
				return d.Name.Value()
			})
			Expect(lo.Uniq(names)).To(HaveLen(len(names)), name)
		}
	})
	It("should publish no counter sets, since MIG partitions are out of scope", func() {
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "g6.48xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())
		templates := resources["g6.48xlarge"].ResourceSliceTemplates
		Expect(templates).To(HaveLen(1))
		Expect(templates[0].SharedCounters).To(BeEmpty())
		for _, device := range templates[0].Devices {
			Expect(device.ConsumesCounters).To(BeEmpty())
			Expect(*device.Attributes["type"].StringValue).To(Equal("gpu"))
		}
	})
})
