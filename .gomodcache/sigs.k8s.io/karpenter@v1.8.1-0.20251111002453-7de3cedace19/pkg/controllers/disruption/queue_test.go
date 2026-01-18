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
	"strconv"
	"time"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption"
	disruptionevents "sigs.k8s.io/karpenter/pkg/controllers/disruption/events"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var nodeClaim1, nodeClaim2 *v1.NodeClaim
var nodePool *v1.NodePool
var node1, node2 *corev1.Node

var _ = Describe("Queue", func() {
	BeforeEach(func() {
		nodePool = test.NodePool()
		nodeClaim1, node1 = test.NodeClaimAndNode(
			v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
						v1.CapacityTypeLabelKey:        cloudProvider.InstanceTypes[0].Offerings.Cheapest().Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       cloudProvider.InstanceTypes[0].Offerings.Cheapest().Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID:  test.RandomProviderID(),
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
				},
			},
		)
		nodeClaim2, node2 = test.NodeClaimAndNode(
			v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
						v1.CapacityTypeLabelKey:        cloudProvider.InstanceTypes[0].Offerings.Cheapest().Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       cloudProvider.InstanceTypes[0].Offerings.Cheapest().Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID:  test.RandomProviderID(),
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
				},
			},
		)
		node1.Spec.Taints = append(node1.Spec.Taints, v1.DisruptedNoScheduleTaint)
		node2.Spec.Taints = append(node2.Spec.Taints, v1.DisruptedNoScheduleTaint)
	})
	Context("Reconcile", func() {
		It("should keep nodes tainted when replacements haven't finished initialization", func() {
			ExpectApplied(ctx, env.Client, nodeClaim1, node1, nodePool)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})

			nct := scheduling.NewNodeClaimTemplate(nodePool)
			nct.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			replacements := []*disruption.Replacement{
				{
					NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct},
				},
			}

			stateNode := ExpectStateNodeExists(cluster, node1)
			cmd := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode, NodePool: nodePool}},
				Replacements:      replacements,
			}
			Expect(queue.StartCommand(ctx, cmd)).To(BeNil())

			node1 = ExpectNodeExists(ctx, env.Client, node1.Name)
			Expect(node1.Spec.Taints).To(ContainElement(v1.DisruptedNoScheduleTaint))

			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)

			// Update state
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
			Expect(ExpectNodeClaims(ctx, env.Client)).To(HaveLen(2))
			node1 = ExpectNodeExists(ctx, env.Client, node1.Name)
			Expect(node1.Spec.Taints).To(ContainElement(v1.DisruptedNoScheduleTaint))
		})
		It("should not return an error when handling commands before the timeout", func() {
			ExpectApplied(ctx, env.Client, nodeClaim1, node1, nodePool)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})
			stateNode := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim1)

			nct := scheduling.NewNodeClaimTemplate(nodePool)
			nct.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			replacements := []*disruption.Replacement{
				{
					NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct},
				},
			}

			cmd := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode, NodePool: nodePool}},
				Replacements:      replacements,
			}
			Expect(queue.StartCommand(ctx, cmd)).To(BeNil())
			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			Expect(queue.HasAny(stateNode.ProviderID())).To(BeTrue()) // Expect the command to still be in the queue
		})
		It("should not return an error when the NodeClaim doesn't exist but the NodeCliam is in cluster state", func() {
			ExpectApplied(ctx, env.Client, nodeClaim1, node1, nodePool)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})
			stateNode := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim1)

			nct := scheduling.NewNodeClaimTemplate(nodePool)
			nct.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			replacements := []*disruption.Replacement{
				{
					NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct},
				},
			}

			cmd := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode, NodePool: nodePool}},
				Replacements:      replacements,
			}
			Expect(queue.StartCommand(ctx, cmd)).To(BeNil())

			replacementNodeClaim := &v1.NodeClaim{}
			Expect(env.Client.Get(ctx, types.NamespacedName{Name: cmd.Replacements[0].Name}, replacementNodeClaim))
			replacementNodeClaim, _ = ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, replacementNodeClaim)

			cluster.UpdateNodeClaim(replacementNodeClaim)
			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			Expect(queue.HasAny(stateNode.ProviderID())).To(BeTrue()) // Expect the command to still be in the queue
		})
		It("should untaint nodes when a command times out", func() {
			ExpectApplied(ctx, env.Client, nodeClaim1, node1, nodePool)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})
			stateNode := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim1)

			nct := scheduling.NewNodeClaimTemplate(nodePool)
			nct.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			replacements := []*disruption.Replacement{
				{
					NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct},
				},
			}

			cmd := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode, NodePool: nodePool}},
				Replacements:      replacements,
			}
			Expect(queue.StartCommand(ctx, cmd)).To(BeNil())

			// Step the clock to trigger the timeout.
			fakeClock.Step(11 * time.Minute)

			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			node1 = ExpectNodeExists(ctx, env.Client, node1.Name)
			Expect(node1.Spec.Taints).ToNot(ContainElement(v1.DisruptedNoScheduleTaint))
		})
		It("should fully handle a command when replacements are initialized", func() {
			ExpectApplied(ctx, env.Client, nodeClaim1, node1, nodePool)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})
			stateNode := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim1)

			nct := scheduling.NewNodeClaimTemplate(nodePool)
			nct.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			replacements := []*disruption.Replacement{
				{
					NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct},
				},
			}

			cmd := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode, NodePool: nodePool}},
				Replacements:      replacements,
			}
			Expect(queue.StartCommand(ctx, cmd)).To(BeNil())

			replacementNodeClaim := &v1.NodeClaim{}
			Expect(env.Client.Get(ctx, types.NamespacedName{Name: cmd.Replacements[0].Name}, replacementNodeClaim))
			replacementNodeClaim, replacementNode := ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, replacementNodeClaim)

			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			// Get the command
			Expect(cmd.Replacements[0].Initialized).To(BeFalse())

			Expect(recorder.DetectedEvent(disruptionevents.Launching(replacementNodeClaim, string(cmd.Reason())).Message)).To(BeTrue())
			Expect(recorder.DetectedEvent(disruptionevents.WaitingOnReadiness(replacementNodeClaim).Message)).To(BeTrue())

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController,
				[]*corev1.Node{replacementNode}, []*v1.NodeClaim{replacementNodeClaim})

			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			Expect(cmd.Replacements[0].Initialized).To(BeTrue())

			terminatingEvents := disruptionevents.Terminating(node1, nodeClaim1, string(cmd.Reason()))
			Expect(recorder.DetectedEvent(terminatingEvents[0].Message)).To(BeTrue())
			Expect(recorder.DetectedEvent(terminatingEvents[1].Message)).To(BeTrue())

			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim1)
			// And expect the nodeClaim and node to be deleted
			ExpectNotFound(ctx, env.Client, nodeClaim1, node1)
		})
		It("should only finish a command when all replacements are initialized", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim1, node1)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})
			stateNode := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim1)

			nct := scheduling.NewNodeClaimTemplate(nodePool)
			nct.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			nct2 := scheduling.NewNodeClaimTemplate(nodePool)
			nct2.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			replacements := []*disruption.Replacement{
				{
					NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct},
				},
				{
					NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct2},
				},
			}

			cmd := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode, NodePool: nodePool}},
				Replacements:      replacements,
			}
			Expect(queue.StartCommand(ctx, cmd)).To(BeNil())

			replacementNodeClaim1 := &v1.NodeClaim{}
			Expect(env.Client.Get(ctx, types.NamespacedName{Name: cmd.Replacements[0].Name}, replacementNodeClaim1))
			replacementNodeClaim1, replacementNode1 := ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, replacementNodeClaim1)
			replacementNodeClaim2 := &v1.NodeClaim{}
			Expect(env.Client.Get(ctx, types.NamespacedName{Name: cmd.Replacements[1].Name}, replacementNodeClaim2))
			replacementNodeClaim2, replacementNode2 := ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, replacementNodeClaim2)

			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			Expect(cmd.Replacements[0].Initialized).To(BeFalse())
			Expect(recorder.DetectedEvent(disruptionevents.WaitingOnReadiness(nodeClaim1).Message)).To(BeTrue())
			Expect(cmd.Replacements[1].Initialized).To(BeFalse())

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{replacementNode1}, []*v1.NodeClaim{replacementNodeClaim1})

			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			Expect(cmd.Replacements[0].Initialized).To(BeTrue())
			Expect(cmd.Replacements[1].Initialized).To(BeFalse())
			Expect(recorder.DetectedEvent(disruptionevents.WaitingOnReadiness(nodeClaim1).Message)).To(BeTrue())

			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{replacementNode2}, []*v1.NodeClaim{replacementNodeClaim2})

			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			Expect(cmd.Replacements[0].Initialized).To(BeTrue())
			Expect(cmd.Replacements[1].Initialized).To(BeTrue())

			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim1)
			// And expect the nodeClaim and node to be deleted
			ExpectNotFound(ctx, env.Client, nodeClaim1, node1)
		})
		It("should not wait for replacements when none are needed", func() {
			ExpectApplied(ctx, env.Client, nodeClaim1, node1, nodePool)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node1}, []*v1.NodeClaim{nodeClaim1})
			stateNode := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim1)

			cmd := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode, NodePool: nodePool}},
				Replacements:      nil,
			}
			Expect(queue.StartCommand(ctx, cmd)).To(BeNil())

			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)

			terminatingEvents := disruptionevents.Terminating(node1, nodeClaim1, string(cmd.Reason()))
			Expect(recorder.DetectedEvent(terminatingEvents[0].Message)).To(BeTrue())
			Expect(recorder.DetectedEvent(terminatingEvents[1].Message)).To(BeTrue())

			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim1)
			// And expect the nodeClaim and node to be deleted
			ExpectNotFound(ctx, env.Client, nodeClaim1, node1)
		})
		It("should finish two commands in order as replacements are intialized", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim1, node1, nodeClaim2, node2)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node1, node2}, []*v1.NodeClaim{nodeClaim1, nodeClaim2})
			stateNode := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim1)
			stateNode2 := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim2)

			nct := scheduling.NewNodeClaimTemplate(nodePool)
			nct.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			replacements := []*disruption.Replacement{{
				NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct},
			}}
			nct2 := scheduling.NewNodeClaimTemplate(nodePool)
			nct2.InstanceTypeOptions = append([]*cloudprovider.InstanceType{}, cloudProvider.InstanceTypes...)
			replacements2 := []*disruption.Replacement{{
				NodeClaim: &scheduling.NodeClaim{NodeClaimTemplate: *nct2},
			}}

			cmd := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode, NodePool: nodePool}},
				Replacements:      replacements,
			}
			Expect(queue.StartCommand(ctx, cmd)).To(BeNil())
			cmd2 := &disruption.Command{
				Method:            disruption.NewDrift(env.Client, cluster, prov, recorder),
				CreationTimestamp: fakeClock.Now(),
				ID:                uuid.New(),
				Results:           scheduling.Results{},
				Candidates:        []*disruption.Candidate{{StateNode: stateNode2, NodePool: nodePool}},
				Replacements:      replacements2,
			}
			Expect(queue.StartCommand(ctx, cmd2)).To(BeNil())

			replacementNodeClaim1 := &v1.NodeClaim{}
			Expect(env.Client.Get(ctx, types.NamespacedName{Name: cmd.Replacements[0].Name}, replacementNodeClaim1))
			replacementNodeClaim2 := &v1.NodeClaim{}
			Expect(env.Client.Get(ctx, types.NamespacedName{Name: cmd2.Replacements[0].Name}, replacementNodeClaim2))

			replacementNodeClaim1, replacementNode1 := ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, replacementNodeClaim1)
			replacementNodeClaim2, replacementNode2 := ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, replacementNodeClaim2)

			// Reconcile the first command and expect nothing to be initialized
			ExpectObjectReconciled(ctx, env.Client, queue, stateNode.NodeClaim)
			Expect(cmd.Replacements[0].Initialized).To(BeFalse())
			Expect(recorder.DetectedEvent(disruptionevents.WaitingOnReadiness(nodeClaim1).Message)).To(BeTrue())
			Expect(cmd2.Replacements[0].Initialized).To(BeFalse())
			Expect(recorder.DetectedEvent(disruptionevents.WaitingOnReadiness(nodeClaim2).Message)).To(BeTrue())

			// Make the first command's node initialized
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{replacementNode1}, []*v1.NodeClaim{replacementNodeClaim1})
			// Reconcile the second command and expect nothing to be initialized
			ExpectObjectReconciled(ctx, env.Client, queue, cmd2.Candidates[0].NodeClaim)
			Expect(cmd.Replacements[0].Initialized).To(BeFalse())
			Expect(recorder.DetectedEvent(disruptionevents.WaitingOnReadiness(nodeClaim1).Message)).To(BeTrue())
			Expect(cmd2.Replacements[0].Initialized).To(BeFalse())
			Expect(recorder.DetectedEvent(disruptionevents.WaitingOnReadiness(nodeClaim2).Message)).To(BeTrue())

			// Reconcile the first command and expect the replacement to be initialized
			ExpectObjectReconciled(ctx, env.Client, queue, cmd.Candidates[0].NodeClaim)
			Expect(cmd.Replacements[0].Initialized).To(BeTrue())
			Expect(cmd2.Replacements[0].Initialized).To(BeFalse())

			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim1)
			ExpectNotFound(ctx, env.Client, nodeClaim1, node1)

			// Make the second command's node initialized
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{replacementNode2}, []*v1.NodeClaim{replacementNodeClaim2})

			// Reconcile the second command and expect the replacement to be initialized
			ExpectObjectReconciled(ctx, env.Client, queue, cmd2.Candidates[0].NodeClaim)
			Expect(cmd.Replacements[0].Initialized).To(BeTrue())
			Expect(cmd2.Replacements[0].Initialized).To(BeTrue())

			ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim2)
			// And expect the nodeClaim and node to be deleted
			ExpectNotFound(ctx, env.Client, nodeClaim2, node2)
		})
		Context("CalculateRetryDuration", func() {
			DescribeTable("should calculate correct timeout based on queue length",
				func(numCommands int, expectedDuration time.Duration) {
					q := disruption.NewQueue(env.Client, recorder, cluster, fakeClock, prov)
					q.Lock()
					for i := range numCommands {
						q.ProviderIDToCommand[strconv.Itoa(i)] = &disruption.Command{}
					}
					q.Unlock()
					actualDuration := q.GetMaxRetryDuration()
					Expect(actualDuration).To(Equal(expectedDuration))
				},
				Entry("very small queue - 100 commands", 100, 10*time.Minute),                  // max(100*80ms, 10min) = 10min
				Entry("small queue - 4000 commands", 4000, 10*time.Minute),                     // max(4000*80ms, 10min) = 10min
				Entry("medium queue - 10000 commands", 10000, 13*time.Minute+20*time.Second),   // 10000*80ms = 13min 20sec
				Entry("large queue - 40000 commands", 40000, 53*time.Minute+20*time.Second),    // 40000*80ms = 53min 20sec
				Entry("very large queue - 80000 commands (capped)", 80000, 1*time.Hour),        // min(80000*80ms, 1hr) = 1hr
				Entry("extremely large queue - 100000 commands (capped)", 100000, 1*time.Hour), // min(100000*80ms, 1hr) = 1hr
			)
		})
	})
})
