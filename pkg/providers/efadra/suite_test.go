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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

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
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "m5.large"},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(resources).To(BeEmpty())
	})
	It("should build one device per PCI device from the hard-coded metadata", func() {
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "p5.48xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(resources["p5.48xlarge"].ResourceSliceTemplates).To(HaveLen(1))

		template := resources["p5.48xlarge"].ResourceSliceTemplates[0]
		Expect(template.Driver.Value()).To(Equal(efadra.DriverName))
		Expect(template.Devices).To(HaveLen(len(efadra.Networks["p5.48xlarge"].Devices)))
		for i, device := range template.Devices {
			// Guards against every device aliasing the same table entry.
			expected := efadra.Networks["p5.48xlarge"].Devices[i]
			Expect(device.Name.Value()).To(Equal(efadra.DeviceName(expected.PCIAddress)))
			Expect(*device.Attributes[efadra.AttributePCIAddress].StringValue).To(Equal(expected.PCIAddress))
			Expect(*device.Attributes[efadra.AttributePCIeRoot].StringValue).To(Equal(expected.PCIeRoot))
			Expect(*device.Attributes[efadra.AttributeNUMANode].IntValue).To(Equal(expected.NUMANode))
			Expect(*device.Attributes[efadra.AttributeRDMA].BoolValue).To(Equal(expected.RDMA))
			Expect(*device.Attributes[efadra.AttributePCIVendor].StringValue).To(Equal("Amazon.com, Inc."))
		}
	})
	It("should only publish the type attribute for interface-backed devices", func() {
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "p5.48xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())
		devices := lo.SliceToMap(resources["p5.48xlarge"].ResourceSliceTemplates[0].Devices, func(d cloudprovider.Device) (string, cloudprovider.Device) {
			return d.Name.Value(), d
		})
		Expect(*devices["pci-0000-48-00-0"].Attributes[efadra.AttributeType].StringValue).To(Equal("device"))
		Expect(devices["pci-0000-4f-00-0"].Attributes).ToNot(HaveKey(efadra.AttributeType))
		// Every attribute we publish is one the driver publishes, and no more.
		Expect(devices["pci-0000-4f-00-0"].Attributes).To(HaveLen(7))
		// The driver's runtime-only attributes all sit on the single ENA device, and the allocator
		// ignores bindings covering fewer than two devices, so there's nothing to bind.
		Expect(resources["p5.48xlarge"].AttributeBindings).To(BeEmpty())
	})
})
