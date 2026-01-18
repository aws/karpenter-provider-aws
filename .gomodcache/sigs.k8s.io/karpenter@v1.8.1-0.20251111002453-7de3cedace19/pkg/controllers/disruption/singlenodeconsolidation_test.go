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
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/utils/pdb"
)

var nodePool1, nodePool2, nodePool3 *v1.NodePool
var consolidation *disruption.SingleNodeConsolidation
var nodePoolMap map[string]*v1.NodePool
var nodePoolInstanceTypeMap map[string]map[string]*cloudprovider.InstanceType

var _ = Describe("SingleNodeConsolidation", func() {
	BeforeEach(func() {
		nodePool1 = test.NodePool(v1.NodePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodepool-1",
			},
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
					ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
				},
			},
		})
		nodePool2 = test.NodePool(v1.NodePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodepool-2",
			},
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
					ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
				},
			},
		})
		nodePool3 = test.NodePool(v1.NodePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nodepool-3",
			},
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
					ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool1, nodePool2, nodePool3)

		// Set up NodePool maps for candidate creation
		nodePoolMap = map[string]*v1.NodePool{
			nodePool1.Name: nodePool1,
			nodePool2.Name: nodePool2,
			nodePool3.Name: nodePool3,
		}
		nodePoolInstanceTypeMap = map[string]map[string]*cloudprovider.InstanceType{
			nodePool1.Name: {leastExpensiveInstance.Name: leastExpensiveInstance},
			nodePool2.Name: {leastExpensiveInstance.Name: leastExpensiveInstance},
			nodePool3.Name: {leastExpensiveInstance.Name: leastExpensiveInstance},
		}

		// Create a single node consolidation controller
		c := disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue)
		consolidation = disruption.NewSingleNodeConsolidation(c)
	})

	AfterEach(func() {
		disruption.SingleNodeConsolidationTimeoutDuration = 3 * time.Minute
		fakeClock.SetTime(time.Now())
		ExpectCleanedUp(ctx, env.Client)
	})

	Context("Candidate Shuffling", func() {
		It("should sort candidates by disruption cost", func() {
			candidates, err := createCandidates(1.0, 3)
			Expect(err).To(BeNil())

			sortedCandidates := consolidation.SortCandidates(ctx, candidates)

			// Verify candidates are sorted by disruption cost
			Expect(sortedCandidates).To(HaveLen(9))
			for i := 0; i < len(sortedCandidates)-1; i++ {
				Expect(sortedCandidates[i].DisruptionCost).To(BeNumerically("<=", sortedCandidates[i+1].DisruptionCost))
			}
		})

		It("should prioritize nodepools that timed out in previous runs", func() {
			candidates, err := createCandidates(1.0, 3)
			Expect(err).To(BeNil())

			consolidation.PreviouslyUnseenNodePools.Insert(nodePool2.Name)
			sortedCandidates := consolidation.SortCandidates(ctx, candidates)

			Expect(sortedCandidates).To(HaveLen(9))
			Expect(sortedCandidates[0].NodePool.Name).To(Equal(nodePool2.Name))
		})

		It("should interweave candidates from different nodepools", func() {
			// Create candidates with different disruption costs
			// We'll create 3 sets of candidates with costs 1.0, 2.0, and 3.0
			// Use 1 node per nodepool to make the test more predictable
			candidates1, err := createCandidates(1.0, 1)
			Expect(err).To(BeNil())

			candidates2, err := createCandidates(2.0, 1)
			Expect(err).To(BeNil())

			candidates3, err := createCandidates(3.0, 1)
			Expect(err).To(BeNil())

			// Combine all candidates
			allCandidates := append(candidates3, append(candidates2, candidates1...)...)

			// Sort candidates
			sortedCandidates := consolidation.SortCandidates(ctx, allCandidates)

			// Verify candidates are interweaved from different nodepools
			// First we should have all candidates with disruption cost 1, then 2, then 3
			// Within each cost group, we should have one from each nodepool
			Expect(sortedCandidates).To(HaveLen(9))

			// Check first three candidates (all with cost 1)
			nodePoolsInFirstGroup := []string{
				sortedCandidates[0].NodePool.Name,
				sortedCandidates[1].NodePool.Name,
				sortedCandidates[2].NodePool.Name,
			}
			Expect(nodePoolsInFirstGroup).To(ConsistOf(nodePool1.Name, nodePool2.Name, nodePool3.Name))
			for i := 0; i < 3; i++ {
				Expect(sortedCandidates[i].DisruptionCost).To(Equal(1.0))
			}

			// Check next three candidates (all with cost 2)
			nodePoolsInSecondGroup := []string{
				sortedCandidates[3].NodePool.Name,
				sortedCandidates[4].NodePool.Name,
				sortedCandidates[5].NodePool.Name,
			}
			Expect(nodePoolsInSecondGroup).To(ConsistOf(nodePool1.Name, nodePool2.Name, nodePool3.Name))
			for i := 3; i < 6; i++ {
				Expect(sortedCandidates[i].DisruptionCost).To(Equal(2.0))
			}

			// Check last three candidates (all with cost 3)
			nodePoolsInThirdGroup := []string{
				sortedCandidates[6].NodePool.Name,
				sortedCandidates[7].NodePool.Name,
				sortedCandidates[8].NodePool.Name,
			}
			Expect(nodePoolsInThirdGroup).To(ConsistOf(nodePool1.Name, nodePool2.Name, nodePool3.Name))
			for i := 6; i < 9; i++ {
				Expect(sortedCandidates[i].DisruptionCost).To(Equal(3.0))
			}
		})

		It("should reset timed out nodepools when all nodepools are evaluated", func() {
			// Create candidates from different nodepools
			candidates, err := createCandidates(1.0, 1)
			Expect(err).To(BeNil())

			// Mark nodePool2 as timed out
			consolidation.PreviouslyUnseenNodePools.Insert(nodePool2.Name)
			// Create a budget mapping that allows all disruptions
			budgetMapping := map[string]int{
				nodePool1.Name: 1,
				nodePool2.Name: 1,
				nodePool3.Name: 1,
			}

			// Call ComputeCommand which should process all nodepools
			_, _ = consolidation.ComputeCommands(ctx, budgetMapping, candidates...)

			// Verify nodePool2 is no longer marked as timed out
			Expect(consolidation.PreviouslyUnseenNodePools.Has(nodePool2.Name)).To(BeFalse())
		})

		It("should mark nodepools as timed out when timeout occurs", func() {
			disruption.SingleNodeConsolidationTimeoutDuration = -5 * time.Second
			// Create many candidates to trigger timeout
			candidates, err := createCandidates(1.0, 10)
			Expect(err).To(BeNil())

			// Create a budget mapping that allows all disruptions
			budgetMapping := map[string]int{
				nodePool1.Name: 30,
				nodePool2.Name: 30,
				nodePool3.Name: 30,
			}

			_, _ = consolidation.ComputeCommands(ctx, budgetMapping, candidates...)

			// Verify all nodepools are marked as timed out
			// since we timed out before processing any candidates
			Expect(consolidation.PreviouslyUnseenNodePools.Has(nodePool1.Name)).To(BeTrue())
			Expect(consolidation.PreviouslyUnseenNodePools.Has(nodePool2.Name)).To(BeTrue())
			Expect(consolidation.PreviouslyUnseenNodePools.Has(nodePool3.Name)).To(BeTrue())
		})
	})
})

func createCandidates(disruptionCost float64, nodesPerNodePool ...int) ([]*disruption.Candidate, error) {
	// Default to 3 nodes per nodepool if not specified
	numNodesPerNodePool := 3
	if len(nodesPerNodePool) > 0 && nodesPerNodePool[0] > 0 {
		numNodesPerNodePool = nodesPerNodePool[0]
	}

	// Create NodeClaims for each NodePool
	nodeClaims := []*v1.NodeClaim{}

	// Create NodeClaims and Nodes for each NodePool
	for _, nodePool := range []*v1.NodePool{nodePool1, nodePool2, nodePool3} {
		for i := 0; i < numNodesPerNodePool; i++ {
			nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
				},
			})
			pod := test.Pod()
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
			ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			ExpectApplied(ctx, env.Client, nodeClaim)

			// Ensure the state is updated after all changes
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaim))

			nodeClaims = append(nodeClaims, nodeClaim)
		}
	}

	limits, err := pdb.NewLimits(ctx, env.Client)
	if err != nil {
		return nil, err
	}

	return lo.Map(nodeClaims, func(nodeClaim *v1.NodeClaim, _ int) *disruption.Candidate {
		stateNode := ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim)
		candidate, err := disruption.NewCandidate(
			ctx,
			env.Client,
			recorder,
			fakeClock,
			stateNode,
			limits,
			nodePoolMap,
			nodePoolInstanceTypeMap,
			queue,
			disruption.GracefulDisruptionClass,
		)
		if err != nil {
			return nil
		}
		candidate.DisruptionCost = disruptionCost
		return candidate
	}), nil
}
