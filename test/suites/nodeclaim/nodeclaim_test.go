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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var _ = Describe("StandaloneNodeClaim", func() {
	It("should create a standard NodeClaim within the 'c' instance family", func() {
		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      v1.LabelInstanceCategory,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceCategory, "c"))
		env.EventuallyExpectNodeClaimsReady(nodeClaim)
	})
	It("should create a standard NodeClaim based on resource requests", func() {
		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				Resources: karpv1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("3"),
						corev1.ResourceMemory: resource.MustParse("64Gi"),
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
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
	It("should create a NodeClaim propagating all the NodeClaim spec details", func() {
		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"custom-annotation": "custom-value",
				},
				Labels: map[string]string{
					"custom-label": "custom-value",
				},
			},
			Spec: karpv1.NodeClaimSpec{
				Taints: []corev1.Taint{
					{
						Key:    "custom-taint",
						Effect: corev1.TaintEffectNoSchedule,
						Value:  "custom-value",
					},
					{
						Key:    "other-custom-taint",
						Effect: corev1.TaintEffectNoExecute,
						Value:  "other-custom-value",
					},
				},
				Resources: karpv1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("3"),
						corev1.ResourceMemory: resource.MustParse("16Gi"),
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		Expect(node.Annotations).To(HaveKeyWithValue("custom-annotation", "custom-value"))
		Expect(node.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
		Expect(node.Spec.Taints).To(ContainElements(
			corev1.Taint{
				Key:    "custom-taint",
				Effect: corev1.TaintEffectNoSchedule,
				Value:  "custom-value",
			},
			corev1.Taint{
				Key:    "other-custom-taint",
				Effect: corev1.TaintEffectNoExecute,
				Value:  "other-custom-value",
			},
		))
		Expect(node.OwnerReferences).To(ContainElement(
			metav1.OwnerReference{
				APIVersion:         object.GVK(nodeClaim).GroupVersion().String(),
				Kind:               "NodeClaim",
				Name:               nodeClaim.Name,
				UID:                nodeClaim.UID,
				BlockOwnerDeletion: lo.ToPtr(true),
			},
		))
		env.EventuallyExpectCreatedNodeClaimCount("==", 1)
		env.EventuallyExpectNodeClaimsReady(nodeClaim)
	})
	It("should remove the cloudProvider NodeClaim when the cluster NodeClaim is deleted", func() {
		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      v1.LabelInstanceCategory,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
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
			g.Expect(env.GetInstanceByID(instanceID).State.Name).To(BeElementOf(ec2types.InstanceStateNameTerminated, ec2types.InstanceStateNameShuttingDown))
		}, time.Second*10).Should(Succeed())
	})
	It("should delete a NodeClaim from the node termination finalizer", func() {
		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      v1.LabelInstanceCategory,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
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
			g.Expect(env.GetInstanceByID(instanceID).State.Name).To(BeElementOf(ec2types.InstanceStateNameTerminated, ec2types.InstanceStateNameShuttingDown))
		}, time.Second*10).Should(Succeed())
	})
	It("should create a NodeClaim with custom labels passed through the userData", func() {
		customAMI := env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", env.K8sVersion()))
		// Update the userData for the instance input with the correct NodePool
		rawContent, err := os.ReadFile("testdata/al2023_userdata_custom_labels_input.yaml")
		Expect(err).ToNot(HaveOccurred())

		// Create userData that adds custom labels through the --node-labels
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: customAMI}}
		nodeClass.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle()))

		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      v1.LabelInstanceCategory,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
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
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: customAMI}}

		// Giving bad clusterName and clusterEndpoint to the userData
		nodeClass.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), "badName", "badEndpoint", env.ExpectCABundle()))

		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      v1.LabelInstanceCategory,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
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
			temp := &karpv1.NodeClaim{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), temp)).To(Succeed())
			g.Expect(temp.StatusConditions().Get(karpv1.ConditionTypeLaunched).IsTrue()).To(BeTrue())
			g.Expect(temp.StatusConditions().Get(karpv1.ConditionTypeRegistered).IsUnknown()).To(BeTrue())
			g.Expect(temp.StatusConditions().Get(karpv1.ConditionTypeInitialized).IsUnknown()).To(BeTrue())
		}).Should(Succeed())

		// Expect that the nodeClaim is eventually de-provisioned due to the registration timeout
		Eventually(func(g Gomega) {
			g.Expect(errors.IsNotFound(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim))).To(BeTrue())
		}).WithTimeout(time.Minute * 20).Should(Succeed())
	})
	It("should delete a NodeClaim if it references a NodeClass that doesn't exist", func() {
		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      v1.LabelInstanceCategory,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		// Don't create the NodeClass and expect that the NodeClaim fails and gets deleted
		env.ExpectCreated(nodeClaim)
		env.EventuallyExpectNotFound(nodeClaim)
	})
	It("should delete a NodeClaim if it references a NodeClass that isn't Ready", func() {
		nodeClaim := test.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      v1.LabelInstanceCategory,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		// Point to an AMI that doesn't exist so that the NodeClass goes NotReady
		nodeClass.Spec.AMIFamily = &v1.AMIFamilyAL2023
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-123456789"}}
		env.ExpectCreated(nodeClass, nodeClaim)
		env.EventuallyExpectNotFound(nodeClaim)
	})
})
