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

package static_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/controllers/state/informer"
	static "sigs.k8s.io/karpenter/pkg/controllers/static/provisioning"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

type failingClient struct {
	client.Client
}

func (f *failingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*v1.NodeClaim); ok {
		return fmt.Errorf("simulated error creating nodeclaims")
	}
	return f.Client.Create(ctx, obj, opts...)
}

var (
	ctx                      context.Context
	fakeClock                *clock.FakeClock
	cluster                  *state.Cluster
	nodeController           *informer.NodeController
	daemonsetController      *informer.DaemonSetController
	cloudProvider            *fake.CloudProvider
	controller               *static.Controller
	env                      *test.Environment
	nodeClaimStateController *informer.NodeClaimController
	prov                     *provisioning.Provisioner
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Provisioning/Static")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	ctx = options.ToContext(ctx, test.Options())
	cloudProvider = fake.NewCloudProvider()
	prov = provisioning.NewProvisioner(env.Client, events.NewRecorder(&record.FakeRecorder{}), cloudProvider, cluster, fakeClock)
	fakeClock = clock.NewFakeClock(time.Now())
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	nodeController = informer.NewNodeController(env.Client, cluster)
	daemonsetController = informer.NewDaemonSetController(env.Client, cluster)
	controller = static.NewController(env.Client, cluster, events.NewRecorder(&record.FakeRecorder{}), cloudProvider, prov, fakeClock)
	nodeClaimStateController = informer.NewNodeClaimController(env.Client, cloudProvider, cluster)
})

var _ = BeforeEach(func() {
	ctx = options.ToContext(ctx, test.Options())
	cloudProvider.Reset()
	cluster.Reset()

	// ensure any waiters on our clock are allowed to proceed before resetting our clock time
	for fakeClock.HasWaiters() {
		fakeClock.Step(1 * time.Minute)
	}
	fakeClock.SetTime(time.Now())
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	cloudProvider.Reset()
	cluster.Reset()
})

var _ = Describe("Static Provisioning Controller", func() {
	Context("Reconcile", func() {
		It("should handle CreateNodeClaims errors gracefully", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(1))
			ExpectApplied(ctx, env.Client, nodePool)

			// Create controller with failing client
			failingController := static.NewController(&failingClient{Client: env.Client}, cluster, events.NewRecorder(&record.FakeRecorder{}), cloudProvider, prov, fakeClock)

			result, err := failingController.Reconcile(ctx, nodePool)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("creating nodeclaims"))
			Expect(result.RequeueAfter).To(BeZero())

			// Should not create any NodeClaims due to error
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(0))
		})
		It("should return early if nodepool is not managed by cloud provider", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(1))
			nodePool.Spec.Template.Spec.NodeClassRef = &v1.NodeClassReference{
				Group: "test.group",
				Kind:  "UnmanagedNodeClass",
				Name:  "test",
			}
			ExpectApplied(ctx, env.Client, nodePool)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeZero())

			// Should not create any NodeClaims
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(0))
		})
		It("should return early if nodepool root condition is not true", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(1))
			nodePool.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "ValidationFailed", "Validation failed")
			ExpectApplied(ctx, env.Client, nodePool)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeZero())

			// Should not create any NodeClaims
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(0))
		})
		It("should return early if nodepool replicas is nil", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = nil
			ExpectApplied(ctx, env.Client, nodePool)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeZero())

			// Should not create any NodeClaims
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(0))
		})
		It("should return early if current node count exceeds desired replicas", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(1))
			// Create 2 nodes and nodeclaims that belong to this nodepool (exceeds desired replicas of 1)
			nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:        nodePool.Name,
						v1.NodeInitializedLabelKey: "true",
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})
			nodeClaim2, node2 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:        nodePool.Name,
						v1.NodeInitializedLabelKey: "true",
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim1, nodeClaim2, node1, node2)

			// Update cluster state to track the nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, []*corev1.Node{node1, node2}, []*v1.NodeClaim{nodeClaim1, nodeClaim2})
			Expect(cluster.Nodes()).To(HaveLen(2))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 2, 0, 0)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should not create any additional NodeClaims
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(2))
		})
		It("should create nodeclaims when current node count is less than desired replicas", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(2))
			ExpectApplied(ctx, env.Client, nodePool)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should create 2 NodeClaims
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(2))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 2, 0, 0)

			// Verify NodeClaims have correct nodepool reference
			for _, nc := range nodeClaims.Items {
				Expect(nc.Spec.NodeClassRef).To(Equal(nodePool.Spec.Template.Spec.NodeClassRef))
				Expect(nc.Labels).To(HaveKeyWithValue(v1.NodePoolLabelKey, nodePool.Name))
			}
		})
		It("should create additional nodeclaims to reach desired replicas", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(3))
			// Create 2 nodes and nodeclaims that belong to this nodepool (exceeds desired replicas of 1)
			nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:        nodePool.Name,
						v1.NodeInitializedLabelKey: "true",
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim1, node1)

			// 	// Update cluster state to track the nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})
			Expect(cluster.Nodes()).To(HaveLen(1))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 1, 0, 0)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 3, 0, 0)

			// Should create 2 additional NodeClaims (3 desired - 1 existing = 2 new)
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(3))
		})
		It("should not create additional nodeclaims", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(3))

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
						},
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
				},
			}}
			nodeClaim1 := test.NodeClaim(nodeClaimOpts...) // has no node
			nodeClaim2 := test.NodeClaim(nodeClaimOpts...) // has node unregistered
			nodeClaim3 := test.NodeClaim(nodeClaimOpts...) // has just providerId
			node2 := test.Node(test.NodeOptions{
				ProviderID: nodeClaim2.Status.ProviderID,
				Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
			})
			node3 := test.Node(test.NodeOptions{
				ProviderID: nodeClaim3.Status.ProviderID,
			})

			ExpectApplied(ctx, env.Client, nodeClaim1)
			ExpectApplied(ctx, env.Client, nodeClaim2, node2)
			ExpectApplied(ctx, env.Client, nodeClaim3, node3)

			// 	// Update cluster state to track the nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, []*corev1.Node{node3, node2}, []*v1.NodeClaim{nodeClaim1, nodeClaim2, nodeClaim3})

			// Reconcile multiple times
			for i := 0; i < 10; i++ {
				_ = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			}

			// Should have just 3 NodeClaims
			existingNodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, existingNodeClaims)).To(Succeed())
			Expect(existingNodeClaims.Items).To(HaveLen(3))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 3, 0, 0)
		})
		It("should not create additional nodeclaims when node limits are reached", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(3))
			nodePool.Spec.Limits = v1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse("1"),
			}
			// Create 2 nodes and nodeclaims that belong to this nodepool (exceeds desired replicas of 1)
			nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:        nodePool.Name,
						v1.NodeInitializedLabelKey: "true",
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim1, node1)

			// 	// Update cluster state to track the nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})
			Expect(cluster.Nodes()).To(HaveLen(1))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 1, 0, 0)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeEquivalentTo(time.Second * 30))

			// Should not create additional NodeClaims (3 desired - 1 existing = 2 new but limit is 1)
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(1))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 1, 0, 0)
		})
		It("should reserve nodepool nodecount during provisioning and release after", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(3))
			nodePool.Spec.Limits = v1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse("10"),
			}

			ExpectApplied(ctx, env.Client, nodePool)
			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should create 3 NodeClaims
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(3))

			// Should be tracking running nodeclaims in nodepool state node
			ExpectStateNodePoolCount(cluster, nodePool.Name, 3, 0, 0)

			// Should be able to reserve remaining 7 NodeCounts
			Expect(cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 10, 10)).To(BeEquivalentTo(7))
			// Release the 7 NodeCount we held
			cluster.NodePoolState.ReleaseNodeCount(nodePool.Name, 7)

			// Size up the replicas to 15 with limit 10
			nodePool.Spec.Replicas = lo.ToPtr(int64(15))
			ExpectApplied(ctx, env.Client, nodePool)

			// Update the state with Created NodeClaims
			for _, nodeClaim := range nodeClaims.Items {
				ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, &nodeClaim)
			}

			result = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should have 10 NodeClaims and not go over limits
			nodeClaims = &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(10))

			// Should not be able to Reserve more
			cluster.NodePoolState.ReleaseNodeCount(nodePool.Name, 10)
			Expect(cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 10, 100)).To(BeEquivalentTo(0))

			ExpectStateNodePoolCount(cluster, nodePool.Name, 10, 0, 0)
		})
		It("should handle zero replicas", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(0))
			ExpectApplied(ctx, env.Client, nodePool)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should not create any NodeClaims
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(0))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 0, 0, 0)

		})
		It("should respect nodepool template specifications", func() {
			npSpecRequirements := []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "karpenter.k8s.aws/instance-category", Operator: corev1.NodeSelectorOpIn, Values: []string{"c", "r"}}, MinValues: lo.ToPtr(int(2))},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "karpenter.k8s.aws/instance-family", Operator: corev1.NodeSelectorOpIn, Values: []string{"c4", "r4"}}, MinValues: lo.ToPtr(int(2))},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "karpenter.k8s.aws/instance-cpu", Operator: corev1.NodeSelectorOpIn, Values: []string{"32"}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "karpenter.k8s.aws/instance-hypervisor", Operator: corev1.NodeSelectorOpIn, Values: []string{"nitro"}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "karpenter.k8s.aws/instance-generation", Operator: corev1.NodeSelectorOpGt, Values: []string{"2"}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"us-west-2a", "us-west-2b"}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"amd64", "arm64"}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{"on-demand", "reserved", "spot"}}},
			}
			nodePool := test.StaticNodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Replicas: lo.ToPtr(int64(4)),
					Template: v1.NodeClaimTemplate{
						ObjectMeta: v1.ObjectMeta{
							Labels: map[string]string{
								"custom-label": "custom-value",
							},
							Annotations: map[string]string{
								"custom-annotation": "custom-value",
							},
						},
					},
				},
			})
			nodePool.Spec.Template.Spec.Requirements = npSpecRequirements
			ExpectApplied(ctx, env.Client, nodePool)

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, []*corev1.Node{}, []*v1.NodeClaim{})

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should create 4 NodeClaim with template specifications
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(4))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 4, 0, 0)

			nc := nodeClaims.Items[0]
			Expect(nc.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
			Expect(nc.Annotations).To(HaveKeyWithValue("custom-annotation", "custom-value"))
			Expect(nc.Spec.Requirements).To(ContainElements(npSpecRequirements))
		})
		It("should handle large replica counts", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(500))
			ExpectApplied(ctx, env.Client, nodePool)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should create 500 NodeClaims
			nodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
			Expect(nodeClaims.Items).To(HaveLen(500))
			ExpectStateNodePoolCount(cluster, nodePool.Name, 500, 0, 0)
		})
		It("handles concurrent reconciliation without exceeding NodePool limits", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Limits = v1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse("10"),
			}
			nodePool.Spec.Replicas = lo.ToPtr(int64(5))
			ExpectApplied(ctx, env.Client, nodePool)

			// Run many reconciles in parallel
			n := 50
			errs := make(chan error, n)
			for i := 0; i < n; i++ {
				go func() {
					defer GinkgoRecover()
					_, e := controller.Reconcile(ctx, nodePool)
					errs <- e
				}()
			}
			for i := 0; i < n; i++ {
				Expect(<-errs).ToNot(HaveOccurred())
			}

			// we should never observe > limit NodeClaims.
			Consistently(func() int {
				ExpectStateNodePoolCount(cluster, nodePool.Name, 10, 0, 0)
				var list v1.NodeClaimList
				_ = env.Client.List(ctx, &list)
				return len(list.Items)
			}, 5*time.Second).Should(BeNumerically("<=", 10))
		})
		It("should wait for cluster to be synced and not over provision", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Limits = v1.Limits{
				corev1.ResourceName("nodes"): resource.MustParse("10"),
			}
			nodePool.Spec.Replicas = lo.ToPtr(int64(5))
			ExpectApplied(ctx, env.Client, nodePool)

			// Run many reconciles in parallel
			n := 50
			errs := make(chan error, n)
			for i := 0; i < n; i++ {
				go func(i int) {
					defer GinkgoRecover()
					if i%4 == 0 {
						cluster.Reset()
					}
					_, e := controller.Reconcile(ctx, nodePool)
					errs <- e
				}(i)
			}
			for i := 0; i < n; i++ {
				Expect(<-errs).ToNot(HaveOccurred())
			}

			// we should never observe > limit NodeClaims.
			Consistently(func() int {
				var list v1.NodeClaimList
				_ = env.Client.List(ctx, &list)
				return len(list.Items)
			}, 5*time.Second).Should(BeNumerically("<=", 10))

			// at the end we should have right counts in StateNodePool
			ExpectStateNodePoolCount(cluster, nodePool.Name, 10, 0, 0)
		})

	})
	Context("Helper Functions", func() {
		DescribeTable("should detect replica or status changes",
			func(oldReplicas, newReplicas *int64, oldReady, newReady bool, expected bool) {
				old := &v1.NodePool{Spec: v1.NodePoolSpec{Replicas: oldReplicas}}
				new := &v1.NodePool{Spec: v1.NodePoolSpec{Replicas: newReplicas}}

				if oldReady {
					old.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
					old.StatusConditions().SetTrue(v1.ConditionTypeNodeClassReady)
				} else {
					old.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "reason", "old not ready")
				}

				if newReady {
					new.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
					new.StatusConditions().SetTrue(v1.ConditionTypeNodeClassReady)
				} else {
					new.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "reason", "new not ready")
				}

				Expect(static.HasNodePoolReplicaOrStatusChanged(old, new)).To(Equal(expected))
			},
			Entry("replica changed", lo.ToPtr(int64(5)), lo.ToPtr(int64(3)), false, false, true),
			Entry("replica same, false → true", lo.ToPtr(int64(5)), lo.ToPtr(int64(5)), false, true, true),
			Entry("replica same, true → false", lo.ToPtr(int64(5)), lo.ToPtr(int64(5)), true, false, false),
			Entry("replica same, both true", lo.ToPtr(int64(5)), lo.ToPtr(int64(5)), true, true, false),
			Entry("replica same, both false", lo.ToPtr(int64(5)), lo.ToPtr(int64(5)), false, false, false),
		)
	})
})
