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

package nodeclaim_test

import (
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	corev1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	providerv1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StandaloneNodeClaim", func() {
	It("should create a standard NodeClaim within the 'c' instance family", func() {
		nodeClaim := test.NodeClaim(corev1.NodeClaim{
			Spec: corev1.NodeClaimSpec{
				Requirements: []corev1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      providerv1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      corev1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		Expect(node.Labels).To(HaveKeyWithValue(providerv1.LabelInstanceCategory, "c"))
		env.EventuallyExpectNodeClaimsReady(nodeClaim)
	})
	It("should create a standard NodeClaim based on resource requests", func() {
		nodeClaim := test.NodeClaim(corev1.NodeClaim{
			Spec: corev1.NodeClaimSpec{
				Resources: corev1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("3"),
						v1.ResourceMemory: resource.MustParse("64Gi"),
					},
				},
				NodeClassRef: &corev1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		Expect(resources.Fits(nodeClaim.Spec.Resources.Requests, node.Status.Allocatable))
		env.EventuallyExpectNodeClaimsReady(nodeClaim)
	})
	It("should remove the cloudProvider NodeClaim when the cluster NodeClaim is deleted", func() {
		nodeClaim := test.NodeClaim(corev1.NodeClaim{
			Spec: corev1.NodeClaimSpec{
				Requirements: []corev1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      providerv1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      corev1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]

		instanceID := env.ExpectParsedProviderID(node.Spec.ProviderID)
		env.GetInstance(node.Name)

		// Node is deleted and now should be not found
		env.ExpectDeleted(nodeClaim)
		env.EventuallyExpectNotFound(nodeClaim, node)

		Eventually(func(g Gomega) {
			g.Expect(lo.FromPtr(env.GetInstanceByID(instanceID).State.Name)).To(BeElementOf("terminated", "shutting-down"))
		}, time.Second*10).Should(Succeed())
	})
	It("should delete a NodeClaim from the node termination finalizer", func() {
		nodeClaim := test.NodeClaim(corev1.NodeClaim{
			Spec: corev1.NodeClaimSpec{
				Requirements: []corev1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      providerv1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      corev1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]

		instanceID := env.ExpectParsedProviderID(node.Spec.ProviderID)
		env.GetInstance(node.Name)

		// Delete the node and expect both the node and nodeClaim to be gone as well as the instance to be shutting-down
		env.ExpectDeleted(node)
		env.EventuallyExpectNotFound(nodeClaim, node)

		Eventually(func(g Gomega) {
			g.Expect(lo.FromPtr(env.GetInstanceByID(instanceID).State.Name)).To(BeElementOf("terminated", "shutting-down"))
		}, time.Second*10).Should(Succeed())
	})
	It("should create a NodeClaim with custom labels passed through the userData", func() {
		customAMI := env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", env.K8sVersion()))
		// Update the userData for the instance input with the correct NodePool
		rawContent, err := os.ReadFile("testdata/al2023_userdata_custom_labels_input.yaml")
		Expect(err).ToNot(HaveOccurred())

		// Create userData that adds custom labels through the --node-labels
		nodeClass.Spec.AMIFamily = &providerv1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []providerv1.AMISelectorTerm{{ID: customAMI}}
		nodeClass.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle()))

		nodeClaim := test.NodeClaim(corev1.NodeClaim{
			Spec: corev1.NodeClaimSpec{
				Requirements: []corev1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      providerv1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      corev1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		Expect(node.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
		Expect(node.Labels).To(HaveKeyWithValue("custom-label2", "custom-value2"))

		env.EventuallyExpectCreatedNodeClaimCount("==", 1)
		env.EventuallyExpectNodeClaimsReady(nodeClaim)
	})
	It("should delete a NodeClaim after the registration timeout when the node doesn't register", func() {
		customAMI := env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", env.K8sVersion()))
		// Update the userData for the instance input with the correct NodePool
		rawContent, err := os.ReadFile("testdata/al2023_userdata_input.yaml")
		Expect(err).ToNot(HaveOccurred())

		// Create userData that adds custom labels through the --node-labels
		nodeClass.Spec.AMIFamily = &providerv1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []providerv1.AMISelectorTerm{{ID: customAMI}}

		// Giving bad clusterName and clusterEndpoint to the userData
		nodeClass.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), "badName", "badEndpoint", env.ExpectCABundle()))

		nodeClaim := test.NodeClaim(corev1.NodeClaim{
			Spec: corev1.NodeClaimSpec{
				Requirements: []corev1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      providerv1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      corev1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})

		env.ExpectCreated(nodeClass, nodeClaim)
		nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]

		// Expect that the nodeClaim eventually launches and has false Registration/Initialization
		Eventually(func(g Gomega) {
			temp := &corev1.NodeClaim{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), temp)).To(Succeed())
			g.Expect(temp.StatusConditions().Get(corev1.ConditionTypeLaunched).IsTrue()).To(BeTrue())
			g.Expect(temp.StatusConditions().Get(corev1.ConditionTypeRegistered).IsFalse()).To(BeTrue())
			g.Expect(temp.StatusConditions().Get(corev1.ConditionTypeInitialized).IsFalse()).To(BeTrue())
		}).Should(Succeed())

		// Expect that the nodeClaim is eventually de-provisioned due to the registration timeout
		env.EventuallyExpectNotFoundAssertion(nodeClaim).WithTimeout(time.Minute * 20).Should(Succeed())
	})
})
