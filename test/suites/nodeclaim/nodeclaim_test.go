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
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StandaloneNodeClaim", func() {
	It("should create a standard NodeClaim within the 'c' instance family", func() {
		nodeClaim := test.NodeClaim(corev1beta1.NodeClaim{
			Spec: corev1beta1.NodeClaimSpec{
				Requirements: []corev1beta1.NodeSelectorRequirementWithFlexibility{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      v1beta1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      corev1beta1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1beta1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		Expect(node.Labels).To(HaveKeyWithValue(v1beta1.LabelInstanceCategory, "c"))
		env.EventuallyExpectNodeClaimsReady(nodeClaim)
	})
	It("should create a standard NodeClaim based on resource requests", func() {
		nodeClaim := test.NodeClaim(corev1beta1.NodeClaim{
			Spec: corev1beta1.NodeClaimSpec{
				Resources: corev1beta1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("3"),
						v1.ResourceMemory: resource.MustParse("64Gi"),
					},
				},
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
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
		nodeClaim := test.NodeClaim(corev1beta1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"custom-annotation": "custom-value",
				},
				Labels: map[string]string{
					"custom-label": "custom-value",
				},
			},
			Spec: corev1beta1.NodeClaimSpec{
				Taints: []v1.Taint{
					{
						Key:    "custom-taint",
						Effect: v1.TaintEffectNoSchedule,
						Value:  "custom-value",
					},
					{
						Key:    "other-custom-taint",
						Effect: v1.TaintEffectNoExecute,
						Value:  "other-custom-value",
					},
				},
				Resources: corev1beta1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("3"),
						v1.ResourceMemory: resource.MustParse("16Gi"),
					},
				},
				Kubelet: &corev1beta1.KubeletConfiguration{
					MaxPods:     lo.ToPtr[int32](110),
					PodsPerCore: lo.ToPtr[int32](10),
					SystemReserved: v1.ResourceList{
						v1.ResourceCPU:              resource.MustParse("200m"),
						v1.ResourceMemory:           resource.MustParse("200Mi"),
						v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					KubeReserved: v1.ResourceList{
						v1.ResourceCPU:              resource.MustParse("200m"),
						v1.ResourceMemory:           resource.MustParse("200Mi"),
						v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					EvictionHard: map[string]string{
						"memory.available":   "5%",
						"nodefs.available":   "5%",
						"nodefs.inodesFree":  "5%",
						"imagefs.available":  "5%",
						"imagefs.inodesFree": "5%",
						"pid.available":      "3%",
					},
					EvictionSoft: map[string]string{
						"memory.available":   "10%",
						"nodefs.available":   "10%",
						"nodefs.inodesFree":  "10%",
						"imagefs.available":  "10%",
						"imagefs.inodesFree": "10%",
						"pid.available":      "6%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available":   {Duration: time.Minute * 2},
						"nodefs.available":   {Duration: time.Minute * 2},
						"nodefs.inodesFree":  {Duration: time.Minute * 2},
						"imagefs.available":  {Duration: time.Minute * 2},
						"imagefs.inodesFree": {Duration: time.Minute * 2},
						"pid.available":      {Duration: time.Minute * 2},
					},
					EvictionMaxPodGracePeriod:   lo.ToPtr[int32](120),
					ImageGCHighThresholdPercent: lo.ToPtr[int32](50),
					ImageGCLowThresholdPercent:  lo.ToPtr[int32](10),
				},
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
				},
			},
		})
		env.ExpectCreated(nodeClass, nodeClaim)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		Expect(node.Annotations).To(HaveKeyWithValue("custom-annotation", "custom-value"))
		Expect(node.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
		Expect(node.Spec.Taints).To(ContainElements(
			v1.Taint{
				Key:    "custom-taint",
				Effect: v1.TaintEffectNoSchedule,
				Value:  "custom-value",
			},
			v1.Taint{
				Key:    "other-custom-taint",
				Effect: v1.TaintEffectNoExecute,
				Value:  "other-custom-value",
			},
		))
		Expect(node.OwnerReferences).To(ContainElement(
			metav1.OwnerReference{
				APIVersion:         corev1beta1.SchemeGroupVersion.String(),
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
		nodeClaim := test.NodeClaim(corev1beta1.NodeClaim{
			Spec: corev1beta1.NodeClaimSpec{
				Requirements: []corev1beta1.NodeSelectorRequirementWithFlexibility{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      v1beta1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      corev1beta1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1beta1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
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
			g.Expect(lo.FromPtr(env.GetInstanceByID(instanceID).State.Name)).To(Equal("shutting-down"))
		}, time.Second*10).Should(Succeed())
	})
	It("should delete a NodeClaim from the node termination finalizer", func() {
		nodeClaim := test.NodeClaim(corev1beta1.NodeClaim{
			Spec: corev1beta1.NodeClaimSpec{
				Requirements: []corev1beta1.NodeSelectorRequirementWithFlexibility{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      v1beta1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c"},
						},
					},
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      corev1beta1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1beta1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
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
			g.Expect(lo.FromPtr(env.GetInstanceByID(instanceID).State.Name)).To(Equal("shutting-down"))
		}, time.Second*10).Should(Succeed())
	})
	It("should create a NodeClaim with custom labels passed through the userData", func() {
		customAMI := env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
		// Update the userData for the instance input with the correct NodePool
		rawContent, err := os.ReadFile("testdata/al2_userdata_custom_labels_input.sh")
		Expect(err).ToNot(HaveOccurred())

		// Create userData that adds custom labels through the --kubelet-extra-args
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: customAMI}}
		nodeClass.Spec.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle()))))

		nodeClaim := test.NodeClaim(corev1beta1.NodeClaim{
			Spec: corev1beta1.NodeClaimSpec{
				Requirements: []corev1beta1.NodeSelectorRequirementWithFlexibility{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      v1beta1.LabelInstanceCategory,
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
							Key:      corev1beta1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1beta1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
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
		customAMI := env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
		// Update the userData for the instance input with the correct NodePool
		rawContent, err := os.ReadFile("testdata/al2_userdata_input.sh")
		Expect(err).ToNot(HaveOccurred())

		// Create userData that adds custom labels through the --kubelet-extra-args
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: customAMI}}

		// Giving bad clusterName and clusterEndpoint to the userData
		nodeClass.Spec.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), "badName", "badEndpoint", env.ExpectCABundle()))))

		nodeClaim := test.NodeClaim(corev1beta1.NodeClaim{
			Spec: corev1beta1.NodeClaimSpec{
				Requirements: []corev1beta1.NodeSelectorRequirementWithFlexibility{
					{
						NodeSelectorRequirement: v1.NodeSelectorRequirement{
							Key:      v1beta1.LabelInstanceCategory,
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
							Key:      corev1beta1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1beta1.CapacityTypeOnDemand},
						},
					},
				},
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
				},
			},
		})

		env.ExpectCreated(nodeClass, nodeClaim)
		nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]

		// Expect that the nodeClaim eventually launches and has false Registration/Initialization
		Eventually(func(g Gomega) {
			temp := &corev1beta1.NodeClaim{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), temp)).To(Succeed())
			g.Expect(temp.StatusConditions().GetCondition(corev1beta1.Launched).IsTrue()).To(BeTrue())
			g.Expect(temp.StatusConditions().GetCondition(corev1beta1.Registered).IsFalse()).To(BeTrue())
			g.Expect(temp.StatusConditions().GetCondition(corev1beta1.Initialized).IsFalse()).To(BeTrue())
		}).Should(Succeed())

		// Expect that the nodeClaim is eventually de-provisioned due to the registration timeout
		env.EventuallyExpectNotFoundAssertion(nodeClaim).WithTimeout(time.Minute * 20).Should(Succeed())
	})
})
