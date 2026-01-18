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

// nolint:gosec
package disruption_test

import (
	"sort"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Emptiness", func() {
	var nodePool *v1.NodePool
	var nodeClaims []*v1.NodeClaim
	var nodeClaim *v1.NodeClaim
	var nodeClaim2 *v1.NodeClaim
	var nodes []*corev1.Node
	var node *corev1.Node
	var node2 *corev1.Node

	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
					ConsolidationPolicy: v1.ConsolidationPolicyWhenEmpty,
					// Disrupt away!
					Budgets: []v1.Budget{{
						Nodes: "100%",
					}},
				},
			},
		})
		nodeClaims, nodes = test.NodeClaimsAndNodes(2, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: leastExpensiveSpotInstance.Name,
					v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		nodeClaim, nodeClaim2 = nodeClaims[0], nodeClaims[1]
		node, node2 = nodes[0], nodes[1]
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		nodeClaim2.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		disruption.EligibleNodes.Reset()
	})
	Context("Metrics", func() {
		It("should correctly report eligible nodes", func() {
			pod := test.Pod()
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			ExpectMetricGaugeValue(disruption.EligibleNodes, 0, map[string]string{
				metrics.ReasonLabel: "empty",
			})

			ExpectDeleted(ctx, env.Client, pod)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			ExpectMetricGaugeValue(disruption.EligibleNodes, 1, map[string]string{
				metrics.ReasonLabel: "empty",
			})
		})
	})
	Context("Budgets", func() {
		var numNodes = 10
		It("should allow all empty nodes to be disrupted", func() {
			nodeClaims, nodes = test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "100%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			ExpectSingletonReconciled(ctx, disruptionController)

			metric, found := FindMetricWithLabelValues("karpenter_nodepools_allowed_disruptions", map[string]string{
				"nodepool": nodePool.Name,
			})
			Expect(found).To(BeTrue())
			Expect(metric.GetGauge().GetValue()).To(BeNumerically("==", 10))

			// Execute command, thus deleting 10 nodes
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(0))
		})
		It("should allow no empty nodes to be disrupted", func() {
			nodeClaims, nodes = test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "0%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			ExpectSingletonReconciled(ctx, disruptionController)

			metric, found := FindMetricWithLabelValues("karpenter_nodepools_allowed_disruptions", map[string]string{
				"nodepool": nodePool.Name,
			})
			Expect(found).To(BeTrue())
			Expect(metric.GetGauge().GetValue()).To(BeNumerically("==", 0))

			// There should be no commands in the queue
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(0))

			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(numNodes))
		})
		It("should only allow 3 empty nodes to be disrupted", func() {
			nodeClaims, nodes = test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
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
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			ExpectSingletonReconciled(ctx, disruptionController)

			metric, found := FindMetricWithLabelValues("karpenter_nodepools_allowed_disruptions", map[string]string{
				"nodepool": nodePool.Name,
			})
			Expect(found).To(BeTrue())
			Expect(metric.GetGauge().GetValue()).To(BeNumerically("==", 3))

			// Execute command, thus deleting 3 nodes
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(7))
		})
		It("should allow 2 nodes from each nodePool to be deleted", func() {
			// Create 10 nodepools
			nps := test.NodePools(10, v1.NodePool{
				Spec: v1.NodePoolSpec{
					Disruption: v1.Disruption{
						ConsolidateAfter:    v1.MustParseNillableDuration("30s"),
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmpty,
						Budgets: []v1.Budget{{
							// 1/2 of 3 nodes == 1.5 nodes. This should round up to 2.
							Nodes: "50%",
						}},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < len(nps); i++ {
				ExpectApplied(ctx, env.Client, nps[i])
			}
			nodeClaims = make([]*v1.NodeClaim, 0, 30)
			nodes = make([]*corev1.Node, 0, 30)
			// Create 3 nodes for each nodePool
			for _, np := range nps {
				ncs, ns := test.NodeClaimsAndNodes(3, v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            np.Name,
							corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
							v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
							corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
						},
					},
					Status: v1.NodeClaimStatus{
						Allocatable: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:  resource.MustParse("32"),
							corev1.ResourcePods: resource.MustParse("100"),
						},
					},
				})
				nodeClaims = append(nodeClaims, ncs...)
				nodes = append(nodes, ns...)
			}
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < len(nodeClaims); i++ {
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			ExpectSingletonReconciled(ctx, disruptionController)

			for _, np := range nps {
				metric, found := FindMetricWithLabelValues("karpenter_nodepools_allowed_disruptions", map[string]string{
					"nodepool": np.Name,
				})
				Expect(found).To(BeTrue())
				Expect(metric.GetGauge().GetValue()).To(BeNumerically("==", 2))
			}

			// Execute the command in the queue, only deleting 20 nodes
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(10))
		})
		It("should allow all nodes from each nodePool to be deleted", func() {
			// Create 10 nodepools
			nps := test.NodePools(10, v1.NodePool{
				Spec: v1.NodePoolSpec{
					Disruption: v1.Disruption{
						ConsolidateAfter:    v1.MustParseNillableDuration("30s"),
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmpty,
						Budgets: []v1.Budget{{
							Nodes: "100%",
						}},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < len(nps); i++ {
				ExpectApplied(ctx, env.Client, nps[i])
			}
			nodeClaims = make([]*v1.NodeClaim, 0, 30)
			nodes = make([]*corev1.Node, 0, 30)
			// Create 3 nodes for each nodePool
			for _, np := range nps {
				ncs, ns := test.NodeClaimsAndNodes(3, v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            np.Name,
							corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
							v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
							corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
						},
					},
					Status: v1.NodeClaimStatus{
						Allocatable: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:  resource.MustParse("32"),
							corev1.ResourcePods: resource.MustParse("100"),
						},
					},
				})
				nodeClaims = append(nodeClaims, ncs...)
				nodes = append(nodes, ns...)
			}
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < len(nodeClaims); i++ {
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			ExpectSingletonReconciled(ctx, disruptionController)

			for _, np := range nps {
				metric, found := FindMetricWithLabelValues("karpenter_nodepools_allowed_disruptions", map[string]string{
					"nodepool": np.Name,
				})
				Expect(found).To(BeTrue())
				Expect(metric.GetGauge().GetValue()).To(BeNumerically("==", 3))
			}

			// Execute the command in the queue, deleting all nodes
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(0))
		})
	})
	Context("Emptiness", func() {
		It("can delete empty nodes", func() {
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
		})
		It("can delete empty and drifted nodes", func() {
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
				metrics.ReasonLabel: "empty",
			})
		})
		It("should ignore nodes without the consolidatable status condition", func() {
			_ = nodeClaim.StatusConditions().Clear(v1.ConditionTypeConsolidatable)
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectSingletonReconciled(ctx, disruptionController)

			// Expect to not create or delete more nodeclaims
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
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
		It("should delete nodes with the karpenter.sh/do-not-disrupt annotation set to false", func() {
			node.Annotations = lo.Assign(node.Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "false"})
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
		})
		It("should ignore nodes that have pods", func() {
			pod := test.Pod()
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
		It("should ignore nodes with the consolidatable status condition set to false", func() {
			nodeClaim.StatusConditions().SetFalse(v1.ConditionTypeConsolidatable, "NotEmpty", "NotEmpty")
			ExpectApplied(ctx, env.Client, nodeClaim, node, nodePool)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectSingletonReconciled(ctx, disruptionController)

			// Expect to not create or delete more nodeclaims
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
		})
	})
	It("can delete multiple empty nodes", func() {
		ExpectApplied(ctx, env.Client, nodeClaim, node, nodeClaim2, node2, nodePool)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})
		ExpectSingletonReconciled(ctx, disruptionController)

		cmds := queue.GetCommands()
		Expect(cmds).To(HaveLen(1))
		ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

		// Cascade any deletion of the nodeclaim to the node
		ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim, nodeClaim2)

		// we should delete the empty nodes
		Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(0))
		Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClaim)
		ExpectNotFound(ctx, env.Client, nodeClaim2)
	})
	It("considers pending pods when consolidating", func() {
		largeTypes := lo.Filter(cloudProvider.InstanceTypes, func(item *cloudprovider.InstanceType, index int) bool {
			return item.Capacity.Cpu().Cmp(resource.MustParse("64")) >= 0
		})
		sort.Slice(largeTypes, func(i, j int) bool {
			return largeTypes[i].Offerings[0].Price < largeTypes[j].Offerings[0].Price
		})

		largeCheapType := largeTypes[0]
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: largeCheapType.Name,
					v1.CapacityTypeLabelKey:        largeCheapType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       largeCheapType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  *largeCheapType.Capacity.Cpu(),
					corev1.ResourcePods: *largeCheapType.Capacity.Pods(),
				},
			},
		})

		// there is a pending pod that should land on the node
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
		})
		unsched := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("62"),
				},
			},
		})

		ExpectApplied(ctx, env.Client, nodeClaim, node, pod, unsched, nodePool)

		// bind one of the pods to the node
		ExpectManualBinding(ctx, env.Client, pod, node)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		ExpectSingletonReconciled(ctx, disruptionController)

		// we don't need any new nodes and consolidation should notice the huge pending pod that needs the large
		// node to schedule, which prevents the large expensive node from being replaced
		Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
		Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
		ExpectExists(ctx, env.Client, nodeClaim)
	})
	It("will consider a node with a DaemonSet pod as empty", func() {
		// assign the nodeclaims to the least expensive offering so we don't get a replacement
		nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{
			corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
			v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
		})
		node.Labels = lo.Assign(node.Labels, map[string]string{
			corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
			v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
		})

		ds := test.DaemonSet()
		ExpectApplied(ctx, env.Client, ds, nodeClaim, node, nodePool)

		// Pods owned by a Deployment
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "DaemonSet",
						Name:               ds.Name,
						UID:                ds.UID,
						Controller:         lo.ToPtr(true),
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		})
		ExpectApplied(ctx, env.Client, pod)

		ExpectManualBinding(ctx, env.Client, pod, node)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
		ExpectSingletonReconciled(ctx, disruptionController)
		ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

		// Cascade any deletion of the nodeclaim to the node
		ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

		// we should delete the empty node
		Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(0))
		Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClaim, node)
	})
	It("will consider a node with terminating Deployment pods as empty", func() {
		// assign the nodeclaims to the least expensive offering so we don't get a replacement
		nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{
			corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
			v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
		})
		node.Labels = lo.Assign(node.Labels, map[string]string{
			corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
			v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
		})

		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs, nodeClaim, node, nodePool)

		// Pod owned by a Deployment
		pods := test.Pods(3, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
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
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		})
		ExpectApplied(ctx, env.Client, lo.Map(pods, func(p *corev1.Pod, _ int) client.Object { return p })...)

		for _, p := range pods {
			ExpectManualBinding(ctx, env.Client, p, node)
		}

		// Evict the pods off of the node
		for _, p := range pods {
			// Trigger an eviction to set the deletion timestamp but not delete the pod
			ExpectEvicted(ctx, env.Client, p)
			ExpectExists(ctx, env.Client, p)
		}

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
		ExpectSingletonReconciled(ctx, disruptionController)
		ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

		// Cascade any deletion of the nodeclaim to the node
		ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

		// we should delete the empty node
		Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(0))
		Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClaim, node)
	})
	It("will not consider a node with a terminating StatefulSet pod as empty", func() {
		// assign the nodeclaims to the least expensive offering so we don't get a replacement
		nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{
			corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
			v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
		})
		node.Labels = lo.Assign(node.Labels, map[string]string{
			corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
			v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
		})

		ss := test.StatefulSet()
		ExpectApplied(ctx, env.Client, ss, nodeClaim, node, nodePool)

		// Pod owned by a StatefulSet
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "StatefulSet",
						Name:               ss.Name,
						UID:                ss.UID,
						Controller:         lo.ToPtr(true),
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		})
		ExpectApplied(ctx, env.Client, pod)

		ExpectManualBinding(ctx, env.Client, pod, node)

		// Trigger an eviction to set the deletion timestamp but not delete the pod
		ExpectEvicted(ctx, env.Client, pod)
		ExpectExists(ctx, env.Client, pod)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		ExpectSingletonReconciled(ctx, disruptionController)
		cmds := queue.GetCommands()
		Expect(cmds).To(HaveLen(0))

		// Cascade any deletion of the nodeclaim to the node
		ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

		// we shouldn't delete the node due to emptiness with a statefulset terminating pod
		Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
		Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
		ExpectExists(ctx, env.Client, nodeClaim)
		ExpectExists(ctx, env.Client, node)
	})
	It("should wait for the node TTL for empty nodes before consolidating", func() {
		disruptionController = disruption.NewController(fakeClock, env.Client, prov, cloudProvider, recorder, cluster, queue, disruption.WithMethods(NewMethodsWithRealValidator()...))
		ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0], nodePool)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0]}, []*v1.NodeClaim{nodeClaims[0]})

		finished := atomic.Bool{}
		ExpectParallelized(
			func() {
				defer finished.Store(true)
				ExpectSingletonReconciled(ctx, disruptionController)
			},
			func() {
				// wait for the controller to block on the validation timeout
				Eventually(fakeClock.HasWaiters, time.Second*10).Should(BeTrue())
				// controller should be blocking during the timeout
				Expect(finished.Load()).To(BeFalse())
				// and the node should not be deleted yet
				ExpectExists(ctx, env.Client, nodeClaims[0])
				// advance the clock so that the timeout expires
				fakeClock.Step(31 * time.Second)
				// controller should finish
				Eventually(finished.Load, 10*time.Second).Should(BeTrue())
			},
		)
		// Process the item so that the nodes can be deleted
		ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

		// Cascade any deletion of the nodeclaim to the node
		ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

		// nodeclaim should be deleted after the TTL due to emptiness
		Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(0))
		Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0])
	})
	Context("Static NodePool", func() {
		It("should not consolidate static NodePool nodes", func() {
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
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			emptyConsolidation := disruption.NewEmptiness(c)
			singleNodeConsolidation := disruption.NewSingleNodeConsolidation(c)
			multiNodeConsolidation := disruption.NewMultiNodeConsolidation(c)

			ExpectApplied(ctx, env.Client, staticNp, nodeClaims[0], nodeClaims[1], nodes[0], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, emptyConsolidation.ShouldDisrupt, emptyConsolidation.Class(), queue)
			Expect(err).To(Succeed())
			Expect(candidates).To(HaveLen(0))

			candidates, err = disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, singleNodeConsolidation.ShouldDisrupt, singleNodeConsolidation.Class(), queue)
			Expect(err).To(Succeed())
			Expect(candidates).To(HaveLen(0))

			candidates, err = disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, multiNodeConsolidation.ShouldDisrupt, multiNodeConsolidation.Class(), queue)
			Expect(err).To(Succeed())
			Expect(candidates).To(HaveLen(0))
		})
	})
})
