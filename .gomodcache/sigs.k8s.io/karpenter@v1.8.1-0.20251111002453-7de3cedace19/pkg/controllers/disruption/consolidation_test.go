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
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption"
	pscheduling "sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
)

var _ = Describe("Consolidation", func() {
	var nodePool *v1.NodePool
	var nodeClaim, spotNodeClaim *v1.NodeClaim
	var node, spotNode *corev1.Node
	var labels = map[string]string{
		"app": "test",
	}
	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
					// Disrupt away!
					Budgets: []v1.Budget{{
						Nodes: "100%",
					}},
					ConsolidateAfter: v1.MustParseNillableDuration("0s"),
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
				Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
			},
		})
		spotNodeClaim, spotNode = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveSpotInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
			},
		})
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		spotNodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{FeatureGates: test.FeatureGates{SpotToSpotConsolidation: lo.ToPtr(true)}}))
	})
	Context("Events", func() {
		It("should not fire an event for ConsolidationDisabled when the NodePool has consolidation set to WhenEmptyOrUnderutilized", func() {
			nodePool.Spec.Disruption.ConsolidationPolicy = v1.ConsolidationPolicyWhenEmptyOrUnderutilized
			nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("0s")
			ExpectApplied(ctx, env.Client, node, nodeClaim, nodePool)

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			Expect(recorder.Calls(events.Unconsolidatable)).To(Equal(0))
		})
		It("should fire an event for ConsolidationDisabled when the NodePool has consolidation set to WhenEmpty", func() {
			pod := test.Pod()
			nodePool.Spec.Disruption.ConsolidationPolicy = v1.ConsolidationPolicyWhenEmpty
			nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("1m")
			ExpectApplied(ctx, env.Client, pod, node, nodeClaim, nodePool)
			ExpectManualBinding(ctx, env.Client, pod, node)

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			Expect(recorder.Calls(events.Unconsolidatable)).To(Equal(4))
		})
		It("should fire an event for ConsolidationDisabled when the NodePool has consolidateAfter set to 'Never'", func() {
			pod := test.Pod()
			nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("Never")
			ExpectApplied(ctx, env.Client, pod, node, nodeClaim, nodePool)
			ExpectManualBinding(ctx, env.Client, pod, node)

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			// We get six calls here because we have Nodes and NodeClaims that fired for this event
			// and each of the consolidation mechanisms specifies that this event should be fired
			Expect(recorder.Calls(events.Unconsolidatable)).To(Equal(6))
		})
		It("should fire an event when a candidate does not have a resolvable instance type", func() {
			pod := test.Pod()
			delete(nodeClaim.Labels, corev1.LabelInstanceTypeStable)
			delete(node.Labels, corev1.LabelInstanceTypeStable)

			ExpectApplied(ctx, env.Client, pod, node, nodeClaim, nodePool)
			ExpectManualBinding(ctx, env.Client, pod, node)

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			// We get four calls since we only care about this since we don't emit for empty node consolidation
			Expect(recorder.Calls(events.Unconsolidatable)).To(Equal(4))
		})
		It("should fire an event when a candidate does not have the capacity type label", func() {
			pod := test.Pod()
			delete(nodeClaim.Labels, v1.CapacityTypeLabelKey)
			delete(node.Labels, v1.CapacityTypeLabelKey)

			ExpectApplied(ctx, env.Client, pod, node, nodeClaim, nodePool)
			ExpectManualBinding(ctx, env.Client, pod, node)

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			// We get four calls since we only care about this since we don't emit for empty node consolidation
			Expect(recorder.Calls(events.Unconsolidatable)).To(Equal(4))
		})
		It("should fire an event when a candidate does not have the zone label", func() {
			pod := test.Pod()
			delete(nodeClaim.Labels, corev1.LabelTopologyZone)
			delete(node.Labels, corev1.LabelTopologyZone)

			ExpectApplied(ctx, env.Client, pod, node, nodeClaim, nodePool)
			ExpectManualBinding(ctx, env.Client, pod, node)

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)
			// We get four calls since we only care about this since we don't emit for empty node consolidation
			Expect(recorder.Calls(events.Unconsolidatable)).To(Equal(4))
		})
	})
	Context("Metrics", func() {
		BeforeEach(func() {
			disruption.FailedValidationsTotal.Reset()
		})
		It("should correctly report eligible nodes", func() {
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
			ExpectMetricGaugeValue(disruption.EligibleNodes, 0, map[string]string{
				metrics.ReasonLabel: "underutilized",
			})

			// remove the do-not-disrupt annotation to make the node eligible for consolidation and update cluster state
			pod.SetAnnotations(map[string]string{})
			ExpectApplied(ctx, env.Client, pod)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			ExpectMetricGaugeValue(disruption.EligibleNodes, 1, map[string]string{
				metrics.ReasonLabel: "underutilized",
			})
		})
		DescribeTable("should correctly report invalidated commands for emptiness disruption", func(validatorOpt TestEmptinessValidatorOption) {
			nodes := []*corev1.Node{node}
			nodeClaims := []*v1.NodeClaim{nodeClaim}
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			emptyConsolidation := disruption.NewEmptiness(c, disruption.WithValidator(NewTestEmptinessValidator(nodes, nodeClaims, nodePool, validatorOpt)))
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, emptyConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, emptyConsolidation.ShouldDisrupt, emptyConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := emptyConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).ToNot(HaveOccurred())
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(emptyConsolidation.IsConsolidated()).To(BeFalse())
			ExpectMetricCounterValue(disruption.FailedValidationsTotal, 1, map[string]string{disruption.ConsolidationTypeLabel: emptyConsolidation.ConsolidationType()})
		},
			Entry("when a candidate is blocked by budgets", WithEmptinessBlockingBudget()),
			Entry("when candidates are filtered out due to pod churn", WithEmptinessChurn()),
			Entry("when candidates are filtered out due to candidate being nominated", WithEmptinessNodeNomination()),
		)
		DescribeTable("should correctly report invalidated commands for multi node disruption", func(validatorOpt TestConsolidationValidatorOption) {
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

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
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("1")},
				},
			})
			nodeClaim2.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], node, node2, nodeClaim, nodeClaim2, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], node)
			ExpectManualBinding(ctx, env.Client, pods[1], node2)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, node2)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			multiNodeConsolidation := disruption.NewMultiNodeConsolidation(c, disruption.WithValidator(NewTestMultiConsolidationValidator(nodePool, validatorOpt)))
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, multiNodeConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, multiNodeConsolidation.ShouldDisrupt, multiNodeConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := multiNodeConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(multiNodeConsolidation.IsConsolidated()).To(BeFalse())
			ExpectMetricCounterValue(disruption.FailedValidationsTotal, 2, map[string]string{disruption.ConsolidationTypeLabel: multiNodeConsolidation.ConsolidationType()})
		},
			Entry("when candidates are blocked by budgets", WithUnderutilizedBlockingBudget()),
			Entry("when candidates are filtered out due to pod churn", WithUnderutilizedChurn()),
			Entry("when candidates are filtered out due to candidate being nominated", WithUnderutilizedNodeNomination()),
		)
		DescribeTable("should correctly report invalidated commands for single node disruption", func(validatorOpt TestConsolidationValidatorOption) {
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
			ExpectApplied(ctx, env.Client, rs, pod, node, nodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, node)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			singleNodeConsolidation := disruption.NewSingleNodeConsolidation(c, disruption.WithValidator(NewTestSingleConsolidationValidator(nodePool, validatorOpt)))
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, singleNodeConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, singleNodeConsolidation.ShouldDisrupt, singleNodeConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := singleNodeConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(singleNodeConsolidation.IsConsolidated()).To(BeFalse())
			ExpectMetricCounterValue(disruption.FailedValidationsTotal, 1, map[string]string{disruption.ConsolidationTypeLabel: singleNodeConsolidation.ConsolidationType()})
		},
			Entry("when a candidate is blocked by budgets", WithUnderutilizedBlockingBudget()),
			Entry("when candidates are filtered out due to pod churn", WithUnderutilizedChurn()),
			Entry("when candidates are filtered out due to candidate being nominated", WithUnderutilizedNodeNomination()),
		)
	})
	Context("Budgets", func() {
		var numNodes = 10
		var nodeClaims []*v1.NodeClaim
		var nodes []*corev1.Node
		var rs *appsv1.ReplicaSet
		BeforeEach(func() {
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
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
			// create our RS so we can link a pod to it
			rs = test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())
		})
		It("should only allow 3 empty nodes to be disrupted", func() {
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "30%"}}
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
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
		It("should allow all empty nodes to be disrupted", func() {
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "100%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
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

			// Execute command, thus deleting all nodes
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)
			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(0))
		})
		It("should allow no empty nodes to be disrupted", func() {
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "0%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
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
			Expect(numNodes).To(Equal(numNodes))
		})
		It("should only allow 3 nodes to be deleted in multi node consolidation delete", func() {
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "30%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// make a pod for each nodes, where they each all fit into one node.
			// this should make the optimal multi node decision to delete 9.
			// budgets will make it so we can only delete 3.
			pods := test.Pods(numNodes, test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						// 100m * 10 = 1 vCPU. This should be less than the largest node capacity.
						corev1.ResourceCPU: resource.MustParse("100m"),
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
			for i := 0; i < numNodes; i++ {
				ExpectApplied(ctx, env.Client, pods[i])
				ExpectManualBinding(ctx, env.Client, pods[i], nodes[i])
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			ExpectSingletonReconciled(ctx, disruptionController)

			// Execute command, thus deleting 3 nodes
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)
			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(7))
		})
		It("should only allow 3 nodes to be deleted in single node consolidation delete", func() {
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "30%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// make a pod for each node, where only two pods can fit each node.
			// this will skip over multi node consolidation and go to single
			// node consolidation delete
			pods := test.Pods(numNodes, test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						// 15 + 15 = 30 < 32
						corev1.ResourceCPU: resource.MustParse("15"),
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
			for i := 0; i < numNodes; i++ {
				ExpectApplied(ctx, env.Client, pods[i])
				ExpectManualBinding(ctx, env.Client, pods[i], nodes[i])
			}

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			// Reconcile 5 times, enqueuing 3 commands total.
			for i := 0; i < 5; i++ {
				ExpectSingletonReconciled(ctx, disruptionController)
			}
			// Execute all commands in the queue, only deleting 3 nodes
			cmds := queue.GetCommands()
			for _, cmd := range cmds {
				ExpectObjectReconciled(ctx, env.Client, queue, cmd.Candidates[0].NodeClaim)
			}
			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(7))
		})
		It("should allow 2 nodes from each nodePool to be deleted", func() {
			// Create 10 nodepools
			nps := test.NodePools(10, v1.NodePool{
				Spec: v1.NodePoolSpec{
					Disruption: v1.Disruption{
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
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

			// Execute the command in the queue, only deleting 20 node claims
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
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
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

			// Execute the command in the queue, deleting all node claims
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)
			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(0))
		})
		It("should allow no nodes from each nodePool to be deleted", func() {
			// Create 10 nodepools
			nps := test.NodePools(10, v1.NodePool{
				Spec: v1.NodePoolSpec{
					Disruption: v1.Disruption{
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
						Budgets: []v1.Budget{{
							Nodes: "0%",
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
				Expect(metric.GetGauge().GetValue()).To(BeNumerically("==", 0))
			}

			// There should be no commands in the queue
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(0))
			Expect(len(ExpectNodeClaims(ctx, env.Client))).To(Equal(30))
		})
		It("should not mark empty node consolidated if the candidates can't be disrupted due to budgets with one nodepool", func() {
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "0%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			emptyConsolidation := disruption.NewEmptiness(c)
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, emptyConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, emptyConsolidation.ShouldDisrupt, emptyConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := emptyConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(emptyConsolidation.IsConsolidated()).To(BeFalse())
		})
		It("should not mark empty node consolidated if all candidates can't be disrupted due to budgets with many nodepools", func() {
			// Create 10 nodepools
			nps := test.NodePools(10, v1.NodePool{
				Spec: v1.NodePoolSpec{
					Disruption: v1.Disruption{
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
						Budgets: []v1.Budget{{
							Nodes: "0%",
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

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			emptyConsolidation := disruption.NewEmptiness(c)
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, emptyConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, emptyConsolidation.ShouldDisrupt, emptyConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := emptyConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(cmds).To(HaveLen(0))
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(emptyConsolidation.IsConsolidated()).To(BeFalse())
		})
		It("should not mark multi node consolidated if the candidates can't be disrupted due to budgets with one nodepool", func() {
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "0%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			multiConsolidation := disruption.NewMultiNodeConsolidation(c)
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, multiConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, multiConsolidation.ShouldDisrupt, multiConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := multiConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(multiConsolidation.IsConsolidated()).To(BeFalse())
		})
		It("should not mark multi node consolidated if all candidates can't be disrupted due to budgets with many nodepools", func() {
			// Create 10 nodepools
			nps := test.NodePools(10, v1.NodePool{
				Spec: v1.NodePoolSpec{
					Disruption: v1.Disruption{
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
						Budgets: []v1.Budget{{
							Nodes: "0%",
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

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			multiConsolidation := disruption.NewMultiNodeConsolidation(c)
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, multiConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, multiConsolidation.ShouldDisrupt, multiConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := multiConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(multiConsolidation.IsConsolidated()).To(BeFalse())
		})
		It("should not mark single node consolidated if the candidates can't be disrupted due to budgets with one nodepool", func() {
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "0%"}}

			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < numNodes; i++ {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			singleConsolidation := disruption.NewSingleNodeConsolidation(c)
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, singleConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, singleConsolidation.ShouldDisrupt, singleConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := singleConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(singleConsolidation.IsConsolidated()).To(BeFalse())
		})
		It("should not mark single node consolidated if all candidates can't be disrupted due to budgets with many nodepools", func() {
			// Create 10 nodepools
			nps := test.NodePools(10, v1.NodePool{
				Spec: v1.NodePoolSpec{
					Disruption: v1.Disruption{
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
						Budgets: []v1.Budget{{
							Nodes: "0%",
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

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			singleConsolidation := disruption.NewSingleNodeConsolidation(c)
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, singleConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, singleConsolidation.ShouldDisrupt, singleConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			cmds, err := singleConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(cmds).To(Equal([]disruption.Command{}))

			Expect(singleConsolidation.IsConsolidated()).To(BeFalse())
		})
	})
	Context("Replace", func() {
		DescribeTable("can replace node",
			func(spotToSpot bool) {
				nodeClaim = lo.Ternary(spotToSpot, spotNodeClaim, nodeClaim)
				node = lo.Ternary(spotToSpot, spotNode, node)
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
				ExpectApplied(ctx, env.Client, rs, pod, node, nodeClaim, nodePool)

				// bind pods to node
				ExpectManualBinding(ctx, env.Client, pod, node)

				// inform cluster state about nodes and nodeClaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
				ExpectSingletonReconciled(ctx, disruptionController)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)
				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

				// should create a new nodeclaim as there is a cheaper one that can hold the pod
				nodeClaims := ExpectNodeClaims(ctx, env.Client)
				nodes := ExpectNodes(ctx, env.Client)
				Expect(nodeClaims).To(HaveLen(1))
				Expect(nodes).To(HaveLen(1))

				// Expect that the new nodeclaim does not request the most expensive instance type
				Expect(nodeClaims[0].Name).ToNot(Equal(nodeClaim.Name))
				Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Has(corev1.LabelInstanceTypeStable)).To(BeTrue())
				Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(mostExpensiveInstance.Name)).To(BeFalse())

				// and delete the old one
				ExpectNotFound(ctx, env.Client, nodeClaim, node)
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		It("cannot replace spot with spot if less than minimum InstanceTypes flexibility", func() {
			// Forcefully shrink the possible instanceTypes to be lower than 15 to replace a nodeclaim
			cloudProvider.InstanceTypes = lo.Slice(fake.InstanceTypesAssorted(), 0, 5)
			// Forcefully assign lowest possible instancePrice to make sure we have atleast one instance
			// that is lower than the current node.
			cloudProvider.InstanceTypes[0].Offerings[0].Price = 0.001
			cloudProvider.InstanceTypes[0].Offerings[0].Requirements[v1.CapacityTypeLabelKey] = scheduling.NewRequirement(
				v1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, v1.CapacityTypeSpot)
			spotInstances = lo.Filter(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
				for _, o := range i.Offerings {
					if o.Requirements.Get(v1.CapacityTypeLabelKey).Any() == v1.CapacityTypeSpot {
						return true
					}
				}
				return false
			})
			// Sort the spot instances by pricing from low to high
			sort.Slice(spotInstances, func(i, j int) bool {
				return spotInstances[i].Offerings.Cheapest().Price < spotInstances[j].Offerings.Cheapest().Price
			})
			mostExpSpotInstance := spotInstances[len(spotInstances)-1]
			mostExpSpotOffering := mostExpSpotInstance.Offerings[0]
			spotNodeClaim.Labels = lo.Assign(spotNodeClaim.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: mostExpSpotInstance.Name,
				v1.CapacityTypeLabelKey:        mostExpSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

			spotNode.Labels = lo.Assign(spotNode.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: mostExpSpotInstance.Name,
				v1.CapacityTypeLabelKey:        mostExpSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

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
			ExpectApplied(ctx, env.Client, rs, pod, spotNode, spotNodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, spotNode)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{spotNode}, []*v1.NodeClaim{spotNodeClaim})

			// consolidation won't delete the old nodeclaim until the new nodeclaim is ready
			ExpectSingletonReconciled(ctx, disruptionController)
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

			// shouldn't delete the node
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))

			// Expect Unconsolidatable events to be fired
			_, ok := lo.Find(recorder.Events(), func(e events.Event) bool {
				return strings.Contains(e.Message, fmt.Sprintf("SpotToSpotConsolidation requires %d cheaper instance type options than the current candidate to consolidate, got %d",
					disruption.MinInstanceTypesForSpotToSpotConsolidation, 1))
			})
			Expect(ok).To(BeTrue())
		})
		It("cannot replace spot with spot if the spotToSpotConsolidation is disabled", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{FeatureGates: test.FeatureGates{SpotToSpotConsolidation: lo.ToPtr(false)}}))
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
			ExpectApplied(ctx, env.Client, rs, pod, spotNode, spotNodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, spotNode)

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{spotNode}, []*v1.NodeClaim{spotNodeClaim})

			// consolidation won't delete the old nodeclaim until the new nodeclaim is ready
			ExpectSingletonReconciled(ctx, disruptionController)
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(0))

			// shouldn't delete the node
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))

			// Expect Unconsolidatable events to be fired
			_, ok := lo.Find(recorder.Events(), func(e events.Event) bool {
				return strings.Contains(e.Message, "SpotToSpotConsolidation is disabled, can't replace a spot node with a spot node")
			})
			Expect(ok).To(BeTrue())
		})
		It("cannot replace spot with spot if it is part of the 15 cheapest instance types.", func() {
			cloudProvider.InstanceTypes = lo.Slice(fake.InstanceTypesAssorted(), 0, 20)
			// Forcefully assign lowest possible instancePrice to make sure we have atleast one instance
			// that is lower than the current node.
			cloudProvider.InstanceTypes[0].Offerings[0].Price = 0.001
			cloudProvider.InstanceTypes[0].Offerings[0].Requirements[v1.CapacityTypeLabelKey] = scheduling.NewRequirement(
				v1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, v1.CapacityTypeSpot)
			// Also sort the cloud provider instances by pricing from low to high
			sort.Slice(cloudProvider.InstanceTypes, func(i, j int) bool {
				return cloudProvider.InstanceTypes[i].Offerings.Cheapest().Price < cloudProvider.InstanceTypes[j].Offerings.Cheapest().Price
			})
			spotInstances = lo.Filter(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
				for _, o := range i.Offerings {
					if o.Requirements.Get(v1.CapacityTypeLabelKey).Any() == v1.CapacityTypeSpot {
						return true
					}
				}
				return false
			})

			spotInstance := spotInstances[1]
			spotOffering := spotInstance.Offerings[0]
			spotNodeClaim.Labels = lo.Assign(spotNodeClaim.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: spotInstance.Name,
				v1.CapacityTypeLabelKey:        spotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       spotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

			spotNode.Labels = lo.Assign(spotNode.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: spotInstance.Name,
				v1.CapacityTypeLabelKey:        spotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       spotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

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
			ExpectApplied(ctx, env.Client, rs, pod, spotNode, spotNodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, spotNode)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{spotNode}, []*v1.NodeClaim{spotNodeClaim})

			// consolidation won't delete the old nodeclaim until the new nodeclaim is ready
			ExpectSingletonReconciled(ctx, disruptionController)

			// we didn't create a new nodeclaim or delete the old one
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, spotNodeClaim)
			ExpectExists(ctx, env.Client, spotNode)
		})
		It("spot to spot consolidation should order the instance types by price before enforcing minimum flexibility.", func() {
			// Fetch 18 spot instances
			spotInstances = lo.Slice(lo.Filter(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
				for _, o := range i.Offerings {
					if o.Requirements.Get(v1.CapacityTypeLabelKey).Any() == v1.CapacityTypeSpot {
						return true
					}
				}
				return false
			}), 0, 18)
			// Assign the prices for 18 spot instance in ascending order incrementally
			for i, inst := range spotInstances {
				inst.Offerings[0].Price = 1.00 + float64(i)*0.1
			}
			// Force an instancetype that is outside the bound of 15 instances to have the cheapest price among the lot.
			spotInstances[16].Offerings[0].Price = 0.001

			// We now have these spot instance in the list as lowest priced and highest priced instanceTypes
			cheapestSpotInstanceType := spotInstances[16]
			mostExpensiveInstanceType := spotInstances[17]

			// Add these spot instance with this special condition to cloud provider instancetypes
			cloudProvider.InstanceTypes = spotInstances

			expectedInstanceTypesForConsolidation := make([]*cloudprovider.InstanceType, len(spotInstances))
			copy(expectedInstanceTypesForConsolidation, spotInstances)
			// Sort the spot instances by pricing from low to high
			sort.Slice(expectedInstanceTypesForConsolidation, func(i, j int) bool {
				return expectedInstanceTypesForConsolidation[i].Offerings[0].Price < expectedInstanceTypesForConsolidation[j].Offerings[0].Price
			})
			// These 15 cheapest instance types should eventually be considered for consolidation.
			var expectedInstanceTypesNames []string
			for i := 0; i < 15; i++ {
				expectedInstanceTypesNames = append(expectedInstanceTypesNames, expectedInstanceTypesForConsolidation[i].Name)
			}

			// Assign the most expensive spot instancetype so that it will definitely be replaced through consolidation
			spotNodeClaim.Labels = lo.Assign(spotNodeClaim.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: mostExpensiveInstanceType.Name,
				v1.CapacityTypeLabelKey:        mostExpensiveInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpensiveInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

			spotNode.Labels = lo.Assign(spotNode.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: mostExpensiveInstanceType.Name,
				v1.CapacityTypeLabelKey:        mostExpensiveInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpensiveInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

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
			ExpectApplied(ctx, env.Client, rs, pod, spotNode, spotNodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, spotNode)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{spotNode}, []*v1.NodeClaim{spotNodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, spotNodeClaim)
			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, spotNodeClaim)

			// should create a new nodeclaim as there is a cheaper one that can hold the pod
			nodeClaims := ExpectNodeClaims(ctx, env.Client)
			nodes := ExpectNodes(ctx, env.Client)
			Expect(nodeClaims).To(HaveLen(1))
			Expect(nodes).To(HaveLen(1))

			// Expect that the new nodeclaim does not request the most expensive instance type
			Expect(nodeClaims[0].Name).ToNot(Equal(spotNodeClaim.Name))
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Has(corev1.LabelInstanceTypeStable)).To(BeTrue())
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(mostExpensiveInstanceType.Name)).To(BeFalse())

			// Make sure that the cheapest instance that was outside the bound of 15 instance types is considered for consolidation.
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(cheapestSpotInstanceType.Name)).To(BeTrue())
			spotInstancesConsideredForConsolidation := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Values()

			// Make sure that we send only 15 instance types.
			Expect(len(spotInstancesConsideredForConsolidation)).To(Equal(15))

			// Make sure we considered the first 15 cheapest instance types.
			for i := 0; i < 15; i++ {
				Expect(spotInstancesConsideredForConsolidation).To(ContainElement(expectedInstanceTypesNames[i]))
			}

			// and delete the old one
			ExpectNotFound(ctx, env.Client, spotNodeClaim, spotNode)
		})
		It("spot to spot consolidation should consider the max of default and minimum number of instanceTypeOptions from minValues in requirement for truncation if minimum number of instanceTypeOptions from minValues in requirement is greater than 15.", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpExists,
					},
					MinValues: lo.ToPtr(16),
				},
			}
			// Fetch 18 spot instances
			spotInstances = lo.Slice(lo.Filter(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
				for _, o := range i.Offerings {
					if o.Requirements.Get(v1.CapacityTypeLabelKey).Any() == v1.CapacityTypeSpot {
						return true
					}
				}
				return false
			}), 0, 18)
			// Assign the prices for 18 spot instance in ascending order incrementally
			for i, inst := range spotInstances {
				inst.Offerings[0].Price = 1.00 + float64(i)*0.1
			}
			// Force an instancetype that is outside the bound of 15 instances to have the cheapest price among the lot.
			spotInstances[16].Offerings[0].Price = 0.001

			// We now have these spot instance in the list as lowest priced and highest priced instanceTypes
			cheapestSpotInstanceType := spotInstances[16]
			mostExpensiveInstanceType := spotInstances[17]

			// Add these spot instance with this special condition to cloud provider instancetypes
			cloudProvider.InstanceTypes = spotInstances

			expectedInstanceTypesForConsolidation := make([]*cloudprovider.InstanceType, len(spotInstances))
			copy(expectedInstanceTypesForConsolidation, spotInstances)
			// Sort the spot instances by pricing from low to high
			sort.Slice(expectedInstanceTypesForConsolidation, func(i, j int) bool {
				return expectedInstanceTypesForConsolidation[i].Offerings[0].Price < expectedInstanceTypesForConsolidation[j].Offerings[0].Price
			})
			// These 15 cheapest instance types should eventually be considered for consolidation.
			var expectedInstanceTypesNames []string
			for i := 0; i < 16; i++ {
				expectedInstanceTypesNames = append(expectedInstanceTypesNames, expectedInstanceTypesForConsolidation[i].Name)
			}

			// Assign the most expensive spot instancetype so that it will definitely be replaced through consolidation
			spotNodeClaim.Labels = lo.Assign(spotNodeClaim.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: mostExpensiveInstanceType.Name,
				v1.CapacityTypeLabelKey:        mostExpensiveInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpensiveInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

			spotNode.Labels = lo.Assign(spotNode.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: mostExpensiveInstanceType.Name,
				v1.CapacityTypeLabelKey:        mostExpensiveInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpensiveInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

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
			ExpectApplied(ctx, env.Client, rs, pod, spotNode, spotNodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, spotNode)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{spotNode}, []*v1.NodeClaim{spotNodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, spotNodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, spotNodeClaim)

			// should create a new nodeclaim as there is a cheaper one that can hold the pod
			nodeClaims := ExpectNodeClaims(ctx, env.Client)
			nodes := ExpectNodes(ctx, env.Client)
			Expect(nodeClaims).To(HaveLen(1))
			Expect(nodes).To(HaveLen(1))

			// Expect that the new nodeclaim does not request the most expensive instance type
			Expect(nodeClaims[0].Name).ToNot(Equal(spotNodeClaim.Name))
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Has(corev1.LabelInstanceTypeStable)).To(BeTrue())
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(mostExpensiveInstanceType.Name)).To(BeFalse())

			// Make sure that the cheapest instance that was outside the bound of 15 instance types is considered for consolidation.
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(cheapestSpotInstanceType.Name)).To(BeTrue())
			spotInstancesConsideredForConsolidation := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Values()

			// Make sure that we send only 16 instance types.
			Expect(len(spotInstancesConsideredForConsolidation)).To(Equal(16))

			// Make sure we considered the first 16 cheapest instance types.
			for i := 0; i < 16; i++ {
				Expect(spotInstancesConsideredForConsolidation).To(ContainElement(expectedInstanceTypesNames[i]))
			}

			// and delete the old one
			ExpectNotFound(ctx, env.Client, spotNodeClaim, spotNode)
		})
		It("should handle failing filterOutSameInstanceType correctly (without a panic) when minValues isn't satisfied", func() {
			// Create a NodePool that has minValues in requirement
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpExists,
					},
					MinValues: lo.ToPtr(2),
				},
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      v1.CapacityTypeLabelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{v1.CapacityTypeOnDemand},
					},
				},
			}
			currentInstanceType := fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "current-on-demand",
				Resources: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("5"),
				},
				Offerings: []*cloudprovider.Offering{
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        0.5,
					},
				},
			})
			otherInstanceType := fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "other-on-demand",
				Resources: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("5"),
				},
				Offerings: []*cloudprovider.Offering{
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        0.4,
					},
				},
			})
			cloudProvider.InstanceTypes = []*cloudprovider.InstanceType{
				currentInstanceType,
				otherInstanceType,
			}
			nodeClaims, nodes := test.NodeClaimsAndNodes(3, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: currentInstanceType.Name,
						v1.CapacityTypeLabelKey:        v1.CapacityTypeOnDemand,
						corev1.LabelTopologyZone:       "test-zone-1a",
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("4"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			for i := range nodeClaims {
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectApplied(ctx, env.Client, lo.Map(nodeClaims, func(o *v1.NodeClaim, _ int) client.Object { return o })...)
			ExpectApplied(ctx, env.Client, lo.Map(nodes, func(o *corev1.Node, _ int) client.Object { return o })...)
			pods := test.Pods(4, test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("2"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, lo.Map(pods, func(o *corev1.Pod, _ int) client.Object { return o })...)

			// Schedule a single pod to each of the first two nodes and two pods to the third
			// Expect that the first two nodes should attempt to multi-node consolidate into each other with a replacement
			// When multi-node consolidation is performed it should hit the edge case where minValues is not satisfied after
			// performing filterOutSameInstanceType since, when the first two nodes combine, there are only two options available
			// for replacement; however, one of them is the same type as the nodes already are, meaning it will get filtered out
			// and no longer satisfy the minValues for the NodePool requirement
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])
			ExpectManualBinding(ctx, env.Client, pods[3], nodes[2])
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
			ExpectSingletonReconciled(ctx, disruptionController)
			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims...)

			// Eventually expect consolidation to evaluate that it can delete one of the first two nodes
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
		})
		It("spot to spot consolidation should consider the default for truncation if minimum number of instanceTypeOptions from minValues in requirement is less than 15.", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpExists,
					},
					MinValues: lo.ToPtr(10),
				},
			}
			// Fetch 18 spot instances
			spotInstances = lo.Slice(lo.Filter(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
				for _, o := range i.Offerings {
					if o.Requirements.Get(v1.CapacityTypeLabelKey).Any() == v1.CapacityTypeSpot {
						return true
					}
				}
				return false
			}), 0, 18)
			// Assign the prices for 18 spot instance in ascending order incrementally
			for i, inst := range spotInstances {
				inst.Offerings[0].Price = 1.00 + float64(i)*0.1
			}
			// Force an instancetype that is outside the bound of 15 instances to have the cheapest price among the lot.
			spotInstances[16].Offerings[0].Price = 0.001

			// We now have these spot instance in the list as lowest priced and highest priced instanceTypes
			cheapestSpotInstanceType := spotInstances[16]
			mostExpensiveInstanceType := spotInstances[17]

			// Add these spot instance with this special condition to cloud provider instancetypes
			cloudProvider.InstanceTypes = spotInstances

			expectedInstanceTypesForConsolidation := make([]*cloudprovider.InstanceType, len(spotInstances))
			copy(expectedInstanceTypesForConsolidation, spotInstances)
			// Sort the spot instances by pricing from low to high
			sort.Slice(expectedInstanceTypesForConsolidation, func(i, j int) bool {
				return expectedInstanceTypesForConsolidation[i].Offerings[0].Price < expectedInstanceTypesForConsolidation[j].Offerings[0].Price
			})
			// These 15 cheapest instance types should eventually be considered for consolidation.
			var expectedInstanceTypesNames []string
			for i := 0; i < 15; i++ {
				expectedInstanceTypesNames = append(expectedInstanceTypesNames, expectedInstanceTypesForConsolidation[i].Name)
			}

			// Assign the most expensive spot instancetype so that it will definitely be replaced through consolidation
			spotNodeClaim.Labels = lo.Assign(spotNodeClaim.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: mostExpensiveInstanceType.Name,
				v1.CapacityTypeLabelKey:        mostExpensiveInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpensiveInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

			spotNode.Labels = lo.Assign(spotNode.Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: mostExpensiveInstanceType.Name,
				v1.CapacityTypeLabelKey:        mostExpensiveInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpensiveInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

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
			ExpectApplied(ctx, env.Client, rs, pod, spotNode, spotNodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, spotNode)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{spotNode}, []*v1.NodeClaim{spotNodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, spotNodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, spotNodeClaim)

			// should create a new nodeclaim as there is a cheaper one that can hold the pod
			nodeClaims := ExpectNodeClaims(ctx, env.Client)
			nodes := ExpectNodes(ctx, env.Client)
			Expect(nodeClaims).To(HaveLen(1))
			Expect(nodes).To(HaveLen(1))

			// Expect that the new nodeclaim does not request the most expensive instance type
			Expect(nodeClaims[0].Name).ToNot(Equal(spotNodeClaim.Name))
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Has(corev1.LabelInstanceTypeStable)).To(BeTrue())
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(mostExpensiveInstanceType.Name)).To(BeFalse())

			// Make sure that the cheapest instance that was outside the bound of 15 instance types is considered for consolidation.
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(cheapestSpotInstanceType.Name)).To(BeTrue())
			spotInstancesConsideredForConsolidation := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Values()

			// Make sure that we send only 15 instance types.
			Expect(len(spotInstancesConsideredForConsolidation)).To(Equal(15))

			// Make sure we considered the first 15 cheapest instance types.
			for i := 0; i < 15; i++ {
				Expect(spotInstancesConsideredForConsolidation).To(ContainElement(expectedInstanceTypesNames[i]))
			}

			// and delete the old one
			ExpectNotFound(ctx, env.Client, spotNodeClaim, spotNode)
		})
		DescribeTable("Consolidation should fail if filterByPrice breaks the minimum requirement from the NodePools.",
			func(spotToSpot bool) {
				// Create a NodePool that has minValues in requirement
				nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      corev1.LabelInstanceTypeStable,
							Operator: corev1.NodeSelectorOpExists,
						},
						MinValues: lo.ToPtr(16),
					},
				}
				// Fetch 18 spot instances
				spotInstances = lo.Slice(spotInstances, 0, 18)
				// Fetch 18 od instances
				onDemandInstances = lo.Slice(onDemandInstances, 0, 18)

				// We now have these spot instance in the list as lowest priced and highest priced instanceTypes
				// This means that we have 15 instance types to replace spot instance which is enough for consolidation
				// but note that we have minValues for instanceTypes as 16. So, we should fail the consolidation.
				mostExpensiveInstanceType := spotInstances[15]
				mostExpensiveODInstanceType := onDemandInstances[15]

				// Assign borderline instanceType as the most expensive so that we have exactly 15 instances to replace for consolidation.
				spotNodeClaim.Labels = lo.Assign(spotNodeClaim.Labels, map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstanceType.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
				})

				spotNode.Labels = lo.Assign(spotNode.Labels, map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstanceType.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
				})

				nodeClaim.Labels = lo.Assign(spotNodeClaim.Labels, map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveODInstanceType.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveODInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveODInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
				})

				node.Labels = lo.Assign(spotNode.Labels, map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveODInstanceType.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveODInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveODInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
				})

				nodeClaim = lo.Ternary(spotToSpot, spotNodeClaim, nodeClaim)
				node = lo.Ternary(spotToSpot, spotNode, node)
				// Add these spot instance with this special condition to cloud provider instancetypes
				cloudProvider.InstanceTypes = lo.Ternary(spotToSpot, spotInstances, onDemandInstances)

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
				ExpectApplied(ctx, env.Client, rs, pod, spotNode, spotNodeClaim, nodePool)

				// bind pods to node
				ExpectManualBinding(ctx, env.Client, pod, spotNode)

				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{spotNode}, []*v1.NodeClaim{spotNodeClaim})

				// consolidation won't delete the old nodeclaim until the new nodeclaim is ready
				ExpectSingletonReconciled(ctx, disruptionController)

				// we didn't create a new nodeclaim or delete the old one
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
				ExpectExists(ctx, env.Client, spotNodeClaim)
				ExpectExists(ctx, env.Client, spotNode)
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("can replace nodes if another nodePool returns no instance types",
			func(spotToSpot bool) {
				nodeClaim = lo.Ternary(spotToSpot, spotNodeClaim, nodeClaim)
				node = lo.Ternary(spotToSpot, spotNode, node)
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

				nodePool2 := test.NodePool()
				cloudProvider.InstanceTypesForNodePool[nodePool2.Name] = nil
				ExpectApplied(ctx, env.Client, rs, pod, node, nodeClaim, nodePool, nodePool2)

				// bind pods to node
				ExpectManualBinding(ctx, env.Client, pod, node)

				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
				ExpectSingletonReconciled(ctx, disruptionController)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

				// should create a new nodeclaim as there is a cheaper one that can hold the pod
				nodeclaims := ExpectNodeClaims(ctx, env.Client)
				nodes := ExpectNodes(ctx, env.Client)
				Expect(nodeclaims).To(HaveLen(1))
				Expect(nodes).To(HaveLen(1))

				// Expect that the new nodeclaim does not request the most expensive instance type
				Expect(nodeclaims[0].Name).ToNot(Equal(nodeClaim.Name))
				Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeclaims[0].Spec.Requirements...).Has(corev1.LabelInstanceTypeStable)).To(BeTrue())
				Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeclaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(mostExpensiveInstance.Name)).To(BeFalse())

				// and delete the old one
				ExpectNotFound(ctx, env.Client, nodeClaim, node)
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("can replace nodes, considers PDB",
			func(spotToSpot bool) {
				nodeClaim = lo.Ternary(spotToSpot, spotNodeClaim, nodeClaim)
				node = lo.Ternary(spotToSpot, spotNode, node)
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

				pods := test.Pods(3, test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
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
				pdb := test.PodDisruptionBudget(test.PDBOptions{
					Labels:         labels,
					MaxUnavailable: fromInt(0),
					Status: &policyv1.PodDisruptionBudgetStatus{
						ObservedGeneration: 1,
						DisruptionsAllowed: 0,
						CurrentHealthy:     1,
						DesiredHealthy:     1,
						ExpectedPods:       1,
					},
				})

				ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaim, node, nodePool, pdb)

				// bind pods to node
				ExpectManualBinding(ctx, env.Client, pods[0], node)
				ExpectManualBinding(ctx, env.Client, pods[1], node)
				ExpectManualBinding(ctx, env.Client, pods[2], node)

				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
				ExpectSingletonReconciled(ctx, disruptionController)

				// we didn't create a new nodeclaim or delete the old one
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
				ExpectExists(ctx, env.Client, nodeClaim)
				ExpectExists(ctx, env.Client, node)
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("can replace nodes, considers PDB policy",
			func(spotToSpot bool) {
				nodeClaim = lo.Ternary(spotToSpot, spotNodeClaim, nodeClaim)
				node = lo.Ternary(spotToSpot, spotNode, node)
				if env.Version.Minor() < 27 {
					Skip("PDB policy ony enabled by default for K8s >= 1.27.x")
				}
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

				pods := test.Pods(3, test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
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

				pdb := test.PodDisruptionBudget(test.PDBOptions{
					Labels:         labels,
					MaxUnavailable: fromInt(0),
					Status: &policyv1.PodDisruptionBudgetStatus{
						ObservedGeneration: 1,
						DisruptionsAllowed: 0,
						CurrentHealthy:     1,
						DesiredHealthy:     1,
						ExpectedPods:       1,
					},
				})
				alwaysAllow := policyv1.AlwaysAllow
				pdb.Spec.UnhealthyPodEvictionPolicy = &alwaysAllow

				ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaim, node, nodePool, pdb)

				// bind pods to node
				ExpectManualBinding(ctx, env.Client, pods[0], node)
				ExpectManualBinding(ctx, env.Client, pods[1], node)
				ExpectManualBinding(ctx, env.Client, pods[2], node)

				// set all of these pods to unhealthy so the PDB won't stop their eviction
				for _, p := range pods {
					p.Status.Conditions = []corev1.PodCondition{
						{
							Type:               corev1.PodReady,
							Status:             corev1.ConditionFalse,
							LastProbeTime:      metav1.Now(),
							LastTransitionTime: metav1.Now(),
						},
					}
					ExpectApplied(ctx, env.Client, p)
				}

				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
				ExpectSingletonReconciled(ctx, disruptionController)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

				// should create a new nodeclaim as there is a cheaper one that can hold the pod
				nodeclaims := ExpectNodeClaims(ctx, env.Client)
				nodes := ExpectNodes(ctx, env.Client)
				Expect(nodeclaims).To(HaveLen(1))
				Expect(nodes).To(HaveLen(1))

				// we didn't create a new nodeclaim or delete the old one
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
				ExpectNotFound(ctx, env.Client, nodeClaim, node)
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("can replace nodes, PDB namespace must match",
			func(spotToSpot bool) {
				nodeClaim = lo.Ternary(spotToSpot, spotNodeClaim, nodeClaim)
				node = lo.Ternary(spotToSpot, spotNode, node)
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

				namespace := test.Namespace()
				pdb := test.PodDisruptionBudget(test.PDBOptions{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace.Name,
					},
					Labels:         labels,
					MaxUnavailable: fromInt(0),
					Status: &policyv1.PodDisruptionBudgetStatus{
						ObservedGeneration: 1,
						DisruptionsAllowed: 0,
						CurrentHealthy:     1,
						DesiredHealthy:     1,
						ExpectedPods:       1,
					},
				})

				// bind pods to node
				ExpectApplied(ctx, env.Client, rs, pod, nodeClaim, node, nodePool, namespace, pdb)
				ExpectManualBinding(ctx, env.Client, pod, node)

				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
				ExpectSingletonReconciled(ctx, disruptionController)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

				// should create a new nodeclaim as there is a cheaper one that can hold the pod
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
				ExpectNotFound(ctx, env.Client, nodeClaim, node)
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("can replace nodes, considers karpenter.sh/do-not-disrupt on nodes",
			func(spotToSpot bool) {
				nodeClaim = lo.Ternary(spotToSpot, spotNodeClaim, nodeClaim)
				node = lo.Ternary(spotToSpot, spotNode, node)
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

				pods := test.Pods(3, test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
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
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				})
				annotatedNodeClaim, annotatedNode := test.NodeClaimAndNode(v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1.DoNotDisruptAnnotationKey: "true",
						},
						Labels: map[string]string{
							v1.NodePoolLabelKey:            nodePool.Name,
							corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
							v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
							corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
						},
					},
					Status: v1.NodeClaimStatus{
						Allocatable: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:  resource.MustParse("5"),
							corev1.ResourcePods: resource.MustParse("100"),
						},
					},
				})

				if spotToSpot {
					annotatedNodeClaim.Labels = lo.Assign(annotatedNodeClaim.Labels, map[string]string{
						corev1.LabelInstanceTypeStable: mostExpensiveSpotInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
					annotatedNode.Labels = lo.Assign(annotatedNode.Labels, map[string]string{
						corev1.LabelInstanceTypeStable: mostExpensiveSpotInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
				}

				ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodePool)
				ExpectApplied(ctx, env.Client, nodeClaim, node, annotatedNodeClaim, annotatedNode)

				// bind pods to node
				ExpectManualBinding(ctx, env.Client, pods[0], node)
				ExpectManualBinding(ctx, env.Client, pods[1], node)
				ExpectManualBinding(ctx, env.Client, pods[2], annotatedNode)

				// inform cluster state about nodes and nodeClaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node, annotatedNode}, []*v1.NodeClaim{nodeClaim, annotatedNodeClaim})
				ExpectSingletonReconciled(ctx, disruptionController)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)
				// Cascade any deletion of the nodeClaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

				// we should delete the non-annotated node and replace with a cheaper node
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
				ExpectNotFound(ctx, env.Client, nodeClaim, node)
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("can replace nodes, considers karpenter.sh/do-not-disrupt on pods",
			func(spotToSpot bool) {
				nodeClaim = lo.Ternary(spotToSpot, spotNodeClaim, nodeClaim)
				node = lo.Ternary(spotToSpot, spotNode, node)
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

				pods := test.Pods(3, test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
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
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
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
						Allocatable: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:  resource.MustParse("5"),
							corev1.ResourcePods: resource.MustParse("100"),
						},
					},
				})
				if spotToSpot {
					nodeClaim2.Labels = lo.Assign(nodeClaim2.Labels, map[string]string{
						corev1.LabelInstanceTypeStable: mostExpensiveSpotInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
					node2.Labels = lo.Assign(node2.Labels, map[string]string{
						corev1.LabelInstanceTypeStable: mostExpensiveSpotInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
				}
				// Block this pod from being disrupted with karpenter.sh/do-not-disrupt
				pods[2].Annotations = lo.Assign(pods[2].Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "true"})

				ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodePool)
				ExpectApplied(ctx, env.Client, nodeClaim, node, nodeClaim2, node2)

				// bind pods to node
				ExpectManualBinding(ctx, env.Client, pods[0], node)
				ExpectManualBinding(ctx, env.Client, pods[1], node)
				ExpectManualBinding(ctx, env.Client, pods[2], node2)

				// inform cluster state about nodes and nodeClaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node, node2}, []*v1.NodeClaim{nodeClaim, nodeClaim2})
				ExpectSingletonReconciled(ctx, disruptionController)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

				// we should delete the non-annotated node and replace with a cheaper node
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
				ExpectNotFound(ctx, env.Client, nodeClaim, node)
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		It("won't replace node if any spot replacement is more expensive", func() {
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
				Name: "potential-spot-replacement",
				Offerings: []*cloudprovider.Offering{
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        1.0,
					},
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1b"}),
						Price:        0.2,
					},
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1c"}),
						Price:        0.4,
					},
				},
			})
			cloudProvider.InstanceTypes = []*cloudprovider.InstanceType{
				currentInstance,
				replacementInstance,
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
			nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: currentInstance.Name,
						v1.CapacityTypeLabelKey:        currentInstance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       currentInstance.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
				},
			})

			ExpectApplied(ctx, env.Client, rs, pod, nodeClaim, node, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Expect to not create or delete more nodeclaims
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
			ExpectExists(ctx, env.Client, node)
		})
		It("won't replace on-demand node if on-demand replacement is more expensive", func() {
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
				Name: "on-demand-replacement",
				Offerings: []*cloudprovider.Offering{
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        0.6,
					},
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1b"}),
						Price:        0.6,
					},
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1b"}),
						Price:        0.2,
					},
					{
						Available:    true,
						Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1c"}),
						Price:        0.3,
					},
				},
			})

			cloudProvider.InstanceTypes = []*cloudprovider.InstanceType{
				currentInstance,
				replacementInstance,
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

			// nodePool should require on-demand instance for this test case
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      v1.CapacityTypeLabelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{v1.CapacityTypeOnDemand},
					},
				},
			}
			nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: currentInstance.Name,
						v1.CapacityTypeLabelKey:        currentInstance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       currentInstance.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
				},
			})

			ExpectApplied(ctx, env.Client, rs, pod, nodeClaim, node, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Expect to not create or delete more nodeclaims
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaim)
			ExpectExists(ctx, env.Client, node)
		})
	})
	Context("Delete", func() {
		var nodeClaims []*v1.NodeClaim
		var nodes []*corev1.Node

		BeforeEach(func() {
			nodeClaims, nodes = test.NodeClaimsAndNodes(2, v1.NodeClaim{
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
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
		})
		It("can delete nodes", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					}}})
			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[1])

			// we don't need a new node, but we should evict everything off one of node2 which only has a single pod
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			// and delete the old one
			ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1])
		})
		It("does not delete nodes with pod churn, deletes nodes without pod churn", func() {
			// create our RS so we can link a pod to it
			ExpectApplied(ctx, env.Client, nodePool)
			for i := range 2 {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			sort.Slice(nodes, func(i, j int) bool {
				return nodes[i].Name < nodes[j].Name
			})

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)

			c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
			emptyConsolidation := disruption.NewEmptiness(c, disruption.WithValidator(NewTestEmptinessValidator(nodes, nodeClaims, nodePool, WithEmptinessChurn())))
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, emptyConsolidation.Reason())
			Expect(err).To(Succeed())

			candidates, err := disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, emptyConsolidation.ShouldDisrupt, emptyConsolidation.Class(), queue)
			Expect(err).To(Succeed())

			// this test validator invalidates the command because it creates pod churn during validaiton
			cmds, err := emptyConsolidation.ComputeCommands(ctx, budgets, candidates...)
			Expect(err).To(Succeed())
			Expect(len(cmds)).To(Equal(1))
			for _, cmd := range cmds {
				Expect(cmd.Results).To(Equal(pscheduling.Results{}))
				Expect(cmd.Candidates).To(HaveLen(1))
				// the test validator manually binds a pod to nodes[0], causing it to no longer be eligible
				Expect(cmd.Candidates[0].StateNode.Node.Name).To(Equal(nodes[1].Name))
				Expect(cmd.Decision()).To(Equal(disruption.DeleteDecision))
			}

			Expect(emptyConsolidation.IsConsolidated()).To(BeFalse())
		})
		It("can delete nodes if another nodePool has no node template", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					}}})
			nodeClassNodePool := test.NodePool()
			nodeClassNodePool.Spec.Template.Spec.NodeClassRef = nil
			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[1])

			// we don't need a new node, but we should evict everything off one of node2 which only has a single pod
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			// and delete the old one
			ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1])
		})
		It("can delete nodes, when non-Karpenter capacity can fit pods", func() {
			unmanagedNode := test.Node(test.NodeOptions{
				ProviderID: test.RandomProviderID(),
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			})
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					},
				},
			})
			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], unmanagedNode, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[0])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], unmanagedNode}, []*v1.NodeClaim{nodeClaims[0]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaims[0])

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

			// we can fit all of our pod capacity on the unmanaged node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(0))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			// and delete the old one
			ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0])
		})
		It("can delete nodes, considers PDB", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

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
					}}})

			// only pod[2] is covered by the PDB
			pods[2].Labels = labels
			pdb := test.PodDisruptionBudget(test.PDBOptions{
				Labels:         labels,
				MaxUnavailable: fromInt(0),
				Status: &policyv1.PodDisruptionBudgetStatus{
					ObservedGeneration: 1,
					DisruptionsAllowed: 0,
					CurrentHealthy:     1,
					DesiredHealthy:     1,
					ExpectedPods:       1,
				},
			})
			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool, pdb)

			// two pods on node 1
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			// one on node 2, but it has a PDB with zero disruptions allowed
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaims[0])

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

			// we don't need a new node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			// but we expect to delete the nodeclaim with more pods (node) as the pod on nodeClaim2 has a PDB preventing
			// eviction
			ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0])
		})
		It("can delete nodes, considers karpenter.sh/do-not-disrupt on nodes", func() {
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
					}}})
			nodeClaims[1].Annotations = lo.Assign(nodeClaims[1].Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "true"})
			nodes[1].Annotations = lo.Assign(nodeClaims[1].Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "true"})

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodePool)
			ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1])

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaims[0])
			// Cascade any deletion of the nodeClaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

			// we should delete the non-annotated node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0])
		})
		It("can delete nodes, considers karpenter.sh/do-not-disrupt on pods", func() {
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
					}}})
			// Block this pod from being disrupted with karpenter.sh/do-not-disrupt
			pods[2].Annotations = lo.Assign(pods[2].Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "true"})

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodePool)
			ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1])

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaims[0])

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

			// we should delete the non-annotated node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0])
		})
		It("does not consolidate nodes with karpenter.sh/do-not-disrupt on pods when the NodePool's TerminationGracePeriod is not nil", func() {
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
					}}})
			// Block this pod from being disrupted with karpenter.sh/do-not-disrupt
			pods[0].Annotations = lo.Assign(pods[0].Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "true"})
			pods[1].Annotations = lo.Assign(pods[1].Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "true"})
			pods[2].Annotations = lo.Assign(pods[2].Annotations, map[string]string{v1.DoNotDisruptAnnotationKey: "true"})

			nodeClaims[0].Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
			nodeClaims[1].Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodePool)
			ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1])

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(0))

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

			// we should delete the non-annotated node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
		})
		It("does not consolidate nodes with pods with blocking PDBs when the NodePool's TerminationGracePeriod is not nil", func() {
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
					}}})

			budget := test.PodDisruptionBudget(test.PDBOptions{
				Labels:         labels,
				MaxUnavailable: fromInt(0),
			})

			nodeClaims[0].Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
			nodeClaims[1].Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodePool, budget)
			ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1])

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(0))

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

			// we should delete the non-annotated node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
		})
		It("can delete nodes, evicts pods without an ownerRef", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

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
					}}})

			// pod[2] is a stand-alone (non ReplicaSet) pod
			pods[2].OwnerReferences = nil
			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

			// two pods on node 1
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			// one on node 2, but it's a standalone pod
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[1])

			// we don't need a new node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			// but we expect to delete the nodeclaim with the fewest pods (nodeclaim 2) even though the pod has no ownerRefs
			// and will not be recreated
			ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1])
		})
		It("won't delete node if it would require pods to schedule on an uninitialized node", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					}}})
			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims, intentionally leaving node as not ready
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[0]))
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaims[0]))
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[1]}, []*v1.NodeClaim{nodeClaims[1]})

			ExpectSingletonReconciled(ctx, disruptionController)
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(0))

			// shouldn't delete the node
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))

			// Expect Unconsolidatable events to be fired
			evts := recorder.Events()
			_, ok := lo.Find(evts, func(e events.Event) bool {
				return strings.Contains(e.Message, "Not all pods would schedule")
			})
			Expect(ok).To(BeTrue())
			_, ok = lo.Find(evts, func(e events.Event) bool {
				return strings.Contains(e.Message, "would schedule against uninitialized nodeclaim")
			})
			Expect(ok).To(BeTrue())
		})
		It("should consider initialized nodes before uninitialized nodes", func() {
			defaultInstanceType := fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "default-instance-type",
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("3"),
					corev1.ResourceMemory: resource.MustParse("3Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
			})
			smallInstanceType := fake.NewInstanceType(fake.InstanceTypeOptions{
				Name: "small-instance-type",
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				},
			})
			cloudProvider.InstanceTypes = []*cloudprovider.InstanceType{
				defaultInstanceType,
				smallInstanceType,
			}
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)

			podCount := 100
			pods := test.Pods(podCount, test.PodOptions{
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
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, rs, nodePool)

			// Setup 100 nodeclaims/nodes with a single nodeclaim/node that is initialized
			elem := rand.Intn(100) //nolint:gosec
			for i := range podCount {
				m, n := test.NodeClaimAndNode(v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            nodePool.Name,
							corev1.LabelInstanceTypeStable: defaultInstanceType.Name,
							v1.CapacityTypeLabelKey:        defaultInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
							corev1.LabelTopologyZone:       defaultInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
						},
					},
					Status: v1.NodeClaimStatus{
						Allocatable: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("3Gi"),
							corev1.ResourcePods:   resource.MustParse("100"),
						},
					},
				})
				m.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				ExpectApplied(ctx, env.Client, pods[i], m, n)
				ExpectManualBinding(ctx, env.Client, pods[i], n)

				if i == elem {
					ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{n}, []*v1.NodeClaim{m})
				} else {
					ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(m))
					ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(n))
				}
			}

			// Create a pod and nodeclaim/node that will eventually be scheduled onto the initialized node
			consolidatableNodeClaim, consolidatableNode := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: smallInstanceType.Name,
						v1.CapacityTypeLabelKey:        smallInstanceType.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       smallInstanceType.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
						corev1.ResourcePods:   resource.MustParse("100"),
					},
				},
			})
			consolidatableNodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)

			// create a new RS so we can link a pod to it
			rs = test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			consolidatablePod := test.Pod(test.PodOptions{
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
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, consolidatableNodeClaim, consolidatableNode, consolidatablePod)
			ExpectManualBinding(ctx, env.Client, consolidatablePod, consolidatableNode)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{consolidatableNode}, []*v1.NodeClaim{consolidatableNodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, consolidatableNodeClaim)
			// Expect no events that state that the pods would schedule against a uninitialized node
			evts := recorder.Events()
			_, ok := lo.Find(evts, func(e events.Event) bool {
				return strings.Contains(e.Message, "would schedule against uninitialized nodeclaim")
			})
			Expect(ok).To(BeFalse())

			// the nodeclaim with the small instance should consolidate onto the initialized node
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(100))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(100))
			ExpectNotFound(ctx, env.Client, consolidatableNodeClaim, consolidatableNode)
		})
		It("can delete nodes with a permanently pending pod", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					}}})

			pending := test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{
					"non-existent": "node-label",
				},
			})

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool, pending)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[1])

			// we don't need a new node, but we should evict everything off one of node2 which only has a single pod
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			// and delete the old one
			ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1])

			// pending pod is still here and hasn't been scheduled anywayre
			pending = ExpectPodExists(ctx, env.Client, pending.Name, pending.Namespace)
			Expect(pending.Spec.NodeName).To(BeEmpty())
		})
		It("won't delete nodes if it would make a non-pending pod go pending", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					}}})

			// setup labels and node selectors so we force the pods onto the nodes we want
			nodes[0].Labels["foo"] = "1"
			nodes[1].Labels["foo"] = "2"

			pods[0].Spec.NodeSelector = map[string]string{"foo": "1"}
			pods[1].Spec.NodeSelector = map[string]string{"foo": "1"}
			pods[2].Spec.NodeSelector = map[string]string{"foo": "2"}

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// No node can be deleted as it would cause one of the three pods to go pending
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
		})
		It("can delete nodes while an invalid node pool exists", func() {
			// this invalid node pool should not be enough to stop all disruption
			badNodePool := &v1.NodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad-nodepool",
				},
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{},
							NodeClassRef: &v1.NodeClassReference{
								Group: "karpenter.test.sh",
								Kind:  "TestNodeClass",
								Name:  "non-existent",
							},
						},
					},
				},
			}
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					}}})

			ExpectApplied(ctx, env.Client, badNodePool, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)
			cloudProvider.ErrorsForNodePool[badNodePool.Name] = fmt.Errorf("unable to fetch instance types")

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[1])

			// we don't need a new node, but we should evict everything off one of node2 which only has a single pod
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			// and delete the old one
			ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1])
		})
	})
	Context("TTL", func() {
		var nodeClaims []*v1.NodeClaim
		var nodes []*corev1.Node

		BeforeEach(func() {
			disruptionController = disruption.NewController(fakeClock, env.Client, prov, cloudProvider, recorder, cluster, queue, disruption.WithMethods(NewMethodsWithRealValidator()...))
			nodeClaims, nodes = test.NodeClaimsAndNodes(2, v1.NodeClaim{
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
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
		})
		It("should wait for the node TTL for non-empty nodes before consolidating", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

			// assign the nodeclaims to the least expensive offering so only one of them gets deleted
			nodeClaims[0].Labels = lo.Assign(nodeClaims[0].Labels, map[string]string{
				corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
				v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
			nodes[0].Labels = lo.Assign(nodes[0].Labels, map[string]string{
				corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
				v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
			nodeClaims[1].Labels = lo.Assign(nodeClaims[1].Labels, map[string]string{
				corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
				v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
			nodes[1].Labels = lo.Assign(nodes[1].Labels, map[string]string{
				corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
				v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})

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
					}}})

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

			// bind pods to nodes
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})

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
					ExpectExists(ctx, env.Client, nodeClaims[1])

					// advance the clock so that the timeout expires
					fakeClock.Step(31 * time.Second)

					// controller should finish
					Eventually(finished.Load, 10*time.Second).Should(BeTrue())
				},
			)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[1])

			// nodeclaim should be deleted after the TTL due to emptiness
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1])
		})
		It("should not consolidate if the action picks different instance types after the node TTL wait", func() {
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
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0], nodePool, pod)
			ExpectManualBinding(ctx, env.Client, pod, nodes[0])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0]}, []*v1.NodeClaim{nodeClaims[0]})

			var wg sync.WaitGroup
			wg.Add(1)
			finished := atomic.Bool{}
			ExpectParallelized(
				func() {
					defer finished.Store(true)
					ExpectSingletonReconciled(ctx, disruptionController)
				},
				func() {
					// wait for the disruptionController to block on the validation timeout
					Eventually(fakeClock.HasWaiters, time.Second*10).Should(BeTrue())
					// controller should be blocking during the timeout
					Expect(finished.Load()).To(BeFalse())

					// and the node should not be deleted yet
					ExpectExists(ctx, env.Client, nodes[0])

					// add an additional pod to the node to change the consolidation decision
					pod2 := test.Pod(test.PodOptions{
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
						ResourceRequirements: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("1"),
							},
						},
					})
					ExpectApplied(ctx, env.Client, pod2)
					ExpectManualBinding(ctx, env.Client, pod2, nodes[0])
					ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[0]))

					// advance the clock so that the timeout expires
					fakeClock.Step(31 * time.Second)
					// controller should finish
					Eventually(finished.Load, 10*time.Second).Should(BeTrue())
				},
			)

			// nothing should be removed since the node is no longer empty
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodes[0])
		})
		It("should not consolidate if the action becomes invalid during the node TTL wait", func() {
			pod := test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			}})
			ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0], nodePool, pod)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0]}, []*v1.NodeClaim{nodeClaims[0]})

			finished := atomic.Bool{}
			ExpectParallelized(
				func() {
					defer finished.Store(true)
					ExpectSingletonReconciled(ctx, disruptionController)
				},
				func() {
					// wait for the disruptionController to block on the validation timeout
					Eventually(fakeClock.HasWaiters, time.Second*10).Should(BeTrue())
					// controller should be blocking during the timeout
					Expect(finished.Load()).To(BeFalse())
					// and the node should not be deleted yet
					ExpectExists(ctx, env.Client, nodeClaims[0])

					// make the node non-empty by binding it
					ExpectManualBinding(ctx, env.Client, pod, nodes[0])
					ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[0]))

					// advance the clock so that the timeout expires
					fakeClock.Step(31 * time.Second)
					// controller should finish
					Eventually(finished.Load, 10*time.Second).Should(BeTrue())
				},
			)

			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(0))

			// nothing should be removed since the node is no longer empty
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClaims[0])
		})
		It("should not replace node if a pod schedules with karpenter.sh/do-not-disrupt during the TTL wait", func() {
			pod := test.Pod()
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectParallelized(
				func() {
					ExpectSingletonReconciled(ctx, disruptionController)
				},
				func() {
					Eventually(fakeClock.HasWaiters, time.Second*10).Should(BeTrue())
					doNotDisruptPod := test.Pod(test.PodOptions{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1.DoNotDisruptAnnotationKey: "true",
							},
						},
					})
					ExpectApplied(ctx, env.Client, doNotDisruptPod)
					ExpectManualBinding(ctx, env.Client, doNotDisruptPod, node)

					// we would normally be able to replace a node, but we are blocked by the do-not-disrupt pods during validation
					Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
					Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
					ExpectExists(ctx, env.Client, node)
					// advance the clock so that the timeout expires
					fakeClock.Step(31 * time.Second)
				},
			)
		})
		It("should not replace node if a pod schedules with a blocking PDB during the TTL wait", func() {
			pod := test.Pod()
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectParallelized(
				func() {
					ExpectSingletonReconciled(ctx, disruptionController)
				},
				func() {
					Eventually(fakeClock.HasWaiters, time.Second*10).Should(BeTrue())
					blockingPDBPod := test.Pod(test.PodOptions{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
					})
					pdb := test.PodDisruptionBudget(test.PDBOptions{
						Labels:         labels,
						MaxUnavailable: fromInt(0),
					})
					ExpectApplied(ctx, env.Client, blockingPDBPod, pdb)
					ExpectManualBinding(ctx, env.Client, blockingPDBPod, node)

					// we would normally be able to replace a node, but we are blocked by the PDB during validation
					Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
					Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
					ExpectExists(ctx, env.Client, node)
					// advance the clock so that the timeout expires
					fakeClock.Step(31 * time.Second)
				},
			)
		})
		It("should not delete node if pods schedule with karpenter.sh/do-not-disrupt during the TTL wait", func() {
			pods := test.Pods(2, test.PodOptions{})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], pods[0], pods[1])

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})

			ExpectParallelized(
				func() {
					ExpectSingletonReconciled(ctx, disruptionController)
				},
				func() {
					Eventually(fakeClock.HasWaiters, time.Second*10).Should(BeTrue())
					doNotDisruptPods := test.Pods(2, test.PodOptions{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1.DoNotDisruptAnnotationKey: "true",
							},
						},
					})
					ExpectApplied(ctx, env.Client, doNotDisruptPods[0], doNotDisruptPods[1])
					ExpectManualBinding(ctx, env.Client, doNotDisruptPods[0], nodes[0])
					ExpectManualBinding(ctx, env.Client, doNotDisruptPods[1], nodes[1])

					// we would normally be able to consolidate down to a single node, but we are blocked by the do-not-disrupt pods during validation
					Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
					Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
					ExpectExists(ctx, env.Client, nodes[0])
					ExpectExists(ctx, env.Client, nodes[1])
					// advance the clock so that the timeout expires
					fakeClock.Step(31 * time.Second)
				},
			)
		})
		It("should not delete node if pods schedule with a blocking PDB during the TTL wait", func() {
			pods := test.Pods(2, test.PodOptions{})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], pods[0], pods[1])

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})

			ExpectParallelized(
				func() {
					ExpectSingletonReconciled(ctx, disruptionController)
				},
				func() {
					Eventually(fakeClock.HasWaiters, time.Second*10).Should(BeTrue())
					blockingPDBPods := test.Pods(2, test.PodOptions{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
					})
					pdb := test.PodDisruptionBudget(test.PDBOptions{
						Labels:         labels,
						MaxUnavailable: fromInt(0),
					})
					ExpectApplied(ctx, env.Client, blockingPDBPods[0], blockingPDBPods[1], pdb)
					ExpectManualBinding(ctx, env.Client, blockingPDBPods[0], nodes[0])
					ExpectManualBinding(ctx, env.Client, blockingPDBPods[1], nodes[1])

					// we would normally be able to consolidate down to a single node, but we are blocked by the PDB during validation
					Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
					Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
					ExpectExists(ctx, env.Client, nodes[0])
					ExpectExists(ctx, env.Client, nodes[1])
					// advance the clock so that the timeout expires
					fakeClock.Step(31 * time.Second)
				},
			)
		})
	})
	Context("Multi-NodeClaim", func() {
		var nodeClaims, spotNodeClaims []*v1.NodeClaim
		var nodes, spotNodes []*corev1.Node

		BeforeEach(func() {
			nodeClaims = []*v1.NodeClaim{}
			spotNodeClaims = []*v1.NodeClaim{}
			nodes = []*corev1.Node{}
			spotNodes = []*corev1.Node{}
			nodeClaims, nodes = test.NodeClaimsAndNodes(3, v1.NodeClaim{
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
			spotNodeClaims, spotNodes = test.NodeClaimsAndNodes(3, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveSpotInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			for i := range nodeClaims {
				nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				spotNodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
		})
		DescribeTable("can merge 3 nodes into 1", func(spotToSpot bool) {
			nodeClaims = lo.Ternary(spotToSpot, spotNodeClaims, nodeClaims)
			nodes = lo.Ternary(spotToSpot, spotNodes, nodes)
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					}}})

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)
			ExpectMakeNodesInitialized(ctx, env.Client, nodes[0], nodes[1], nodes[2])

			// bind pods to nodes
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0], nodeClaims[1], nodeClaims[2])

			// three nodeclaims should be replaced with a single nodeclaim
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2])
		},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		It("can merge 3 nodes into 1 if the candidates have both spot and on-demand", func() {
			// By default all the 3 nodeClaims are OD.
			nodeClaims = lo.Ternary(false, spotNodeClaims, nodeClaims)
			nodes = lo.Ternary(false, spotNodes, nodes)
			// Change one of them to spot.
			nodeClaims[2].Labels = lo.Assign(nodeClaims[2].Labels, map[string]string{
				corev1.LabelInstanceTypeStable: mostExpensiveSpotInstance.Name,
				v1.CapacityTypeLabelKey:        mostExpensiveSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpensiveSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
			nodes[2].Labels = lo.Assign(nodeClaims[2].Labels, map[string]string{
				corev1.LabelInstanceTypeStable: mostExpensiveSpotInstance.Name,
				v1.CapacityTypeLabelKey:        mostExpensiveSpotOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				corev1.LabelTopologyZone:       mostExpensiveSpotOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
			})
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
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
					}}})

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)
			ExpectMakeNodesInitialized(ctx, env.Client, nodes[0], nodes[1], nodes[2])

			// bind pods to nodes
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0], nodeClaims[1], nodeClaims[2])

			// three nodeclaims should be replaced with a single nodeclaim
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2])
		})
		DescribeTable("won't merge 2 nodes into 1 of the same type",
			func(spotToSpot bool) {
				leastExpInstance := lo.Ternary(spotToSpot, leastExpensiveInstance, leastExpensiveSpotInstance)
				leastExpOffering := lo.Ternary(spotToSpot, leastExpensiveOffering, leastExpensiveSpotOffering)
				nodeClaims = lo.Ternary(spotToSpot, nodeClaims, spotNodeClaims)
				nodes = lo.Ternary(spotToSpot, nodes, spotNodes)
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
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
						}}})

				// Make the nodeclaims the least expensive instance type and make them of the same type
				nodeClaims[0].Labels = lo.Assign(nodeClaims[0].Labels, map[string]string{
					corev1.LabelInstanceTypeStable: leastExpInstance.Name,
					v1.CapacityTypeLabelKey:        leastExpOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       leastExpOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				})
				nodes[0].Labels = lo.Assign(nodes[0].Labels, map[string]string{
					corev1.LabelInstanceTypeStable: leastExpInstance.Name,
					v1.CapacityTypeLabelKey:        leastExpOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       leastExpOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				})
				nodeClaims[1].Labels = lo.Assign(nodeClaims[1].Labels, map[string]string{
					corev1.LabelInstanceTypeStable: leastExpInstance.Name,
					v1.CapacityTypeLabelKey:        leastExpOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       leastExpOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				})
				nodes[1].Labels = lo.Assign(nodes[1].Labels, map[string]string{
					corev1.LabelInstanceTypeStable: leastExpInstance.Name,
					v1.CapacityTypeLabelKey:        leastExpOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       leastExpOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				})
				ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)
				ExpectMakeNodesInitialized(ctx, env.Client, nodes[0], nodes[1])

				// bind pods to nodes
				ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
				ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
				ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
				ExpectSingletonReconciled(ctx, disruptionController)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

				// We have [cheap-node, cheap-node] which multi-node consolidation could consolidate via
				// [delete cheap-node, delete cheap-node, launch cheap-node]. This isn't the best method though
				// as we should instead just delete one of the nodes instead of deleting both and launching a single
				// identical replacement. This test verifies the filterOutSameType function from multi-node consolidation
				// works to ensure we perform the least-disruptive action.
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
				// should have just deleted the node with the fewest pods
				ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0])
				// and left the other node alone
				ExpectExists(ctx, env.Client, nodeClaims[1])
				ExpectExists(ctx, env.Client, nodes[1])
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("should wait for the node TTL for non-empty nodes before consolidating (multi-node)",
			func(spotToSpot bool) {
				disruptionController = disruption.NewController(fakeClock, env.Client, prov, cloudProvider, recorder, cluster, queue, disruption.WithMethods(NewMethodsWithRealValidator()...))
				nodeClaims = lo.Ternary(spotToSpot, nodeClaims, spotNodeClaims)
				nodes = lo.Ternary(spotToSpot, nodes, spotNodes)
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
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
						}}})

				ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

				// bind pods to nodes
				ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
				ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
				ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})

				finished := atomic.Bool{}
				ExpectParallelized(
					func() {
						defer finished.Store(true)
						ExpectSingletonReconciled(ctx, disruptionController)
					},
					func() {
						// wait for the controller to block on the validation timeout
						Eventually(fakeClock.HasWaiters, time.Second*5).Should(BeTrue())
						// controller should be blocking during the timeout
						Expect(finished.Load()).To(BeFalse())
						// and the node should not be deleted yet
						ExpectExists(ctx, env.Client, nodeClaims[0])
						ExpectExists(ctx, env.Client, nodeClaims[1])

						// advance the clock so that the timeout expires
						fakeClock.Step(31 * time.Second)

						// controller should finish
						Eventually(finished.Load, 10*time.Second).Should(BeTrue())
					},
				)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0], nodeClaims[1])

				// should launch a single smaller replacement node
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
				// and delete the two large ones
				ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1])
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("should continue to multi-nodeclaim consolidation when emptiness fails validation after the node ttl",
			func(spotToSpot bool) {
				disruptionController = disruption.NewController(fakeClock, env.Client, prov, cloudProvider, recorder, cluster, queue, disruption.WithMethods(NewMethodsWithRealValidator()...))
				nodeClaims = lo.Ternary(spotToSpot, nodeClaims, spotNodeClaims)
				nodes = lo.Ternary(spotToSpot, nodes, spotNodes)
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
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
						}}})

				ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})

				finished := atomic.Bool{}
				ExpectParallelized(
					func() {
						defer finished.Store(true)
						ExpectSingletonReconciled(ctx, disruptionController)
					},
					func() {
						// wait for the controller to block on the validation timeout
						Eventually(fakeClock.HasWaiters, time.Second*5).Should(BeTrue())
						// controller should be blocking during the timeout
						Expect(finished.Load()).To(BeFalse())
						// and the node should not be deleted yet
						ExpectExists(ctx, env.Client, nodeClaims[0])
						ExpectExists(ctx, env.Client, nodeClaims[1])
						ExpectExists(ctx, env.Client, nodeClaims[2])

						// bind pods to nodes
						ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
						ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
						ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])

						ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[0]))
						ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[1]))
						ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[2]))
						// advance the clock so that the timeout expires for emptiness
						Eventually(fakeClock.HasWaiters, time.Second*5).Should(BeTrue())
						fakeClock.Step(31 * time.Second)

						// Succeed on multi node consolidation
						Eventually(fakeClock.HasWaiters, time.Second*5).Should(BeTrue())
						fakeClock.Step(31 * time.Second)
						//ExpectMakeNewNodeClaimsReady(ctx, env.Client, &wg, cluster, cloudProvider, 1)
						Eventually(finished.Load, 10*time.Second).Should(BeTrue())
					},
				)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0], nodeClaims[1], nodeClaims[2])

				// should have 2 nodes after multi nodeclaim consolidation deletes one
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
				// and delete node3 in single nodeclaim consolidation
				ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1], nodeClaims[2], nodes[2])
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
		DescribeTable("should continue to single nodeclaim consolidation when multi-nodeclaim consolidation fails validation after the node ttl",
			func(spotToSpot bool) {
				disruptionController = disruption.NewController(fakeClock, env.Client, prov, cloudProvider, recorder, cluster, queue, disruption.WithMethods(NewMethodsWithRealValidator()...))
				nodeClaims = lo.Ternary(spotToSpot, nodeClaims, spotNodeClaims)
				nodes = lo.Ternary(spotToSpot, nodes, spotNodes)
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
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
						}}})

				ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)

				// bind pods to nodes
				ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
				ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
				ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])

				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})

				var wg sync.WaitGroup
				wg.Add(1)
				finished := atomic.Bool{}
				ExpectParallelized(
					func() {
						defer finished.Store(true)
						ExpectSingletonReconciled(ctx, disruptionController)
					},
					func() {
						// wait for the controller to block on the validation timeout
						Eventually(fakeClock.HasWaiters, time.Second*5).Should(BeTrue())
						// controller should be blocking during the timeout
						Expect(finished.Load()).To(BeFalse())

						// and the node should not be deleted yet
						ExpectExists(ctx, env.Client, nodeClaims[0])
						ExpectExists(ctx, env.Client, nodeClaims[1])
						ExpectExists(ctx, env.Client, nodeClaims[2])

						var extraPods []*corev1.Pod
						for i := 0; i < 2; i++ {
							extraPods = append(extraPods, test.Pod(test.PodOptions{
								ResourceRequirements: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI)},
								},
							}))
						}
						ExpectApplied(ctx, env.Client, extraPods[0], extraPods[1])
						// bind the extra pods to node1 and node 2 to make the consolidation decision invalid
						// we bind to 2 nodes so we can deterministically expect that node3 is consolidated in
						// single nodeclaim consolidation
						ExpectManualBinding(ctx, env.Client, extraPods[0], nodes[0])
						ExpectManualBinding(ctx, env.Client, extraPods[1], nodes[1])

						ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[0]))
						ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[1]))

						// advance the clock so that the timeout expires for multi-nodeclaim consolidation
						fakeClock.Step(31 * time.Second)

						// wait for the controller to block on the validation timeout for single nodeclaim consolidation
						Eventually(fakeClock.HasWaiters, time.Second*5).Should(BeTrue())
						// advance the clock so that the timeout expires for single nodeclaim consolidation
						fakeClock.Step(31 * time.Second)

						// controller should finish
						Eventually(finished.Load, 10*time.Second).Should(BeTrue())
					},
				)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0], nodeClaims[1], nodeClaims[2])

				// should have 2 nodes after single nodeclaim consolidation deletes one
				Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
				Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
				// and delete node3 in single nodeclaim consolidation
				ExpectNotFound(ctx, env.Client, nodeClaims[2], nodes[2])
			},
			Entry("if the candidate is on-demand node", false),
			Entry("if the candidate is spot node", true),
		)
	})
	Context("Node Lifetime Consideration", func() {
		var nodeClaims []*v1.NodeClaim
		var nodes []*corev1.Node

		BeforeEach(func() {
			nodePool.Spec.Template.Spec.ExpireAfter = v1.MustParseNillableDuration("3s")
			nodeClaims, nodes = test.NodeClaimsAndNodes(2, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Spec: v1.NodeClaimSpec{
					ExpireAfter: v1.MustParseNillableDuration("3s"),
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
		})
		It("should consider node lifetime remaining when calculating disruption cost", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)

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
					}}})

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodePool)
			ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0]) // ensure node1 is the oldest node
			time.Sleep(2 * time.Second)                             // this sleep is unfortunate, but necessary.  The creation time is from etcd, and we can't mock it, so we
			// need to sleep to force the second node to be created a bit after the first node.
			ExpectApplied(ctx, env.Client, nodeClaims[1], nodes[1])

			// two pods on node 1, one on node 2
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			fakeClock.SetTime(time.Now())
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaims[0])

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[0])

			// the second node has more pods, so it would normally not be picked for consolidation, except it very little
			// lifetime remaining, so it should be deleted
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			ExpectNotFound(ctx, env.Client, nodeClaims[0], nodes[0])
		})
	})
	Context("Topology Consideration", func() {
		var nodeClaims []*v1.NodeClaim
		var nodes []*corev1.Node
		var oldNodeClaimNames sets.Set[string]

		BeforeEach(func() {
			testZone1Instance := leastExpensiveInstanceWithZone("test-zone-1")
			testZone2Instance := mostExpensiveInstanceWithZone("test-zone-2")
			testZone3Instance := leastExpensiveInstanceWithZone("test-zone-3")

			nodeClaims, nodes = test.NodeClaimsAndNodes(3, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelTopologyZone:       "test-zone-1",
						corev1.LabelInstanceTypeStable: testZone1Instance.Name,
						v1.CapacityTypeLabelKey:        testZone1Instance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("1")},
				},
			})
			nodeClaims[1].Labels = lo.Assign(nodeClaims[1].Labels, map[string]string{
				corev1.LabelTopologyZone:       "test-zone-2",
				corev1.LabelInstanceTypeStable: testZone2Instance.Name,
				v1.CapacityTypeLabelKey:        testZone2Instance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			})
			nodes[1].Labels = lo.Assign(nodes[1].Labels, map[string]string{
				corev1.LabelTopologyZone:       "test-zone-2",
				corev1.LabelInstanceTypeStable: testZone2Instance.Name,
				v1.CapacityTypeLabelKey:        testZone2Instance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			})

			nodeClaims[2].Labels = lo.Assign(nodeClaims[2].Labels, map[string]string{
				corev1.LabelTopologyZone:       "test-zone-3",
				corev1.LabelInstanceTypeStable: testZone3Instance.Name,
				v1.CapacityTypeLabelKey:        testZone3Instance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			})
			nodes[2].Labels = lo.Assign(nodes[2].Labels, map[string]string{
				corev1.LabelTopologyZone:       "test-zone-3",
				corev1.LabelInstanceTypeStable: testZone3Instance.Name,
				v1.CapacityTypeLabelKey:        testZone3Instance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			})
			oldNodeClaimNames = sets.New(nodeClaims[0].Name, nodeClaims[1].Name, nodeClaims[2].Name)
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
		})
		It("can replace node maintaining zonal topology spread", func() {
			labels = map[string]string{
				"app": "test-zonal-spread",
			}
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)

			tsc := corev1.TopologySpreadConstraint{
				MaxSkew:           1,
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
			}
			pods := test.Pods(4, test.PodOptions{
				ResourceRequirements:      corev1.ResourceRequirements{Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("1")}},
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{tsc},
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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

			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)

			// bind pods to nodes
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})

			ExpectSkew(ctx, env.Client, "default", &tsc).To(ConsistOf(1, 1, 1))
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[1])

			// should create a new node as there is a cheaper one that can hold the pod
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(3))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(3))
			ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1])

			// Find the new node associated with the nodeclaim
			newNodeClaim, ok := lo.Find(ExpectNodeClaims(ctx, env.Client), func(m *v1.NodeClaim) bool {
				return !oldNodeClaimNames.Has(m.Name)
			})
			Expect(ok).To(BeTrue())
			newNode, ok := lo.Find(ExpectNodes(ctx, env.Client), func(n *corev1.Node) bool {
				return newNodeClaim.Status.ProviderID == n.Spec.ProviderID
			})
			Expect(ok).To(BeTrue())

			// we need to emulate the replicaset controller and bind a new pod to the newly created node
			ExpectApplied(ctx, env.Client, pods[3])
			ExpectManualBinding(ctx, env.Client, pods[3], newNode)

			// we should maintain our skew, the new node must be in the same zone as the old node it replaced
			ExpectSkew(ctx, env.Client, "default", &tsc).To(ConsistOf(1, 1, 1))
		})
		It("won't delete node if it would violate pod anti-affinity", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			pods := test.Pods(3, test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("1")}},
				PodAntiRequirements: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
						TopologyKey:   corev1.LabelHostname,
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
					},
				},
			})

			// Make the Zone 2 instance also the least expensive instance
			zone2Instance := leastExpensiveInstanceWithZone("test-zone-2")
			nodes[1].Labels = lo.Assign(nodes[1].Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelTopologyZone:       "test-zone-2",
				corev1.LabelInstanceTypeStable: zone2Instance.Name,
				v1.CapacityTypeLabelKey:        zone2Instance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			})
			nodeClaims[1].Labels = lo.Assign(nodeClaims[1].Labels, map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelTopologyZone:       "test-zone-2",
				corev1.LabelInstanceTypeStable: zone2Instance.Name,
				v1.CapacityTypeLabelKey:        zone2Instance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
			})
			ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)

			// bind pods to nodes
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// our nodes are already the cheapest available, so we can't replace them.  If we delete, it would
			// violate the anti-affinity rule, so we can't do anything.
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(3))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(3))
			ExpectExists(ctx, env.Client, nodeClaims[0])
			ExpectExists(ctx, env.Client, nodeClaims[1])
			ExpectExists(ctx, env.Client, nodeClaims[2])
		})
	})
	Context("Parallelization", func() {
		It("should not schedule an additional node when receiving pending pods while consolidating", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)

			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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

			node.Finalizers = []string{"karpenter.sh/test-finalizer"}
			nodeClaim.Finalizers = []string{"karpenter.sh/test-finalizer"}

			ExpectApplied(ctx, env.Client, rs, pod, nodeClaim, node, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

			ExpectSingletonReconciled(ctx, disruptionController)
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])

			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))

			// Add a new pending pod that should schedule while node is not yet deleted
			pod = test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(2))
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should not consolidate a node that is launched for pods on a deleting node", func() {
			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)

			podOpts := test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			}

			var pods []*corev1.Pod
			for i := 0; i < 5; i++ {
				pod := test.UnschedulablePod(podOpts)
				pods = append(pods, pod)
			}
			ExpectApplied(ctx, env.Client, rs, nodePool)
			ExpectProvisionedNoBinding(ctx, env.Client, cluster, cloudProvider, prov, lo.Map(pods, func(p *corev1.Pod, _ int) *corev1.Pod { return p.DeepCopy() })...)

			nodeClaims := ExpectNodeClaims(ctx, env.Client)
			Expect(nodeClaims).To(HaveLen(1))
			nodes := ExpectNodes(ctx, env.Client)
			Expect(nodes).To(HaveLen(1))

			// Update cluster state with new node
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(nodes[0]))

			// Mark the node for deletion and re-trigger reconciliation
			oldNodeName := nodes[0].Name
			cluster.MarkForDeletion(nodes[0].Spec.ProviderID)
			ExpectProvisionedNoBinding(ctx, env.Client, cluster, cloudProvider, prov, lo.Map(pods, func(p *corev1.Pod, _ int) *corev1.Pod { return p.DeepCopy() })...)

			// Make sure that the cluster state is aware of the current node state
			nodes = ExpectNodes(ctx, env.Client)
			Expect(nodes).To(HaveLen(2))
			newNode, _ := lo.Find(nodes, func(n *corev1.Node) bool { return n.Name != oldNodeName })

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nil)

			// Wait for the nomination cache to expire
			time.Sleep(time.Second * 11)

			// Re-create the pods to re-bind them
			for i := 0; i < 2; i++ {
				ExpectDeleted(ctx, env.Client, pods[i])
				pod := test.UnschedulablePod(podOpts)
				ExpectApplied(ctx, env.Client, pod)
				ExpectManualBinding(ctx, env.Client, pod, newNode)
			}

			// Trigger a reconciliation run which should take into account the deleting node
			// consolidation shouldn't trigger additional actions
			result, err := disruptionController.Reconcile(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})
	})
	Context("Reserved Capacity", func() {
		var reservedNodeClaim *v1.NodeClaim
		var reservedNode *corev1.Node
		var mostExpensiveReservationID string

		BeforeEach(func() {
			mostExpensiveReservationID = fmt.Sprintf("r-%s", mostExpensiveInstance.Name)
			mostExpensiveInstance.Requirements.Add(scheduling.NewRequirement(
				cloudprovider.ReservationIDLabel,
				corev1.NodeSelectorOpIn,
				mostExpensiveReservationID,
			))
			mostExpensiveInstance.Requirements.Get(v1.CapacityTypeLabelKey).Insert(v1.CapacityTypeReserved)
			mostExpensiveInstance.Offerings = append(mostExpensiveInstance.Offerings, &cloudprovider.Offering{
				Price:               mostExpensiveOffering.Price / 1_000_000.0,
				Available:           true,
				ReservationCapacity: 10,
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					v1.CapacityTypeLabelKey:     v1.CapacityTypeReserved,
					corev1.LabelTopologyZone:    mostExpensiveOffering.Zone(),
					v1alpha1.LabelReservationID: mostExpensiveReservationID,
				}),
			})
			reservedNodeClaim, reservedNode = test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:              nodePool.Name,
						corev1.LabelInstanceTypeStable:   mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:          v1.CapacityTypeReserved,
						corev1.LabelTopologyZone:         mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
						cloudprovider.ReservationIDLabel: mostExpensiveReservationID,
					},
				},
			})
			reservedNodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{FeatureGates: test.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
		})
		It("can consolidate from one reserved offering to another", func() {
			leastExpensiveReservationID := fmt.Sprintf("r-%s", leastExpensiveInstance.Name)
			leastExpensiveInstance.Requirements.Add(scheduling.NewRequirement(
				cloudprovider.ReservationIDLabel,
				corev1.NodeSelectorOpIn,
				leastExpensiveReservationID,
			))
			leastExpensiveInstance.Requirements.Get(v1.CapacityTypeLabelKey).Insert(v1.CapacityTypeReserved)
			leastExpensiveInstance.Offerings = append(leastExpensiveInstance.Offerings, &cloudprovider.Offering{
				Price:               leastExpensiveOffering.Price / 1_000_000.0,
				Available:           true,
				ReservationCapacity: 10,
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					v1.CapacityTypeLabelKey:     v1.CapacityTypeReserved,
					corev1.LabelTopologyZone:    leastExpensiveOffering.Zone(),
					v1alpha1.LabelReservationID: leastExpensiveReservationID,
				}),
			})

			// create our RS so we can link a pod to it
			rs := test.ReplicaSet()
			ExpectApplied(ctx, env.Client, rs)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

			pod := test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
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
			}})
			ExpectApplied(ctx, env.Client, rs, pod, reservedNode, reservedNodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, reservedNode)

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{reservedNode}, []*v1.NodeClaim{reservedNodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, reservedNodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, reservedNodeClaim)

			// should create a new nodeclaim as there is a cheaper one that can hold the pod
			nodeClaims := ExpectNodeClaims(ctx, env.Client)
			nodes := ExpectNodes(ctx, env.Client)
			Expect(nodeClaims).To(HaveLen(1))
			Expect(nodes).To(HaveLen(1))

			Expect(nodeClaims[0].Name).ToNot(Equal(reservedNodeClaim.Name))

			// We should have consolidated into the same instance type, just into reserved.
			Expect(nodes[0].Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, leastExpensiveInstance.Name))
			Expect(nodes[0].Labels).To(HaveKeyWithValue(v1.CapacityTypeLabelKey, v1.CapacityTypeReserved))
			Expect(nodes[0].Labels).To(HaveKeyWithValue(cloudprovider.ReservationIDLabel, leastExpensiveReservationID))

			// and delete the old one
			ExpectNotFound(ctx, env.Client, reservedNodeClaim, reservedNode)
		})
		DescribeTable(
			"can consolidate into reserved capacity for the same instance pool",
			func(initialCapacityType string) {
				if initialCapacityType == v1.CapacityTypeSpot {
					nodeClaim = spotNodeClaim
					node = spotNode
				}

				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

				pod := test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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
				}})
				ExpectApplied(ctx, env.Client, rs, pod, node, nodeClaim, nodePool)

				// bind pods to node
				ExpectManualBinding(ctx, env.Client, pod, node)

				// inform cluster state about nodes and nodeClaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
				ExpectSingletonReconciled(ctx, disruptionController)

				// Process the item so that the nodes can be deleted.
				cmds := queue.GetCommands()
				Expect(cmds).To(HaveLen(1))
				ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
				ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

				// Cascade any deletion of the nodeclaim to the node
				ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

				// should create a new nodeclaim as there is a cheaper one that can hold the pod
				nodeClaims := ExpectNodeClaims(ctx, env.Client)
				nodes := ExpectNodes(ctx, env.Client)
				Expect(nodeClaims).To(HaveLen(1))
				Expect(nodes).To(HaveLen(1))

				Expect(nodeClaims[0].Name).ToNot(Equal(nodeClaim.Name))

				// We should have consolidated into the same instance type, just into reserved.
				Expect(nodes[0].Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, mostExpensiveInstance.Name))
				Expect(nodes[0].Labels).To(HaveKeyWithValue(v1.CapacityTypeLabelKey, v1.CapacityTypeReserved))
				Expect(nodes[0].Labels).To(HaveKeyWithValue(cloudprovider.ReservationIDLabel, mostExpensiveReservationID))

				// and delete the old one
				ExpectNotFound(ctx, env.Client, nodeClaim, node)
			},
			Entry("from on-demand", v1.CapacityTypeOnDemand),
			Entry("from spot", v1.CapacityTypeSpot),
		)
	})
	Context("Preferences", func() {
		It("should consolidate a node through deletion when ignoring preferences", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{PreferencePolicy: lo.ToPtr(options.PreferencePolicyIgnore)}))
			nodeClaims, nodes := test.NodeClaimsAndNodes(2, v1.NodeClaim{
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
						corev1.ResourceCPU:  resource.MustParse("5"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
			pods := test.Pods(3, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "foo"},
				},
				PodAntiPreferences: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 1,
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey: corev1.LabelHostname,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "foo",
								},
							},
						},
					},
				},
			})

			ExpectApplied(ctx, env.Client, pods[0], pods[1], pods[2], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
			ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectObjectReconciled(ctx, env.Client, queue, cmds[0].Candidates[0].NodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaims[1])

			// we don't need a new node, but we should evict everything off one of node2 which only has a single pod
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			// and delete the old one
			ExpectNotFound(ctx, env.Client, nodeClaims[1], nodes[1])
		})
		It("should consolidate a node through replacement when ignoring preferences", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{PreferencePolicy: lo.ToPtr(options.PreferencePolicyIgnore)}))
			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "foo"},
				},
				NodePreferences: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{mostExpensiveInstance.Name},
					},
				},
			})
			ExpectApplied(ctx, env.Client, pod, node, nodeClaim, nodePool)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pod, node)

			// inform cluster state about nodes and nodeClaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			ExpectSingletonReconciled(ctx, disruptionController)

			// Process the item so that the nodes can be deleted.
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(1))
			ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, cmds[0])
			ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

			// Cascade any deletion of the nodeclaim to the node
			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

			// should create a new nodeclaim as there is a cheaper one that can hold the pod
			nodeClaims := ExpectNodeClaims(ctx, env.Client)
			nodes := ExpectNodes(ctx, env.Client)
			Expect(nodeClaims).To(HaveLen(1))
			Expect(nodes).To(HaveLen(1))

			// Expect that the new nodeclaim does not request the most expensive instance type
			Expect(nodeClaims[0].Name).ToNot(Equal(nodeClaim.Name))
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Has(corev1.LabelInstanceTypeStable)).To(BeTrue())
			Expect(scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaims[0].Spec.Requirements...).Get(corev1.LabelInstanceTypeStable).Has(mostExpensiveInstance.Name)).To(BeFalse())

			// and delete the old one
			ExpectNotFound(ctx, env.Client, nodeClaim, node)
		})
	})
	Context("MinValuesPolicy", func() {
		var nodePoolWithMinValues *v1.NodePool

		BeforeEach(func() {
			// Create a nodepool with instance type minValues requirement
			nodePoolWithMinValues = test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Weight: lo.ToPtr(int32(100)),
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelInstanceTypeStable,
										Operator: corev1.NodeSelectorOpIn,
										Values: lo.Map(cloudProvider.InstanceTypes, func(it *cloudprovider.InstanceType, _ int) string {
											return it.Name
										}),
									},
									MinValues: lo.ToPtr(3),
								},
							},
						},
					},
					Disruption: v1.Disruption{
						ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						Budgets: []v1.Budget{{
							Nodes: "100%",
						}},
						ConsolidateAfter: v1.MustParseNillableDuration("0s"),
					},
				},
			})

			// Update instance types to ensure that min values won't be satisfied.
			cloudProvider.InstanceTypes = []*cloudprovider.InstanceType{leastExpensiveInstance, mostExpensiveInstance}
		})

		It("should not consolidate a node by relaxing min values when policy is set to BestEffort", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{MinValuesPolicy: lo.ToPtr(options.MinValuesPolicyBestEffort)}))
			nodeClaims, nodes := test.NodeClaimsAndNodes(1, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePoolWithMinValues.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveInstance.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveInstance.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("5"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}
			pods := test.Pods(1, test.PodOptions{})

			ExpectApplied(ctx, env.Client, pods[0], nodeClaims[0], nodes[0], nodePoolWithMinValues)

			// bind pods to node
			ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])

			// inform cluster state about nodes and nodeclaims
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0]}, []*v1.NodeClaim{nodeClaims[0]})
			result, err := disruptionController.Reconcile(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			// Validate that nothing changed.
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodeClaims(ctx, env.Client)[0].Name).To(Equal(nodeClaims[0].Name))
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(1))
			Expect(ExpectNodes(ctx, env.Client)[0].Name).To(Equal(nodes[0].Name))

			// Expect Unconsolidatable events to be fired for min values violation.
			Expect(lo.Filter(recorder.Events(), func(e events.Event, _ int) bool {
				return e.Reason == events.Unconsolidatable && strings.Contains(e.Message, "minValues requirement is not met for label(s) (label(s)=[node.kubernetes.io/instance-type])")
			})).To(HaveLen(2))
		})
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
			// This is not possible as we dont set ConditionTypeConsolidatable on static NodeClaims, incase user sets it
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}

			ExpectApplied(ctx, env.Client, staticNp, nodeClaims[0], nodeClaims[1], nodes[0], nodes[1])
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})

			ExpectSingletonReconciled(ctx, disruptionController)

			// Should not consolidate static NodeClaims
			cmds := queue.GetCommands()
			Expect(cmds).To(HaveLen(0))
		})
	})
})
