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

package instancetype_test

import (
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	kubeletcel "github.com/aws/karpenter-provider-aws/pkg/cel"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var _ = Describe("KubeletExpressionPreview", func() {
	var celEnv *kubeletcel.CELEnvironment
	var amiFamily amifamily.AMIFamily
	// A 2 vCPU / 7808 MiB instance type with 3 ENIs and 10 IPs per ENI, giving an ENI-limited default
	// max_pods of 3 * (10 - 1) + 2 = 29. The numbers are what the expectations below are written against.
	var info ec2types.InstanceTypeInfo

	BeforeEach(func() {
		celEnv = lo.Must(kubeletcel.NewEnvironment())
		amiFamily = amifamily.GetAMIFamily(v1.AMIFamilyAL2023, &amifamily.Options{})
		info = ec2types.InstanceTypeInfo{
			InstanceType: ec2types.InstanceType("m5.large"),
			VCpuInfo:     &ec2types.VCpuInfo{DefaultVCpus: lo.ToPtr(int32(2))},
			MemoryInfo:   &ec2types.MemoryInfo{SizeInMiB: lo.ToPtr(int64(7808))},
			NetworkInfo: &ec2types.NetworkInfo{
				DefaultNetworkCardIndex:   lo.ToPtr(int32(0)),
				Ipv4AddressesPerInterface: lo.ToPtr(int32(10)),
				NetworkCards:              []ec2types.NetworkCardInfo{{MaximumNetworkInterfaces: lo.ToPtr(int32(3))}},
			},
		}
		// Previewing an expression requires the gate that makes the controller evaluate one.
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			FeatureGates: test.FeatureGates{NodeClassCEL: lo.ToPtr(true)},
		}))
	})

	preview := func(kc instancetype.KubeletExpressionInput) instancetype.KubeletExpressionPreview {
		return instancetype.PreviewKubeletExpressions(ctx, celEnv, info, kc, amiFamily, nil)
	}
	// expressionByField indexes a preview's results so an assertion can name the field it cares about.
	expressionByField := func(p instancetype.KubeletExpressionPreview, field string) instancetype.PreviewedExpression {
		e, ok := lo.Find(p.Expressions, func(e instancetype.PreviewedExpression) bool { return e.Field == field })
		ExpectWithOffset(1, ok).To(BeTrue(), "expected the preview to include %s, got %v", field, p.Expressions)
		return e
	}

	It("should evaluate a maxPods expression against the AMI family default max_pods", func() {
		p := preview(instancetype.KubeletExpressionInput{MaxPods: lo.ToPtr(intstr.FromString("min(max_pods, vcpus * 8)"))})
		Expect(p.InstanceType).To(Equal("m5.large"))
		Expect(p.MaxPodsVars.MaxPods).To(BeNumerically("==", 29))
		Expect(expressionByField(p, "maxPods")).To(Equal(instancetype.PreviewedExpression{
			Field: "maxPods", Expression: "min(max_pods, vcpus * 8)", Value: "16", Applied: true,
		}))
	})

	It("should expose the instance type's inputs as CEL variables", func() {
		p := preview(instancetype.KubeletExpressionInput{MaxPods: lo.ToPtr(intstr.FromString("vcpus"))})
		Expect(p.MaxPodsVars).To(Equal(kubeletcel.InstanceTypeVars{
			VCPUs: 2, MemoryMiB: 7808, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 29, InstanceType: "m5.large",
		}))
	})

	// This is the invariant the whole preview exists to demonstrate: max_pods means the AMI family default in
	// a maxPods expression and the *resolved* maxPods in a reserved expression. Getting it backwards would
	// make the tool report values a real launch never produces.
	It("should evaluate reserved expressions against the resolved maxPods, not the default", func() {
		p := preview(instancetype.KubeletExpressionInput{
			MaxPods:      lo.ToPtr(intstr.FromString("min(max_pods, vcpus * 8)")),
			KubeReserved: map[string]string{"memory": "max_pods * 10"},
		})
		Expect(p.MaxPodsVars.MaxPods).To(BeNumerically("==", 29))
		Expect(p.ReservedVarsBuilt).To(BeTrue())
		Expect(p.ReservedVars.MaxPods).To(BeNumerically("==", 16))
		// 16 * 10 = 160, already a multiple of the 16Mi memory rounding granularity.
		Expect(expressionByField(p, "kubeReserved[memory]").Value).To(Equal("160Mi"))
	})

	It("should apply podsPerCore when capping the resolved maxPods", func() {
		p := preview(instancetype.KubeletExpressionInput{
			PodsPerCore:  lo.ToPtr(int32(5)),
			KubeReserved: map[string]string{"pid": "max_pods"},
		})
		// podsPerCore caps the default at 5 * 2 vCPUs = 10, below the ENI-limited 29.
		Expect(p.MaxPodsVars.MaxPods).To(BeNumerically("==", 10))
		Expect(expressionByField(p, "kubeReserved[pid]").Value).To(Equal("10"))
	})

	It("should not build the reserved variables when only maxPods holds an expression", func() {
		p := preview(instancetype.KubeletExpressionInput{
			MaxPods:      lo.ToPtr(intstr.FromString("vcpus * 8")),
			KubeReserved: map[string]string{"memory": "100Mi"},
		})
		Expect(p.ReservedVarsBuilt).To(BeFalse())
		Expect(p.ReservedVars).To(Equal(kubeletcel.InstanceTypeVars{}))
	})

	It("should attach the unit suffix implied by each reserved key", func() {
		p := preview(instancetype.KubeletExpressionInput{
			KubeReserved: map[string]string{
				"cpu":               "vcpus * 30",
				"memory":            "memory_mib / 100",
				"ephemeral-storage": "vcpus",
			},
			SystemReserved: map[string]string{"pid": "vcpus * 100"},
		})
		Expect(expressionByField(p, "kubeReserved[cpu]").Value).To(Equal("60m"))
		// 7808 / 100 = 78, rounded up to the next multiple of 16Mi.
		Expect(expressionByField(p, "kubeReserved[memory]").Value).To(Equal("80Mi"))
		Expect(expressionByField(p, "kubeReserved[ephemeral-storage]").Value).To(Equal("2Gi"))
		Expect(expressionByField(p, "systemReserved[pid]").Value).To(Equal("200"))
	})

	It("should omit static quantities, which are never evaluated", func() {
		p := preview(instancetype.KubeletExpressionInput{
			MaxPods: lo.ToPtr(intstr.FromInt32(50)),
			KubeReserved: map[string]string{
				"cpu":               "vcpus * 30",
				"memory":            "100Mi",
				"ephemeral-storage": "2Gi",
			},
		})
		Expect(lo.Map(p.Expressions, func(e instancetype.PreviewedExpression, _ int) string { return e.Field })).
			To(ConsistOf("kubeReserved[cpu]"))
	})

	It("should report a reserved expression that fails to evaluate as dropped", func() {
		p := preview(instancetype.KubeletExpressionInput{KubeReserved: map[string]string{"memory": "memory_mib / 0"}})
		e := expressionByField(p, "kubeReserved[memory]")
		Expect(e.Applied).To(BeFalse())
		Expect(e.Value).To(BeEmpty())
		Expect(e.Err).To(MatchError(ContainSubstring("division by zero")))
	})

	It("should report a reserved expression that goes negative as dropped", func() {
		p := preview(instancetype.KubeletExpressionInput{KubeReserved: map[string]string{"cpu": "0 - vcpus"}})
		e := expressionByField(p, "kubeReserved[cpu]")
		Expect(e.Applied).To(BeFalse())
		Expect(e.Err).To(MatchError(ContainSubstring("negative")))
	})

	It("should report a maxPods expression that overflows int32 as dropped", func() {
		p := preview(instancetype.KubeletExpressionInput{MaxPods: lo.ToPtr(intstr.FromString("vcpus * 2000000000"))})
		e := expressionByField(p, "maxPods")
		Expect(e.Applied).To(BeFalse())
		Expect(e.Err).To(MatchError(ContainSubstring("outside the valid range")))
	})

	// A dropped maxPods leaves the AMI family default in place, which is what the reserved expressions must
	// then see -- the same fallback a real launch takes.
	It("should evaluate reserved expressions against the default when maxPods is dropped", func() {
		p := preview(instancetype.KubeletExpressionInput{
			MaxPods:      lo.ToPtr(intstr.FromString("vcpus / 0")),
			KubeReserved: map[string]string{"pid": "max_pods"},
		})
		Expect(expressionByField(p, "maxPods").Applied).To(BeFalse())
		Expect(p.ReservedVars.MaxPods).To(BeNumerically("==", 29))
		Expect(expressionByField(p, "kubeReserved[pid]").Value).To(Equal("29"))
	})

	// Every dropped expression must carry a reason. A nil Err on a non-applied expression would nil-deref in
	// any caller that prints it, celtest included.
	It("should always pair a dropped expression with a reason", func() {
		p := preview(instancetype.KubeletExpressionInput{
			MaxPods:        lo.ToPtr(intstr.FromString("vcpus * 2000000000")),
			KubeReserved:   map[string]string{"cpu": "0 - vcpus", "memory": "memory_mib / 0"},
			SystemReserved: map[string]string{"pid": "vcpus"},
		})
		Expect(p.Expressions).To(HaveLen(4))
		for _, e := range p.Expressions {
			if !e.Applied {
				Expect(e.Err).ToNot(BeNil(), "expression %s was dropped without a reason", e.Field)
			}
		}
	})

	It("should report every expression as dropped when the NodeClassCEL gate is disabled", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			FeatureGates: test.FeatureGates{NodeClassCEL: lo.ToPtr(false)},
		}))
		p := preview(instancetype.KubeletExpressionInput{
			MaxPods:      lo.ToPtr(intstr.FromString("vcpus * 8")),
			KubeReserved: map[string]string{"cpu": "vcpus * 30", "memory": "100Mi"},
		})
		Expect(p.Expressions).To(HaveLen(2))
		for _, e := range p.Expressions {
			Expect(e.Applied).To(BeFalse())
			Expect(e.Err).To(MatchError(ContainSubstring("NodeClassCEL feature gate is disabled")))
		}
	})

	It("should reflect reserved ENIs in the default max_pods", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			ReservedENIs: lo.ToPtr(1),
			FeatureGates: test.FeatureGates{NodeClassCEL: lo.ToPtr(true)},
		}))
		p := preview(instancetype.KubeletExpressionInput{MaxPods: lo.ToPtr(intstr.FromString("max_pods"))})
		// One fewer usable ENI: (3 - 1) * (10 - 1) + 2 = 20.
		Expect(p.MaxPodsVars.MaxPods).To(BeNumerically("==", 20))
		Expect(expressionByField(p, "maxPods").Value).To(Equal("20"))
	})

	It("should report nothing for a configuration with no expressions", func() {
		p := preview(instancetype.KubeletExpressionInput{
			MaxPods:      lo.ToPtr(intstr.FromInt32(50)),
			KubeReserved: map[string]string{"memory": "100Mi"},
		})
		Expect(p.Expressions).To(BeEmpty())
	})
})

var _ = Describe("KubeletExpressionInput", func() {
	DescribeTable("HasExpressions",
		func(kc instancetype.KubeletExpressionInput, expected bool) {
			Expect(kc.HasExpressions()).To(Equal(expected))
		},
		Entry("empty", instancetype.KubeletExpressionInput{}, false),
		Entry("static maxPods", instancetype.KubeletExpressionInput{MaxPods: lo.ToPtr(intstr.FromInt32(50))}, false),
		Entry("string maxPods", instancetype.KubeletExpressionInput{MaxPods: lo.ToPtr(intstr.FromString("vcpus * 8"))}, true),
		Entry("static kubeReserved", instancetype.KubeletExpressionInput{KubeReserved: map[string]string{"memory": "100Mi"}}, false),
		Entry("kubeReserved expression", instancetype.KubeletExpressionInput{KubeReserved: map[string]string{"memory": "memory_mib / 100"}}, true),
		Entry("systemReserved expression", instancetype.KubeletExpressionInput{SystemReserved: map[string]string{"cpu": "vcpus * 30"}}, true),
		Entry("podsPerCore alone", instancetype.KubeletExpressionInput{PodsPerCore: lo.ToPtr(int32(8))}, false),
	)

	It("should list expressions labeled by field, in a stable order, excluding statics", func() {
		kc := instancetype.KubeletExpressionInput{
			MaxPods: lo.ToPtr(intstr.FromString("vcpus * 8")),
			KubeReserved: map[string]string{
				"memory":            "memory_mib / 100",
				"cpu":               "vcpus * 30",
				"ephemeral-storage": "2Gi",
			},
			SystemReserved: map[string]string{"pid": "vcpus * 100"},
		}
		Expect(kc.Expressions()).To(Equal([]instancetype.FieldExpression{
			{Field: "maxPods", Expression: "vcpus * 8"},
			{Field: "kubeReserved[cpu]", Expression: "vcpus * 30"},
			{Field: "kubeReserved[memory]", Expression: "memory_mib / 100"},
			{Field: "systemReserved[pid]", Expression: "vcpus * 100"},
		}))
	})

	It("should list no expressions for a static configuration", func() {
		kc := instancetype.KubeletExpressionInput{
			MaxPods:      lo.ToPtr(intstr.FromInt32(50)),
			KubeReserved: map[string]string{"memory": "100Mi", "cpu": "500m"},
		}
		Expect(kc.Expressions()).To(BeEmpty())
	})
})
