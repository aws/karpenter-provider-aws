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

package integration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	karpv1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	karptest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var _ = Describe("Conversion Webhooks", func() {
	Context("NodePool", func() {
		It("should not update a metadata generation when the same resource is applied for the v1beta1 APIs", func() {
			// created v1beta1 resource
			storedv1beta1NodePool := &karpv1beta1.NodePool{
				ObjectMeta: karptest.ObjectMeta(),
				Spec: karpv1beta1.NodePoolSpec{
					Template: karpv1beta1.NodeClaimTemplate{
						Spec: karpv1beta1.NodeClaimSpec{
							NodeClassRef: &karpv1beta1.NodeClassReference{
								Name: "test-nodeclass",
							},
							Requirements: []karpv1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelOSStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{string(corev1.Linux)},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      karpv1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{karpv1.CapacityTypeOnDemand},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceCategory,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"c", "m", "r"},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceGeneration,
										Operator: corev1.NodeSelectorOpGt,
										Values:   []string{"2"},
									},
								},
								// Filter out a1 instance types, which are incompatible with AL2023 AMIs
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceFamily,
										Operator: corev1.NodeSelectorOpNotIn,
										Values:   []string{"a1"},
									},
								},
							},
							Kubelet: &karpv1beta1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](20)},
						},
					},
				},
			}

			env.ExpectCreated(storedv1beta1NodePool.DeepCopy())
			v1beta1NodePool := env.ExpectExists(storedv1beta1NodePool.DeepCopy()).(*karpv1beta1.NodePool)
			Expect(v1beta1NodePool.Generation).To(BeNumerically("==", 1))

			// Second apply of the same NodePool does not increase the generation
			env.ExpectUpdated(storedv1beta1NodePool.DeepCopy())
			v1beta1NodePool = env.ExpectExists(storedv1beta1NodePool.DeepCopy()).(*karpv1beta1.NodePool)
			Expect(v1beta1NodePool.Generation).To(BeNumerically("==", 1))
		})
		It("should not update a metadata generation when the same resource is applied for the v1 APIs", func() {
			env.ExpectCreated(nodePool.DeepCopy())
			v1NodePool := env.ExpectExists(nodePool).(*karpv1.NodePool)
			Expect(v1NodePool.Generation).To(BeNumerically("==", 1))

			// Second apply of the same NodePool does not increase the generation
			env.ExpectUpdated(nodePool.DeepCopy())
			v1NodePool = env.ExpectExists(nodePool).(*karpv1.NodePool)
			Expect(v1NodePool.Generation).To(BeNumerically("==", 1))
		})
	})
	Context("EC2NodeClass", func() {
		It("should not update a metadata generation when the same resource is applied for the v1beta1 APIs", func() {
			// created v1beta1 resource
			storedv1beta1nodeclass := test.BetaEC2NodeClass()

			env.ExpectCreated(storedv1beta1nodeclass.DeepCopy())
			v1beta1nodeclass := env.ExpectExists(storedv1beta1nodeclass.DeepCopy()).(*v1beta1.EC2NodeClass)
			Expect(v1beta1nodeclass.Generation).To(BeNumerically("==", 1))

			// Second apply of the same NodeClass does not increase the generation
			env.ExpectUpdated(storedv1beta1nodeclass.DeepCopy())
			v1beta1nodeclass = env.ExpectExists(storedv1beta1nodeclass).(*v1beta1.EC2NodeClass)
			Expect(v1beta1nodeclass.Generation).To(BeNumerically("==", 1))
		})
		It("should not update a metadata generation when the same resource is applied for v1 APIs", func() {
			env.ExpectCreated(nodeClass.DeepCopy())
			v1nodeclass := env.ExpectExists(nodeClass.DeepCopy()).(*v1.EC2NodeClass)
			Expect(v1nodeclass.Generation).To(BeNumerically("==", 1))

			// Second apply of the same NodeClass does not increase the generation
			env.ExpectUpdated(nodeClass.DeepCopy())
			v1nodeclass = env.ExpectExists(nodeClass.DeepCopy()).(*v1.EC2NodeClass)
			Expect(v1nodeclass.Generation).To(BeNumerically("==", 1))
		})
	})
})
