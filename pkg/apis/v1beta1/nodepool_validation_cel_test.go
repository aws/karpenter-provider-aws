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

package v1beta1_test

import (
	"strings"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
)

var _ = Describe("CEL/Validation", func() {
	var nodePool *v1beta1.NodePool

	BeforeEach(func() {
		if env.Version.Minor() < 25 {
			Skip("CEL Validation is for 1.25>")
		}
		nodePool = &v1beta1.NodePool{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: v1beta1.NodePoolSpec{
				Template: v1beta1.NodeClaimTemplate{
					Spec: v1beta1.NodeClaimSpec{
						NodeClassRef: &v1beta1.NodeClassReference{
							Kind: "NodeClaim",
							Name: "default",
						},
						Requirements: []v1beta1.NodeSelectorRequirementWithFlexibility{
							{
								NodeSelectorRequirement: v1.NodeSelectorRequirement{
									Key:      v1beta1.CapacityTypeLabelKey,
									Operator: v1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		}
	})
	Context("Requirements", func() {
		It("should allow restricted domains exceptions", func() {
			oldNodePool := nodePool.DeepCopy()
			for label := range v1beta1.LabelDomainExceptions {
				nodePool.Spec.Template.Spec.Requirements = []v1beta1.NodeSelectorRequirementWithFlexibility{
					{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: label + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}},
				}
				Expect(env.Client.Create(ctx, nodePool)).To(Succeed())
				Expect(nodePool.RuntimeValidate()).To(Succeed())
				Expect(env.Client.Delete(ctx, nodePool)).To(Succeed())
				nodePool = oldNodePool.DeepCopy()
			}
		})
		It("should allow well known label exceptions", func() {
			oldNodePool := nodePool.DeepCopy()
			for label := range v1beta1.WellKnownLabels.Difference(sets.New(v1beta1.NodePoolLabelKey)) {
				nodePool.Spec.Template.Spec.Requirements = []v1beta1.NodeSelectorRequirementWithFlexibility{
					{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: label, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}},
				}
				Expect(env.Client.Create(ctx, nodePool)).To(Succeed())
				Expect(nodePool.RuntimeValidate()).To(Succeed())
				Expect(env.Client.Delete(ctx, nodePool)).To(Succeed())
				nodePool = oldNodePool.DeepCopy()
			}
		})
	})
	Context("Labels", func() {
		It("should allow restricted domains exceptions", func() {
			oldNodePool := nodePool.DeepCopy()
			for label := range v1beta1.LabelDomainExceptions {
				nodePool.Spec.Template.Labels = map[string]string{
					label: "test",
				}
				Expect(env.Client.Create(ctx, nodePool)).To(Succeed())
				Expect(nodePool.RuntimeValidate()).To(Succeed())
				Expect(env.Client.Delete(ctx, nodePool)).To(Succeed())
				nodePool = oldNodePool.DeepCopy()
			}
		})
		It("should allow well known label exceptions", func() {
			oldNodePool := nodePool.DeepCopy()
			for label := range v1beta1.WellKnownLabels.Difference(sets.New(v1beta1.NodePoolLabelKey)) {
				nodePool.Spec.Template.Labels = map[string]string{
					label: "test",
				}
				Expect(env.Client.Create(ctx, nodePool)).To(Succeed())
				Expect(nodePool.RuntimeValidate()).To(Succeed())
				Expect(env.Client.Delete(ctx, nodePool)).To(Succeed())
				nodePool = oldNodePool.DeepCopy()
			}
		})
	})
})
