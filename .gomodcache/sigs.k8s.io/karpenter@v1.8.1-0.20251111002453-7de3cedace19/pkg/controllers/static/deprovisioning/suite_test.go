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
	clock "k8s.io/utils/clock/testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/controllers/state/informer"
	static "sigs.k8s.io/karpenter/pkg/controllers/static/deprovisioning"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

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
)

type failingClient struct {
	client.Client
}

func (f *failingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*v1.NodeClaim); ok {
		return fmt.Errorf("simulated error deleting nodeclaims")
	}
	return f.Client.Delete(ctx, obj, opts...)
}

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Deprovisioning/Static")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	ctx = options.ToContext(ctx, test.Options())
	cloudProvider = fake.NewCloudProvider()
	fakeClock = clock.NewFakeClock(time.Now())
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	nodeController = informer.NewNodeController(env.Client, cluster)
	daemonsetController = informer.NewDaemonSetController(env.Client, cluster)
	controller = static.NewController(env.Client, cluster, cloudProvider, fakeClock)
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

var _ = Describe("Static Deprovisioning Controller", func() {
	Context("Reconcile", func() {
		It("should return early if nodepool is not managed by cloud provider", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(1))
			nodePool.Spec.Template.Spec.NodeClassRef = &v1.NodeClassReference{
				Group: "test.group",
				Kind:  "UnmanagedNodeClass",
				Name:  "test",
			}
			nodeClaims, nodes := test.NodeClaimsAndNodes(1, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						v1.NodeInitializedLabelKey:     "true",
						corev1.LabelInstanceTypeStable: "stable.instance",
					},
				},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})

			ExpectApplied(ctx, env.Client, nodePool, nodeClaims[0], nodes[0])

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeZero())

			// Should not delete any NodeClaims
			existingNodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, existingNodeClaims)).To(Succeed())
			Expect(existingNodeClaims.Items).To(HaveLen(1))
		})
		It("should return early if nodepool replicas is nil", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = nil
			ExpectApplied(ctx, env.Client, nodePool)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)

			Expect(result.RequeueAfter).To(BeZero())
		})
		It("should return early if current node count is less than or equal to desired replicas", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(3))

			// Create 2 nodes (less than desired 3)
			nodeClaims, nodes := test.NodeClaimsAndNodes(2, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						v1.NodeInitializedLabelKey:     "true",
						corev1.LabelInstanceTypeStable: "stable.instance",
					},
				},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})

			ExpectApplied(ctx, env.Client, nodePool, nodeClaims[0], nodeClaims[1], nodes[0], nodes[1])

			// Update cluster state to track the nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			Expect(cluster.Nodes()).To(HaveLen(2))
			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 2, 0, 0)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)

			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should not delete any NodeClaims
			existingNodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, existingNodeClaims)).To(Succeed())
			Expect(existingNodeClaims.Items).To(HaveLen(2))
		})
		It("should only consider running nodeclaims and not deleting nodeclaims", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(1))

			nodeClaims, nodes := test.NodeClaimsAndNodes(4, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						v1.NodeInitializedLabelKey:     "true",
						corev1.LabelInstanceTypeStable: "stable.instance",
					},
				},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < 4; i++ {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}

			// Update cluster state to track the nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, nodes, nodeClaims)
			Expect(cluster.Nodes()).To(HaveLen(4))

			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 4, 0, 0)

			// If 3 of the nodes are Marked for deletion then do not deprovision any
			for i := 0; i < 3; i++ {
				cluster.MarkForDeletion(nodes[i].Spec.ProviderID)
			}

			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 1, 3, 0)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should terminate 0 NodeClaims as 3 NodeClaims are deleting
			remainingNodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, remainingNodeClaims)).To(Succeed())
			Expect(remainingNodeClaims.Items).To(HaveLen(4))
			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 1, 3, 0)
		})
		It("should terminate excess nodeclaims when current count exceeds desired replicas", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(2))

			nodeClaims, nodes := test.NodeClaimsAndNodes(4, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						v1.NodeInitializedLabelKey:     "true",
						corev1.LabelInstanceTypeStable: "stable.instance",
					},
				},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < 4; i++ {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// Update cluster state to track the nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, nodes, nodeClaims)
			Expect(cluster.Nodes()).To(HaveLen(4))
			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 4, 0, 0)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should terminate 2 NodeClaims (4 current - 2 desired = 2 to terminate)
			remainingNodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, remainingNodeClaims)).To(Succeed())
			Expect(remainingNodeClaims.Items).To(HaveLen(2))
			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 2, 2, 0)
		})
		It("should handle zero replicas by terminating all nodeclaims", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(0))

			nodeClaims, nodes := test.NodeClaimsAndNodes(3, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						v1.NodeInitializedLabelKey:     "true",
						corev1.LabelInstanceTypeStable: "stable.instance",
					},
				},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
			})

			ExpectApplied(ctx, env.Client, nodePool)
			for i := range 3 {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// Update cluster state to track the nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, nodes, nodeClaims)
			Expect(cluster.Nodes()).To(HaveLen(3))
			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 3, 0, 0)

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			// Should terminate all NodeClaims
			remainingNodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, remainingNodeClaims)).To(Succeed())
			Expect(remainingNodeClaims.Items).To(HaveLen(0))
			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 0, 3, 0)
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaims[0]))
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaims[1]))
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaims[2]))

			// Verify StateNodePool Has been updated
			ExpectStateNodePoolCount(cluster, nodePool.Name, 0, 0, 0)
		})
		It("should handle no active nodeclaims gracefully", func() {
			nodePool := test.StaticNodePool()
			nodePool.Spec.Replicas = lo.ToPtr(int64(0))
			ExpectApplied(ctx, env.Client, nodePool)

			// Update cluster state with no nodes
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, []*corev1.Node{}, []*v1.NodeClaim{})

			result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

			existingNodeClaims := &v1.NodeClaimList{}
			Expect(env.Client.List(ctx, existingNodeClaims)).To(Succeed())
			Expect(existingNodeClaims.Items).To(HaveLen(0))
		})
		Context("Failing Scenarios", func() {
			It("should return error when nodeclaim deletion fails", func() {
				nodePool := test.StaticNodePool()
				nodePool.Spec.Replicas = lo.ToPtr(int64(1))
				ExpectApplied(ctx, env.Client, nodePool)

				failingController := static.NewController(&failingClient{Client: env.Client}, cluster, cloudProvider, fakeClock)

				// Create 3 nodeclaims, so 2 need to be terminated
				nodeClaims, nodes := test.NodeClaimsAndNodes(3, v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            nodePool.Name,
							v1.NodeInitializedLabelKey:     "true",
							corev1.LabelInstanceTypeStable: "stable.instance",
						},
					},
					Status: v1.NodeClaimStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("1000Mi"),
						},
					},
				})

				ExpectApplied(ctx, env.Client, nodePool)
				for i := range 3 {
					ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
				}

				// Update cluster state to track the nodes
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, nodes, nodeClaims)
				Expect(cluster.Nodes()).To(HaveLen(3))

				// Verify StateNodePool Has been updated
				ExpectStateNodePoolCount(cluster, nodePool.Name, 3, 0, 0)

				_, err := failingController.Reconcile(ctx, nodePool)
				Expect(err).To(HaveOccurred())

				// Verify that some nodeclaims still exist (deletion didn't complete as expected)
				remainingNodeClaims := &v1.NodeClaimList{}
				Expect(env.Client.List(ctx, remainingNodeClaims)).To(Succeed())

				// At least one nodeclaim should still be active due to the finalizer
				activeNodeClaims := lo.Filter(remainingNodeClaims.Items, func(nc v1.NodeClaim, _ int) bool {
					return nc.DeletionTimestamp.IsZero()
				})
				Expect(len(activeNodeClaims)).To(BeNumerically(">", 1)) // More than desired replicas (1)
				// Verify StateNodePool Has been updated
				ExpectStateNodePoolCount(cluster, nodePool.Name, 3, 0, 0)
			})
		})
		Context("Deprovision Candidate Selection", func() {
			It("should prioritize empty nodes (with only daemonset pods) for termination", func() {
				nodePool := test.StaticNodePool()
				nodePool.Spec.Replicas = lo.ToPtr(int64(2))

				nodeClaims, nodes := test.NodeClaimsAndNodes(4, v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            nodePool.Name,
							v1.NodeInitializedLabelKey:     "true",
							corev1.LabelInstanceTypeStable: "stable.instance",
						},
					},
					Status: v1.NodeClaimStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("1000Mi"),
						},
					},
				})
				ExpectApplied(ctx, env.Client, nodePool)

				// Nodes 0 and 2: Add only DaemonSet pods (reschedulable)
				for i := range 4 {
					pod := test.Pod(test.PodOptions{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: lo.Ternary(i == 0 || i == 2, "kube-system", "default"),
							OwnerReferences: lo.Ternary(i == 0 || i == 2,
								[]metav1.OwnerReference{{
									APIVersion: "apps/v1",
									Kind:       "DaemonSet",
									Name:       "test-daemonset",
									UID:        "test-uid",
								}},
								nil),
						},
						NodeName: nodes[i].Name,
					})
					ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
					ExpectApplied(ctx, env.Client, pod)
				}

				// Update cluster state to track the nodes
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, nodes, nodeClaims)
				Expect(cluster.Nodes()).To(HaveLen(4))

				// Verify StateNodePool Has been updated
				ExpectStateNodePoolCount(cluster, nodePool.Name, 4, 0, 0)

				result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

				// Should terminate 2 NodeClaims (4 current - 2 desired = 2 to terminate)
				remainingNodeClaims := &v1.NodeClaimList{}
				Expect(env.Client.List(ctx, remainingNodeClaims)).To(Succeed())

				activeNodeClaims := lo.Filter(remainingNodeClaims.Items, func(nc v1.NodeClaim, _ int) bool {
					return nc.DeletionTimestamp.IsZero()
				})
				activeNodeClaimNames := lo.Map(activeNodeClaims, func(nc v1.NodeClaim, _ int) string {
					return nc.Name
				})
				Expect(activeNodeClaimNames).To(HaveLen(2))
				Expect(activeNodeClaimNames).To(ContainElements(nodeClaims[1].Name, nodeClaims[3].Name))
				ExpectStateNodePoolCount(cluster, nodePool.Name, 2, 2, 0)

			})
			It("should terminate non-empty nodes when empty nodes are insufficient", func() {
				nodePool := test.StaticNodePool()
				nodePool.Spec.Replicas = lo.ToPtr(int64(1))

				nodeClaims, nodes := test.NodeClaimsAndNodes(4, v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            nodePool.Name,
							v1.NodeInitializedLabelKey:     "true",
							corev1.LabelInstanceTypeStable: "stable.instance",
						},
					},
					Status: v1.NodeClaimStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("1000Mi"),
						},
					},
				})
				ExpectApplied(ctx, env.Client, nodePool)
				for i := range 4 {
					ExpectApplied(ctx, env.Client, nodes[i], nodeClaims[i])
				}

				pod1 := test.Pod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{},
					NodeName:   nodes[0].Name,
				})
				pod2 := test.Pod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{},
					NodeName:   nodes[2].Name,
				})

				ExpectApplied(ctx, env.Client, pod1, pod2)

				// Update cluster state to track the nodes
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeController, nodeClaimStateController, nodes, nodeClaims)
				Expect(cluster.Nodes()).To(HaveLen(4))
				ExpectStateNodePoolCount(cluster, nodePool.Name, 4, 0, 0)

				result := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))

				// Should terminate 3 NodeClaims (4 current - 1 desired = 3 to terminate)
				remainingNodeClaims := &v1.NodeClaimList{}
				Expect(env.Client.List(ctx, remainingNodeClaims)).To(Succeed())
				Expect(remainingNodeClaims.Items).To(HaveLen(1))
				ExpectStateNodePoolCount(cluster, nodePool.Name, 1, 3, 0)

			})
			Describe("disruption cost ordering", func() {
				var (
					nodePool   *v1.NodePool
					nodes      []*corev1.Node
					nodeClaims []*v1.NodeClaim
					pods       []*corev1.Pod
				)

				remainingNames := func() []string {
					var ncl v1.NodeClaimList
					Expect(env.Client.List(ctx, &ncl, client.MatchingLabels{
						v1.NodePoolLabelKey: nodePool.Name,
					})).To(Succeed())
					out := make([]string, 0, len(ncl.Items))
					for i := range ncl.Items {
						out = append(out, ncl.Items[i].Name)
					}
					return out
				}
				name := func(i int) string { return nodeClaims[i].Name }

				BeforeEach(func() {
					nodePool = test.StaticNodePool()
					nodePool.Spec.Replicas = lo.ToPtr(int64(8))
					ExpectApplied(ctx, env.Client, nodePool)

					nodes = nil
					nodeClaims = nil
					pods = nil

					// create two of each: low, high, dnd, ds (order matters only for indices)
					priority := []string{"low", "high", "dnd", "ds"}
					for i := 0; i < 2; i++ {
						for _, p := range priority {
							nc, n := test.NodeClaimAndNode(v1.NodeClaim{
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

							switch p {
							case "low":
								pods = append(pods, test.Pod(test.PodOptions{
									ObjectMeta: metav1.ObjectMeta{
										Name:      fmt.Sprintf("system-pod-low-%d", i),
										Namespace: "kube-system",
									},
									NodeName: n.Name,
								}))
							case "high":
								pods = append(pods,
									test.Pod(test.PodOptions{
										ObjectMeta: metav1.ObjectMeta{
											Name:      fmt.Sprintf("system-pod-high-%d", i),
											Namespace: "kube-system",
										},
										NodeName: n.Name,
									}),
									test.Pod(test.PodOptions{
										ObjectMeta: metav1.ObjectMeta{
											Name:      fmt.Sprintf("app-pod-high-%d", i),
											Namespace: "default",
										},
										NodeName: n.Name,
									}),
								)
							case "dnd":
								pods = append(pods, test.Pod(test.PodOptions{
									ObjectMeta: metav1.ObjectMeta{
										Name:      fmt.Sprintf("app-pod-dnd-%d", i),
										Namespace: "default",
										Annotations: map[string]string{
											v1.DoNotDisruptAnnotationKey: "true",
										},
									},
									NodeName: n.Name,
								}))
							case "ds":
								pods = append(pods, test.Pod(test.PodOptions{
									ObjectMeta: metav1.ObjectMeta{
										Name:      fmt.Sprintf("dmn-pod-%d", i),
										Namespace: "kube-system",
										OwnerReferences: []metav1.OwnerReference{{
											APIVersion: "apps/v1",
											Kind:       "DaemonSet",
											Name:       "test-daemonset",
											UID:        "test-uid",
										}},
									},
									NodeName: n.Name,
								}))
							}

							nodes = append(nodes, n)
							nodeClaims = append(nodeClaims, nc)
							ExpectApplied(ctx, env.Client, nc, n)
						}
					}
					for _, p := range pods {
						ExpectApplied(ctx, env.Client, p)
					}

					ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(
						ctx, env.Client, nodeController, nodeClaimStateController, nodes, nodeClaims,
					)
					Expect(cluster.Nodes()).To(HaveLen(8))
				})
				DescribeTable("scales down in disruption cost order",
					func(replicas int64, expectIdx []int) {
						nodePool.Spec.Replicas = lo.ToPtr(replicas)
						ExpectApplied(ctx, env.Client, nodePool)
						ExpectStateNodePoolCount(cluster, nodePool.Name, 8, 0, 0)

						res := ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
						Expect(res.RequeueAfter).To(BeNumerically("~", time.Minute*1, time.Second))
						ExpectStateNodePoolCount(cluster, nodePool.Name, int(replicas), int(8-replicas), 0)

						want := make([]string, 0, len(expectIdx))
						for _, i := range expectIdx {
							want = append(want, name(i))
						}
						Eventually(remainingNames).Should(ConsistOf(want))
					},

					Entry("to 6 (drops DS-only first)", int64(6), []int{0, 1, 2, 4, 5, 6}),
					Entry("to 4 (drops low next)", int64(4), []int{1, 2, 5, 6}),
					Entry("to 2 (drops high next)", int64(2), []int{2, 6}),
					Entry("to 0 (drops do-not-disrupt last)", int64(0), []int{}),
				)
			})
		})
		Context("Helper Functions", func() {
			Describe("hasNodePoolReplicaOrStatusChanged", func() {
				It("should detect replica changes", func() {
					old := &v1.NodePool{Spec: v1.NodePoolSpec{Replicas: lo.ToPtr(int64(5))}}
					new := &v1.NodePool{Spec: v1.NodePoolSpec{Replicas: lo.ToPtr(int64(10))}}
					Expect(static.HasNodePoolReplicaCountChanged(old, new)).To(BeTrue())
				})
				It("should return false for identical replicas", func() {
					old := &v1.NodePool{Spec: v1.NodePoolSpec{Replicas: lo.ToPtr(int64(5))}}
					new := old
					Expect(static.HasNodePoolReplicaCountChanged(old, new)).To(BeFalse())
				})
			})
		})
	})
})
