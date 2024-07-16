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
	"fmt"
	"strings"
	"time"

	"github.com/awslabs/operatorpkg/object"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	karpv1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
)

var _ = Describe("EC2nodeClass Kubelet Configuration", func() {
	var betaNodePool *karpv1beta1.NodePool
	var betaNodeClass *v1beta1.EC2NodeClass
	BeforeEach(func() {
		betaNodeClass = test.BetaEC2NodeClass(v1beta1.EC2NodeClass{
			Spec: v1beta1.EC2NodeClassSpec{
				AMIFamily: lo.ToPtr(v1beta1.AMIFamilyAL2023),
				Role:      fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName),
				Tags: map[string]string{
					"testing/cluster": env.ClusterName,
				},
				SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					},
				},
				SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					},
				},
			},
		})
		betaNodePool = &karpv1beta1.NodePool{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: karpv1beta1.NodePoolSpec{
				Template: karpv1beta1.NodeClaimTemplate{
					Spec: karpv1beta1.NodeClaimSpec{
						NodeClassRef: &karpv1beta1.NodeClassReference{
							APIVersion: object.GVK(betaNodeClass).GroupVersion().String(),
							Kind:       object.GVK(betaNodeClass).Kind,
							Name:       betaNodeClass.Name,
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
	})
	It("should expect v1beta1 nodePool to have kubelet compatibility annotation", func() {
		env.ExpectCreated(betaNodeClass, betaNodePool)
		Eventually(func(g Gomega) {
			np := &karpv1.NodePool{}
			expectExists(betaNodePool, np)

			hash, found := np.Annotations[karpv1.KubeletCompatibilityAnnotationKey]
			g.Expect(found).To(BeTrue())
			g.Expect(hash).To(Equal(np.Hash()))
		})
	})
	It("should expect nodeClaim to not drift when kubelet configuration on v1 nodeClass is same as v1beta1 nodePool", func() {
		pod := coretest.Pod()
		env.ExpectCreated(pod, betaNodeClass, betaNodePool)
		env.EventuallyExpectHealthy(pod)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		_, found := nodeClaim.Annotations[v1.AnnotationKubeletCompatibilityHash]
		Expect(found).To(BeTrue())

		np := &karpv1.NodePool{}
		expectExists(betaNodePool, np)
		nc := &v1.EC2NodeClass{}
		expectExists(betaNodeClass, nc)

		delete(np.Annotations, karpv1.KubeletCompatibilityAnnotationKey)
		nc.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](20)}
		env.ExpectUpdated(np, nc)

		Eventually(func(g Gomega) {
			g.Expect(nodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted).IsUnknown()).To(BeTrue())
		}).Should(Succeed())
	})
	It("should expect nodeClaim to drift when kubelet configuration on v1 nodeClass is different from v1beta1 nodePool", func() {
		pod := coretest.Pod()
		env.ExpectCreated(pod, betaNodeClass, betaNodePool)
		env.EventuallyExpectHealthy(pod)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		_, found := nodeClaim.Annotations[v1.AnnotationKubeletCompatibilityHash]
		Expect(found).To(BeTrue())

		np := &karpv1.NodePool{}
		expectExists(betaNodePool, np)
		nc := &v1.EC2NodeClass{}
		expectExists(betaNodeClass, nc)

		delete(np.Annotations, karpv1.KubeletCompatibilityAnnotationKey)
		nc.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](10)}
		env.ExpectUpdated(np, nc)
		env.EventuallyExpectDrifted(nodeClaim)
	})
})

func expectExists(v1beta1object client.Object, v1object client.Object) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(v1beta1object), v1object)).To(Succeed())
	}).WithTimeout(time.Second * 5).Should(Succeed())
}
