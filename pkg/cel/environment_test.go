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

	"github.com/go-logr/logr"
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
	It("should evaluate an expression that uses instance_type", func() {
		// instance_type is a string variable usable in conditionals; the result must still be numeric.
		vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20, InstanceType: "m5.large"}
		result, err := cel.EvaluateExpression(`instance_type == "m5.large" ? vcpus * 2 : vcpus`, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(8)))
		// A non-matching instance type takes the other branch.
		vars.InstanceType = "c5.xlarge"
		result, err = cel.EvaluateExpression(`instance_type == "m5.large" ? vcpus * 2 : vcpus`, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(4)))
	})
	It("should return a negative result without erroring (dropping is handled by ResolveResourceMap)", func() {
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		result, err := cel.EvaluateExpression("vcpus - 100", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(-98)))
	})
	It("should error on integer division by zero", func() {
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		_, err := cel.EvaluateExpression("100 / (vcpus - vcpus)", vars)
		Expect(err).To(HaveOccurred())
	})
	It("should error on integer modulo by zero", func() {
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		_, err := cel.EvaluateExpression("100 % (vcpus - vcpus)", vars)
		Expect(err).To(HaveOccurred())
	})
	It("should error on double division by zero (+Inf is non-finite)", func() {
		// Unlike integer division, CEL follows IEEE-754 and returns +Inf rather than erroring; we must
		// reject the non-finite result so it can't be truncated to a garbage int64 and slip through.
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		_, err := cel.EvaluateExpression("100.0 / 0.0", vars)
		Expect(err).To(HaveOccurred())
	})
	It("should error on a NaN result", func() {
		// 0.0 / 0.0 is NaN under IEEE-754; it must be rejected like the infinities.
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		_, err := cel.EvaluateExpression("0.0 / 0.0", vars)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("EvaluateExpression across instance type sizes", func() {
	// The NodeClass expressions under test, mirroring what a user configures in spec.kubelet.
	const (
		maxPodsExpr        = "min(110, default_enis * (ips_per_eni - 1))"
		kubeReservedMem    = "max_pods * 11 * 1048576"
		systemReservedMem  = "memory_mib * 1048576 / 100"
		kubeReservedCPU    = "max(60, vcpus * 30) * 1000000"
		systemReservedMemP = "max(104857600, memory_mib * 1048576 / 64)"
	)
	// Real instance type properties spanning the smallest to the largest-memory shapes, so that any
	// int64 overflow or per-instance-type divergence in the reserved-resource math surfaces here rather
	// than only at node launch. memory_mib on u-24tb1.metal (24 TiB) is the worst case for overflow.
	DescribeTable("evaluates reserved-resource expressions without overflow or error",
		func(vars cel.InstanceTypeVars) {
			for _, expr := range []string{maxPodsExpr, kubeReservedMem, systemReservedMem, kubeReservedCPU, systemReservedMemP} {
				result, err := cel.EvaluateExpression(expr, vars)
				Expect(err).ToNot(HaveOccurred(), "expression %q overflowed or errored for %s (memory_mib=%d)", expr, vars.InstanceType, vars.MemoryMiB)
				Expect(result).To(BeNumerically(">=", int64(0)), "expression %q produced a negative value for %s", expr, vars.InstanceType)
			}
			// systemReserved.memory must equal memory_mib*1048576/100 exactly (the value the cluster test verified).
			sysMem, err := cel.EvaluateExpression(systemReservedMem, vars)
			Expect(err).ToNot(HaveOccurred())
			Expect(sysMem).To(Equal(vars.MemoryMiB * 1048576 / 100))
			// kubeReserved.memory must equal max_pods*11*1048576 exactly.
			kubeMem, err := cel.EvaluateExpression(kubeReservedMem, vars)
			Expect(err).ToNot(HaveOccurred())
			Expect(kubeMem).To(Equal(vars.MaxPods * 11 * 1048576))
		},
		Entry("m5.large", cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 27, InstanceType: "m5.large"}),
		Entry("c5.xlarge", cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 4, IPsPerENI: 15, MaxPods: 58, InstanceType: "c5.xlarge"}),
		Entry("m5.4xlarge", cel.InstanceTypeVars{VCPUs: 16, MemoryMiB: 65536, DefaultENIs: 8, IPsPerENI: 30, MaxPods: 234, InstanceType: "m5.4xlarge"}),
		Entry("m5.24xlarge", cel.InstanceTypeVars{VCPUs: 96, MemoryMiB: 393216, DefaultENIs: 15, IPsPerENI: 50, MaxPods: 737, InstanceType: "m5.24xlarge"}),
		Entry("x1e.32xlarge (~4TiB)", cel.InstanceTypeVars{VCPUs: 128, MemoryMiB: 3997696, DefaultENIs: 8, IPsPerENI: 30, MaxPods: 234, InstanceType: "x1e.32xlarge"}),
		Entry("u-24tb1.metal (24TiB, overflow worst case)", cel.InstanceTypeVars{VCPUs: 448, MemoryMiB: 25165824, DefaultENIs: 15, IPsPerENI: 50, MaxPods: 737, InstanceType: "u-24tb1.metal"}),
	)
})

var _ = Describe("max and min overloads", func() {
	// Exercises each registered overload (int/int, double/double, int/double, double/int) for both
	// functions. Double results are truncated toward zero on the way back to int64.
	vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
	DescribeTable("evaluates to the expected result",
		func(expr string, expected int64) {
			result, err := cel.EvaluateExpression(expr, vars)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
		Entry("max int/int", "max(3, 7)", int64(7)),
		Entry("max double/double", "max(3.9, 7.9)", int64(7)),
		Entry("max int/double", "max(3, 7.9)", int64(7)),
		Entry("max double/int", "max(7.9, 3)", int64(7)),
		Entry("min int/int", "min(3, 7)", int64(3)),
		Entry("min double/double", "min(3.9, 7.9)", int64(3)),
		Entry("min int/double", "min(3, 7.9)", int64(3)),
		Entry("min double/int", "min(7.9, 3)", int64(3)),
	)
})

var _ = Describe("ResolveResourceMap", func() {
	vars := func() cel.InstanceTypeVars {
		return cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20, InstanceType: "m5.large"}
	}
	It("should pass through values that are already valid resource quantities", func() {
		resolved := cel.ResolveResourceMap(map[string]string{"cpu": "100m", "memory": "256Mi"}, vars, logr.Discard())
		Expect(resolved).To(Equal(map[string]string{"cpu": "100m", "memory": "256Mi"}))
	})
	It("should evaluate an expression and replace it with its integer result", func() {
		resolved := cel.ResolveResourceMap(map[string]string{"cpu": "vcpus * 30"}, vars, logr.Discard())
		Expect(resolved).To(Equal(map[string]string{"cpu": "60"}))
	})
	It("should drop entries whose expression evaluates to a negative value", func() {
		resolved := cel.ResolveResourceMap(map[string]string{"cpu": "vcpus - 100"}, vars, logr.Discard())
		Expect(resolved).ToNot(HaveKey("cpu"))
	})
	It("should drop entries whose expression errors (e.g. division by zero)", func() {
		resolved := cel.ResolveResourceMap(map[string]string{"cpu": "100 / (vcpus - vcpus)"}, vars, logr.Discard())
		Expect(resolved).ToNot(HaveKey("cpu"))
	})
	It("should keep valid entries while dropping invalid ones", func() {
		resolved := cel.ResolveResourceMap(map[string]string{
			"good":     "vcpus * 30",
			"negative": "vcpus - 100",
			"literal":  "128Mi",
		}, vars, logr.Discard())
		Expect(resolved).To(Equal(map[string]string{"good": "60", "literal": "128Mi"}))
	})
	It("should keep an entry that evaluates to zero (only negatives are dropped)", func() {
		resolved := cel.ResolveResourceMap(map[string]string{"cpu": "vcpus - vcpus"}, vars, logr.Discard())
		Expect(resolved).To(Equal(map[string]string{"cpu": "0"}))
	})
	It("should truncate a double-returning expression to its integer string", func() {
		// double(memory_mib) * 0.5 = 4096.0, truncated to "4096".
		resolved := cel.ResolveResourceMap(map[string]string{"memory": "double(memory_mib) * 0.5"}, vars, logr.Discard())
		Expect(resolved).To(Equal(map[string]string{"memory": "4096"}))
	})
	It("should return an empty map unchanged without invoking varsFn", func() {
		called := false
		countingVars := func() cel.InstanceTypeVars {
			called = true
			return vars()
		}
		resolved := cel.ResolveResourceMap(map[string]string{}, countingVars, logr.Discard())
		Expect(resolved).To(BeEmpty())
		Expect(called).To(BeFalse())
	})
	It("should return a nil map unchanged without invoking varsFn", func() {
		called := false
		countingVars := func() cel.InstanceTypeVars {
			called = true
			return vars()
		}
		resolved := cel.ResolveResourceMap(nil, countingVars, logr.Discard())
		Expect(resolved).To(BeEmpty())
		Expect(called).To(BeFalse())
	})
	It("should not invoke varsFn when the map contains only literal quantities", func() {
		called := false
		countingVars := func() cel.InstanceTypeVars {
			called = true
			return vars()
		}
		resolved := cel.ResolveResourceMap(map[string]string{"cpu": "100m", "memory": "256Mi"}, countingVars, logr.Discard())
		Expect(resolved).To(Equal(map[string]string{"cpu": "100m", "memory": "256Mi"}))
		Expect(called).To(BeFalse(), "varsFn must not be built when there are no expressions to evaluate")
	})
	It("should invoke varsFn exactly once even with multiple expressions", func() {
		callCount := 0
		countingVars := func() cel.InstanceTypeVars {
			callCount++
			return vars()
		}
		resolved := cel.ResolveResourceMap(map[string]string{
			"cpu":    "vcpus * 30",
			"memory": "max_pods * 11",
		}, countingVars, logr.Discard())
		Expect(resolved).To(Equal(map[string]string{"cpu": "60", "memory": "220"}))
		Expect(callCount).To(Equal(1), "varsFn should be built at most once and reused across expressions")
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
	It("should reject a string return type", func() {
		// instance_type is a valid variable, but a bare reference returns a string, which the
		// output-type check must reject just like a boolean.
		Expect(cel.ValidateExpression("instance_type")).ToNot(Succeed())
	})
	It("should reject an empty expression", func() {
		Expect(cel.ValidateExpression("")).ToNot(Succeed())
	})
	It("should reject a whitespace-only expression", func() {
		Expect(cel.ValidateExpression("   ")).ToNot(Succeed())
	})
})
