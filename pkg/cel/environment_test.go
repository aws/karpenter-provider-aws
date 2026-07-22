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

package cel_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-provider-aws/pkg/cel"
)

func TestCel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CEL Suite")
}

var _ = Describe("EvaluateExpression", func() {
	It("should evaluate the ENI formula", func() {
		// m5.large: 3 ENIs, 10 IPs/ENI -> ((3-1) * (10-1)) + 2 = 20
		vars := cel.InstanceTypeVars{
			VCPUs:       2,
			MemoryMiB:   8192,
			DefaultENIs: 3,
			IPsPerENI:   10,
			MaxPods:     20,
		}
		result, err := cel.EvaluateExpression("((default_enis - 1) * (ips_per_eni - 1)) + 2", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(20)))
	})
	It("should evaluate prefix delegation with min", func() {
		// m5.large with prefix delegation: min(250, ((3-1) * (10-1)) * 16 + 2) = min(250, 290) = 250
		vars := cel.InstanceTypeVars{
			VCPUs:       2,
			MemoryMiB:   8192,
			DefaultENIs: 3,
			IPsPerENI:   10,
			MaxPods:     20,
		}
		result, err := cel.EvaluateExpression("min(250, ((default_enis - 1) * (ips_per_eni - 1)) * 16 + 2)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(250)))
	})
	It("should evaluate the kube-reserved CPU formula", func() {
		// 16 vCPUs: max(60, 16 * 30) * 1000000 = 480000000 (480m in nanocores)
		vars := cel.InstanceTypeVars{
			VCPUs:       16,
			MemoryMiB:   65536,
			DefaultENIs: 8,
			IPsPerENI:   30,
			MaxPods:     58,
		}
		result, err := cel.EvaluateExpression("max(60, vcpus * 30) * 1000000", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(480000000)))
	})
	It("should evaluate the kube-reserved memory formula", func() {
		// (11 * 58 + 255) * 1048576
		vars := cel.InstanceTypeVars{
			VCPUs:       16,
			MemoryMiB:   65536,
			DefaultENIs: 8,
			IPsPerENI:   30,
			MaxPods:     58,
		}
		result, err := cel.EvaluateExpression("(11 * max_pods + 255) * 1048576", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64((11*58 + 255) * 1048576)))
	})
	It("should evaluate min against max_pods", func() {
		vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		result, err := cel.EvaluateExpression("min(110, max_pods)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(20)))
	})
	It("should evaluate max with mixed int and double args", func() {
		// max(int, double): max(vcpus, 60.5) = max(4, 60.5) = 60.5 -> truncated to 60
		vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		result, err := cel.EvaluateExpression("max(vcpus, 60.5)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(60)))
	})
	It("should evaluate min with mixed double and int args", func() {
		// min(double, int): min(110.5, max_pods) = min(110.5, 20) = 20
		vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		result, err := cel.EvaluateExpression("min(110.5, max_pods)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(20)))
	})
})

var _ = Describe("ValidateExpression", func() {
	It("should accept a valid expression", func() {
		Expect(cel.ValidateExpression("((default_enis - 1) * (ips_per_eni - 1)) + 2")).To(Succeed())
	})
	It("should reject invalid syntax", func() {
		Expect(cel.ValidateExpression("((default_enis -")).ToNot(Succeed())
	})
	It("should reject undefined variables", func() {
		Expect(cel.ValidateExpression("undefined_var + 1")).ToNot(Succeed())
	})
	It("should reject a boolean return type", func() {
		Expect(cel.ValidateExpression("vcpus > 4")).ToNot(Succeed())
	})
})
