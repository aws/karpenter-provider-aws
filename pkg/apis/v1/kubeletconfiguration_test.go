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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"github.com/aws/karpenter-provider-aws/pkg/test"
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
			kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				MaxPods:        lo.ToPtr(intstr.FromInt32(110)),
				KubeReserved:   map[string]string{"cpu": "100m", "memory": "256Mi"},
				SystemReserved: map[string]string{"cpu": "50m", "memory": "128Mi", "ephemeral-storage": "1Gi"},
			})
			Expect(kc.HasExpressions()).To(BeFalse())
		})
		It("should detect a string-typed maxPods expression", func() {
			kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{MaxPods: lo.ToPtr(intstr.FromString("vcpus * 10"))})
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should not treat an integer maxPods as an expression", func() {
			kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{MaxPods: lo.ToPtr(intstr.FromInt32(58))})
			Expect(kc.HasExpressions()).To(BeFalse())
		})
		It("should detect a kubeReserved expression", func() {
			kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{KubeReserved: map[string]string{"cpu": "vcpus * 30"}})
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should detect a systemReserved expression", func() {
			kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{SystemReserved: map[string]string{"memory": "max_pods * 11"}})
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should detect an expression mixed in with static quantities", func() {
			kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
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
			kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
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
			kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				MaxPods:      lo.ToPtr(intstr.FromString("vcpus * 10")),
				KubeReserved: map[string]string{"cpu": "100m"},
			})
			Expect(kc.HasResourceExpressions()).To(BeFalse(), "a maxPods expression alone needs no reserved-capacity evaluation")
			Expect(kc.HasExpressions()).To(BeTrue())
		})
		It("should detect expressions in either reserved map", func() {
			Expect(test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				KubeReserved: map[string]string{"cpu": "vcpus * 30"},
			}).HasResourceExpressions()).To(BeTrue())
			Expect(test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
				SystemReserved: map[string]string{"cpu": "vcpus * 30"},
			}).HasResourceExpressions()).To(BeTrue())
		})
	})
})

var _ = Describe("ParseKubeletConfig", func() {
	// Extraction is the boundary between the open map and the 12 fields Karpenter interprets. A bug here
	// either drops a field Karpenter needs for scheduling, or lets a malformed value through to a node.
	It("should return an empty struct for a nil configuration", func() {
		var kc v1.KubeletConfiguration
		parsed, err := v1.ParseKubeletConfig(kc)
		Expect(err).ToNot(HaveOccurred())
		Expect(parsed).To(Equal(&v1.ParsedKubeletConfig{}))
	})
	It("should return an empty struct for an empty configuration", func() {
		parsed, err := v1.ParseKubeletConfig(v1.KubeletConfiguration{})
		Expect(err).ToNot(HaveOccurred())
		Expect(parsed).To(Equal(&v1.ParsedKubeletConfig{}))
	})
	It("should extract every interpreted field", func() {
		parsed, err := v1.ParseKubeletConfig(v1.KubeletConfiguration{
			"clusterDNS":                  v1.JSONValue([]string{"10.0.0.10"}),
			"maxPods":                     v1.JSONValue(110),
			"podsPerCore":                 v1.JSONValue(10),
			"systemReserved":              v1.JSONValue(map[string]string{"cpu": "100m"}),
			"kubeReserved":                v1.JSONValue(map[string]string{"memory": "256Mi"}),
			"evictionHard":                v1.JSONValue(map[string]string{"memory.available": "5%"}),
			"evictionSoft":                v1.JSONValue(map[string]string{"memory.available": "10%"}),
			"evictionSoftGracePeriod":     v1.JSONValue(map[string]string{"memory.available": "1m"}),
			"evictionMaxPodGracePeriod":   v1.JSONValue(30),
			"imageGCHighThresholdPercent": v1.JSONValue(80),
			"imageGCLowThresholdPercent":  v1.JSONValue(60),
			"cpuCFSQuota":                 v1.JSONValue(false),
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(parsed.ClusterDNS).To(Equal([]string{"10.0.0.10"}))
		Expect(parsed.MaxPods).To(Equal(lo.ToPtr(intstr.FromInt32(110))))
		Expect(parsed.PodsPerCore).To(Equal(lo.ToPtr[int32](10)))
		Expect(parsed.SystemReserved).To(Equal(map[string]string{"cpu": "100m"}))
		Expect(parsed.KubeReserved).To(Equal(map[string]string{"memory": "256Mi"}))
		Expect(parsed.EvictionHard).To(Equal(map[string]string{"memory.available": "5%"}))
		Expect(parsed.EvictionSoft).To(Equal(map[string]string{"memory.available": "10%"}))
		Expect(parsed.EvictionMaxPodGracePeriod).To(Equal(lo.ToPtr[int32](30)))
		Expect(parsed.ImageGCHighThresholdPercent).To(Equal(lo.ToPtr[int32](80)))
		Expect(parsed.ImageGCLowThresholdPercent).To(Equal(lo.ToPtr[int32](60)))
		Expect(parsed.CPUCFSQuota).To(Equal(lo.ToPtr(false)))
	})
	It("should ignore passthrough fields it has no representation for", func() {
		// A field only the kubelet library knows about parses cleanly into an empty struct: extraction
		// pulls the 12 interpreted fields and leaves everything else to the raw map.
		parsed, err := v1.ParseKubeletConfig(v1.KubeletConfiguration{
			"serializeImagePulls":   v1.JSONValue(false),
			"topologyManagerPolicy": v1.JSONValue("best-effort"),
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(parsed).To(Equal(&v1.ParsedKubeletConfig{}))
	})
	It("should keep maxPods as a string when it holds a CEL expression", func() {
		// maxPods is typed IntOrString precisely so an expression parses rather than failing the whole
		// document. It stays a string here and is resolved per instance type downstream.
		parsed, err := v1.ParseKubeletConfig(v1.KubeletConfiguration{"maxPods": v1.JSONValue("vcpus * 10")})
		Expect(err).ToNot(HaveOccurred())
		Expect(parsed.MaxPods.Type).To(Equal(intstr.String))
		Expect(parsed.MaxPods.StrVal).To(Equal("vcpus * 10"))
		_, ok := parsed.MaxPodsValue()
		Expect(ok).To(BeFalse(), "an expression has no concrete value until evaluated against an instance type")
	})
	It("should return an error for a field that doesn't decode", func() {
		// A single wrong-typed field fails the one Unmarshal. The design leans on callers falling back to
		// an empty struct rather than making scheduling decisions from a partial decode.
		_, err := v1.ParseKubeletConfig(v1.KubeletConfiguration{"podsPerCore": v1.JSONValue("ten")})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("MaxPodsValue", func() {
	It("should report a concrete integer", func() {
		parsed := &v1.ParsedKubeletConfig{MaxPods: lo.ToPtr(intstr.FromInt32(58))}
		value, ok := parsed.MaxPodsValue()
		Expect(ok).To(BeTrue())
		Expect(lo.FromPtr(value)).To(BeNumerically("==", 58))
	})
	It("should report not-set for a nil receiver, unset maxPods, or an expression", func() {
		var nilParsed *v1.ParsedKubeletConfig
		_, ok := nilParsed.MaxPodsValue()
		Expect(ok).To(BeFalse())

		_, ok = (&v1.ParsedKubeletConfig{}).MaxPodsValue()
		Expect(ok).To(BeFalse())

		_, ok = (&v1.ParsedKubeletConfig{MaxPods: lo.ToPtr(intstr.FromString("vcpus * 10"))}).MaxPodsValue()
		Expect(ok).To(BeFalse())
	})
})

// maxPodsInt resolves maxPods to a literal *int32, or nil when it's unset, a CEL expression, or a config
// that fails to decode. It's a test-only convenience over the exported API; production reads the resolved
// value straight off the parsed struct via MaxPodsValue.
func maxPodsInt(kc v1.KubeletConfiguration) *int32 {
	parsed, err := v1.ParseKubeletConfig(kc)
	if err != nil {
		return nil
	}
	value, _ := parsed.MaxPodsValue()
	return value
}

var _ = Describe("maxPodsInt", func() {
	It("should return the integer value for a literal maxPods", func() {
		kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{MaxPods: lo.ToPtr(intstr.FromInt32(58))})
		Expect(lo.FromPtr(maxPodsInt(kc))).To(BeNumerically("==", 58))
	})
	It("should return nil for an expression, and for a config that fails to decode", func() {
		// maxPodsInt swallows a decode error and reports unset rather than surfacing it; validation reports
		// the malformed config separately.
		Expect(maxPodsInt(test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
			MaxPods: lo.ToPtr(intstr.FromString("vcpus * 10")),
		}))).To(BeNil())
		Expect(maxPodsInt(v1.KubeletConfiguration{"podsPerCore": v1.JSONValue("ten")})).To(BeNil())
	})
})

var _ = Describe("ParsedKubeletConfig DeepCopy", func() {
	It("should produce an independent copy", func() {
		original := &v1.ParsedKubeletConfig{
			MaxPods:      lo.ToPtr(intstr.FromInt32(110)),
			CPUCFSQuota:  lo.ToPtr(true),
			KubeReserved: map[string]string{"cpu": "100m"},
		}
		clone := original.DeepCopy()
		Expect(clone).To(Equal(original))

		// Mutating the clone's map and pointer fields must not reach back into the original.
		clone.KubeReserved["cpu"] = "200m"
		clone.MaxPods = lo.ToPtr(intstr.FromInt32(58))
		clone.CPUCFSQuota = lo.ToPtr(false)
		Expect(original.KubeReserved["cpu"]).To(Equal("100m"))
		Expect(original.MaxPods).To(Equal(lo.ToPtr(intstr.FromInt32(110))))
		Expect(lo.FromPtr(original.CPUCFSQuota)).To(BeTrue())
	})
	It("should return nil for a nil receiver", func() {
		var nilParsed *v1.ParsedKubeletConfig
		Expect(nilParsed.DeepCopy()).To(BeNil())
	})
})

var _ = Describe("UnmanagedKubeletFields", func() {
	// These are the fields Karpenter extracts into ParsedKubeletConfig and applies via bootstrap. If a
	// field is added there but this list isn't updated, the reflection-derived managed set would still
	// pick it up -- this spec guards that derivation against silently treating a mapped field as unmanaged.
	It("should treat every field Karpenter maps as managed", func() {
		kc := test.MustMakeKubeletConfiguration(v1.ParsedKubeletConfig{
			ClusterDNS:                  []string{"10.0.0.10"},
			MaxPods:                     lo.ToPtr(intstr.FromInt32(110)),
			PodsPerCore:                 lo.ToPtr[int32](2),
			SystemReserved:              map[string]string{"cpu": "50m"},
			KubeReserved:                map[string]string{"cpu": "100m"},
			EvictionHard:                map[string]string{"memory.available": "5%"},
			EvictionSoft:                map[string]string{"memory.available": "10%"},
			EvictionSoftGracePeriod:     map[string]metav1.Duration{"memory.available": {Duration: time.Minute}},
			EvictionMaxPodGracePeriod:   lo.ToPtr[int32](60),
			ImageGCHighThresholdPercent: lo.ToPtr[int32](80),
			ImageGCLowThresholdPercent:  lo.ToPtr[int32](60),
			CPUCFSQuota:                 lo.ToPtr(true),
		})
		Expect(v1.UnmanagedKubeletFields(kc)).To(BeEmpty())
	})
	It("should return passthrough fields Karpenter doesn't map, sorted", func() {
		kc := v1.KubeletConfiguration{
			"maxPods":             v1.JSONValue(110),
			"serializeImagePulls": v1.JSONValue(false),
			"registryPullQPS":     v1.JSONValue(int32(10)),
		}
		Expect(v1.UnmanagedKubeletFields(kc)).To(Equal([]string{"registryPullQPS", "serializeImagePulls"}))
	})
	It("should return empty for a nil or empty configuration", func() {
		Expect(v1.UnmanagedKubeletFields(nil)).To(BeEmpty())
		Expect(v1.UnmanagedKubeletFields(v1.KubeletConfiguration{})).To(BeEmpty())
	})
})
