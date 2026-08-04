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

package v1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var _ = Describe("KubeletConfiguration Expression Detection", func() {
	// These predicates gate whether per-instance-type CEL evaluation runs at all. A false negative means
	// an expression silently skips validation, so the definition of "is an expression" must stay identical
	// to what the evaluators use: a string-typed maxPods, or a reserved value that isn't a valid quantity.
	Describe("HasExpressions", func() {
		It("should return false for a nil configuration", func() {
			var kc v1.KubeletConfiguration
			Expect(kc.HasExpressions()).To(BeFalse())
		})
		It("should return false for an empty configuration", func() {
			Expect(v1.KubeletConfiguration{}.HasExpressions()).To(BeFalse())
		})
		It("should return false when every value is static", func() {
			kc := v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				MaxPods:        lo.ToPtr(intstr.FromInt32(110)),
				KubeReserved:   map[string]string{"cpu": "100m", "memory": "256Mi"},
				SystemReserved: map[string]string{"cpu": "50m", "memory": "128Mi", "ephemeral-storage": "1Gi"},
			})
			Expect(kc.HasExpressions()).To(BeFalse())
		})
		It("should detect a string-typed maxPods expression", func() {
			kc := v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{MaxPods: lo.ToPtr(intstr.FromString("vcpus * 10"))})
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should not treat an integer maxPods as an expression", func() {
			kc := v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{MaxPods: lo.ToPtr(intstr.FromInt32(58))})
			Expect(kc.HasExpressions()).To(BeFalse())
		})
		It("should detect a kubeReserved expression", func() {
			kc := v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{KubeReserved: map[string]string{"cpu": "vcpus * 30"}})
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should detect a systemReserved expression", func() {
			kc := v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{SystemReserved: map[string]string{"memory": "max_pods * 11"}})
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should detect an expression mixed in with static quantities", func() {
			kc := v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				MaxPods: lo.ToPtr(intstr.FromInt32(110)),
				KubeReserved: map[string]string{
					"cpu":               "100m",
					"memory":            "((default_enis - 1) * (ips_per_eni - 1)) + 2",
					"ephemeral-storage": "1Gi",
				},
			})
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should ignore fields that never hold expressions", func() {
			// Only maxPods, kubeReserved and systemReserved are evaluated as CEL. Eviction thresholds
			// support percentages ("10%") that aren't valid quantities but are never CEL expressions.
			kc := v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				EvictionHard: map[string]string{"memory.available": "10%"},
				EvictionSoft: map[string]string{"memory.available": "15%"},
				PodsPerCore:  lo.ToPtr[int32](10),
			})
			Expect(kc.HasExpressions()).To(BeFalse())
		})
	})
	Describe("HasResourceExpressions", func() {
		It("should return false for a nil configuration", func() {
			var kc v1.KubeletConfiguration
			Expect(kc.HasResourceExpressions()).To(BeFalse())
		})
		It("should return false when a maxPods expression is the only expression", func() {
			kc := v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				MaxPods:      lo.ToPtr(intstr.FromString("vcpus * 10")),
				KubeReserved: map[string]string{"cpu": "100m"},
			})
			Expect(kc.HasResourceExpressions()).To(BeFalse(), "a maxPods expression alone needs no reserved-capacity evaluation")
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should detect expressions in either reserved map", func() {
			Expect(v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				KubeReserved: map[string]string{"cpu": "vcpus * 30"},
			}).HasResourceExpressions()).To(BeTrue())
			Expect(v1.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				SystemReserved: map[string]string{"cpu": "vcpus * 30"},
			}).HasResourceExpressions()).To(BeTrue())
		})
	})
})
