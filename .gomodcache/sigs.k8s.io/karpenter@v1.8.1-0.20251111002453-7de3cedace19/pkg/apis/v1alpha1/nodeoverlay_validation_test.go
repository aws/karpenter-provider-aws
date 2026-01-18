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

package v1alpha1_test

import (
	"fmt"
	"strings"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	. "sigs.k8s.io/karpenter/pkg/apis/v1alpha1"
)

var _ = Describe("CEL/Validation", func() {
	var nodeOverlay *NodeOverlay

	BeforeEach(func() {
		nodeOverlay = &NodeOverlay{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: NodeOverlaySpec{
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      v1.CapacityTypeLabelKey,
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
		}
	})
	Context("Requirements", func() {
		It("should fail for no values for In operator", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
				{Key: "Test", Operator: corev1.NodeSelectorOpIn},
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).NotTo(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).NotTo(Succeed())
		})
		It("should fail for no values for NotIn operator", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
				{Key: "Test", Operator: corev1.NodeSelectorOpNotIn},
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).NotTo(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).NotTo(Succeed())
		})
		It("should succeed for valid requirement keys", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
				{Key: "Test", Operator: corev1.NodeSelectorOpExists},
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
		})
		It("should succeed for valid requirement keys", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
				{Key: "Test", Operator: corev1.NodeSelectorOpExists},
				{Key: "test.com/Test", Operator: corev1.NodeSelectorOpExists},
				{Key: "test.com.com/test", Operator: corev1.NodeSelectorOpExists},
				{Key: "key-only", Operator: corev1.NodeSelectorOpExists},
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
		})
		It("should fail for invalid requirement keys", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{{Key: "test.com.com}", Operator: corev1.NodeSelectorOpExists}}
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{{Key: "Test.com/test", Operator: corev1.NodeSelectorOpExists}}
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{{Key: "test/test/test", Operator: corev1.NodeSelectorOpExists}}
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{{Key: "test/", Operator: corev1.NodeSelectorOpExists}}
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{{Key: "/test", Operator: corev1.NodeSelectorOpExists}}
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
		})
		It("should allow for the karpenter.sh/nodepool label", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
				{Key: v1.NodePoolLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{randomdata.SillyName()}},
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
		})
		It("should fail at runtime for requirement keys that are too long", func() {
			oldnodeOverlay := nodeOverlay.DeepCopy()
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{{Key: fmt.Sprintf("test.com.test.%s/test", strings.ToLower(randomdata.Alphanumeric(250))), Operator: corev1.NodeSelectorOpExists}}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(env.Client.Delete(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
			nodeOverlay = oldnodeOverlay.DeepCopy()
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{{Key: fmt.Sprintf("test.com.test/test-%s", strings.ToLower(randomdata.Alphanumeric(250))), Operator: corev1.NodeSelectorOpExists}}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
		})
		It("should allow supported ops", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpGt, Values: []string{"1"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpLt, Values: []string{"1"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpNotIn, Values: []string{"1"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpExists},
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
		})
		It("should fail for unsupported ops", func() {
			for _, op := range []corev1.NodeSelectorOperator{"unknown"} {
				nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelTopologyZone, Operator: op, Values: []string{"test"}},
				}
				Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
				Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
			}
		})
		It("should fail for restricted domains", func() {
			for label := range v1.RestrictedLabelDomains {
				nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
					{Key: label + "/test", Operator: corev1.NodeSelectorOpIn, Values: []string{"test"}},
				}
				Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
				Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
			}
		})
		It("should allow restricted domains exceptions", func() {
			oldnodeOverlay := nodeOverlay.DeepCopy()
			for label := range v1.LabelDomainExceptions {
				nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
					{Key: label + "/test", Operator: corev1.NodeSelectorOpIn, Values: []string{"test"}},
				}
				Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
				Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
				Expect(env.Client.Delete(ctx, nodeOverlay)).To(Succeed())
				nodeOverlay = oldnodeOverlay.DeepCopy()
			}
		})
		It("should allow restricted subdomains exceptions", func() {
			oldnodeOverlay := nodeOverlay.DeepCopy()
			for label := range v1.LabelDomainExceptions {
				nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
					{Key: "subdomain." + label + "/test", Operator: corev1.NodeSelectorOpIn, Values: []string{"test"}},
				}
				Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
				Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
				Expect(env.Client.Delete(ctx, nodeOverlay)).To(Succeed())
				nodeOverlay = oldnodeOverlay.DeepCopy()
			}
		})
		It("should allow well known label exceptions", func() {
			oldnodeOverlay := nodeOverlay.DeepCopy()
			// Capacity Type is runtime validated
			for label := range v1.WellKnownLabels.Difference(sets.New(v1.NodePoolLabelKey, v1.CapacityTypeLabelKey)) {
				nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
					{Key: label, Operator: corev1.NodeSelectorOpIn, Values: []string{"test"}},
				}
				Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
				Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
				Expect(env.Client.Delete(ctx, nodeOverlay)).To(Succeed())
				nodeOverlay = oldnodeOverlay.DeepCopy()
			}
		})
		It("should allow non-empty set after removing overlapped value", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test", "foo"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpNotIn, Values: []string{"test", "bar"}},
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
		})
		It("should allow empty requirements", func() {
			nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{
				{
					Key:      "test",
					Operator: corev1.NodeSelectorOpExists,
				},
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
		})
		It("should fail with invalid GT or LT values", func() {
			for _, requirement := range []corev1.NodeSelectorRequirement{
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpGt, Values: []string{}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpGt, Values: []string{"1", "2"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpGt, Values: []string{"a"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpGt, Values: []string{"-1"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpLt, Values: []string{}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpLt, Values: []string{"1", "2"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpLt, Values: []string{"a"}},
				{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpLt, Values: []string{"-1"}},
			} {
				nodeOverlay.Spec.Requirements = []corev1.NodeSelectorRequirement{requirement}
				Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
				Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
			}
		})
	})
	It("shout not be able to set both price and priceAdjustment", func() {
		nodeOverlay.Spec.Price = lo.ToPtr("0.432")
		nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("+10%")
		Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
	})
	Context("priceAdjustment", func() {
		DescribeTable("Invalid Input",
			func(input string) {
				nodeOverlay.Spec.Price = lo.ToPtr(input)
				Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			},
			Entry("No explicit plus sign allowed", "+42"),
			Entry("Must have leading digit", ".5"),
			Entry("Must have trailing digits after decimal", "42."),
			Entry("No percentage sign allowed", "42%"),
			Entry("No commas allowed", "3,14"),
			Entry("No scientific notation", "1e10"),
			Entry("No hex notation", "0x42"),
			Entry("No text", "forty-two"),
			Entry("No letters", "42a"),
			Entry("No spaces", "42 "),
			Entry("No spaces", " 42"),
			Entry("Multiple decimal points", "42.0.0"),
			Entry("Just a sign", "-"),
			Entry("Just a decimal point", "."),
			Entry("No leading digit after sign", "-100.0%"),
			Entry("less -100% float ", "-101.1%"),
			Entry("less -100% integer ", "-129"),
		)
		It("should not allow an unsigned priceAdjustment percentage", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("1%")
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
		})
		It("should not allow an unsigned priceAdjustment integer", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("1")
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
		})
		It("should not allow an unsigned priceAdjustment float", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("1.3")
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
		})
		It("should allow positive percentage value for priceAdjustment field", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("+1%")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow negative percentage less then 0%", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("-1%")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow negative percentage -100%", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("-100%")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow positive percentage greater then 100%", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("+100.102%")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow positive percentage greater then 100% with an integer", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("+298%")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow positive integer value", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("+43")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow negative integer value", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("-43")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow positive float value", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("+34.43")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow negative float value", func() {
			nodeOverlay.Spec.PriceAdjustment = lo.ToPtr("-34.43")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
	})
	Context("price", func() {
		DescribeTable("Invalid Input",
			func(input string) {
				nodeOverlay.Spec.Price = lo.ToPtr(input)
				Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			},
			Entry("No explicit plus sign allowed", "+42"),
			Entry("Must have leading digit", ".5"),
			Entry("Must have trailing digits after decimal", "42."),
			Entry("No percentage sign allowed", "42%"),
			Entry("No commas allowed", "3,14"),
			Entry("No scientific notation", "1e10"),
			Entry("No hex notation", "0x42"),
			Entry("No text", "forty-two"),
			Entry("No letters", "42a"),
			Entry("No spaces", "42 "),
			Entry("No spaces", " 42"),
			Entry("Multiple decimal points", "42.0.0"),
			Entry("Just a sign", "-"),
			Entry("Just a decimal point", "."),
			Entry("No leading digit after sign", "-.42"),
		)
		It("should allow integer value", func() {
			nodeOverlay.Spec.Price = lo.ToPtr("43")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
		It("should allow float value", func() {
			nodeOverlay.Spec.Price = lo.ToPtr("34.43")
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
		})
	})
	Context("Capacity", func() {
		It("should allow custom resources", func() {
			nodeOverlay.Spec.Capacity = corev1.ResourceList{
				corev1.ResourceName("smarter-devices/fuse"): resource.MustParse("1"),
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).To(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).To(Succeed())
		})
		It("should not allow cpu resources override", func() {
			nodeOverlay.Spec.Capacity = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
		})
		It("should not allow memory resources override", func() {
			nodeOverlay.Spec.Capacity = corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("34Gi"),
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
		})
		It("should not allow pod resources override", func() {
			nodeOverlay.Spec.Capacity = corev1.ResourceList{
				corev1.ResourcePods: resource.MustParse("324"),
			}
			Expect(env.Client.Create(ctx, nodeOverlay)).ToNot(Succeed())
			Expect(nodeOverlay.RuntimeValidate(ctx)).ToNot(Succeed())
		})
	})
})
