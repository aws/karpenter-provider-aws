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
	"errors"
	"time"

	"github.com/awslabs/operatorpkg/object"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/lifecycle"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Termination", func() {
	var nodePool *v1.NodePool
	var nodeClaim *v1.NodeClaim

	BeforeEach(func() {
		nodePool = test.NodePool()
		nodeClaim = test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
				Finalizers: []string{
					v1.TerminationFinalizer,
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
		lifecycle.InstanceTerminationDurationSeconds.Reset()
	})
	DescribeTable(
		"Termination",
		func(isNodeClaimManaged bool) {
			if !isNodeClaimManaged {
				nodeClaim.Spec.NodeClassRef = &v1.NodeClassReference{
					Group: "karpenter.test.sh",
					Kind:  "UnmanagedNodeClass",
					Name:  "default",
				}
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

			if isNodeClaimManaged {
				nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
				_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
				Expect(err).ToNot(HaveOccurred())
			}

			node := test.NodeClaimLinkedNode(nodeClaim)
			ExpectApplied(ctx, env.Client, node)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(Equal(isNodeClaimManaged))

			if !isNodeClaimManaged {
				nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeRegistered)
				ExpectApplied(ctx, env.Client, nodeClaim)
			}

			// Expect the node and the nodeClaim to both be gone
			Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // triggers the node deletion
			ExpectFinalizersRemoved(ctx, env.Client, node)
			if isNodeClaimManaged {
				ExpectNotFound(ctx, env.Client, node)
				result := ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // now all the nodes are gone so nodeClaim deletion continues
				Expect(result.RequeueAfter).To(BeEquivalentTo(5 * time.Second))
				nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
				Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInstanceTerminating).IsTrue()).To(BeTrue())

				ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // this will call cloudProvider Get to check if the instance is still around

				ExpectMetricHistogramSampleCountValue("karpenter_nodeclaims_instance_termination_duration_seconds", 1, map[string]string{"nodepool": nodePool.Name})
				ExpectMetricHistogramSampleCountValue("karpenter_nodeclaims_termination_duration_seconds", 1, map[string]string{"nodepool": nodePool.Name})
				ExpectNotFound(ctx, env.Client, nodeClaim, node)

				// Expect the nodeClaim to be gone from the cloudprovider
				_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
				Expect(cloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
			} else {
				ExpectExists(ctx, env.Client, node)
				ExpectExists(ctx, env.Client, nodeClaim)
			}
		},
		Entry("should delete the node and the CloudProvider NodeClaim when NodeClaim deletion is triggered", true),
		Entry("should ignore NodeClaims which aren't managed by this Karpenter instance", false),
	)
	It("shouldn't mark the root condition of the NodeClaim as unknown when setting the Termination condition", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())

		// Initialize the NodeClaim
		ExpectMakeNodesReady(ctx, env.Client, node) // Remove the not-ready taint
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
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

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).IsTrue()).To(BeTrue())
		Expect(nodeClaim.StatusConditions().Root().IsTrue())

		// Start deleting the NodeClaim and update the status condition
		// We need to ensure that the root condition doesn't change to Unknown
		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())

		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // Delete the Node
		ExpectFinalizersRemoved(ctx, env.Client, node)
		ExpectNotFound(ctx, env.Client, node)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // Trigger CloudProvider Delete
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInstanceTerminating).IsTrue()).To(BeTrue())
		Expect(nodeClaim.StatusConditions().Root().IsTrue())
	})
	It("should delete the NodeClaim when the spec resource.Quantity values will change during deserialization", func() {
		nodeClaim.SetGroupVersionKind(object.GVK(nodeClaim)) // This is needed so that the GVK is set on the unstructured object
		u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(nodeClaim)
		Expect(err).ToNot(HaveOccurred())
		// Set a value in resources that will get to converted to a value with a suffix e.g. 50k
		Expect(unstructured.SetNestedStringMap(u, map[string]string{"memory": "50000"}, "spec", "resources", "requests")).To(Succeed())

		obj := &unstructured.Unstructured{}
		Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(u, obj)).To(Succeed())

		ExpectApplied(ctx, env.Client, nodePool, obj)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		// Expect the node and the nodeClaim to both be gone
		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		result := ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // triggers the nodeclaim deletion

		Expect(result.RequeueAfter).To(BeEquivalentTo(5 * time.Second))
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInstanceTerminating).IsTrue()).To(BeTrue())

		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // this will call cloudProvider Get to check if the instance is still around
		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
	It("should requeue reconciliation if cloudProvider Delete returns an error other than NodeClaimNotFoundError", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(err).ToNot(HaveOccurred())
		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		result := ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // trigger nodeClaim Deletion that will set the nodeClaim status as terminating
		Expect(result.RequeueAfter).To(BeEquivalentTo(5 * time.Second))
		cloudProvider.NextDeleteErr = errors.New("fake error")
		// trigger nodeClaim Deletion that will make cloudProvider Delete and requeue reconciliation due to error
		Expect(ExpectObjectReconcileFailed(ctx, env.Client, nodeClaimController, nodeClaim)).To(HaveOccurred())
		result = ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // trigger nodeClaim Deletion that will succeed
		//nolint:staticcheck
		Expect(result.Requeue).To(BeFalse())
		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
	It("should not remove the finalizer and terminate the NodeClaim if the cloudProvider instance is still around", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(err).ToNot(HaveOccurred())
		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		// The delete call that happened first will remove the cloudProvider instance from cloudProvider.CreatedNodeClaims[].
		// To model the behavior of having cloudProvider instance not terminated, we add it back here.
		cloudProvider.CreatedNodeClaims[nodeClaim.Status.ProviderID] = nodeClaim
		result := ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // this will ensure that we call cloudProvider Get to check if the instance is still around
		Expect(result.RequeueAfter).To(BeEquivalentTo(5 * time.Second))
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInstanceTerminating).IsTrue()).To(BeTrue())
	})
	It("should delete multiple Nodes if multiple Nodes map to the NodeClaim", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(err).ToNot(HaveOccurred())

		// First register a single Node to ensure the NodeClaim can successfully register, then apply the remaining nodes.
		node1 := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, node1)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())

		node2 := test.NodeClaimLinkedNode(nodeClaim)
		node3 := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, node2, node3)

		// Expect the node and the nodeClaim to both be gone
		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // triggers the node deletion
		ExpectFinalizersRemoved(ctx, env.Client, node1, node2, node3)
		ExpectNotFound(ctx, env.Client, node1, node2, node3)

		result := ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // now all nodes are gone so nodeClaim deletion continues
		Expect(result.RequeueAfter).To(BeEquivalentTo(5 * time.Second))
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // this will call cloudProvider Get to check if the instance is still around

		ExpectMetricHistogramSampleCountValue("karpenter_nodeclaims_instance_termination_duration_seconds", 1, map[string]string{"nodepool": nodePool.Name})
		ExpectMetricHistogramSampleCountValue("karpenter_nodeclaims_termination_duration_seconds", 1, map[string]string{"nodepool": nodePool.Name})
		ExpectNotFound(ctx, env.Client, nodeClaim, node1, node2, node3)

		// Expect the nodeClaim to be gone from the cloudprovider
		_, err = cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(cloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
	})
	It("should not delete the NodeClaim until all the Nodes are removed", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(err).ToNot(HaveOccurred())

		node := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())

		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // triggers the node deletion
		ExpectExists(ctx, env.Client, nodeClaim)                                // the node still hasn't been deleted, so the nodeClaim should remain
		ExpectExists(ctx, env.Client, node)
		ExpectFinalizersRemoved(ctx, env.Client, node)
		ExpectNotFound(ctx, env.Client, node)

		result := ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // now the nodeClaim should be gone
		Expect(result.RequeueAfter).To(BeEquivalentTo(5 * time.Second))
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // this will call cloudProvider Get to check if the instance is still around

		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
	It("should not call Delete() on the CloudProvider if the NodeClaim hasn't been launched yet", func() {
		nodeClaim.Status.ProviderID = ""
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

		// Expect the nodeClaim to be gone
		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		Expect(cloudProvider.DeleteCalls).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
	It("should not delete nodes without provider ids if the NodeClaim hasn't been launched yet", func() {
		// Generate 10 nodes, none of which have a provider id
		var nodes []*corev1.Node
		for i := 0; i < 10; i++ {
			nodes = append(nodes, test.Node())
		}
		ExpectApplied(ctx, env.Client, lo.Map(nodes, func(n *corev1.Node, _ int) client.Object { return n })...)

		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

		// Expect the nodeClaim to be gone
		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		ExpectMetricHistogramSampleCountValue("karpenter_nodeclaims_instance_termination_duration_seconds", 1, map[string]string{"nodepool": nodePool.Name})
		ExpectMetricHistogramSampleCountValue("karpenter_nodeclaims_termination_duration_seconds", 1, map[string]string{"nodepool": nodePool.Name})
		ExpectNotFound(ctx, env.Client, nodeClaim)
		for _, node := range nodes {
			ExpectExists(ctx, env.Client, node)
		}
	})
	It("should not annotate the node if the NodeClaim has no terminationGracePeriod", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(err).ToNot(HaveOccurred())

		node := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())

		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // triggers the node deletion
		ExpectExists(ctx, env.Client, node)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		Expect(nodeClaim.ObjectMeta.Annotations).To(BeNil())
	})
	It("should annotate the node if the NodeClaim has a terminationGracePeriod", func() {
		nodeClaim.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(err).ToNot(HaveOccurred())

		node := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())

		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // triggers the node deletion
		ExpectExists(ctx, env.Client, node)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		_, annotationExists := nodeClaim.Annotations[v1.NodeClaimTerminationTimestampAnnotationKey]
		Expect(annotationExists).To(BeTrue())
	})
	It("should not change the annotation if the NodeClaim has a terminationGracePeriod and the annotation already exists", func() {
		nodeClaim.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
		nodeClaim.Annotations = map[string]string{
			v1.NodeClaimTerminationTimestampAnnotationKey: "2024-04-01T12:00:00-05:00",
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(err).ToNot(HaveOccurred())

		node := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, node, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())

		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim) // triggers the node deletion
		ExpectExists(ctx, env.Client, node)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		Expect(nodeClaim.ObjectMeta.Annotations).To(Equal(map[string]string{
			v1.NodeClaimTerminationTimestampAnnotationKey: "2024-04-01T12:00:00-05:00",
		}))
	})
	It("should not delete Nodes if the NodeClaim is not registered", func() {
		node := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
		Expect(err).ToNot(HaveOccurred())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectExists(ctx, env.Client, node)
		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
})
