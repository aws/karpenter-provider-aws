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

package disruption_test

import (
	"sync"
	"time"

	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Drift", func() {
	var nodePool *v1.NodePool
	var nodeClaim *v1.NodeClaim
	var node *corev1.Node

	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidateAfter: v1.MustParseNillableDuration("1h"),
					// Disrupt away!
					Budgets: []v1.Budget{{
						Nodes: "100%",
					}},
				},
			},
		})
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
	})
	Context("Metrics", func() {
		It("should correctly report eligible drifted nodes", func() {
			eligibleNodesLabels := map[string]string{
				metrics.ReasonLabel: "drifted",
			}
			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1.DoNotDisruptAnnotationKey: "true",
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			ExpectMetricGaugeValue(disruption.EligibleNodes, 0, eligibleNodesLabels)

			// remove the do-not-disrupt annotation to make the node eligible for drift and update cluster state
			pod.SetAnnotations(map[string]string{})
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
			ExpectApplied(ctx, env.Client, pod, nodeClaim)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			ExpectMetricGaugeValue(disruption.EligibleNodes, 1, eligibleNodesLabels)
		})
	})
	Context("Budgets", func() {
		var numNodes = 10
		var nodeClaims []*v1.NodeClaim
		var nodes []*corev1.Node
		var rs *appsv1.ReplicaSet
		labels := map[string]string{
			"app": "test",
		}
		BeforeEach(func() {
			// create our RS so we can link a pod to it
			rs = test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())
		})
		It("should only consider 3 nodes allowed to be disrupted because of budgets", func() {
			nodeClaims, nodes = test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})

			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "30%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeDrifted)
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			ExpectSingletonReconciled(ctx, disruptionController)

			ExpectMetricGaugeValue(disruption.NodePoolAllowedDisruptions, 3, map[string]string{
				metrics.NodePoolLabel: nodePool.Name,
				metrics.ReasonLabel:   string(v1.DisruptionReasonDrifted),
			})
		})
		It("should disrupt 3 nodes, taking into account commands in progress", func() {
			nodeClaims, nodes = test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "30%"}}

			ExpectApplied(ctx, env.Client, nodePool)

			// Mark the first five as drifted
			for i := range lo.Range(5) {
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeDrifted)
			}

			for i := 0; i < numNodes; i++ {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// 3 pods to fit on 3 nodes that will be disrupted so that they're not empty
			// and have to be in 3 different commands
			pods := test.Pods(5, test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
				ObjectMeta: metav1.ObjectMeta{Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "ReplicaSet",
							Name:               rs.Name,
							UID:                rs.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					}}})
			// Bind the pods to the first n nodes.
			for i := 0; i < len(pods); i++ {
				ExpectApplied(ctx, env.Client, pods[i])
				ExpectManualBinding(ctx, env.Client, pods[i], nodes[i])
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)

			// Reconcile 5 times, enqueuing 3 commands total.
			for i := 0; i < 5; i++ {
				ExpectSingletonReconciled(ctx, disruptionController)
			}

			nodes = ExpectNodes(ctx, env.Client)
			Expect(len(lo.Filter(nodes, func(nc *corev1.Node, _ int) bool {
				return lo.Contains(nc.Spec.Taints, v1.DisruptedNoScheduleTaint)
			}))).To(Equal(3))
			// Execute all commands in the queue, only deleting 3 nodes
			cmds := queue.GetCommands()
			for _, cmd := range cmds {
				ExpectObjectReconciled(ctx, env.Client, queue, cmd.Candidates[0].NodeClaim)
			}
			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(7))
		})
		It("should respect budgets for multiple nodepools", func() {
			// Create 10 nodepools
			nps := test.NodePools(10, v1.NodePool{
				Spec: v1.NodePoolSpec{
					Disruption: v1.Disruption{
						ConsolidateAfter: v1.MustParseNillableDuration("Never"),
						Budgets: []v1.Budget{{
							Nodes: "1",
						}},
					},
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							ExpireAfter: v1.MustParseNillableDuration("Never"),
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < len(nps); i++ {
				ExpectApplied(ctx, env.Client, nps[i])
			}
			nodeClaims = make([]*v1.NodeClaim, 0, 30)
			nodes = make([]*corev1.Node, 0, 30)
			// Create 3 nodes for each nodePool, 2 of which have a pod
			for _, np := range nps {
				ncs, ns := test.NodeClaimsAndNodes(3, v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            np.Name,
							corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
							v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
							corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
						},
					},
					Status: v1.NodeClaimStatus{
						Allocatable: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:  resource.MustParse("32"),
							corev1.ResourcePods: resource.MustParse("100"),
						},
					},
				})
				for i := range ncs {
					ncs[i].StatusConditions().SetTrue(v1.ConditionTypeDrifted)
					ExpectApplied(ctx, env.Client, ncs[i], ns[i])
				}
				nodeClaims = append(nodeClaims, ncs...)
				nodes = append(nodes, ns...)
				pods := test.Pods(2, test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					ObjectMeta: metav1.ObjectMeta{Labels: labels,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "apps/v1",
								Kind:               "ReplicaSet",
								Name:               rs.Name,
								UID:                rs.UID,
								Controller:         lo.ToPtr(true),
								BlockOwnerDeletion: lo.ToPtr(true),
							},
						}}})
				// Bind the pods to the first 2 nodes for each nodepool.
				for i := range pods {
					ExpectApplied(ctx, env.Client, pods[i])
					ExpectManualBinding(ctx, env.Client, pods[i], ns[i])
				}
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			// Reconcile 30 times, enqueuing 10 commands total
			for range 30 {
				ExpectSingletonReconciled(ctx, disruptionController)
			}

			for _, np := range nps {
				ExpectMetricGaugeValue(disruption.NodePoolAllowedDisruptions, 1, map[string]string{
					metrics.NodePoolLabel: np.Name,
					metrics.ReasonLabel:   string(v1.DisruptionReasonDrifted),
				})
			}

			// Delete 10 nodes, 1 node per nodepool, according to their budgets
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(10))
			for _, cmd := range cmds {
				ExpectObjectReconciled(ctx, env.Client, queue, cmd.Candidates[0].NodeClaim)
			}
			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(20))
			// These nodes will disrupt because of Drift instead of Emptiness because they are not marked consolidatable
			ExpectMetricCounterValue(disruption.DecisionsPerformedTotal, 10, map[string]string{
				metrics.ReasonLabel: "drifted",
			})
		})
	})
	Context("Drift", func() {
		BeforeEach(func() {
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
			Expect(nodeClaim.StatusConditions().Clear(v1.ConditionTypeConsolidatable)).To(BeNil())
		})
		It("should continue to the next drifted node if the first cannot reschedule all pods", func() {
			pod := test.Pod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("150"),
					},
				},
			})
			podToExpire := test.Pod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			nodeClaim2, node2 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("1"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			nodeClaim2.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
			ExpectApplied(ctx, env.Client, nodeClaim2, node2, podToExpire)
			ExpectManualBinding(ctx, env.Client, podToExpire, node2)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node2}, []*v1.NodeClaim{nodeClaim2})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim, nodeClaim2)

			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
			ExpectExists(ctx, env.Client, nodeClaim)
			ExpectNotFound(ctx, env.Client, nodeClaim2)
		})
		It("should ignore nodes without the drifted status condition", func() {
			_ = nodeClaim.StatusConditions().Clear(v1.ConditionTypeDrifted)
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Expect to not create or delete more nodeclaims
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
		})
		It("should ignore nodes with the drifted status condition set to false", func() {
			nodeClaim.StatusConditions().SetFalse(v1.ConditionTypeDrifted, "NotDrifted", "NotDrifted")
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Expect to not create or delete more nodeclaims
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
		})
		It("should ignore nodes with the karpenter.sh/do-not-disrupt annotation", func() {
			node.Annotations = lo.Assign(node.Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "true"})
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectSingletonReconciled(ctx, disruptionController)

			// Expect to not create or delete more nodeclaims
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
		})
		It("should delete drifted nodes with the karpenter.sh/do-not-disrupt annotation set to false", func() {
			node.Annotations = lo.Assign(node.Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "false"})
			labels := map[string]string{
				"app": "test",
			}
			// create replicaset
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "ReplicaSet",
							Name:               rs.Name,
							UID:                rs.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					}}})
			nodeClaim2, node2 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})

			ExpectApplied(ctx, env.Client, rs, pod, nodeClaim, nodeClaim2, node, node2, nodePool)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})

			// Process candidates
			ExpectSingletonReconciled(ctx, disruptionController)
			// Process the eligible candidate so that the node can be deleted.
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)
			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

			// We should delete the nodeClaim that has drifted
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectNotFound(ctx, env.Client, nodeClaim, node)
		})
		It("should not create replacements for drifted nodes that have pods with the karpenter.sh/do-not-disrupt annotation", func() {
			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1.DoNotDisruptAnnotationKey: "true",
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectSingletonReconciled(ctx, disruptionController)

			// Expect to not create or delete more nodeclaims
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
		})
		It("should not create replacements for drifted nodes that have pods with the karpenter.sh/do-not-disrupt annotation when the NodePool's TerminationGracePeriod is not nil", func() {
			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1.DoNotDisruptAnnotationKey: "true",
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectSingletonReconciled(ctx, disruptionController)

			// Pods with `karpenter.sh/do-not-disrupt` can't be evicted and hence can't be rescheduled on a new node.
			// Expect no new nodeclaims to be created.
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
		})
		It("should not create replacements for drifted nodes that have pods with the blocking PDBs when the NodePool's TerminationGracePeriod is not nil", func() {
			nodeClaim.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
			podLabels := map[string]string{"test": "value"}
			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
			})
			budget := test.PodDisruptionBudget(test.PDBOptions{
				Labels:         podLabels,
				MaxUnavailable: fromInt(0),
			})
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool, pod, budget)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectSingletonReconciled(ctx, disruptionController)

			// Pods with blocking PDBs can't be evicted and hence can't be rescheduled on a new node.
			// Expect no new nodeclaims to be created.
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
		})
		It("should replace drifted nodes", func() {
			labels := map[string]string{
				"app": "test",
			}
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "ReplicaSet",
							Name:               rs.Name,
							UID:                rs.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					}}})

			ExpectApplied(ctx, env.Client, rs, pod, nodeClaim, node, nodePool)

			// bind the pods to the node
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)
			ExpectNotFound(ctx, env.Client, nodeClaim, node)

			// Expect that the new nodeClaim was created and its different than the original
			nodeclaims := ExpectNodeClaims(ctx, env.Client)
			nodes := ExpectNodes(ctx, env.Client)
			Expect(nodeclaims).To(HaveLen(1))
			Expect(nodes).To(HaveLen(1))
			Expect(nodeclaims[0].Name).ToNot(Equal(nodeClaim.Name))
			Expect(nodes[0].Name).ToNot(Equal(node.Name))
		})
		It("should delete drifted nodes", func() {
			labels := map[string]string{
				"app": "test",
			}
			// create replicaset
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "ReplicaSet",
							Name:               rs.Name,
							UID:                rs.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					}}})
			nodeClaim2, node2 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})

			ExpectApplied(ctx, env.Client, rs, pod, nodeClaim, nodeClaim2, node, node2, nodePool)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})

			// Process candidates
			ExpectSingletonReconciled(ctx, disruptionController)
			// Process the eligible candidate so that the node can be deleted.
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)
			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

			// We should delete the nodeClaim that has drifted
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectNotFound(ctx, env.Client, nodeClaim, node)
		})
		It("should delete drifted nodes when they are empty and consolidation is disabled", func() {
			nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("Never")
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

			// we should delete the empty node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(0))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(0))
			ExpectNotFound(ctx, env.Client, nodeClaim, node)
			ExpectMetricGaugeValue(disruption.EligibleNodes, 1, map[string]string{
				metrics.ReasonLabel: "drifted",
			})
		})
		It("should drift empty nodes before non-empty nodes", func() {
			nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("Never")
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrifted)

			labels := map[string]string{
				"app": "test",
			}
			// create replicaset
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "ReplicaSet",
							Name:               rs.Name,
							UID:                rs.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					}}})
			nodeClaim2, node2 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			nodeClaim2.StatusConditions().SetTrue(v1.ConditionTypeDrifted)

			ExpectApplied(ctx, env.Client, rs, pod, nodePool, nodeClaim, nodeClaim2, node, node2)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})
			ExpectSingletonReconciled(ctx, disruptionController)
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim2)

			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim2)

			// we should delete the empty node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
			ExpectExists(ctx, env.Client, node)
			ExpectNotFound(ctx, env.Client, nodeClaim2, node2)
			ExpectMetricGaugeValue(disruption.EligibleNodes, 2, map[string]string{
				metrics.ReasonLabel: "drifted",
			})
			ExpectMetricCounterValue(disruption.DecisionsPerformedTotal, 1, map[string]string{
				metrics.ReasonLabel: "drifted",
			})
		})
		It("should give emptiness priority to delete drifted nodes when they are empty and consolidatable", func() {
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

			// we should delete the empty node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(0))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(0))
			ExpectNotFound(ctx, env.Client, nodeClaim, node)
			ExpectMetricGaugeValue(disruption.EligibleNodes, 1, map[string]string{
				metrics.ReasonLabel: "empty",
			})
		})
		It("should untaint nodes when drift replacement fails", func() {
			labels := map[string]string{
				"app": "test",
			}
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "ReplicaSet",
							Name:               rs.Name,
							UID:                rs.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, rs, nodeClaim, node, nodePool, pod)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			var wg sync.WaitGroup
			ExpectSingletonReconciled(ctx, disruptionController)
			wg.Wait()

			// Simulate the new NodeClaim being created and then deleted
			newNodeClaim, ok := lo.Find(ExpectNodeClaims(ctx, env.Client), func(nc *v1.NodeClaim) bool {
				return nc.Name != nodeClaim.Name
			})
			Expect(ok).To(BeTrue())
			ExpectDeleted(ctx, env.Client, newNodeClaim)
			cluster.DeleteNodeClaim(newNodeClaim.Name)

			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)
			// We should have tried to create a new nodeClaim but failed to do so; therefore, we untainted the existing node
			node = ExpectExists(ctx, env.Client, node)
			Expect(node.Spec.Taints).ToNot(ContainElement(v1.DisruptedNoScheduleTaint))
		})
		It("can replace drifted nodes with multiple nodes", func() {
			currentInstance := fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "current-on-demand",
				Offerings: []*cloudprovider.Offering{
					{
						Available:    false,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        0.5,
					},
				},
			})
			replacementInstance := fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "replacement-on-demand",
				Offerings: []*cloudprovider.Offering{
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        0.3,
					},
				},
				Resources: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("3")},
			})
			cloudProvider.InstanceTypes = []*cloudprovider.InstanceType{
				currentInstance,
				replacementInstance,
			}

			labels := map[string]string{
				"app": "test",
			}
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

			pods := test.Pods(3, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "ReplicaSet",
							Name:               rs.Name,
							UID:                rs.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					}},
				// Make each pod request about a third of the allocatable on the node
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("2")},
				},
			})

			nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{
				corev1.LabelInstanceTypeStable: currentInstance.Name,
				v1.CapacityTypeLabelKey:        currentInstance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       currentInstance.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
			nodeClaim.Status.Allocatable = map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("8")}
			node.Labels = lo.Assign(node.Labels, map[string]string{
				corev1.LabelInstanceTypeStable: currentInstance.Name,
				v1.CapacityTypeLabelKey:        currentInstance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       currentInstance.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
			node.Status.Allocatable = map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("8")}

			ExpectApplied(ctx, env.Client, rs, nodeClaim, node, nodePool, pods[0], pods[1], pods[2])

			// bind the pods to the node
			ExpectManualBinding(ctx, env.Client, pods[0], node)
			ExpectManualBinding(ctx, env.Client, pods[1], node)
			ExpectManualBinding(ctx, env.Client, pods[2], node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)
			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

			// expect that drift provisioned three nodes, one for each pod
			ExpectNotFound(ctx, env.Client, nodeClaim, node)
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(3))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(3))
		})
		It("should drift one non-empty node at a time, starting with the earliest drift", func() {
			labels := map[string]string{
				"app": "test",
			}

			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)

			pods := test.Pods(2, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "ReplicaSet",
							Name:               rs.Name,
							UID:                rs.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					},
				},
				// Make each pod request only fit on a single node
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("30")},
				},
			})

			nodeClaim2, node2 := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID:  test.RandomProviderID(),
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
				},
			})
			nodeClaim2.Status.Conditions = append(nodeClaim2.Status.Conditions, status.Condition{
				Type:               v1.ConditionTypeDrifted,
				Status:             metav1.ConditionTrue,
				Reason:             v1.ConditionTypeDrifted,
				Message:            v1.ConditionTypeDrifted,
				LastTransitionTime: metav1.Time{Time: time.Now().Add(-time.Hour)},
			})

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], nodeClaim, node, nodeClaim2, node2, nodePool)

			// bind pods to node so that they're not empty and don't disrupt in parallel.
			ExpectManualBinding(ctx, env.Client, pods[0], node)
			ExpectManualBinding(ctx, env.Client, pods[1], node2)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)
			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim, nodeClaim2)

			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
			ExpectNotFound(ctx, env.Client, nodeClaim2, node2)
			ExpectExists(ctx, env.Client, nodeClaim)
			ExpectExists(ctx, env.Client, node)
		})
	})

	Context("Static NodePool", func() {
		It("should not consider static nodepool for drift", func() {
			staticNp := test.StaticNodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Replicas:   lo.ToPtr(int64(2)), // Static nodepool with 2 desired replica
					Disruption: v1.Disruption{},
				},
			})

			nodeClaims, nodes := test.NodeClaimsAndNodes(2, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:        staticNp.Name,
						v1.NodeInitializedLabelKey: "true",
					},
				},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
						corev1.ResourcePods:   resource.MustParse("2"),
					},
				},
			})
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
			}
			drift := disruption.NewDrift(env.Client, cluster, prov, recorder)

			ExpectApplied(ctx, env.Client, staticNp, nodeClaims[0], nodeClaims[1], nodes[0], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, drift.ShouldDisrupt, drift.Class(), queue)
			Expect(err).To(Succeed())
			Expect(candidates).To(HaveLen(0))
		})
	})
})
