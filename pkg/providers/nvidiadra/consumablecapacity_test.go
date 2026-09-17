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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/providers/drametadata"
	"github.com/aws/karpenter-provider-aws/pkg/providers/nvidiadra"
)

// The L4 on a g6 has 23034Mi of memory, which is the ceiling every memory request policy is bounded
// by. Captured from a live g6.16xlarge running driver 0.5.0.
const l4Memory = "23034Mi"

// devicesFor resolves one instance type and returns its GPU devices under the given sharing mode.
func devicesFor(mode *nvidiadra.ConsumableCapacityMode) []cloudprovider.Device {
	provider := nvidiadra.NewDefaultProvider()
	resources, err := provider.ResolveDynamicResources(context.Background(), []*cloudprovider.InstanceType{
		{Name: "g6.12xlarge"},
	}, mode)
	Expect(err).ToNot(HaveOccurred())
	Expect(resources["g6.12xlarge"].ResourceSliceTemplates).To(HaveLen(1))
	return resources["g6.12xlarge"].ResourceSliceTemplates[0].Devices
}

func expectQuantity(actual *resource.Quantity, want string) {
	GinkgoHelper()
	Expect(actual).ToNot(BeNil())
	Expect(actual.Equal(resource.MustParse(want))).To(BeTrue(), "got %s, want %s", actual, want)
}

var _ = Describe("Consumable capacity", func() {
	Describe("parsing the annotation", func() {
		It("should treat an absent or disabled value as not shareable", func() {
			for _, value := range []string{"", "disabled"} {
				mode, err := nvidiadra.ParseConsumableCapacity(value)
				Expect(err).ToNot(HaveOccurred())
				Expect(mode).To(BeNil(), value)
			}
		})
		It("should distinguish memory from unlimited by their default request", func() {
			memory, err := nvidiadra.ParseConsumableCapacity("memory")
			Expect(err).ToNot(HaveOccurred())
			Expect(memory.Memory).To(BeTrue())
			// "memory" defaults an absent request to the whole GPU; "unlimited" defaults it to zero.
			Expect(memory.FullMemoryDefault).To(BeTrue())
			Expect(memory.Shares).To(BeZero())

			unlimited, err := nvidiadra.ParseConsumableCapacity("unlimited")
			Expect(err).ToNot(HaveOccurred())
			Expect(unlimited.Memory).To(BeTrue())
			Expect(unlimited.FullMemoryDefault).To(BeFalse())
		})
		It("should read a positive integer as a share count", func() {
			mode, err := nvidiadra.ParseConsumableCapacity("4")
			Expect(err).ToNot(HaveOccurred())
			Expect(mode.Shares).To(BeEquivalentTo(4))
			Expect(mode.Memory).To(BeFalse())
		})
		It("should reject values the driver would not accept", func() {
			for _, value := range []string{"0", "-1", "shares", "4.5", "memory=true", "1Gi"} {
				_, err := nvidiadra.ParseConsumableCapacity(value)
				Expect(err).To(HaveOccurred(), value)
			}
		})
	})

	Describe("the templates it produces", func() {
		It("should mark no device shareable without a mode", func() {
			for _, device := range devicesFor(nil) {
				Expect(device.AllowMultipleAllocations).To(BeFalse())
				// Memory keeps a bare value, exactly as the driver publishes it with sharing disabled.
				Expect(device.Capacity).To(HaveLen(1))
				Expect(device.Capacity[nvidiadra.CapacityMemory].RequestPolicy).To(BeNil())
				Expect(device.Capacity).ToNot(HaveKey(nvidiadra.CapacityShares))
			}
		})
		It("should publish shares alongside a zero-defaulted memory for a share count", func() {
			mode := lo.Must(nvidiadra.ParseConsumableCapacity("4"))
			devices := devicesFor(mode)
			Expect(devices).To(HaveLen(4))
			for _, device := range devices {
				Expect(device.AllowMultipleAllocations).To(BeTrue())

				shares := device.Capacity[nvidiadra.CapacityShares]
				expectQuantity(lo.ToPtr(shares.Value), "4")
				expectQuantity(shares.RequestPolicy.Default, "1")
				expectQuantity(shares.RequestPolicy.ValidRange.Min, "1")
				expectQuantity(shares.RequestPolicy.ValidRange.Max, "4")
				expectQuantity(shares.RequestPolicy.ValidRange.Step, "1")

				// Memory still gets a policy in this mode, but defaults to consuming none, so a claim that
				// asks only for shares does not exhaust the GPU's memory budget.
				memory := device.Capacity[nvidiadra.CapacityMemory]
				expectQuantity(lo.ToPtr(memory.Value), l4Memory)
				expectQuantity(memory.RequestPolicy.Default, "0")
				expectQuantity(memory.RequestPolicy.ValidRange.Min, "0")
				expectQuantity(memory.RequestPolicy.ValidRange.Max, l4Memory)
				expectQuantity(memory.RequestPolicy.ValidRange.Step, "1Mi")
			}
		})
		It("should default a memory-mode request to the whole GPU", func() {
			mode := lo.Must(nvidiadra.ParseConsumableCapacity("memory"))
			for _, device := range devicesFor(mode) {
				Expect(device.AllowMultipleAllocations).To(BeTrue())
				// No shares dimension in this mode.
				Expect(device.Capacity).ToNot(HaveKey(nvidiadra.CapacityShares))

				memory := device.Capacity[nvidiadra.CapacityMemory]
				expectQuantity(memory.RequestPolicy.Default, l4Memory)
				expectQuantity(memory.RequestPolicy.ValidRange.Min, "1Mi")
				expectQuantity(memory.RequestPolicy.ValidRange.Max, l4Memory)
				expectQuantity(memory.RequestPolicy.ValidRange.Step, "1Mi")
			}
		})
		It("should default an unlimited-mode request to nothing", func() {
			mode := lo.Must(nvidiadra.ParseConsumableCapacity("unlimited"))
			for _, device := range devicesFor(mode) {
				Expect(device.AllowMultipleAllocations).To(BeTrue())
				Expect(device.Capacity).ToNot(HaveKey(nvidiadra.CapacityShares))

				memory := device.Capacity[nvidiadra.CapacityMemory]
				expectQuantity(memory.RequestPolicy.Default, "0")
				expectQuantity(memory.RequestPolicy.ValidRange.Min, "0")
				expectQuantity(memory.RequestPolicy.ValidRange.Max, l4Memory)
			}
		})
		It("should not mutate the shared base tables", func() {
			// The generated capacity maps are handed out as-is when no mode is set, so a sharing mode has to
			// build its own rather than write through. Check the table itself, then a plain resolve.
			_ = devicesFor(lo.Must(nvidiadra.ParseConsumableCapacity("4")))
			for _, device := range drametadata.GPUMetadataByInstanceType["g6.12xlarge"].Devices {
				Expect(device.Capacity).To(HaveLen(1))
				Expect(device.Capacity).ToNot(HaveKey(nvidiadra.CapacityShares))
				Expect(device.Capacity[nvidiadra.CapacityMemory].RequestPolicy).To(BeNil())
			}
			for _, device := range devicesFor(nil) {
				Expect(device.AllowMultipleAllocations).To(BeFalse())
				Expect(device.Capacity).To(HaveLen(1))
				Expect(device.Capacity[nvidiadra.CapacityMemory].RequestPolicy).To(BeNil())
			}
		})
		It("should carry the attribute bindings through unchanged", func() {
			provider := nvidiadra.NewDefaultProvider()
			its := []*cloudprovider.InstanceType{{Name: "g6.12xlarge"}}
			plain, err := provider.ResolveDynamicResources(context.Background(), its, nil)
			Expect(err).ToNot(HaveOccurred())
			shared, err := provider.ResolveDynamicResources(context.Background(), its, lo.Must(nvidiadra.ParseConsumableCapacity("2")))
			Expect(err).ToNot(HaveOccurred())

			names := func(r cloudprovider.DynamicResources) []resourcev1.QualifiedName {
				return lo.Map(r.AttributeBindings, func(b *cloudprovider.AttributeBinding, _ int) resourcev1.QualifiedName {
					return b.Attribute
				})
			}
			Expect(names(shared["g6.12xlarge"])).To(ConsistOf(names(plain["g6.12xlarge"])))
			for _, binding := range shared["g6.12xlarge"].AttributeBindings {
				Expect(binding.Devices).To(HaveLen(4))
			}
		})
	})
})
