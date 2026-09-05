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

package bootstrap

import (
	"context"
	"testing"

	"github.com/samber/lo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrap")
}

var _ = Describe("Bottlerocket", func() {
	Describe("EnableDefaultMountPaths", func() {
		It("should use the configured flag value", func() {
			bottlerocket := Bottlerocket{EnableDefaultMountPaths: true}
			Expect(bottlerocket.EnableDefaultMountPaths).To(BeTrue())

			bottlerocket = Bottlerocket{EnableDefaultMountPaths: false}
			Expect(bottlerocket.EnableDefaultMountPaths).To(BeFalse())
		})
	})

	Describe("MemoryManagerReservedMemory", func() {
		It("should unmarshal memory-manager-reserved-memory from TOML", func() {
			userData := `
[settings.kubernetes]
"memory-manager-policy" = "Static"

[settings.kubernetes.memory-manager-reserved-memory.0]
enabled = true
memory = "727Mi"
`
			config, err := NewBottlerocketConfig(context.Background(), &userData)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Settings.Kubernetes.MemoryManagerPolicy).ToNot(BeNil())
			Expect(*config.Settings.Kubernetes.MemoryManagerPolicy).To(Equal("Static"))
			Expect(config.Settings.Kubernetes.MemoryManagerReservedMemory).To(HaveKey("0"))
			Expect(config.Settings.Kubernetes.MemoryManagerReservedMemory["0"].Enabled).To(BeTrue())
			Expect(config.Settings.Kubernetes.MemoryManagerReservedMemory["0"].Memory).To(Equal("727Mi"))
		})

		It("should unmarshal hugepages fields", func() {
			userData := `
[settings.kubernetes]
"memory-manager-policy" = "Static"

[settings.kubernetes.memory-manager-reserved-memory.0]
enabled = true
memory = "727Mi"
hugepages-2Mi = "64Mi"
hugepages-1Gi = "2Gi"
`
			config, err := NewBottlerocketConfig(context.Background(), &userData)
			Expect(err).ToNot(HaveOccurred())
			entry := config.Settings.Kubernetes.MemoryManagerReservedMemory["0"]
			Expect(entry.Enabled).To(BeTrue())
			Expect(entry.Memory).To(Equal("727Mi"))
			Expect(entry.HugePages2Mi).To(Equal("64Mi"))
			Expect(entry.HugePages1Gi).To(Equal("2Gi"))
		})

		It("should round-trip through marshal/unmarshal", func() {
			userData := `
[settings.kubernetes]
"memory-manager-policy" = "Static"

[settings.kubernetes.memory-manager-reserved-memory.0]
enabled = true
memory = "727Mi"
`
			config, err := NewBottlerocketConfig(context.Background(), &userData)
			Expect(err).ToNot(HaveOccurred())

			out, err := config.MarshalTOML()
			Expect(err).ToNot(HaveOccurred())

			// Re-parse using UnmarshalTOML (not toml.Unmarshal) since Settings is tagged toml:"-"
			roundTrip, err := NewBottlerocketConfig(context.Background(), lo.ToPtr(string(out)))
			Expect(err).ToNot(HaveOccurred())
			Expect(roundTrip.Settings.Kubernetes.MemoryManagerPolicy).ToNot(BeNil())
			Expect(*roundTrip.Settings.Kubernetes.MemoryManagerPolicy).To(Equal("Static"))
			Expect(roundTrip.Settings.Kubernetes.MemoryManagerReservedMemory).To(HaveKey("0"))
			Expect(roundTrip.Settings.Kubernetes.MemoryManagerReservedMemory["0"].Enabled).To(BeTrue())
			Expect(roundTrip.Settings.Kubernetes.MemoryManagerReservedMemory["0"].Memory).To(Equal("727Mi"))
		})

		It("should support multiple NUMA nodes", func() {
			userData := `
[settings.kubernetes]
"memory-manager-policy" = "Static"

[settings.kubernetes.memory-manager-reserved-memory.0]
enabled = true
memory = "400Mi"

[settings.kubernetes.memory-manager-reserved-memory.1]
enabled = true
memory = "327Mi"
`
			config, err := NewBottlerocketConfig(context.Background(), &userData)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Settings.Kubernetes.MemoryManagerReservedMemory).To(HaveLen(2))
			Expect(config.Settings.Kubernetes.MemoryManagerReservedMemory["0"].Memory).To(Equal("400Mi"))
			Expect(config.Settings.Kubernetes.MemoryManagerReservedMemory["1"].Memory).To(Equal("327Mi"))
		})
	})
})
