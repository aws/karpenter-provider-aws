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

package efadra_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/providers/drametadata"
	"github.com/aws/karpenter-provider-aws/pkg/providers/efadra"
)

func TestEFADRA(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EFADRA")
}

var _ = Describe("EFA DRA Provider", func() {
	var provider efadra.Provider
	BeforeEach(func() {
		provider = efadra.NewDefaultProvider()
	})
	It("should omit instance types with no network device metadata", func() {
		resources := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "m5.large"},
		})
		Expect(resources).To(BeEmpty())
	})
	It("should build one device per PCI device from the scraped metadata", func() {
		resources := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "g6.48xlarge"},
		})
		Expect(resources["g6.48xlarge"].ResourceSliceTemplates).To(HaveLen(1))

		metadata := drametadata.EFAMetadataByInstanceType["g6.48xlarge"]
		template := resources["g6.48xlarge"].ResourceSliceTemplates[0]
		Expect(template.Driver.Value()).To(Equal(efadra.DriverName))
		Expect(template.Devices).To(HaveLen(len(metadata.Devices)))

		device := template.Devices[0]
		Expect(device.Name.Value()).To(Equal("efa-0"))
		// Every attribute the driver publishes for this device, and no more.
		Expect(device.Attributes).To(HaveLen(len(metadata.Devices[0].Attributes)))
		Expect(*device.Attributes["dra.net/pciDevice"].StringValue).To(Equal("Elastic Fabric Adapter (EFA)"))
		Expect(*device.Attributes["dra.net/pciVendor"].StringValue).To(Equal("Amazon.com, Inc."))
		Expect(*device.Attributes["resource.kubernetes.io/pcieRoot"].StringValue).To(Equal("pci0000:84"))
		// rdma is a bool and numaNode an int, so each has to land in its own typed field for a CEL
		// selector on them to evaluate.
		Expect(*device.Attributes["dra.net/rdma"].BoolValue).To(BeTrue())
		Expect(*device.Attributes["dra.net/numaNode"].IntValue).To(BeEquivalentTo(0))
		// Network devices carry no capacity.
		Expect(device.Capacity).To(BeEmpty())
	})
	It("should advertise every scraped instance type with its full device count", func() {
		instanceTypes := lo.MapToSlice(drametadata.EFAMetadataByInstanceType, func(name string, _ *drametadata.DeviceMetadata) *cloudprovider.InstanceType {
			return &cloudprovider.InstanceType{Name: name}
		})
		resources := provider.ResolveDynamicResources(context.Background(), instanceTypes)
		Expect(resources).To(HaveLen(len(drametadata.EFAMetadataByInstanceType)))

		for name, metadata := range drametadata.EFAMetadataByInstanceType {
			// Count and the device list have to agree, or the template understates the node.
			Expect(metadata.Devices).To(HaveLen(metadata.Count), name)
			Expect(resources[name].ResourceSliceTemplates[0].Devices).To(HaveLen(metadata.Count), name)
			// Names must be unique within a pool, and the index is the only thing keeping them apart.
			// Note every scraped EFA type has exactly one device, so this cannot distinguish naming by
			// index from naming by a constant -- that needs a multi-device type in the metadata.
			for i, device := range resources[name].ResourceSliceTemplates[0].Devices {
				Expect(device.Name.Value()).To(Equal(fmt.Sprintf("efa-%d", i)), name)
			}
			// The driver's runtime-only attributes sit on devices we don't model, and the allocator
			// ignores bindings covering fewer than two devices, so there's nothing to bind.
			Expect(resources[name].AttributeBindings).To(BeEmpty(), name)
		}
	})
})
