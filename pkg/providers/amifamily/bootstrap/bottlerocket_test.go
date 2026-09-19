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
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/samber/lo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrap")
}

var _ = Describe("Bottlerocket", func() {
	Describe("Image garbage collection thresholds", func() {
		DescribeTable("should emit string thresholds from custom user data", func(high, low, expectedHigh, expectedLow string) {
			userData := fmt.Sprintf(`
[settings.kubernetes]
image-gc-high-threshold-percent = %s
image-gc-low-threshold-percent = %s
memory-manager-policy = "Static"

[settings.host-containers.admin]
enabled = true
`, high, low)
			bottlerocket := Bottlerocket{Options: Options{CustomUserData: &userData}}
			script, err := bottlerocket.Script(context.Background())
			Expect(err).ToNot(HaveOccurred())
			decoded, err := base64.StdEncoding.DecodeString(script)
			Expect(err).ToNot(HaveOccurred())

			var raw map[string]any
			Expect(toml.Unmarshal(decoded, &raw)).To(Succeed())
			settings := raw["settings"].(map[string]any)
			kubernetes := settings["kubernetes"].(map[string]any)
			Expect(kubernetes).To(HaveKeyWithValue("image-gc-high-threshold-percent", expectedHigh))
			Expect(kubernetes).To(HaveKeyWithValue("image-gc-low-threshold-percent", expectedLow))
			Expect(kubernetes).To(HaveKeyWithValue("memory-manager-policy", "Static"))
			Expect(settings["host-containers"]).To(HaveKeyWithValue("admin", HaveKeyWithValue("enabled", true)))

			config, err := NewBottlerocketConfig(context.Background(), lo.ToPtr(string(decoded)))
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Settings.Kubernetes.ImageGCHighThresholdPercent).To(Equal(lo.ToPtr(expectedHigh)))
			Expect(config.Settings.Kubernetes.ImageGCLowThresholdPercent).To(Equal(lo.ToPtr(expectedLow)))
		},
			Entry("integers", "75", "45", "75", "45"),
			Entry("strings", `"75"`, `"45"`, "75", "45"),
			Entry("integer high and string low", "75", `"45"`, "75", "45"),
			Entry("string high and integer low", `"75"`, "45", "75", "45"),
			Entry("boundary values", "100", "0", "100", "0"),
		)

		It("should omit unspecified thresholds", func() {
			config, err := NewBottlerocketConfig(context.Background(), lo.ToPtr("[settings.kubernetes]"))
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Settings.Kubernetes.ImageGCHighThresholdPercent).To(BeNil())
			Expect(config.Settings.Kubernetes.ImageGCLowThresholdPercent).To(BeNil())
			data, err := config.MarshalTOML()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(data)).ToNot(ContainSubstring("image-gc-high-threshold-percent"))
			Expect(string(data)).ToNot(ContainSubstring("image-gc-low-threshold-percent"))
		})

		It("should prefer kubelet configuration over integer thresholds in custom user data", func() {
			bottlerocket := Bottlerocket{Options: Options{
				CustomUserData: lo.ToPtr(`[settings.kubernetes]
image-gc-high-threshold-percent = 75
image-gc-low-threshold-percent = 45
`),
				KubeletConfig: &v1.ParsedKubeletConfig{
					ImageGCHighThresholdPercent: lo.ToPtr[int32](85),
					ImageGCLowThresholdPercent:  lo.ToPtr[int32](80),
				},
			}}
			script, err := bottlerocket.Script(context.Background())
			Expect(err).ToNot(HaveOccurred())
			decoded, err := base64.StdEncoding.DecodeString(script)
			Expect(err).ToNot(HaveOccurred())
			var raw map[string]any
			Expect(toml.Unmarshal(decoded, &raw)).To(Succeed())
			kubernetes := raw["settings"].(map[string]any)["kubernetes"]
			Expect(kubernetes).To(HaveKeyWithValue("image-gc-high-threshold-percent", "85"))
			Expect(kubernetes).To(HaveKeyWithValue("image-gc-low-threshold-percent", "80"))
		})

		DescribeTable("should reject non-string, non-integer thresholds", func(value string) {
			for _, key := range []string{"image-gc-high-threshold-percent", "image-gc-low-threshold-percent"} {
				userData := fmt.Sprintf("[settings.kubernetes]\n%s = %s\n", key, value)
				_, err := NewBottlerocketConfig(context.Background(), &userData)
				Expect(err).To(HaveOccurred(), "expected %s to reject %s", key, value)
			}
		},
			Entry("booleans", "true"),
			Entry("floats", "75.0"),
			Entry("arrays", "[75]"),
			Entry("tables", "{ value = 75 }"),
		)
	})

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
