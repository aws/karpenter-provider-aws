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
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/nvidiadra"
)

// gatedCtx carries the NVIDIADynamicMIG feature gate, which is what lets Karpenter advertise MIG
// partitions: it states that the cluster's DRA driver runs with its own DynamicMIG gate on.
func gatedCtx() context.Context {
	return options.ToContext(context.Background(), &options.Options{
		FeatureGates: options.FeatureGates{NVIDIADynamicMIG: true},
	})
}

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
	It("should build one device per GPU from the hard-coded metadata", func() {
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "p5.48xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(resources["p5.48xlarge"].ResourceSliceTemplates).To(HaveLen(1))

		template := resources["p5.48xlarge"].ResourceSliceTemplates[0]
		Expect(template.Driver.Value()).To(Equal(nvidiadra.DriverName))
		Expect(template.Devices).To(HaveLen(len(nvidiadra.GPUs["p5.48xlarge"].Devices)))
		for i, device := range template.Devices {
			Expect(device.Name.Value()).To(Equal(fmt.Sprintf("gpu-%d", i)))
			Expect(*device.Attributes[nvidiadra.AttributeProductName].StringValue).To(Equal("NVIDIA H100 80GB HBM3"))
			Expect(*device.Attributes[nvidiadra.AttributeCUDAComputeCapability].VersionValue).To(Equal("9.0.0"))
			Expect(device.Capacity[nvidiadra.CapacityMemory].Value.Equal(resource.MustParse("81559Mi"))).To(BeTrue())
			// Guards against every device aliasing the same table entry.
			expected := nvidiadra.GPUs["p5.48xlarge"].Devices[i]
			Expect(*device.Attributes[nvidiadra.AttributePCIBusID].StringValue).To(Equal(expected.PCIBusID))
			Expect(*device.Attributes[nvidiadra.AttributePCIeRoot].StringValue).To(Equal(expected.PCIeRoot))
			Expect(*device.Attributes[nvidiadra.AttributeNUMANode].IntValue).To(Equal(expected.NUMANode))
		}
		// Every attribute the driver publishes statically, and no more. Verified against a real
		// ResourceSlice captured from a p5.48xlarge running driver 0.5.0.
		Expect(template.Devices[0].Attributes).To(HaveLen(9))
	})
	It("should bind the runtime-only attributes across every GPU", func() {
		resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "p5.48xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())

		bindings := resources["p5.48xlarge"].AttributeBindings
		Expect(lo.Map(bindings, func(b *cloudprovider.AttributeBinding, _ int) resourcev1.QualifiedName {
			return b.Attribute
		})).To(ConsistOf(nvidiadra.AttributeDriverVersion, nvidiadra.AttributeCUDADriverVersion))
		for _, binding := range bindings {
			// A binding covering fewer than two devices is ignored by the allocator.
			Expect(len(binding.Devices)).To(BeNumerically(">=", 2))
			Expect(lo.Map(binding.Devices, func(d cloudprovider.DeviceID, _ int) string {
				return d.Device.Value()
			})).To(ConsistOf("gpu-0", "gpu-1", "gpu-2", "gpu-3", "gpu-4", "gpu-5", "gpu-6", "gpu-7"))
			Expect(binding.Devices[0].Driver.Value()).To(Equal(nvidiadra.DriverName))
		}
	})
	It("should describe every A100 MIG placement without running off the memory slices", func() {
		// Keyed by GPU model, so the layout must line up with the product name in the GPU table.
		layout := nvidiadra.MIGLayouts[nvidiadra.GPUs["p4d.24xlarge"].ProductName]
		Expect(layout).ToNot(BeNil())

		var placements int
		for _, profile := range layout.Profiles {
			Expect(profile.Placements).ToNot(BeEmpty())
			for _, start := range profile.Placements {
				placements++
				// A placement occupies SlicesPerPlacement slices from start, so it has to fit.
				Expect(start + profile.SlicesPerPlacement).To(BeNumerically("<=", layout.CounterSet.MemorySlices))
			}
		}
		// 25 candidate partitions per GPU, matching the live driver's advertisement.
		Expect(placements).To(Equal(25))
	})
	It("should never advertise partitions for a GPU that cannot switch MIG mode without a reset", func() {
		// The A100 has a layout, but an A100 node publishes either whole GPUs or partitions depending on
		// the MIG mode it booted with, and Karpenter cannot know which. Advertising the partitions would
		// send a MIG claim to a node that may have none, so only whole GPUs go out.
		Expect(nvidiadra.MIGLayouts[nvidiadra.GPUs["p4d.24xlarge"].ProductName].ModeTogglable).To(BeFalse())

		resources, err := provider.ResolveDynamicResources(gatedCtx(), []*cloudprovider.InstanceType{
			{Name: "p4d.24xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())
		templates := resources["p4d.24xlarge"].ResourceSliceTemplates
		Expect(templates).To(HaveLen(1))
		Expect(templates[0].SharedCounters).To(BeEmpty())
		Expect(templates[0].Devices).To(HaveLen(8))
		for _, device := range templates[0].Devices {
			Expect(*device.Attributes[nvidiadra.AttributeType].StringValue).To(Equal(nvidiadra.TypeGPU))
			Expect(device.ConsumesCounters).To(BeEmpty())
		}
	})
	It("should advertise no partitions without the NVIDIADynamicMIG feature gate", func() {
		// The gate defaults off, and so does the driver's own DynamicMIG gate. With it off the driver
		// publishes no partitions until a claim asks for one, so advertising them would send MIG claims
		// to nodes that cannot serve them. The safe default is whole GPUs only, even for a GPU model
		// whose layout could otherwise be advertised.
		Expect(nvidiadra.MIGLayouts[nvidiadra.GPUs["p6-b200.48xlarge"].ProductName].ModeTogglable).To(BeTrue())

		for _, ctx := range []context.Context{
			context.Background(), // no options in context at all
			options.ToContext(context.Background(), &options.Options{}), // options present, gate unset
		} {
			resources, err := provider.ResolveDynamicResources(ctx, []*cloudprovider.InstanceType{
				{Name: "p6-b200.48xlarge"},
			})
			Expect(err).ToNot(HaveOccurred())
			templates := resources["p6-b200.48xlarge"].ResourceSliceTemplates
			Expect(templates).To(HaveLen(1))
			Expect(templates[0].SharedCounters).To(BeEmpty())
			Expect(templates[0].Devices).To(HaveLen(8))
			for _, device := range templates[0].Devices {
				Expect(*device.Attributes[nvidiadra.AttributeType].StringValue).To(Equal(nvidiadra.TypeGPU))
				Expect(device.ConsumesCounters).To(BeEmpty())
			}
		}
	})
	It("should report the whole GPU with MIG-derived capacities once partitions are advertised", func() {
		// The driver swaps the whole GPU's capacities on the partitionable path. A live p5.48xlarge with
		// the driver gate on reported memory 81152Mi plus the six engine capacities, where the same GPU
		// with the gate off reported only memory 81559Mi. Advertising the larger figure would let a
		// capacity selector pass the simulation and then fail on the node.
		layout := nvidiadra.MIGLayouts[nvidiadra.GPUs["p5.48xlarge"].ProductName]
		Expect(layout.CounterSet.Memory).ToNot(Equal(nvidiadra.GPUs["p5.48xlarge"].Memory))

		all, err := provider.ResolveDynamicResources(gatedCtx(), []*cloudprovider.InstanceType{
			{Name: "p5.48xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())
		devices := all["p5.48xlarge"].ResourceSliceTemplates[1].Devices
		whole, ok := lo.Find(devices, func(d cloudprovider.Device) bool { return d.Name.Value() == "gpu-0" })
		Expect(ok).To(BeTrue())
		Expect(whole.Capacity).To(HaveLen(7))
		Expect(whole.Capacity[nvidiadra.CapacityMemory].Value.Equal(resource.MustParse(layout.CounterSet.Memory))).To(BeTrue())
		sms := whole.Capacity[nvidiadra.CapacityMultiprocessors].Value
		Expect(sms.Value()).To(BeEquivalentTo(layout.CounterSet.Multiprocessors))

		// With the gate off the same GPU keeps the plain framebuffer size and memory alone.
		off, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
			{Name: "p5.48xlarge"},
		})
		Expect(err).ToNot(HaveOccurred())
		plain := off["p5.48xlarge"].ResourceSliceTemplates[0].Devices[0]
		Expect(plain.Capacity).To(HaveLen(1))
		Expect(plain.Capacity[nvidiadra.CapacityMemory].Value.Equal(resource.MustParse(nvidiadra.GPUs["p5.48xlarge"].Memory))).To(BeTrue())
	})
	Context("with the NVIDIADynamicMIG feature gate on", func() {
		// B200 is Blackwell, so the driver publishes the whole GPUs and every candidate partition
		// whether or not MIG mode is on. Karpenter therefore advertises both, always.
		var resources cloudprovider.DynamicResources
		var counters, devices *cloudprovider.ResourceSliceTemplate
		BeforeEach(func() {
			all, err := provider.ResolveDynamicResources(gatedCtx(), []*cloudprovider.InstanceType{
				{Name: "p6-b200.48xlarge"},
			})
			Expect(err).ToNot(HaveOccurred())
			resources = all["p6-b200.48xlarge"]
			Expect(resources.ResourceSliceTemplates).To(HaveLen(2))
			counters, devices = resources.ResourceSliceTemplates[0], resources.ResourceSliceTemplates[1]
		})
		It("should split counters and devices across two templates on one pool", func() {
			// SharedCounters and Devices are mutually exclusive per template, and devices are silently
			// dropped if both are set, so each template must carry exactly one of the two.
			Expect(counters.SharedCounters).To(HaveLen(8))
			Expect(counters.Devices).To(BeEmpty())
			Expect(devices.SharedCounters).To(BeEmpty())
			// Both templates have to land in the same pool for the counters to apply to the devices.
			Expect(counters.Pool.Name).To(Equal(devices.Pool.Name))
		})
		It("should advertise the whole GPUs alongside every partition", func() {
			byType := lo.CountValuesBy(devices.Devices, func(d cloudprovider.Device) string {
				return *d.Attributes[nvidiadra.AttributeType].StringValue
			})
			// Matches a live p6-b200.48xlarge with MIG mode off: 8 whole GPUs, 8 x 25 placements.
			Expect(byType).To(Equal(map[string]int{"gpu": 8, nvidiadra.TypeMIG: 200}))
		})
		It("should make each whole GPU exclusive with its own partitions", func() {
			byName := lo.SliceToMap(devices.Devices,
				func(d cloudprovider.Device) (string, cloudprovider.Device) { return d.Name.Value(), d })

			// The whole GPU consumes its entire counter set, so allocating it starves every partition on
			// that GPU, and allocating any partition makes it unsatisfiable. This is how the driver does it.
			whole := byName["gpu-0"]
			Expect(whole.ConsumesCounters).To(HaveLen(1))
			Expect(whole.ConsumesCounters[0].CounterSet).To(Equal("gpu-0-counter-set"))
			set := counters.SharedCounters[0]
			Expect(set.Name).To(Equal("gpu-0-counter-set"))
			Expect(whole.ConsumesCounters[0].Counters).To(HaveLen(len(set.Counters)))
			for name, total := range set.Counters {
				Expect(whole.ConsumesCounters[0].Counters[name].Value.Equal(total.Value)).To(BeTrue(), name)
			}

			// A partition draws from its own GPU's counter set, not a neighbour's.
			Expect(byName["gpu-3-mig-1g23gb-19-0"].ConsumesCounters[0].CounterSet).To(Equal("gpu-3-counter-set"))
			// ...and overlapping placements collide on a memory slice while disjoint ones do not.
			Expect(byName["gpu-0-mig-1g23gb-19-0"].ConsumesCounters[0].Counters).To(HaveKey("memory-slice-0"))
			Expect(byName["gpu-0-mig-1g45gb-15-0"].ConsumesCounters[0].Counters).To(HaveKey("memory-slice-0"))
			Expect(byName["gpu-0-mig-1g23gb-19-1"].ConsumesCounters[0].Counters).ToNot(HaveKey("memory-slice-0"))
		})
		It("should bind parentUUID per GPU and the node-wide attributes across everything", func() {
			parent := lo.Filter(resources.AttributeBindings, func(b *cloudprovider.AttributeBinding, _ int) bool {
				return b.Attribute == nvidiadra.AttributeParentUUID
			})
			Expect(parent).To(HaveLen(8))
			for _, binding := range parent {
				// Each binding covers one GPU's 25 placements and no whole GPU: parentUUID differs between
				// GPUs, and a whole GPU has no parent.
				Expect(binding.Devices).To(HaveLen(25))
				gpus := lo.Uniq(lo.Map(binding.Devices, func(d cloudprovider.DeviceID, _ int) string {
					return strings.Split(d.Device.Value(), "-mig-")[0]
				}))
				Expect(gpus).To(HaveLen(1))
			}
			// driverVersion and cudaDriverVersion are node-wide, so they cover whole GPUs and partitions.
			for _, attribute := range []resourcev1.QualifiedName{nvidiadra.AttributeDriverVersion, nvidiadra.AttributeCUDADriverVersion} {
				binding, ok := lo.Find(resources.AttributeBindings, func(b *cloudprovider.AttributeBinding) bool {
					return b.Attribute == attribute
				})
				Expect(ok).To(BeTrue())
				Expect(binding.Devices).To(HaveLen(208))
			}
		})
	})
})
