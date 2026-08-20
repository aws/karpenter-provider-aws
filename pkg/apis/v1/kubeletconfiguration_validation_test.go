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
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

// These cover the rules the API server can't: spec.kubelet is a preserve-unknown-fields map, so
// nothing below is enforced at admission. A gap here is a configuration that reaches a node.
var _ = Describe("ValidateKubeletConfig", func() {
	DescribeTable(
		"should accept",
		func(kubelet v1.KubeletConfiguration) {
			Expect(v1.ValidateKubeletConfig(kubelet)).To(BeEmpty())
		},
		Entry("an empty configuration", v1.KubeletConfiguration{}),
		Entry("a nil configuration", nil),
		Entry("the fields Karpenter reads for scheduling", v1.KubeletConfiguration{
			"maxPods":        v1.JSONValue(110),
			"podsPerCore":    v1.JSONValue(10),
			"kubeReserved":   v1.JSONValue(map[string]string{"cpu": "200m", "memory": "100Mi"}),
			"systemReserved": v1.JSONValue(map[string]string{"ephemeral-storage": "1Gi", "pid": "1000"}),
			"evictionHard":   v1.JSONValue(map[string]string{"memory.available": "5%"}),
		}),
		// A field Karpenter has no Go representation for is accepted purely because the upstream
		// type declares it. This is the passthrough guarantee the open map exists to provide.
		Entry("a field only the kubelet library knows about", v1.KubeletConfiguration{
			"serializeImagePulls": v1.JSONValue(false),
		}),
		Entry("a nested subtree", v1.KubeletConfiguration{
			"logging": v1.JSONValue(map[string]any{
				"format":         "json",
				"flushFrequency": "5s",
				"options":        map[string]any{"json": map[string]any{"infoBufferSize": "100Mi", "splitStream": true}},
			}),
		}),
		Entry("an eviction threshold as a quantity rather than a percentage", v1.KubeletConfiguration{
			"evictionHard": v1.JSONValue(map[string]string{"nodefs.available": "500Mi"}),
		}),
		Entry("matching evictionSoft and evictionSoftGracePeriod", v1.KubeletConfiguration{
			"evictionSoft":            v1.JSONValue(map[string]string{"memory.available": "10%"}),
			"evictionSoftGracePeriod": v1.JSONValue(map[string]string{"memory.available": "1m"}),
		}),
		Entry("imageGC thresholds in the correct order", v1.KubeletConfiguration{
			"imageGCHighThresholdPercent": v1.JSONValue(80),
			"imageGCLowThresholdPercent":  v1.JSONValue(60),
		}),
		Entry("zero-valued fields", v1.KubeletConfiguration{
			"maxPods":                   v1.JSONValue(0),
			"podsPerCore":               v1.JSONValue(0),
			"evictionMaxPodGracePeriod": v1.JSONValue(0),
		}),
	)
	// The three fields Karpenter widens beyond the upstream type, because the value is resolved
	// against a specific instance type before the kubelet ever sees it. An expression must not be
	// mistaken for a malformed quantity or a type error.
	DescribeTable(
		"should accept a CEL expression for",
		func(kubelet v1.KubeletConfiguration) {
			Expect(v1.ValidateKubeletConfig(kubelet)).To(BeEmpty())
		},
		Entry("maxPods", v1.KubeletConfiguration{"maxPods": v1.JSONValue("vcpus * 10")}),
		Entry("kubeReserved", v1.KubeletConfiguration{
			"kubeReserved": v1.JSONValue(map[string]string{"memory": "memory * 0.1"}),
		}),
		Entry("systemReserved", v1.KubeletConfiguration{
			"systemReserved": v1.JSONValue(map[string]string{"cpu": "vcpus * 10m"}),
		}),
		// An expression in one field can't cost the validation of the others: the whole config is
		// one decode, so a field that doesn't fit the upstream type would otherwise mask the rest.
		Entry("maxPods alongside other fields", v1.KubeletConfiguration{
			"maxPods":     v1.JSONValue("vcpus * 10"),
			"podsPerCore": v1.JSONValue(10),
		}),
	)
	DescribeTable(
		"should reject",
		func(kubelet v1.KubeletConfiguration) {
			Expect(v1.ValidateKubeletConfig(kubelet)).ToNot(BeEmpty())
		},
		Entry("a field the kubelet library doesn't declare", v1.KubeletConfiguration{
			"notAKubeletField": v1.JSONValue(true),
		}),
		Entry("an unknown field nested in a subtree", v1.KubeletConfiguration{
			"logging": v1.JSONValue(map[string]any{"bogusLoggingField": true}),
		}),
		Entry("an unknown field nested three levels deep", v1.KubeletConfiguration{
			"logging": v1.JSONValue(map[string]any{
				"options": map[string]any{"json": map[string]any{"nope": "x"}},
			}),
		}),
		Entry("a field with the wrong type", v1.KubeletConfiguration{
			"podsPerCore": v1.JSONValue("ten"),
		}),
		Entry("a nested field with the wrong type", v1.KubeletConfiguration{
			"logging": v1.JSONValue(map[string]any{"format": 12345}),
		}),
		Entry("arbitrary junk", v1.KubeletConfiguration{
			"a": v1.JSONValue(map[string]any{"b": []any{1, "two", true}}),
		}),
	)
	DescribeTable(
		"should reject a negative",
		func(field string) {
			Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{field: v1.JSONValue(-1)})).ToNot(BeEmpty())
		},
		Entry("maxPods", "maxPods"),
		Entry("podsPerCore", "podsPerCore"),
		Entry("evictionMaxPodGracePeriod", "evictionMaxPodGracePeriod"),
	)
	DescribeTable(
		"should reject an out-of-range percentage for",
		func(field string, value int) {
			Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{field: v1.JSONValue(value)})).ToNot(BeEmpty())
		},
		Entry("imageGCHighThresholdPercent above 100", "imageGCHighThresholdPercent", 101),
		Entry("imageGCHighThresholdPercent below 0", "imageGCHighThresholdPercent", -1),
		Entry("imageGCLowThresholdPercent above 100", "imageGCLowThresholdPercent", 101),
		Entry("imageGCLowThresholdPercent below 0", "imageGCLowThresholdPercent", -1),
	)
	DescribeTable(
		"should reject an unreservable resource in",
		func(field string) {
			kubelet := v1.KubeletConfiguration{field: v1.JSONValue(map[string]string{"gpu": "1"})}
			Expect(v1.ValidateKubeletConfig(kubelet)).ToNot(BeEmpty())
		},
		Entry("kubeReserved", "kubeReserved"),
		Entry("systemReserved", "systemReserved"),
	)
	DescribeTable(
		"should reject a negative reservation in",
		func(field string) {
			kubelet := v1.KubeletConfiguration{field: v1.JSONValue(map[string]string{"cpu": "-1"})}
			Expect(v1.ValidateKubeletConfig(kubelet)).ToNot(BeEmpty())
		},
		Entry("kubeReserved", "kubeReserved"),
		Entry("systemReserved", "systemReserved"),
	)
	// The kubelet ignores a key it doesn't recognize as an eviction signal, so accepting one
	// would silently drop the threshold the user asked for.
	DescribeTable(
		"should reject an invalid eviction signal in",
		func(field string, value string) {
			kubelet := v1.KubeletConfiguration{field: v1.JSONValue(map[string]string{"notASignal": value})}
			Expect(v1.ValidateKubeletConfig(kubelet)).ToNot(BeEmpty())
		},
		Entry("evictionHard", "evictionHard", "5%"),
		Entry("evictionSoft", "evictionSoft", "5%"),
		Entry("evictionMinimumReclaim", "evictionMinimumReclaim", "500Mi"),
		Entry("evictionSoftGracePeriod", "evictionSoftGracePeriod", "1m"),
	)
	DescribeTable(
		"should reject an eviction threshold that is neither a percentage nor a quantity in",
		func(field string) {
			kubelet := v1.KubeletConfiguration{field: v1.JSONValue(map[string]string{"memory.available": "lots"})}
			Expect(v1.ValidateKubeletConfig(kubelet)).ToNot(BeEmpty())
		},
		Entry("evictionHard", "evictionHard"),
		Entry("evictionSoft", "evictionSoft"),
		Entry("evictionMinimumReclaim", "evictionMinimumReclaim"),
	)
	It("should reject an evictionSoftGracePeriod that isn't a duration", func() {
		// Upstream types this as a plain map[string]string, so a malformed value would otherwise
		// only fail once the kubelet parsed it on the node.
		Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"evictionSoft":            v1.JSONValue(map[string]string{"memory.available": "10%"}),
			"evictionSoftGracePeriod": v1.JSONValue(map[string]string{"memory.available": "soon"}),
		})).ToNot(BeEmpty())
	})
	It("should reject a negative evictionSoftGracePeriod", func() {
		Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"evictionSoft":            v1.JSONValue(map[string]string{"memory.available": "10%"}),
			"evictionSoftGracePeriod": v1.JSONValue(map[string]string{"memory.available": "-1m"}),
		})).ToNot(BeEmpty())
	})
	It("should reject an evictionSoft threshold with no matching grace period", func() {
		// The kubelet ignores a soft threshold it has no grace period for, so the node would
		// never evict on the signal the user configured.
		Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"evictionSoft":            v1.JSONValue(map[string]string{"memory.available": "10%"}),
			"evictionSoftGracePeriod": v1.JSONValue(map[string]string{"nodefs.available": "1m"}),
		})).ToNot(BeEmpty())
	})
	It("should reject an evictionSoftGracePeriod with no matching threshold", func() {
		Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"evictionSoftGracePeriod": v1.JSONValue(map[string]string{"memory.available": "1m"}),
		})).ToNot(BeEmpty())
	})
	It("should reject evictionSoft with no evictionSoftGracePeriod at all", func() {
		Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"evictionSoft": v1.JSONValue(map[string]string{"memory.available": "10%"}),
		})).ToNot(BeEmpty())
	})
	It("should reject an imageGC high threshold at or below the low threshold", func() {
		// The kubelet collects above the high threshold and stops at the low one, so an inverted
		// pair never collects.
		Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"imageGCHighThresholdPercent": v1.JSONValue(60),
			"imageGCLowThresholdPercent":  v1.JSONValue(80),
		})).ToNot(BeEmpty())
		Expect(v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"imageGCHighThresholdPercent": v1.JSONValue(70),
			"imageGCLowThresholdPercent":  v1.JSONValue(70),
		})).ToNot(BeEmpty())
	})
	// registerWithTaints is a legitimate kubelet field, so the decode against the upstream type
	// accepts it. Bootstrap then overwrites it with the NodeClaim's taints, which would discard the
	// user's value on the way to the node -- rejecting it is what keeps that from being silent.
	It("should reject a field Karpenter sets itself", func() {
		errs := v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"registerWithTaints": v1.JSONValue([]map[string]string{
				{"key": "example.com/dedicated", "value": "gpu", "effect": "NoSchedule"},
			}),
		})
		Expect(errs).To(HaveLen(1))
		Expect(errs[0].Error()).To(ContainSubstring("registerWithTaints"))
	})
	It("should report every violation rather than stopping at the first", func() {
		// The user sees these as one status condition message, so a config with several
		// mistakes shouldn't take several apply-and-wait cycles to fix.
		errs := v1.ValidateKubeletConfig(v1.KubeletConfiguration{
			"podsPerCore":  v1.JSONValue(-1),
			"kubeReserved": v1.JSONValue(map[string]string{"gpu": "1", "cpu": "-1"}),
			"evictionHard": v1.JSONValue(map[string]string{"notASignal": "5%"}),
		})
		Expect(len(errs)).To(BeNumerically(">=", 4))
	})
	It("should name the offending field in the error", func() {
		// The message is the only thing the user gets, since nothing rejected their apply.
		errs := v1.ValidateKubeletConfig(v1.KubeletConfiguration{"notAKubeletField": v1.JSONValue(true)})
		Expect(errs).To(HaveLen(1))
		Expect(errs[0].Error()).To(ContainSubstring("notAKubeletField"))
	})
})

// Every consumer of the kubelet config -- instance type resolution and launch template resolution alike --
// treats a ParseKubeletConfig failure as fatal, on the premise that ValidateKubeletConfig has already
// rejected anything that can't decode and reported it on the EC2NodeClass. That premise is what keeps the
// fatal path unreachable in normal operation, and nothing in the types enforces it:
// ParsedKubeletConfig may type a field more strictly than the upstream KubeletConfiguration does, and
// then only a hand-written semantic rule closes the gap. evictionSoftGracePeriod is that case today --
// upstream types it map[string]string, so "5x" decodes there and is caught only by
// validateEvictionGracePeriod. Deleting a rule like that one would open the gap silently, which is what
// these specs exist to prevent.
var _ = Describe("ValidateKubeletConfig/ParseKubeletConfig invariant", func() {
	DescribeTable(
		"should reject anything ParseKubeletConfig can't decode",
		func(kubelet v1.KubeletConfiguration) {
			// Asserted rather than assumed: an entry that no longer fails to decode silently stops
			// testing the invariant instead of failing.
			_, err := v1.ParseKubeletConfig(kubelet)
			Expect(err).To(HaveOccurred(), "this input no longer fails ParseKubeletConfig, so the entry no longer covers the invariant")
			Expect(v1.ValidateKubeletConfig(kubelet)).ToNot(BeEmpty(),
				"ParseKubeletConfig rejects this but ValidateKubeletConfig accepts it, so it would reach the read path and be replaced by defaults with the user's value silently dropped")
		},
		// The one field ParsedKubeletConfig types more strictly than upstream, so the upstream decode
		// can't catch it and a semantic rule must.
		Entry("evictionSoftGracePeriod that isn't a duration", v1.KubeletConfiguration{
			"evictionSoft":            v1.JSONValue(map[string]string{"memory.available": "5%"}),
			"evictionSoftGracePeriod": v1.JSONValue(map[string]string{"memory.available": "5x"}),
		}),
		Entry("clusterDNS as a string rather than a list", v1.KubeletConfiguration{"clusterDNS": v1.JSONValue("10.0.0.10")}),
		Entry("maxPods as a bool", v1.KubeletConfiguration{"maxPods": v1.JSONValue(true)}),
		Entry("podsPerCore as a string", v1.KubeletConfiguration{"podsPerCore": v1.JSONValue("10")}),
		Entry("kubeReserved with non-string values", v1.KubeletConfiguration{"kubeReserved": v1.JSONValue(map[string]int{"cpu": 1})}),
		Entry("systemReserved with non-string values", v1.KubeletConfiguration{"systemReserved": v1.JSONValue(map[string]int{"cpu": 1})}),
		Entry("evictionHard with non-string values", v1.KubeletConfiguration{"evictionHard": v1.JSONValue(map[string]int{"memory.available": 5})}),
		Entry("evictionSoft with non-string values", v1.KubeletConfiguration{"evictionSoft": v1.JSONValue(map[string]int{"memory.available": 5})}),
		Entry("evictionMaxPodGracePeriod as a string", v1.KubeletConfiguration{"evictionMaxPodGracePeriod": v1.JSONValue("30")}),
		Entry("imageGCHighThresholdPercent as a string", v1.KubeletConfiguration{"imageGCHighThresholdPercent": v1.JSONValue("80")}),
		Entry("imageGCLowThresholdPercent as a string", v1.KubeletConfiguration{"imageGCLowThresholdPercent": v1.JSONValue("70")}),
		Entry("cpuCFSQuota as a string", v1.KubeletConfiguration{"cpuCFSQuota": v1.JSONValue("true")}),
	)
	// The table only covers the fields someone thought to list. A new field on ParsedKubeletConfig is
	// precisely how the invariant would break -- one typed more strictly than upstream, with no semantic
	// rule behind it -- so adding one has to fail here rather than pass unnoticed.
	It("should have an entry above for every field of ParsedKubeletConfig", func() {
		covered := []string{
			"clusterDNS", "cpuCFSQuota", "evictionHard", "evictionMaxPodGracePeriod", "evictionSoft",
			"evictionSoftGracePeriod", "imageGCHighThresholdPercent", "imageGCLowThresholdPercent",
			"kubeReserved", "maxPods", "podsPerCore", "systemReserved",
		}
		var fields []string
		t := reflect.TypeOf(v1.ParsedKubeletConfig{})
		for i := 0; i < t.NumField(); i++ {
			if name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ","); name != "" && name != "-" {
				fields = append(fields, name)
			}
		}
		Expect(fields).To(ConsistOf(covered),
			"ParsedKubeletConfig gained or lost a field: add an entry to the table above that fails ParseKubeletConfig for it, and confirm ValidateKubeletConfig rejects that input too")
	})
})
