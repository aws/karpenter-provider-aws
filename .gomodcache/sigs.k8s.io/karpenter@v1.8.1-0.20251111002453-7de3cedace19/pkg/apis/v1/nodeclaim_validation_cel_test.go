/*
Copyright The Kubernetes Authors.

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
	"strconv"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/operatorpkg/object"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	. "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
)

var _ = Describe("Validation", func() {
	var nodeClaim *NodeClaim

	BeforeEach(func() {
		if env.Version.Minor() < 25 {
			Skip("CEL Validation is for 1.25>")
		}
		nodeClaim = &NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: NodeClaimSpec{
				NodeClassRef: &NodeClassReference{
					Group: "karpenter.test.sh",
					Kind:  "TestNodeClaim",
					Name:  "default",
				},
				Requirements: []NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpExists,
						},
					},
				},
			},
		}
		nodeClaim.SetGroupVersionKind(object.GVK(nodeClaim)) // This is needed so that the GVK is set on the unstructured object
	})

	Context("Taints", func() {
		It("should succeed for valid taints", func() {
			nodeClaim.Spec.Taints = []v1.Taint{
				{Key: "a", Value: "b", Effect: v1.TaintEffectNoSchedule},
				{Key: "c", Value: "d", Effect: v1.TaintEffectNoExecute},
				{Key: "e", Value: "f", Effect: v1.TaintEffectPreferNoSchedule},
				{Key: "key-only", Effect: v1.TaintEffectNoExecute},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
		})
		It("should fail for invalid taint keys", func() {
			nodeClaim.Spec.Taints = []v1.Taint{{Key: "???"}}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
		It("should fail for missing taint key", func() {
			nodeClaim.Spec.Taints = []v1.Taint{{Effect: v1.TaintEffectNoSchedule}}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
		It("should fail for invalid taint value", func() {
			nodeClaim.Spec.Taints = []v1.Taint{{Key: "invalid-value", Effect: v1.TaintEffectNoSchedule, Value: "???"}}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
		It("should fail for invalid taint effect", func() {
			nodeClaim.Spec.Taints = []v1.Taint{{Key: "invalid-effect", Effect: "???"}}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
		It("should not fail for same key with different effects", func() {
			nodeClaim.Spec.Taints = []v1.Taint{
				{Key: "a", Effect: v1.TaintEffectNoSchedule},
				{Key: "a", Effect: v1.TaintEffectNoExecute},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
		})
	})
	Context("NodeClassRef", func() {
		It("should succeed for valid group", func() {
			nodeClaim.Spec.NodeClassRef = &NodeClassReference{
				Kind:  object.GVK(&v1alpha1.TestNodeClass{}).Kind,
				Name:  "nodeclass-test",
				Group: object.GVK(&v1alpha1.TestNodeClass{}).Group,
			}
			Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
		})
		It("should fail for invalid group", func() {
			nodeClaim.Spec.NodeClassRef = &NodeClassReference{
				Kind:  object.GVK(&v1alpha1.TestNodeClass{}).Kind,
				Name:  "nodeclass-test",
				Group: "karpenter.test.sh/v1",
			}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
	})
	Context("Requirements", func() {
		It("should allow supported ops", func() {
			nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{"1"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{"1"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists}},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
		})
		It("should fail for unsupported ops", func() {
			for _, op := range []v1.NodeSelectorOperator{"unknown"} {
				nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
					{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: op, Values: []string{"test"}}},
				}
				Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
			}
		})
		It("should fail for restricted domains", func() {
			for label := range RestrictedLabelDomains {
				nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
					{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: label + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}},
				}
				Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
			}
		})
		It("should allow restricted domains exceptions", func() {
			oldNodeClaim := nodeClaim.DeepCopy()
			for label := range LabelDomainExceptions {
				nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
					{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: label + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}},
				}
				Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
				Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
				nodeClaim = oldNodeClaim.DeepCopy()
			}
		})
		It("should allow restricted subdomains exceptions", func() {
			oldNodeClaim := nodeClaim.DeepCopy()
			for label := range LabelDomainExceptions {
				nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
					{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: "subdomain." + label + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}},
				}
				Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
				Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
				nodeClaim = oldNodeClaim.DeepCopy()
			}
		})
		It("should allow well known label exceptions", func() {
			oldNodeClaim := nodeClaim.DeepCopy()
			for label := range WellKnownLabels.Difference(sets.New(NodePoolLabelKey)) {
				nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
					{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: label, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}},
				}
				Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
				Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
				nodeClaim = oldNodeClaim.DeepCopy()
			}
		})
		It("should allow non-empty set after removing overlapped value", func() {
			nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test", "bar"}}},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
		})
		It("should allow empty requirements", func() {
			nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{}
			Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
		})
		It("should fail with invalid GT or LT values", func() {
			for _, requirement := range []NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{"1", "2"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{"a"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{"-1"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{"1", "2"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{"a"}}},
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{"-1"}}},
			} {
				nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{requirement}
				Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
			}
		})
		It("should error when minValues is negative", func() {
			nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"insance-type-1"}}, MinValues: lo.ToPtr(-1)},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
		It("should error when minValues is zero", func() {
			nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"insance-type-1"}}, MinValues: lo.ToPtr(0)},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
		It("should error when minValues is more than 50", func() {
			nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpExists}, MinValues: lo.ToPtr(51)},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
		It("should allow more than 50 values if minValues is not specified.", func() {
			var instanceTypes []string
			for i := 0; i < 90; i++ {
				instanceTypes = append(instanceTypes, "instance"+strconv.Itoa(i))
			}
			nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: instanceTypes}},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).To(Succeed())
		})
		It("should error when minValues is greater than the number of unique values specified within In operator", func() {
			nodeClaim.Spec.Requirements = []NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"insance-type-1"}}, MinValues: lo.ToPtr(2)},
			}
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
		It("should error when requirements is greater than 100", func() {
			var req []NodeSelectorRequirementWithMinValues
			for i := 0; i < 101; i++ {
				req = append(req, NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: test.RandomName(), Operator: v1.NodeSelectorOpIn, Values: []string{test.RandomName()}}})
			}
			nodeClaim.Spec.Requirements = req
			Expect(env.Client.Create(ctx, nodeClaim)).ToNot(Succeed())
		})
	})
	Context("TerminationGracePeriod", func() {
		DescribeTable("should succeed on a valid terminationGracePeriod", func(value string) {
			u := lo.Must(runtime.DefaultUnstructuredConverter.ToUnstructured(nodeClaim))
			lo.Must0(unstructured.SetNestedField(u, value, "spec", "terminationGracePeriod"))
			obj := &unstructured.Unstructured{}
			lo.Must0(runtime.DefaultUnstructuredConverter.FromUnstructured(u, obj))

			Expect(env.Client.Create(ctx, obj)).To(Succeed())
		},
			Entry("single unit", "30s"),
			Entry("multiple units", "1h30m5s"),
		)
		DescribeTable("should fail on an invalid terminationGracePeriod", func(value string) {
			u := lo.Must(runtime.DefaultUnstructuredConverter.ToUnstructured(nodeClaim))
			lo.Must0(unstructured.SetNestedField(u, value, "spec", "terminationGracePeriod"))
			obj := &unstructured.Unstructured{}
			lo.Must0(runtime.DefaultUnstructuredConverter.FromUnstructured(u, obj))

			Expect(env.Client.Create(ctx, obj)).To(Not(Succeed()))
		},
			Entry("negative", "-1s"),
			Entry("invalid unit", "1hr"),
			Entry("never", "Never"),
			Entry("partial match", "FooNever"),
		)
	})
	Context("ExpireAfter", func() {
		DescribeTable("should succeed on a valid expireAfter", func(value string) {
			u := lo.Must(runtime.DefaultUnstructuredConverter.ToUnstructured(nodeClaim))
			lo.Must0(unstructured.SetNestedField(u, value, "spec", "expireAfter"))
			obj := &unstructured.Unstructured{}
			lo.Must0(runtime.DefaultUnstructuredConverter.FromUnstructured(u, obj))

			Expect(env.Client.Create(ctx, obj)).To(Succeed())
		},
			Entry("single unit", "30s"),
			Entry("multiple units", "1h30m5s"),
			Entry("never", "Never"),
		)
		DescribeTable("should fail on an invalid expireAfter", func(value string) {
			u := lo.Must(runtime.DefaultUnstructuredConverter.ToUnstructured(nodeClaim))
			lo.Must0(unstructured.SetNestedField(u, value, "spec", "expireAfter"))
			obj := &unstructured.Unstructured{}
			lo.Must0(runtime.DefaultUnstructuredConverter.FromUnstructured(u, obj))

			Expect(env.Client.Create(ctx, obj)).To(Not(Succeed()))
		},
			Entry("negative", "-1s"),
			Entry("invalid unit", "1hr"),
			Entry("partial match", "FooNever"),
		)
	})
})
