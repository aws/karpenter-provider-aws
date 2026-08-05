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
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/karpenter-provider-aws/pkg/cel"
)

// celEnv is the shared CEL environment under test, constructed once for the suite.
var celEnv *cel.CELEnvironment

// ctx carries a discarding logger so the dropped-entry diagnostics ResolveResourceMap emits stay out of
// the test output.
var ctx context.Context

func TestCel(t *testing.T) {
	RegisterFailHandler(Fail)
	var err error
	celEnv, err = cel.NewEnvironment()
	if err != nil {
		t.Fatalf("building CEL environment: %v", err)
	}
	ctx = log.IntoContext(context.Background(), logr.Discard())
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
		result, err := celEnv.EvaluateExpression("((default_enis - 1) * (ips_per_eni - 1)) + 2", vars)
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
		result, err := celEnv.EvaluateExpression("min(250, ((default_enis - 1) * (ips_per_eni - 1)) * 16 + 2)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(250)))
	})
	It("should evaluate the kube-reserved CPU formula", func() {
		// 16 vCPUs: max(60, 16 * 30) = 480, i.e. 480 millicores. cpu expressions are in millicores; ResolveResourceMap attaches the "m" suffix.
		vars := cel.InstanceTypeVars{
			VCPUs:       16,
			MemoryMiB:   65536,
			DefaultENIs: 8,
			IPsPerENI:   30,
			MaxPods:     58,
		}
		result, err := celEnv.EvaluateExpression("max(60, vcpus * 30)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(480)))
	})
	It("should evaluate the kube-reserved memory formula", func() {
		// 11 MiB per max-pod plus a 255 MiB base, in MiB: 11 * 58 + 255
		vars := cel.InstanceTypeVars{
			VCPUs:       16,
			MemoryMiB:   65536,
			DefaultENIs: 8,
			IPsPerENI:   30,
			MaxPods:     58,
		}
		result, err := celEnv.EvaluateExpression("11 * max_pods + 255", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(11*58 + 255)))
	})
	It("should evaluate min against max_pods", func() {
		vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		result, err := celEnv.EvaluateExpression("min(110, max_pods)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(20)))
	})
	It("should evaluate max with mixed int and double args", func() {
		// max(int, double): max(vcpus, 60.5) = max(4, 60.5) = 60.5 -> truncated to 60
		vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		result, err := celEnv.EvaluateExpression("max(vcpus, 60.5)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(60)))
	})
	It("should evaluate min with mixed double and int args", func() {
		// min(double, int): min(110.5, max_pods) = min(110.5, 20) = 20
		vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		result, err := celEnv.EvaluateExpression("min(110.5, max_pods)", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(20)))
	})
	It("should evaluate an expression that uses instance_type", func() {
		// instance_type is a string variable usable in conditionals; the result must still be numeric.
		vars := cel.InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20, InstanceType: "m5.large"}
		result, err := celEnv.EvaluateExpression(`instance_type == "m5.large" ? vcpus * 2 : vcpus`, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(8)))
		// A non-matching instance type takes the other branch.
		vars.InstanceType = "c5.xlarge"
		result, err = celEnv.EvaluateExpression(`instance_type == "m5.large" ? vcpus * 2 : vcpus`, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(4)))
	})
	It("should return a negative result without erroring (dropping is handled by ResolveResourceMap)", func() {
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		result, err := celEnv.EvaluateExpression("vcpus - 100", vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(int64(-98)))
	})
	It("should error on integer division by zero", func() {
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		_, err := celEnv.EvaluateExpression("100 / (vcpus - vcpus)", vars)
		Expect(err).To(HaveOccurred())
	})
	It("should error on integer modulo by zero", func() {
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		_, err := celEnv.EvaluateExpression("100 % (vcpus - vcpus)", vars)
		Expect(err).To(HaveOccurred())
	})
	It("should error on double division by zero (+Inf is non-finite)", func() {
		// Unlike integer division, CEL follows IEEE-754 and returns +Inf rather than erroring; we must
		// reject the non-finite result so it can't be truncated to a garbage int64 and slip through.
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		_, err := celEnv.EvaluateExpression("100.0 / 0.0", vars)
		Expect(err).To(HaveOccurred())
	})
	It("should error on a NaN result", func() {
		// 0.0 / 0.0 is NaN under IEEE-754; it must be rejected like the infinities.
		vars := cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
		_, err := celEnv.EvaluateExpression("0.0 / 0.0", vars)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("EvaluateExpression across instance type sizes", func() {
	// The NodeClass expressions under test, mirroring what a user configures in spec.kubelet.
	const (
		maxPodsExpr        = "min(110, default_enis * (ips_per_eni - 1))"
		kubeReservedMem    = "max_pods * 11"
		systemReservedMem  = "memory_mib / 100"
		kubeReservedCPU    = "max(60, vcpus * 30)"
		systemReservedMemP = "max(100, memory_mib / 64)"
	)
	// Real instance type properties spanning the smallest to the largest-memory shapes, so that any
	// int64 overflow or per-instance-type divergence in the reserved-resource math surfaces here rather
	// than only at node launch. memory_mib on u-24tb1.metal (24 TiB) is the worst case for overflow.
	DescribeTable("evaluates reserved-resource expressions without overflow or error",
		func(vars cel.InstanceTypeVars) {
			for _, expr := range []string{maxPodsExpr, kubeReservedMem, systemReservedMem, kubeReservedCPU, systemReservedMemP} {
				result, err := celEnv.EvaluateExpression(expr, vars)
				Expect(err).ToNot(HaveOccurred(), "expression %q overflowed or errored for %s (memory_mib=%d)", expr, vars.InstanceType, vars.MemoryMiB)
				Expect(result).To(BeNumerically(">=", int64(0)), "expression %q produced a negative value for %s", expr, vars.InstanceType)
			}
			// systemReserved.memory is 1% of node memory, in MiB.
			sysMem, err := celEnv.EvaluateExpression(systemReservedMem, vars)
			Expect(err).ToNot(HaveOccurred())
			Expect(sysMem).To(Equal(vars.MemoryMiB / 100))
			// kubeReserved.memory is ~11 MiB per max-pod.
			kubeMem, err := celEnv.EvaluateExpression(kubeReservedMem, vars)
			Expect(err).ToNot(HaveOccurred())
			Expect(kubeMem).To(Equal(vars.MaxPods * 11))
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
			result, err := celEnv.EvaluateExpression(expr, vars)
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
	vars := func() (cel.InstanceTypeVars, error) {
		return cel.InstanceTypeVars{VCPUs: 2, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20, InstanceType: "m5.large"}, nil
	}
	It("should pass through values that are already valid resource quantities", func() {
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "100m", "memory": "256Mi"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "100m", "memory": "256Mi"}))
	})
	It("should evaluate an expression and replace it with its integer result", func() {
		// cpu results are millicores, so they carry an "m" suffix.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "vcpus * 30"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "60m"}))
	})
	It("should suffix a cpu result with m so it is interpreted as millicores", func() {
		// Without the suffix, "480" would parse as 480 whole cores rather than 480m, driving allocatable
		// CPU to zero on the node.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "max(60, vcpus * 30)"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "60m"}))
		q, err := resource.ParseQuantity(resolved["cpu"])
		Expect(err).ToNot(HaveOccurred())
		Expect(q.MilliValue()).To(Equal(int64(60)))
	})
	It("should allow a sub-core cpu reservation to be expressed", func() {
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "vcpus * 250"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "500m"}))
		q, err := resource.ParseQuantity(resolved["cpu"])
		Expect(err).ToNot(HaveOccurred())
		Expect(q.String()).To(Equal("500m"))
		Expect(q.Value()).To(Equal(int64(1)), "500m rounds up to 1 whole core, i.e. it is a sub-core reservation")
	})
	It("should suffix memory with Mi and ephemeral-storage with Gi, leaving pid bare", func() {
		// Each key uses the unit its static quantities use: Mi for memory, Gi for ephemeral-storage.
		// pid is a unitless process count.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{
			"memory":            "max_pods * 11",
			"ephemeral-storage": "1 + 1",
			"pid":               "max_pods * 10",
		}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{
			// 20 * 11 = 220, rounded up to the next multiple of 16.
			"memory": "224Mi",
			// ephemeral-storage is already whole GiB, so it is not rounded further.
			"ephemeral-storage": "2Gi",
			// pid is a process count with no natural granularity, so it is not rounded.
			"pid": "200",
		}))
		mem, err := resource.ParseQuantity(resolved["memory"])
		Expect(err).ToNot(HaveOccurred())
		Expect(mem.Value()).To(Equal(int64(224 * 1048576)))
		storage, err := resource.ParseQuantity(resolved["ephemeral-storage"])
		Expect(err).ToNot(HaveOccurred())
		Expect(storage.Value()).To(Equal(int64(2 * 1073741824)))
	})
	It("should express a percentage of node memory without a byte scale factor", func() {
		// memory_mib / 100 is 1% of node memory. On m5.large (8192 MiB) the exact value is 81.92Mi, which
		// truncates to 81Mi and then rounds up to 96Mi. The 14Mi of over-reservation is 0.18% of the
		// node's memory, and shrinks in relative terms as instance size grows.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"memory": "memory_mib / 100"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"memory": "96Mi"}))
	})
	It("should be exact for power-of-two divisors of node memory", func() {
		// memory_mib / 64 divides evenly and 128 is already a multiple of 16, so neither MiB granularity
		// nor rounding costs anything: 8192/64 = 128Mi exactly.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"memory": "memory_mib / 64"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"memory": "128Mi"}))
		q, err := resource.ParseQuantity(resolved["memory"])
		Expect(err).ToNot(HaveOccurred())
		Expect(q.Value()).To(Equal(int64(8192) * 1048576 / 64))
	})
	It("should round a cpu result up to the next 10m", func() {
		// 70 + vcpus = 72, which is not a multiple of 10.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "70 + vcpus"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "80m"}))
	})
	It("should leave a cpu result that is already a multiple of 10m unchanged", func() {
		// vcpus * 30 = 60, already a multiple of 10, so rounding is a no-op. Expressions whose results
		// step by more than the granularity are unaffected by rounding entirely.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "vcpus * 30"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "60m"}))
	})
	It("should round up rather than down, so a reservation is never smaller than requested", func() {
		// 60 + vcpus - 1 = 61m, which must not become 60m: under-reserving risks node instability, so
		// rounding is always upward.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "60 + vcpus - 1"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "70m"}))
		q, err := resource.ParseQuantity(resolved["cpu"])
		Expect(err).ToNot(HaveOccurred())
		Expect(q.MilliValue()).To(BeNumerically(">=", 61))
	})
	It("should collapse cpu results that differ by less than the rounding granularity", func() {
		// This is the mechanism by which rounding reduces launch templates: two instance types whose
		// expressions yield 71m and 78m resolve to the same 80m, so they share a launch template.
		low, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "70 + vcpus - 1"}, vars)
		Expect(err).ToNot(HaveOccurred())
		high, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "80 - vcpus"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(low["cpu"]).To(Equal(high["cpu"]))
		Expect(low["cpu"]).To(Equal("80m"))
	})
	It("should round a memory result up to the next 16Mi", func() {
		// max_pods + vcpus = 22, which rounds up to 32.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"memory": "max_pods + vcpus"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"memory": "32Mi"}))
	})
	It("should not round a zero result up to the granularity", func() {
		// A zero reservation is meaningful (reserve nothing) and must not become 10m or 16Mi.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{
			"cpu":    "vcpus - vcpus",
			"memory": "vcpus - vcpus",
		}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "0m", "memory": "0Mi"}))
	})
	It("should not round pid or ephemeral-storage", func() {
		// pid has no natural granularity and ephemeral-storage is already whole GiB. 70 + vcpus - 1 = 71
		// and vcpus + 1 = 3, neither of which is a multiple of 10 or 16.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{
			"pid":               "70 + vcpus - 1",
			"ephemeral-storage": "vcpus + 1",
		}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"pid": "71", "ephemeral-storage": "3Gi"}))
	})
	It("should not overflow into a negative quantity when rounding a near-max result", func() {
		// int64 max is not a sensible reservation, but rounding it must not wrap it negative: the
		// non-negative check in ResolveResourceMap has already run by the time formatting happens.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "9223372036854775807 - vcpus + 2"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(HaveKey("cpu"))
		q, err := resource.ParseQuantity(resolved["cpu"])
		Expect(err).ToNot(HaveOccurred())
		Expect(q.MilliValue()).To(BeNumerically(">", 0), "rounding must not wrap a near-max value negative")
	})
	It("should drop entries whose expression evaluates to a negative value", func() {
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "vcpus - 100"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).ToNot(HaveKey("cpu"))
	})
	It("should drop entries whose expression errors (e.g. division by zero)", func() {
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "100 / (vcpus - vcpus)"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).ToNot(HaveKey("cpu"))
	})
	It("should keep valid entries while dropping invalid ones", func() {
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{
			"good":     "vcpus * 30",
			"negative": "vcpus - 100",
			"literal":  "128Mi",
		}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"good": "60", "literal": "128Mi"}))
	})
	It("should keep an entry that evaluates to zero (only negatives are dropped)", func() {
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "vcpus - vcpus"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "0m"}))
	})
	It("should truncate a double-returning expression to its integer string", func() {
		// double(memory_mib) * 0.5 = 4096.0, truncated to "4096".
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"memory": "double(memory_mib) * 0.5"}, vars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"memory": "4096Mi"}))
	})
	It("should return an empty map unchanged without invoking varsFn", func() {
		called := false
		countingVars := func() (cel.InstanceTypeVars, error) {
			called = true
			return vars()
		}
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{}, countingVars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(BeEmpty())
		Expect(called).To(BeFalse())
	})
	It("should return a nil map unchanged without invoking varsFn", func() {
		called := false
		countingVars := func() (cel.InstanceTypeVars, error) {
			called = true
			return vars()
		}
		resolved, err := celEnv.ResolveResourceMap(ctx, nil, countingVars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(BeEmpty())
		Expect(called).To(BeFalse())
	})
	It("should not invoke varsFn when the map contains only literal quantities", func() {
		called := false
		countingVars := func() (cel.InstanceTypeVars, error) {
			called = true
			return vars()
		}
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "100m", "memory": "256Mi"}, countingVars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "100m", "memory": "256Mi"}))
		Expect(called).To(BeFalse(), "varsFn must not be built when there are no expressions to evaluate")
	})
	It("should invoke varsFn exactly once even with multiple expressions", func() {
		callCount := 0
		countingVars := func() (cel.InstanceTypeVars, error) {
			callCount++
			return vars()
		}
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{
			"cpu":    "vcpus * 30",
			"memory": "max_pods * 11",
		}, countingVars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "60m", "memory": "224Mi"}))
		Expect(callCount).To(Equal(1), "varsFn should be built at most once and reused across expressions")
	})
	It("should return the error when varsFn fails, without resolving any entries", func() {
		failingVars := func() (cel.InstanceTypeVars, error) {
			return cel.InstanceTypeVars{}, fmt.Errorf("instance type inputs unavailable")
		}
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "vcpus * 30", "literal": "128Mi"}, failingVars)
		Expect(err).To(MatchError(ContainSubstring("instance type inputs unavailable")))
		Expect(resolved).To(BeNil(), "a varsFn failure must not return a partially resolved map")
	})
	It("should not invoke varsFn (and so not fail) when there are no expressions to evaluate", func() {
		failingVars := func() (cel.InstanceTypeVars, error) {
			return cel.InstanceTypeVars{}, fmt.Errorf("instance type inputs unavailable")
		}
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{"cpu": "100m"}, failingVars)
		Expect(err).ToNot(HaveOccurred())
		Expect(resolved).To(Equal(map[string]string{"cpu": "100m"}))
	})
})

var _ = Describe("ValidateExpression", func() {
	It("should accept a valid expression", func() {
		Expect(celEnv.ValidateExpression("((default_enis - 1) * (ips_per_eni - 1)) + 2")).To(Succeed())
	})
	It("should reject invalid syntax", func() {
		Expect(celEnv.ValidateExpression("((default_enis -")).ToNot(Succeed())
	})
	It("should reject undefined variables", func() {
		Expect(celEnv.ValidateExpression("undefined_var + 1")).ToNot(Succeed())
	})
	It("should reject a boolean return type", func() {
		Expect(celEnv.ValidateExpression("vcpus > 4")).ToNot(Succeed())
	})
	It("should reject a string return type", func() {
		// instance_type is a valid variable, but a bare reference returns a string, which the
		// output-type check must reject just like a boolean.
		Expect(celEnv.ValidateExpression("instance_type")).ToNot(Succeed())
	})
	It("should reject an empty expression", func() {
		Expect(celEnv.ValidateExpression("")).ToNot(Succeed())
	})
	It("should reject a whitespace-only expression", func() {
		Expect(celEnv.ValidateExpression("   ")).ToNot(Succeed())
	})
})
