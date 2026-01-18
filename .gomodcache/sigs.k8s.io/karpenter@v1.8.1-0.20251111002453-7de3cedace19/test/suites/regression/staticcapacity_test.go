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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("StaticCapacity", func() {
	Context("Provisioning", func() {
		BeforeEach(func() {
			nodePool.Spec.Replicas = lo.ToPtr(int64(1))
			if env.IsDefaultNodeClassKWOK() {
				nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
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

		It("should create static NodeClaims to meet desired replicas", func() {
			nodePool.Spec.Replicas = lo.ToPtr(int64(10))
			env.ExpectCreated(nodeClass, nodePool)
			env.EventuallyExpectInitializedNodeCount("==", 10)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 10)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			nodePool.Spec.Replicas = lo.ToPtr(int64(15))
			env.ExpectUpdated(nodePool)
			nodes := env.EventuallyExpectInitializedNodeCount("==", 15)
			nodeClaims = env.EventuallyExpectCreatedNodeClaimCount("==", 15)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			for _, node := range nodes {
				Expect(node.Labels).To(HaveKeyWithValue(v1.NodePoolLabelKey, nodePool.Name))
			}
		})

		It("should create static NodeClaim propagating all the NodePool spec details", func() {
			nodePool.Spec.Template.ObjectMeta = v1.ObjectMeta{
				Annotations: map[string]string{
					"custom-annotation": "custom-value",
				},
				Labels: map[string]string{
					"custom-label": "custom-value",
				},
			}
			nodePool.Spec.Template.Spec.Taints = []corev1.Taint{
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
			}
			env.ExpectCreated(nodeClass, nodePool)
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

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 1)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)
		})

		It("should respect node limits when provisioning", func() {
			nodePool.Spec.Replicas = lo.ToPtr(int64(10))
			nodePool.Spec.Limits = v1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse("5"),
			}
			env.ExpectCreated(nodeClass, nodePool)

			// Should only create 5 nodes due to limit
			env.ConsistentlyExpectNodeClaimCountNotExceed(5*time.Minute, 5)

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 5)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)
		})
	})
	Context("Deprovisioning", func() {
		BeforeEach(func() {
			nodePool.Spec.Replicas = lo.ToPtr(int64(3))
			if env.IsDefaultNodeClassKWOK() {
				nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
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
			env.ExpectCreated(nodeClass, nodePool)
		})

		It("should scale down to zero", func() {
			// Initially should have 3 nodes
			env.EventuallyExpectInitializedNodeCount("==", 3)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			// Scale down to 0
			nodePool.Spec.Replicas = lo.ToPtr(int64(0))
			env.ExpectUpdated(nodePool)

			// Create no more
			env.EventuallyExpectInitializedNodeCount("==", 0)
			nodeClaims = env.EventuallyExpectCreatedNodeClaimCount("==", 0)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)
		})

		It("should terminate empty nodes first, then nodes with least cost, then respect do-not-disrupt", func() {
			nodePool.Spec.Replicas = lo.ToPtr(int64(5))
			env.ExpectUpdated(nodeClass, nodePool)
			nodes := env.EventuallyExpectInitializedNodeCount("==", 5)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 5)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			pods := test.Pods(2, test.PodOptions{})

			for i, pod := range pods {
				pod.Spec.NodeName = nodes[i].Name
				env.ExpectCreated(pod)
			}

			doNotDisruptPod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1.DoNotDisruptAnnotationKey: "true",
					},
				},
			})
			doNotDisruptPod.Spec.NodeName = nodes[2].Name
			env.ExpectCreated(doNotDisruptPod)

			nodePool.Spec.Replicas = lo.ToPtr(int64(3))
			env.ExpectUpdated(nodePool)

			remainingNodes := env.EventuallyExpectInitializedNodeCount("==", 3)
			env.EventuallyExpectCreatedNodeClaimCount("==", 3)

			// The nodes with pods and do-not-disrupt should remain
			nodeNames := lo.Map(remainingNodes, func(n *corev1.Node, _ int) string { return n.Name })
			Expect(nodeNames).To(ContainElement(nodes[0].Name)) // node with regular pod
			Expect(nodeNames).To(ContainElement(nodes[1].Name)) // node with regular pod
			Expect(nodeNames).To(ContainElement(nodes[2].Name)) // node with do-not-disrupt pod

			nodePool.Spec.Replicas = lo.ToPtr(int64(1))
			env.ExpectUpdated(nodePool)

			finalNodes := env.EventuallyExpectInitializedNodeCount("==", 1)
			env.EventuallyExpectCreatedNodeClaimCount("==", 1)

			// The node with the pod should still exist
			Expect(finalNodes[0].Name).To(Equal(nodes[2].Name))
		})

		It("should handle graceful pod eviction during scale down", func() {
			// Initially should have 3 nodes
			env.EventuallyExpectInitializedNodeCount("==", 3)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			// Create a deployment with pods on multiple nodes
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: lo.ToPtr(int32(2)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "test"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "test",
								Image: "nginx",
							}},
						},
					},
				},
			}
			env.ExpectCreated(deployment)
			selector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)

			env.EventuallyExpectHealthyPodCount(selector, 2)

			// Scale down to 1
			nodePool.Spec.Replicas = lo.ToPtr(int64(1))
			env.ExpectUpdated(nodePool)

			// Should eventually scale down to 1 node
			env.EventuallyExpectInitializedNodeCount("==", 1)
			env.EventuallyExpectCreatedNodeClaimCount("==", 1)

			// Pods should be rescheduled or remain running
			env.EventuallyExpectHealthyPodCount(selector, 2)
		})
	})

	Context("Drift", func() {
		BeforeEach(func() {
			nodePool.Spec.Replicas = lo.ToPtr(int64(10))
			if env.IsDefaultNodeClassKWOK() {
				nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
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
			env.ExpectCreated(nodeClass, nodePool)
		})

		It("should replace drifted nodes", func() {
			// Initially should have 10 nodes
			env.EventuallyExpectInitializedNodeCount("==", 10)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 10)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			// Drift the nodeclaims
			nodePool.Spec.Template.Annotations = map[string]string{"test": "annotation"}
			env.ExpectUpdated(nodePool)

			// Verify the drifted node was replaced
			env.EventuallyExpectDrifted(nodeClaims...)

			// Should create a replacement node and then remove the drifted one
			env.ConsistentlyExpectDisruptionsUntilNoneLeft(10, 10, 5*time.Minute)
		})

		It("should handle drift with node limits when budget allows", func() {
			// Initially should have 10 nodes
			env.EventuallyExpectInitializedNodeCount("==", 10)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 10)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			// Allows 4 node drift
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "4"}}

			// Allows 2 drifts
			nodePool.Spec.Limits = v1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse("12"),
			}
			nodePool.Spec.Template.Annotations = map[string]string{"test": "annotation"}
			env.ExpectUpdated(nodePool)

			// Verify the drifted node was replaced
			env.EventuallyExpectDrifted(nodeClaims...)

			// Should create a replacement node and then remove the drifted one 2 at a time
			env.ConsistentlyExpectDisruptionsUntilNoneLeft(10, 2, 5*time.Minute)
		})

		It("should handle drift with node limits when budget restricts", func() {
			// Initially should have 10 nodes
			env.EventuallyExpectInitializedNodeCount("==", 10)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 10)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			// Allows 2 node drift
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "2"}}

			// Allows 5 drifts
			nodePool.Spec.Limits = v1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse("15"),
			}
			nodePool.Spec.Template.Annotations = map[string]string{"test": "annotation"}
			env.ExpectUpdated(nodePool)

			// Verify the drifted node was replaced
			env.EventuallyExpectDrifted(nodeClaims...)

			// Should create a replacement node and then remove the drifted one 2 at a time
			env.ConsistentlyExpectDisruptionsUntilNoneLeft(10, 2, 5*time.Minute)
		})
	})

	Context("Dynamic NodeClaim Interaction", func() {
		var dynamicNodePool *v1.NodePool
		var label map[string]string
		BeforeEach(func() {
			// Create a static NodePool
			nodePool.Spec.Replicas = lo.ToPtr(int64(2))
			if env.IsDefaultNodeClassKWOK() {
				nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
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
			nodePool.Spec.Template.Spec.Taints = []corev1.Taint{
				{
					Key:    "static",
					Effect: corev1.TaintEffectNoExecute,
				},
			}
			// Create a dynamic NodePool
			dynamicNodePool = test.NodePool(v1.NodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dynamic-nodepool",
				},
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: nodePool.Spec.Template.Spec.Requirements,
							NodeClassRef: nodePool.Spec.Template.Spec.NodeClassRef,
						},
					},
				},
			})
			label = map[string]string{"app": "large-app"}
		})

		It("should provision dynamic nodeclaims when static nodeclaims cant satisfy pod constraints", func() {
			env.ExpectCreated(nodeClass, nodePool, dynamicNodePool)

			env.EventuallyExpectInitializedNodeCount("==", 2)
			staticNodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 2)
			env.EventuallyExpectNodeClaimsReady(staticNodeClaims...)

			for _, nc := range staticNodeClaims {
				Expect(nc.Labels).To(HaveKeyWithValue(v1.NodePoolLabelKey, nodePool.Name))
			}

			// Create pods that require dynamic provisioning
			pods := test.Pods(3, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1.DoNotDisruptAnnotationKey: "true",
					},
					Labels: label,
				},
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       corev1.LabelHostname,
						WhenUnsatisfiable: corev1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: label,
						},
					},
				},
				PodAntiRequirements: []corev1.PodAffinityTerm{{
					TopologyKey: corev1.LabelHostname,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: label,
					},
				}},
			})

			for _, pod := range pods {
				env.ExpectCreated(pod)
			}

			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(label), 3)

			// Should create dynamic nodes for the pods
			env.EventuallyExpectInitializedNodeCount("==", 5) // At least 3 nodes (2 static + 1+ dynamic)
			allNodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 5)

			dynamicNodeClaims := lo.Filter(allNodeClaims, func(nc *v1.NodeClaim, _ int) bool {
				return nc.Labels[v1.NodePoolLabelKey] == dynamicNodePool.Name
			})
			Expect(len(dynamicNodeClaims)).To(BeNumerically("==", 3))

			// Verify static NodePool still has exactly 2 nodes
			staticNodeClaimsAfter := lo.Filter(allNodeClaims, func(nc *v1.NodeClaim, _ int) bool {
				return nc.Labels[v1.NodePoolLabelKey] == nodePool.Name
			})
			Expect(len(staticNodeClaimsAfter)).To(Equal(2))
		})
	})

	Context("Edge Cases", func() {
		BeforeEach(func() {
			if env.IsDefaultNodeClassKWOK() {
				nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
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

		It("should not go over limit during concurrent changes/executions", func() {
			// Start with 3 replicas and set a limit of 5 nodes
			nodePool.Spec.Replicas = lo.ToPtr(int64(3))
			nodePool.Spec.Limits = v1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse("5"),
			}
			env.ExpectCreated(nodeClass, nodePool)

			// Wait for initial nodes to be created
			env.EventuallyExpectInitializedNodeCount("==", 3)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			// Trigger drift by changing NodePool template
			nodePool.Spec.Template.Annotations = map[string]string{"drift-trigger": "test-value"}

			// Simultaneously scale up to 4 replicas while drift is happening
			// This should test that we don't exceed the limit of 5 during replacement
			nodePool.Spec.Replicas = lo.ToPtr(int64(4))
			env.ExpectUpdated(nodePool)

			// Verify drift is detected on existing nodes
			env.EventuallyExpectDrifted(nodeClaims...)

			// Throughout the entire process, we should never exceed 5 nodes
			// This includes temporary nodes created during drift replacement
			env.ConsistentlyExpectNodeClaimCountNotExceed(3*time.Minute, 5)

			// Should eventually have exactly 4 nodes (the target replica count)
			env.EventuallyExpectInitializedNodeCount("==", 4)
			finalNodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 4)
			env.EventuallyExpectNodeClaimsReady(finalNodeClaims...)

			// All final nodes should have the new annotation (either newly created or replaced due to drift)
			nodes := env.EventuallyExpectInitializedNodeCount("==", 4)
			for _, node := range nodes {
				Expect(node.Annotations).To(HaveKeyWithValue("drift-trigger", "test-value"))
			}
		})

		It("should handle NodePool deletion gracefully", func() {
			nodePool.Spec.Replicas = lo.ToPtr(int64(3))
			env.ExpectCreated(nodeClass, nodePool)

			env.EventuallyExpectInitializedNodeCount("==", 3)
			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			env.EventuallyExpectNodeClaimsReady(nodeClaims...)

			// Delete the NodePool
			env.ExpectDeleted(nodePool)

			// All nodes should eventually be cleaned up
			env.EventuallyExpectInitializedNodeCount("==", 0)
			env.EventuallyExpectCreatedNodeClaimCount("==", 0)
		})
	})
})
