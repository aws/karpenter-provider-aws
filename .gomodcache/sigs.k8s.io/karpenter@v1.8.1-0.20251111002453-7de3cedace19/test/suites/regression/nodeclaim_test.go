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

package integration_test

import (
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/utils/resources"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("NodeClaim", func() {
	Describe("StandaloneNodeClaim", func() {
		var requirements []v1.NodeSelectorRequirementWithMinValues
		BeforeEach(func() {
			requirements = nodePool.Spec.Template.Spec.Requirements
			if env.IsDefaultNodeClassKWOK() {
				requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values: []string{
							"c-16x-amd64-linux",
							"c-16x-arm64-linux",
						},
					},
				})
			}
		})
		It("should create a standard NodeClaim", func() {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
					},
				},
			})
			env.ExpectCreated(nodeClass, nodeClaim)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			Expect(node.Labels).To(HaveKeyWithValue(v1.CapacityTypeLabelKey, v1.CapacityTypeOnDemand))
			env.EventuallyExpectNodeClaimsReady(nodeClaim)
		})
		It("should create a standard NodeClaim based on resource requests", func() {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
					Resources: v1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("11Gi"),
						},
					},
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
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
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"custom-annotation": "custom-value",
					},
					Labels: map[string]string{
						"custom-label": "custom-value",
					},
				},
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
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
					Resources: v1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("16Gi"),
						},
					},
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
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
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
					},
				},
			})
			env.ExpectCreated(nodeClass, nodeClaim)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]

			// Node is deleted and now should be not found
			env.ExpectDeleted(nodeClaim)
			env.EventuallyExpectNotFound(nodeClaim, node)
		})
		It("should delete a NodeClaim from the node termination finalizer", func() {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
					},
				},
			})
			env.ExpectCreated(nodeClass, nodeClaim)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]

			// Delete the node and expect both the node and nodeClaim to be gone as well as the instance to be shutting-down
			env.ExpectDeleted(node)
			env.EventuallyExpectNotFound(nodeClaim, node)
		})
		It("should delete a NodeClaim after the registration timeout when the node doesn't register", func() {
			env.ExpectBlockNodeRegistration()

			nodeClaim := test.NodeClaim(v1.NodeClaim{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{"registration": "fail"},
				}),
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
					},
				},
			})
			env.ExpectCreated(nodeClass, nodeClaim)
			nodeClaim = env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]

			// Expect that the nodeClaim eventually launches and has unknown Registration/Initialization
			Consistently(func(g Gomega) {
				temp := &v1.NodeClaim{}
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), temp)).To(Succeed())
				g.Expect(temp.StatusConditions().Get(v1.ConditionTypeRegistered).IsUnknown()).To(BeTrue())
				g.Expect(temp.StatusConditions().Get(v1.ConditionTypeInitialized).IsUnknown()).To(BeTrue())
			}).Should(Succeed())

			// Expect that the nodeClaim is eventually de-provisioned due to the registration timeout
			Eventually(func(g Gomega) {
				g.Expect(errors.IsNotFound(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim))).To(BeTrue())
			}).WithTimeout(time.Minute * 17).Should(Succeed())
		})
		It("should delete a NodeClaim if it references a NodeClass that doesn't exist", func() {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
					},
				},
			})
			// Don't create the NodeClass and expect that the NodeClaim fails and gets deleted
			env.ExpectCreated(nodeClaim)
			env.EventuallyExpectNotFound(nodeClaim)
		})
		It("should delete a NodeClaim if it references a NodeClass that isn't Ready", func() {
			env.ExpectCreated(nodeClass)
			By("Validating the NodeClass status condition has been reconciled")
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
				_, found, err := unstructured.NestedSlice(nodeClass.Object, "status", "conditions")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
			}, 5*time.Second).Should(Succeed())

			env.ExpectBlockNodeClassStatus(nodeClass)
			nodeClass = env.ExpectReplaceNodeClassCondition(nodeClass, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: nodeClass.GetGeneration(),
				Reason:             "TestingNotReady",
				Message:            "NodeClass is not ready",
			})
			env.ExpectStatusUpdated(nodeClass)

			nodeClaim := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
					},
				},
			})
			env.ExpectCreated(nodeClaim)
			env.EventuallyExpectNotFound(nodeClaim)
		})
		It("should succeed to garbage collect an Instance that was launched by a NodeClaim but has no Instance mapping", func() {
			env.ExpectBlockNodeRegistration()

			nodeClaim := test.NodeClaim(v1.NodeClaim{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{"registration": "fail"},
				}),
				Spec: v1.NodeClaimSpec{
					Requirements: requirements,
					NodeClassRef: &v1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.GetName(),
					},
				},
			})
			env.ExpectCreated(nodeClass, nodeClaim)
			nodeClaim = env.ExpectExists(nodeClaim).(*v1.NodeClaim)

			By("Updated NodeClaim Status")
			nodeClaim.Status.ProviderID = "Provider:///AZ/i-01234567890123456"
			nodeClaim.Status.NodeName = "test-node-name"
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeLaunched)
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeRegistered)
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeInitialized)
			env.ExpectStatusUpdated(nodeClaim)
			env.EventuallyExpectNotFound(nodeClaim)
		})
	})
})
