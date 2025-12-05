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
	"encoding/base64"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
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

	Describe("Bottlerocket Configuration", func() {
		var br Bottlerocket

		BeforeEach(func() {
			br = Bottlerocket{
				Options: Options{
					ClusterName:     "test-cluster",
					ClusterEndpoint: "https://example.com",
					CABundle:        lo.ToPtr("dGVzdC1jYS1idW5kbGU="), // "test-ca-bundle" in base64
				},
			}
		})

		Context("ClusterDNSIP Array Support", func() {
			It("should handle single DNS IP from KubeletConfig", func() {
				br.KubeletConfig = &v1.KubeletConfiguration{
					ClusterDNS: []string{"10.0.0.10"},
				}

				script, err := br.Script()
				Expect(err).ToNot(HaveOccurred())

				decoded, err := base64.StdEncoding.DecodeString(script)
				Expect(err).ToNot(HaveOccurred())

				tomlContent := string(decoded)
				Expect(tomlContent).To(ContainSubstring("cluster-dns-ip = '10.0.0.10'"))
			})

			It("should handle multiple DNS IPs from KubeletConfig", func() {
				br.KubeletConfig = &v1.KubeletConfiguration{
					ClusterDNS: []string{"1.1.1.1", "2.2.2.2"},
				}

				script, err := br.Script()
				Expect(err).ToNot(HaveOccurred())

				decoded, err := base64.StdEncoding.DecodeString(script)
				Expect(err).ToNot(HaveOccurred())

				tomlContent := string(decoded)
				Expect(tomlContent).To(ContainSubstring("cluster-dns-ip = ['1.1.1.1', '2.2.2.2']"))
			})

			It("should handle empty ClusterDNS from KubeletConfig", func() {
				br.KubeletConfig = &v1.KubeletConfiguration{
					ClusterDNS: []string{},
				}

				script, err := br.Script()
				Expect(err).ToNot(HaveOccurred())

				decoded, err := base64.StdEncoding.DecodeString(script)
				Expect(err).ToNot(HaveOccurred())

				tomlContent := string(decoded)
				Expect(tomlContent).ToNot(ContainSubstring("cluster-dns-ip"))
			})

			It("should handle nil KubeletConfig", func() {
				br.KubeletConfig = nil

				script, err := br.Script()
				Expect(err).ToNot(HaveOccurred())

				decoded, err := base64.StdEncoding.DecodeString(script)
				Expect(err).ToNot(HaveOccurred())

				tomlContent := string(decoded)
				Expect(tomlContent).ToNot(ContainSubstring("cluster-dns-ip"))
			})
		})

		Context("Custom User Data Parsing", func() {
			It("should convert single DNS IP string to array", func() {
				customData := `[settings]
[settings.kubernetes]
cluster-dns-ip = "8.8.8.8"`

				config, err := NewBottlerocketConfig(&customData)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Settings.Kubernetes.ClusterDNSIP).To(Equal([]string{"8.8.8.8"}))
			})

			It("should handle DNS IP array in custom user data", func() {
				customData := `[settings]
[settings.kubernetes]
cluster-dns-ip = ["8.8.8.8", "8.8.4.4"]`

				config, err := NewBottlerocketConfig(&customData)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Settings.Kubernetes.ClusterDNSIP).To(Equal([]string{"8.8.8.8", "8.8.4.4"}))
			})

			It("should handle empty custom user data", func() {
				config, err := NewBottlerocketConfig(nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Settings.Kubernetes.ClusterDNSIP).To(BeEmpty())
			})

			It("should handle custom user data without cluster-dns-ip", func() {
				customData := `[settings]
[settings.kubernetes]
max-pods = 110`

				config, err := NewBottlerocketConfig(&customData)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Settings.Kubernetes.ClusterDNSIP).To(BeEmpty())
			})

			It("should handle malformed custom user data gracefully", func() {
				customData := `[settings]
[settings.kubernetes]
cluster-dns-ip = invalid-value`

				_, err := NewBottlerocketConfig(&customData)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("KubeletConfig Override Behavior", func() {
			It("should override custom user data DNS IP with KubeletConfig values", func() {
				customData := `[settings]
[settings.kubernetes]
cluster-dns-ip = "8.8.8.8"
max-pods = 50`

				br.CustomUserData = &customData
				br.KubeletConfig = &v1.KubeletConfiguration{
					ClusterDNS: []string{"1.1.1.1", "2.2.2.2"},
				}

				script, err := br.Script()
				Expect(err).ToNot(HaveOccurred())

				decoded, err := base64.StdEncoding.DecodeString(script)
				Expect(err).ToNot(HaveOccurred())

				tomlContent := string(decoded)
				// Should contain KubeletConfig values, not custom user data values
				Expect(tomlContent).To(ContainSubstring("cluster-dns-ip = ['1.1.1.1', '2.2.2.2']"))
				Expect(tomlContent).ToNot(ContainSubstring("cluster-dns-ip = ['8.8.8.8']"))
				// But should preserve other custom settings
				Expect(tomlContent).To(ContainSubstring("max-pods = 50"))
			})

			It("should use custom user data when KubeletConfig ClusterDNS is empty", func() {
				customData := `[settings]
[settings.kubernetes]
cluster-dns-ip = ["8.8.8.8", "8.8.4.4"]`

				br.CustomUserData = &customData
				br.KubeletConfig = &v1.KubeletConfiguration{
					ClusterDNS: []string{}, // Empty
				}

				script, err := br.Script()
				Expect(err).ToNot(HaveOccurred())

				decoded, err := base64.StdEncoding.DecodeString(script)
				Expect(err).ToNot(HaveOccurred())

				tomlContent := string(decoded)
				// Should contain custom user data values since KubeletConfig is empty
				Expect(tomlContent).To(ContainSubstring("cluster-dns-ip = ['8.8.8.8', '8.8.4.4']"))
			})
		})

		Context("TOML Serialization", func() {
			It("should generate valid TOML with all required fields", func() {
				br.KubeletConfig = &v1.KubeletConfiguration{
					ClusterDNS: []string{"1.1.1.1", "2.2.2.2"},
					MaxPods:    lo.ToPtr[int32](110),
				}

				script, err := br.Script()
				Expect(err).ToNot(HaveOccurred())

				decoded, err := base64.StdEncoding.DecodeString(script)
				Expect(err).ToNot(HaveOccurred())

				tomlContent := string(decoded)

				// Verify all expected fields are present
				Expect(tomlContent).To(ContainSubstring("[settings]"))
				Expect(tomlContent).To(ContainSubstring("[settings.kubernetes]"))
				Expect(tomlContent).To(ContainSubstring("api-server = 'https://example.com'"))
				Expect(tomlContent).To(ContainSubstring("cluster-name = 'test-cluster'"))
				Expect(tomlContent).To(ContainSubstring("cluster-certificate = 'dGVzdC1jYS1idW5kbGU='"))
				Expect(tomlContent).To(ContainSubstring("cluster-dns-ip = ['1.1.1.1', '2.2.2.2']"))
				Expect(tomlContent).To(ContainSubstring("max-pods = 110"))
			})

			It("should handle complex configuration with multiple fields", func() {
				br.KubeletConfig = &v1.KubeletConfiguration{
					ClusterDNS: []string{"10.0.0.10", "10.0.0.11", "10.0.0.12"},
					MaxPods:    lo.ToPtr[int32](250),
					SystemReserved: map[string]string{
						"memory": "1Gi",
						"cpu":    "100m",
					},
					KubeReserved: map[string]string{
						"memory": "500Mi",
						"cpu":    "50m",
					},
				}

				script, err := br.Script()
				Expect(err).ToNot(HaveOccurred())

				decoded, err := base64.StdEncoding.DecodeString(script)
				Expect(err).ToNot(HaveOccurred())

				tomlContent := string(decoded)

				Expect(tomlContent).To(ContainSubstring("cluster-dns-ip = ['10.0.0.10', '10.0.0.11', '10.0.0.12']"))
				Expect(tomlContent).To(ContainSubstring("max-pods = 250"))
				Expect(tomlContent).To(ContainSubstring("[settings.kubernetes.system-reserved]"))
				Expect(tomlContent).To(ContainSubstring("[settings.kubernetes.kube-reserved]"))
			})
		})
	})
})
