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

package counter_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/nodepool/counter"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/controllers/state/informer"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var nodePoolController *counter.Controller
var nodePoolInformerController *informer.NodePoolController
var nodeClaimController *informer.NodeClaimController
var nodeController *informer.NodeController
var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var fakeClock *clock.FakeClock
var cloudProvider *fake.CloudProvider
var node, node2 *corev1.Node

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Counter")
}

var _ = BeforeSuite(func() {
	cloudProvider = fake.NewCloudProvider()
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	fakeClock = clock.NewFakeClock(time.Now())
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	nodeClaimController = informer.NewNodeClaimController(env.Client, cloudProvider, cluster)
	nodeController = informer.NewNodeController(env.Client, cluster)
	nodePoolInformerController = informer.NewNodePoolController(env.Client, cloudProvider, cluster)
	nodePoolController = counter.NewController(env.Client, cloudProvider, cluster)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var nodePool *v1.NodePool
var nodeClaim, nodeClaim2 *v1.NodeClaim
var expected corev1.ResourceList

var _ = Describe("Counter", func() {
	BeforeEach(func() {
		cloudProvider.InstanceTypes = fake.InstanceTypesAssorted()
		nodePool = test.NodePool()
		instanceType := cloudProvider.InstanceTypes[0]
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: instanceType.Name,
			}},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourcePods:   resource.MustParse("256"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		})
		nodeClaim2, node2 = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: instanceType.Name,
			}},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourcePods:   resource.MustParse("1000"),
					corev1.ResourceMemory: resource.MustParse("5Gi"),
				},
			},
		})
		expected = counter.BaseResources.DeepCopy()
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectObjectReconciled(ctx, env.Client, nodePoolInformerController, nodePool)
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
	})
	It("should ignore NodePools which aren't managed by this instance of Karpenter", func() {
		nodePool = test.NodePool(v1.NodePool{Spec: v1.NodePoolSpec{Template: v1.NodeClaimTemplate{Spec: v1.NodeClaimTemplateSpec{
			NodeClassRef: &v1.NodeClassReference{
				Group: "karpenter.test.sh",
				Kind:  "UnmanagedNodeClass",
				Name:  "default",
			},
		}}}})
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey: nodePool.Name,
			}},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourcePods:   resource.MustParse("256"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			Spec: v1.NodeClaimSpec{
				NodeClassRef: nodePool.Spec.Template.Spec.NodeClassRef,
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodePoolInformerController, nodePool)
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.Status.Resources).To(BeNil())
	})
	It("should set well-known resource to zero when no nodes exist in the cluster", func() {
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))
	})
	It("should set the counter from the nodeClaim and then to the node when it initializes", func() {
		ExpectApplied(ctx, env.Client, node, nodeClaim)
		// Don't initialize the node yet
		ExpectMakeNodeClaimsInitialized(ctx, env.Client, nodeClaim)
		// Inform cluster state about node and nodeClaim readiness
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		expected = resources.MergeInto(expected, nodeClaim.Status.Capacity)
		expected[corev1.ResourceName("nodes")] = resource.MustParse("1")
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))

		// Change the node capacity to be different than the nodeClaim capacity
		node.Status.Capacity = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourcePods:   resource.MustParse("512"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		}
		ExpectApplied(ctx, env.Client, node, nodeClaim)
		// Don't initialize the node yet
		ExpectMakeNodesInitialized(ctx, env.Client, node)
		// Inform cluster state about node and nodeClaim readiness
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		expected = counter.BaseResources.DeepCopy()
		expected = resources.MergeInto(expected, node.Status.Capacity)
		expected[corev1.ResourceName("nodes")] = resource.MustParse("1")
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))
	})
	It("should increase the counter when new nodes are created", func() {
		ExpectApplied(ctx, env.Client, node, nodeClaim)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		// Should equal both the nodeClaim and node capacity
		expected = resources.MergeInto(expected, nodeClaim.Status.Capacity)
		expected[corev1.ResourceName("nodes")] = resource.MustParse("1")
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))
		expected = counter.BaseResources.DeepCopy()
		expected = resources.MergeInto(expected, node.Status.Capacity)
		expected[corev1.ResourceName("nodes")] = resource.MustParse("1")
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))
	})
	It("should decrease the counter when an existing node is deleted", func() {
		ExpectApplied(ctx, env.Client, node, nodeClaim, node2, nodeClaim2)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		// Should equal the sums of the nodeClaims and nodes
		res := corev1.ResourceList{
			corev1.ResourceCPU:           resource.MustParse("600m"),
			corev1.ResourcePods:          resource.MustParse("1256"),
			corev1.ResourceMemory:        resource.MustParse("6Gi"),
			corev1.ResourceName("nodes"): resource.MustParse("2"),
		}
		expected = resources.MergeInto(expected, res)
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))

		ExpectDeleted(ctx, env.Client, node, nodeClaim)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		// Should equal both the nodeClaim and node capacity
		expected = counter.BaseResources.DeepCopy()
		expected = resources.MergeInto(expected, nodeClaim2.Status.Capacity)
		expected[corev1.ResourceName("nodes")] = resource.MustParse("1")
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))
		expected = counter.BaseResources.DeepCopy()
		expected = resources.MergeInto(expected, node2.Status.Capacity)
		expected[corev1.ResourceName("nodes")] = resource.MustParse("1")
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))
	})
	It("should zero out the counter when all nodes are deleted", func() {
		ExpectApplied(ctx, env.Client, node, nodeClaim)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		// Should equal both the nodeClaim and node capacity
		expected = resources.MergeInto(expected, nodeClaim.Status.Capacity)
		expected[corev1.ResourceName("nodes")] = resource.MustParse("1")
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))
		expected = counter.BaseResources.DeepCopy()
		expected = resources.MergeInto(expected, node.Status.Capacity)
		expected[corev1.ResourceName("nodes")] = resource.MustParse("1")
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))

		ExpectDeleted(ctx, env.Client, node, nodeClaim)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		expected = counter.BaseResources.DeepCopy()
		Expect(nodePool.Status.Resources).To(BeComparableTo(expected))
	})

	Context("Status.Nodes Field", func() {
		It("should set Status.Nodes to zero when no nodes exist", func() {
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
			nodePool = ExpectExists(ctx, env.Client, nodePool)

			Expect(nodePool.Status.Nodes).ToNot(BeNil())
			Expect(*nodePool.Status.Nodes).To(Equal(int64(0)))
		})
		It("should set Status.Nodes to 2 when two nodes exist", func() {
			ExpectApplied(ctx, env.Client, node, nodeClaim, node2, nodeClaim2)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})

			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
			nodePool = ExpectExists(ctx, env.Client, nodePool)

			Expect(nodePool.Status.Nodes).ToNot(BeNil())
			Expect(*nodePool.Status.Nodes).To(Equal(int64(2)))
		})
		It("should update Status.Nodes when nodes are added and removed", func() {
			// Start with no nodes
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
			nodePool = ExpectExists(ctx, env.Client, nodePool)
			Expect(*nodePool.Status.Nodes).To(Equal(int64(0)))

			// Add first node
			ExpectApplied(ctx, env.Client, node, nodeClaim)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
			nodePool = ExpectExists(ctx, env.Client, nodePool)
			Expect(*nodePool.Status.Nodes).To(Equal(int64(1)))

			// Add second node
			ExpectApplied(ctx, env.Client, node2, nodeClaim2)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
			nodePool = ExpectExists(ctx, env.Client, nodePool)
			Expect(*nodePool.Status.Nodes).To(Equal(int64(2)))

			// Remove first node
			ExpectDeleted(ctx, env.Client, node, nodeClaim)
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
			nodePool = ExpectExists(ctx, env.Client, nodePool)
			Expect(*nodePool.Status.Nodes).To(Equal(int64(1)))

			// Remove second node
			ExpectDeleted(ctx, env.Client, node2, nodeClaim2)
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node2))
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim2))
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
			nodePool = ExpectExists(ctx, env.Client, nodePool)
			Expect(*nodePool.Status.Nodes).To(Equal(int64(0)))
		})

		It("should handle multiple nodepools including static with different node counts", func() {
			// Create a second nodepool
			nodePool2 := test.StaticNodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Replicas: lo.ToPtr(int64(2)),
				},
			})
			ExpectApplied(ctx, env.Client, nodePool2)
			ExpectObjectReconciled(ctx, env.Client, nodePoolInformerController, nodePool2)

			// Create nodes for first nodepool
			ExpectApplied(ctx, env.Client, node, nodeClaim)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			// Create nodes for second nodepool
			instanceType := cloudProvider.InstanceTypes[0]
			nodeClaim3, node3 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool2.Name,
					corev1.LabelInstanceTypeStable: instanceType.Name,
					v1.NodeInitializedLabelKey:     "true",
				}},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("300m"),
					},
				},
			})
			nodeClaim4, node4 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool2.Name,
					corev1.LabelInstanceTypeStable: instanceType.Name,
					v1.NodeInitializedLabelKey:     "true",
				}},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("300m"),
					},
				},
			})

			ExpectApplied(ctx, env.Client, node3, nodeClaim3, node4, nodeClaim4)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{node3, node4}, []*v1.NodeClaim{nodeClaim3, nodeClaim4})

			// Reconcile both nodepools
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool2)

			// Verify node counts
			nodePool = ExpectExists(ctx, env.Client, nodePool)
			nodePool2 = ExpectExists(ctx, env.Client, nodePool2)

			Expect(*nodePool.Status.Nodes).To(Equal(int64(1)))  // First nodepool has 1 node
			Expect(*nodePool2.Status.Nodes).To(Equal(int64(2))) // Second nodepool has 2 nodes
		})

		It("should handle static nodepools with replicas correctly", func() {
			// Create a static nodepool with 3 desired replicas
			staticNodePool := test.StaticNodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Replicas: lo.ToPtr(int64(3)),
				},
			})
			ExpectApplied(ctx, env.Client, staticNodePool)
			ExpectObjectReconciled(ctx, env.Client, nodePoolInformerController, staticNodePool)

			// Create 2 nodes for the static nodepool (less than desired replicas)
			instanceType := cloudProvider.InstanceTypes[0]
			staticNodeClaim1, staticNode1 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					v1.NodePoolLabelKey:            staticNodePool.Name,
					corev1.LabelInstanceTypeStable: instanceType.Name,
					v1.NodeInitializedLabelKey:     "true",
				}},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("300m"),
					},
				},
			})
			staticNodeClaim2, staticNode2 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					v1.NodePoolLabelKey:            staticNodePool.Name,
					corev1.LabelInstanceTypeStable: instanceType.Name,
					v1.NodeInitializedLabelKey:     "true",
				}},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("300m"),
					},
				},
			})

			ExpectApplied(ctx, env.Client, staticNode1, staticNodeClaim1, staticNode2, staticNodeClaim2)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimController, []*corev1.Node{staticNode1, staticNode2}, []*v1.NodeClaim{staticNodeClaim1, staticNodeClaim2})

			// Reconcile the static nodepool
			ExpectObjectReconciled(ctx, env.Client, nodePoolController, staticNodePool)
			staticNodePool = ExpectExists(ctx, env.Client, staticNodePool)

			// Verify that Status.Nodes reflects actual node count (2), not desired replicas (3)
			Expect(staticNodePool.Status.Nodes).ToNot(BeNil())
			Expect(*staticNodePool.Status.Nodes).To(Equal(int64(2)))

			// Verify that the static nodepool has replicas set
			Expect(staticNodePool.Spec.Replicas).ToNot(BeNil())
			Expect(*staticNodePool.Spec.Replicas).To(Equal(int64(3)))
		})
	})
})
