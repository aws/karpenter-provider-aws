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

package lifecycle_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudproviderapi "k8s.io/cloud-provider/api"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Initialization", func() {
	var nodePool *v1.NodePool

	BeforeEach(func() {
		nodePool = test.NodePool()
	})
	DescribeTable(
		"Initialization",
		func(isNodeClaimManaged bool) {
			nodeClaimOpts := []v1.NodeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
				Spec: v1.NodeClaimSpec{
					Resources: v1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
							corev1.ResourcePods:   resource.MustParse("5"),
						},
					},
				},
			}}
			if !isNodeClaimManaged {
				nodeClaimOpts = append(nodeClaimOpts, v1.NodeClaim{
					Spec: v1.NodeClaimSpec{
						NodeClassRef: &v1.NodeClassReference{
							Group: "karpenter.test.sh",
							Kind:  "UnmanagedNodeClass",
							Name:  "default",
						},
					},
				})
			}
			nodeClaim := test.NodeClaim(nodeClaimOpts...)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

			node := test.Node(test.NodeOptions{
				ProviderID: nodeClaim.Status.ProviderID,
				Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
			})
			ExpectApplied(ctx, env.Client, node)

			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
			ExpectMakeNodesReady(ctx, env.Client, node) // Remove the not-ready taint

			// If we're testing that Karpenter correctly ignores unmanaged NodeClaims, we must set the registered
			// status condition manually since the registration sub-reconciler should also ignore it.
			if !isNodeClaimManaged {
				nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeRegistered)
				ExpectApplied(ctx, env.Client, nodeClaim)
			}

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).IsUnknown()).To(BeTrue())

			node = ExpectExists(ctx, env.Client, node)
			node.Status.Capacity = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			}
			node.Status.Allocatable = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			}
			ExpectApplied(ctx, env.Client, node)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).IsTrue()).To(Equal(isNodeClaimManaged))
		},
		Entry("should consider the NodeClaim initialized when all initialization conditions are met", true),
		Entry("should ignore NodeClaims which aren't managed by this Karpenter instance", false),
	)
	It("shouldn't consider the nodeClaim initialized when it has not registered", func() {
		nodeClaim := test.NodeClaim()
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node1 := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
		})
		node2 := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
		})
		ExpectApplied(ctx, env.Client, node1, node2)

		// does not error but will not be registered because this reconcile returned multiple nodes
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectMakeNodesReady(ctx, env.Client, node1, node2) // Remove the not-ready taint

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionFalse))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))

		node1 = ExpectExists(ctx, env.Client, node1)
		node1.Status.Capacity = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
			corev1.ResourcePods:   resource.MustParse("110"),
		}
		node1.Status.Allocatable = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("8"),
			corev1.ResourceMemory: resource.MustParse("80Mi"),
			corev1.ResourcePods:   resource.MustParse("110"),
		}
		node2 = ExpectExists(ctx, env.Client, node2)
		node2.Status.Capacity = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
			corev1.ResourcePods:   resource.MustParse("110"),
		}
		node2.Status.Allocatable = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("8"),
			corev1.ResourceMemory: resource.MustParse("80Mi"),
			corev1.ResourcePods:   resource.MustParse("110"),
		}
		ExpectApplied(ctx, env.Client, node1, node2)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionFalse))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))
	})
	It("should add the initialization label to the node when the nodeClaim is initialized", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
						corev1.ResourcePods:   resource.MustParse("5"),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectMakeNodesReady(ctx, env.Client, node) // Remove the not-ready taint
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		node = ExpectExists(ctx, env.Client, node)
		Expect(node.Labels).To(HaveKeyWithValue(v1.NodeInitializedLabelKey, "true"))
	})
	It("should not consider the Node to be initialized when the status of the Node is NotReady", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
						corev1.ResourcePods:   resource.MustParse("5"),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			ReadyStatus: corev1.ConditionFalse,
			Taints:      []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))
	})
	It("should not consider the Node to be initialized when all requested resources aren't registered", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:      resource.MustParse("2"),
						corev1.ResourceMemory:   resource.MustParse("50Mi"),
						corev1.ResourcePods:     resource.MustParse("5"),
						fake.ResourceGPUVendorA: resource.MustParse("1"),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		// Update the nodeClaim to add mock the instance type having an extended resource
		nodeClaim.Status.Capacity[fake.ResourceGPUVendorA] = resource.MustParse("2")
		nodeClaim.Status.Allocatable[fake.ResourceGPUVendorA] = resource.MustParse("2")
		ExpectApplied(ctx, env.Client, nodeClaim)

		// Extended resource hasn't registered yet by the daemonset
		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectMakeNodesReady(ctx, env.Client, node) // Remove the not-ready taint
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))
	})
	It("should consider the node to be initialized once all the resources are registered", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:      resource.MustParse("2"),
						corev1.ResourceMemory:   resource.MustParse("50Mi"),
						corev1.ResourcePods:     resource.MustParse("5"),
						fake.ResourceGPUVendorA: resource.MustParse("1"),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		// Update the nodeClaim to add mock the instance type having an extended resource
		nodeClaim.Status.Capacity[fake.ResourceGPUVendorA] = resource.MustParse("2")
		nodeClaim.Status.Allocatable[fake.ResourceGPUVendorA] = resource.MustParse("2")
		ExpectApplied(ctx, env.Client, nodeClaim)

		// Extended resource hasn't registered yet by the daemonset
		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectMakeNodesReady(ctx, env.Client, node) // Remove the not-ready taint

		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))

		// Node now registers the resource
		node = ExpectExists(ctx, env.Client, node)
		node.Status.Capacity[fake.ResourceGPUVendorA] = resource.MustParse("2")
		node.Status.Allocatable[fake.ResourceGPUVendorA] = resource.MustParse("2")
		ExpectApplied(ctx, env.Client, node)

		// Reconcile the nodeClaim and the nodeClaim/Node should now be initilized
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionTrue))
	})
	It("should not consider the Node to be initialized when all startupTaints aren't removed", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
						corev1.ResourcePods:   resource.MustParse("5"),
					},
				},
				StartupTaints: []corev1.Taint{
					{
						Key:    "custom-startup-taint",
						Effect: corev1.TaintEffectNoSchedule,
						Value:  "custom-startup-value",
					},
					{
						Key:    "other-custom-startup-taint",
						Effect: corev1.TaintEffectNoExecute,
						Value:  "other-custom-startup-value",
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectMakeNodesReady(ctx, env.Client, node) // Remove the not-ready taint

		// Should add the startup taints to the node
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)
		Expect(node.Spec.Taints).To(ContainElements(
			corev1.Taint{
				Key:    "custom-startup-taint",
				Effect: corev1.TaintEffectNoSchedule,
				Value:  "custom-startup-value",
			},
			corev1.Taint{
				Key:    "other-custom-startup-taint",
				Effect: corev1.TaintEffectNoExecute,
				Value:  "other-custom-startup-value",
			},
		))

		// Shouldn't consider the node ready since the startup taints still exist
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))
	})
	It("should consider the Node to be initialized once the startupTaints are removed", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
						corev1.ResourcePods:   resource.MustParse("5"),
					},
				},
				StartupTaints: []corev1.Taint{
					{
						Key:    "custom-startup-taint",
						Effect: corev1.TaintEffectNoSchedule,
						Value:  "custom-startup-value",
					},
					{
						Key:    "other-custom-startup-taint",
						Effect: corev1.TaintEffectNoExecute,
						Value:  "other-custom-startup-value",
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectMakeNodesReady(ctx, env.Client, node) // Remove the not-ready taint

		// Shouldn't consider the node ready since the startup taints still exist
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))

		node = ExpectExists(ctx, env.Client, node)
		node.Spec.Taints = []corev1.Taint{}
		ExpectApplied(ctx, env.Client, node)

		// nodeClaim should now be ready since all startup taints are removed
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionTrue))
	})
	It("should not consider the Node to be initialized when all ephemeralTaints aren't removed", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
						corev1.ResourcePods:   resource.MustParse("5"),
					},
				},
				StartupTaints: []corev1.Taint{
					{
						Key:    "custom-startup-taint",
						Effect: corev1.TaintEffectNoSchedule,
						Value:  "custom-startup-value",
					},
					{
						Key:    "other-custom-startup-taint",
						Effect: corev1.TaintEffectNoExecute,
						Value:  "other-custom-startup-value",
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Taints: []corev1.Taint{
				{
					Key:    corev1.TaintNodeNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    corev1.TaintNodeUnreachable,
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    cloudproviderapi.TaintExternalCloudProvider,
					Effect: corev1.TaintEffectNoSchedule,
					Value:  "true",
				},
				v1.UnregisteredNoExecuteTaint,
			},
		})
		ExpectApplied(ctx, env.Client, node)

		// Shouldn't consider the node ready since the ephemeral taints still exist
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))
	})
	It("should consider the Node to be initialized once the ephemeralTaints are removed", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
						corev1.ResourcePods:   resource.MustParse("5"),
					},
				},
				StartupTaints: []corev1.Taint{
					{
						Key:    "custom-startup-taint",
						Effect: corev1.TaintEffectNoSchedule,
						Value:  "custom-startup-value",
					},
					{
						Key:    "other-custom-startup-taint",
						Effect: corev1.TaintEffectNoExecute,
						Value:  "other-custom-startup-value",
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("80Mi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Taints: []corev1.Taint{
				{
					Key:    corev1.TaintNodeNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    corev1.TaintNodeUnreachable,
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    cloudproviderapi.TaintExternalCloudProvider,
					Effect: corev1.TaintEffectNoSchedule,
					Value:  "true",
				},
				v1.UnregisteredNoExecuteTaint,
			},
		})
		ExpectApplied(ctx, env.Client, node)

		// Shouldn't consider the node ready since the ephemeral taints still exist
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionUnknown))

		node = ExpectExists(ctx, env.Client, node)
		node.Spec.Taints = []corev1.Taint{}
		ExpectApplied(ctx, env.Client, node)

		// nodeClaim should now be ready since all startup taints are removed
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeRegistered).Status).To(Equal(metav1.ConditionTrue))
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeInitialized).Status).To(Equal(metav1.ConditionTrue))
	})
})
