/*
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

package consolidation_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/test/pkg/debug"
	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *environmentaws.Environment

func TestConsolidation(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Consolidation")
}

var nodeClass *v1beta1.EC2NodeClass

var _ = BeforeEach(func() {
	nodeClass = env.DefaultEC2NodeClass()
	env.BeforeEach()
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Consolidation", func() {
	Context("Budgets", func() {
		var nodePool *corev1beta1.NodePool
		var dep *appsv1.Deployment
		var selector labels.Selector
		var numPods int32
		BeforeEach(func() {
			nodePool = env.DefaultNodePool(nodeClass)
			nodePool.Spec.Disruption.ConsolidateAfter = nil

			numPods = 5
			dep = test.Deployment(test.DeploymentOptions{
				Replicas: numPods,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "regular-app"},
					},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
				},
			})
			selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		})
		It("should respect budgets for empty delete consolidation", func() {
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{
				{
					Nodes: "40%",
				},
			}

			// Hostname anti-affinity to require one pod on each node
			dep.Spec.Template.Spec.Affinity = &v1.Affinity{
				PodAntiAffinity: &v1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
						{
							LabelSelector: dep.Spec.Selector,
							TopologyKey:   v1.LabelHostname,
						},
					},
				},
			}
			env.ExpectCreated(nodeClass, nodePool, dep)

			env.EventuallyExpectCreatedNodeClaimCount("==", 5)
			nodes := env.EventuallyExpectCreatedNodeCount("==", 5)
			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			By("adding finalizers to the nodes to prevent termination")
			for _, node := range nodes {
				Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				env.ExpectUpdated(node)
			}

			dep.Spec.Replicas = lo.ToPtr[int32](1)
			By("making the nodes empty")
			// Update the deployment to only contain 1 replica.
			env.ExpectUpdated(dep)

			// Ensure that we get two nodes tainted, and they have overlap during the drift
			env.EventuallyExpectTaintedNodeCount("==", 2)
			nodes = env.ConsistentlyExpectDisruptionsWithNodeCount(2, 5, 5*time.Second)

			// Remove the finalizer from each node so that we can terminate
			for _, node := range nodes {
				Expect(env.ExpectTestingFinalizerRemoved(node)).To(Succeed())
			}

			// After the deletion timestamp is set and all pods are drained
			// the node should be gone
			env.EventuallyExpectNotFound(nodes[0], nodes[1])

			// This check ensures that we are consolidating nodes at the same time
			env.EventuallyExpectTaintedNodeCount("==", 2)
			nodes = env.ConsistentlyExpectDisruptionsWithNodeCount(2, 3, 5*time.Second)

			for _, node := range nodes {
				Expect(env.ExpectTestingFinalizerRemoved(node)).To(Succeed())
			}
			env.EventuallyExpectNotFound(nodes[0], nodes[1])

			// Expect there to only be one node remaining for the last replica
			env.ExpectNodeCount("==", 1)
		})
		It("should respect budgets for non-empty delete consolidation", func() {
			// This test will hold consolidation until we are ready to execute it
			nodePool.Spec.Disruption.ConsolidateAfter = &corev1beta1.NillableDuration{}

			nodePool = test.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: v1beta1.LabelInstanceSize,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"2xlarge"},
					},
				},
			)
			// We're expecting to create 3 nodes, so we'll expect to see at most 2 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes: "50%",
			}}
			numPods = 9
			dep = test.Deployment(test.DeploymentOptions{
				Replicas: numPods,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "large-app"},
					},
					// Each 2xlarge has 8 cpu, so each node should fit no more than 3 pods.
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("2100m"),
						},
					},
				},
			})
			selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			env.ExpectCreated(nodeClass, nodePool, dep)

			env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			nodes := env.EventuallyExpectCreatedNodeCount("==", 3)
			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			By("scaling down the deployment")
			// Update the deployment to a third of the replicas.
			dep.Spec.Replicas = lo.ToPtr[int32](3)
			env.ExpectUpdated(dep)

			env.ForcePodsToSpread(nodes...)
			env.EventuallyExpectHealthyPodCount(selector, 3)

			By("cordoning and adding finalizer to the nodes")
			// Add a finalizer to each node so that we can stop termination disruptions
			for _, node := range nodes {
				Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				env.ExpectUpdated(node)
			}

			By("enabling consolidation")
			nodePool.Spec.Disruption.ConsolidateAfter = nil
			env.ExpectUpdated(nodePool)

			// Ensure that we get two nodes tainted, and they have overlap during consolidation
			env.EventuallyExpectTaintedNodeCount("==", 2)
			nodes = env.ConsistentlyExpectDisruptionsWithNodeCount(2, 3, 5*time.Second)

			for _, node := range nodes {
				Expect(env.ExpectTestingFinalizerRemoved(node)).To(Succeed())
			}
			env.EventuallyExpectNotFound(nodes[0], nodes[1])
			env.ExpectNodeCount("==", 1)
		})
		It("should respect budgets for non-empty replace consolidation", func() {
			appLabels := map[string]string{"app": "large-app"}
			// This test will hold consolidation until we are ready to execute it
			nodePool.Spec.Disruption.ConsolidateAfter = &corev1beta1.NillableDuration{}

			nodePool = test.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceSize,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"xlarge", "2xlarge"},
					},
				},
				// Add an Exists operator so that we can select on a fake partition later
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      "test-partition",
						Operator: v1.NodeSelectorOpExists,
					},
				},
			)
			nodePool.Labels = appLabels
			// We're expecting to create 5 nodes, so we'll expect to see at most 3 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes: "3",
			}}

			ds := test.DaemonSet(test.DaemonSetOptions{
				Selector: appLabels,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: appLabels,
					},
					// Each 2xlarge has 8 cpu, so each node should fit no more than 1 pod since each node will have.
					// an equivalently sized daemonset
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("3"),
						},
					},
				},
			})

			env.ExpectCreated(ds)

			// Make 5 pods all with different deployments and different test partitions, so that each pod can be put
			// on a separate node.
			selector = labels.SelectorFromSet(appLabels)
			numPods = 5
			deployments := make([]*appsv1.Deployment, numPods)
			for i := range lo.Range(int(numPods)) {
				deployments[i] = test.Deployment(test.DeploymentOptions{
					Replicas: 1,
					PodOptions: test.PodOptions{
						ObjectMeta: metav1.ObjectMeta{
							Labels: appLabels,
						},
						NodeSelector: map[string]string{"test-partition": fmt.Sprintf("%d", i)},
						// Each 2xlarge has 8 cpu, so each node should fit no more than 1 pod since each node will have.
						// an equivalently sized daemonset
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU: resource.MustParse("3"),
							},
						},
					},
				})
			}

			env.ExpectCreated(nodeClass, nodePool, deployments[0], deployments[1], deployments[2], deployments[3], deployments[4])

			originalNodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 5)
			originalNodes := env.EventuallyExpectCreatedNodeCount("==", 5)

			// Check that all daemonsets and deployment pods are online
			env.EventuallyExpectHealthyPodCount(selector, int(numPods)*2)

			By("cordoning and adding finalizer to the nodes")
			// Add a finalizer to each node so that we can stop termination disruptions
			for _, node := range originalNodes {
				Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				env.ExpectUpdated(node)
			}

			// Delete the daemonset so that the nodes can be consolidated to smaller size
			env.ExpectDeleted(ds)
			// Check that all daemonsets and deployment pods are online
			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			By("enabling consolidation")
			nodePool.Spec.Disruption.ConsolidateAfter = nil
			env.ExpectUpdated(nodePool)

			// Ensure that we get three nodes tainted, and they have overlap during the consolidation
			env.EventuallyExpectTaintedNodeCount("==", 3)
			env.EventuallyExpectNodeClaimCount("==", 8)
			env.EventuallyExpectNodeCount("==", 8)
			env.ConsistentlyExpectDisruptionsWithNodeCount(3, 8, 5*time.Second)

			for _, node := range originalNodes {
				Expect(env.ExpectTestingFinalizerRemoved(node)).To(Succeed())
			}

			// Eventually expect all the nodes to be rolled and completely removed
			// Since this completes the disruption operation, this also ensures that we aren't leaking nodes into subsequent
			// tests since nodeclaims that are actively replacing but haven't brought-up nodes yet can register nodes later
			env.EventuallyExpectNotFound(lo.Map(originalNodes, func(n *v1.Node, _ int) client.Object { return n })...)
			env.EventuallyExpectNotFound(lo.Map(originalNodeClaims, func(n *corev1beta1.NodeClaim, _ int) client.Object { return n })...)
			env.ExpectNodeClaimCount("==", 5)
			env.ExpectNodeCount("==", 5)
		})
		It("should not allow consolidation if the budget is fully blocking", func() {
			// We're going to define a budget that doesn't allow any consolidation to happen
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes: "0",
			}}

			// Hostname anti-affinity to require one pod on each node
			dep.Spec.Template.Spec.Affinity = &v1.Affinity{
				PodAntiAffinity: &v1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
						{
							LabelSelector: dep.Spec.Selector,
							TopologyKey:   v1.LabelHostname,
						},
					},
				},
			}
			env.ExpectCreated(nodeClass, nodePool, dep)

			env.EventuallyExpectCreatedNodeClaimCount("==", 5)
			env.EventuallyExpectCreatedNodeCount("==", 5)
			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			dep.Spec.Replicas = lo.ToPtr[int32](1)
			By("making the nodes empty")
			// Update the deployment to only contain 1 replica.
			env.ExpectUpdated(dep)

			env.ConsistentlyExpectNoDisruptions(5, time.Minute)
		})
		It("should not allow consolidation if the budget is fully blocking during a scheduled time", func() {
			// We're going to define a budget that doesn't allow any drift to happen
			// This is going to be on a schedule that only lasts 30 minutes, whose window starts 15 minutes before
			// the current time and extends 15 minutes past the current time
			// Times need to be in UTC since the karpenter containers were built in UTC time
			windowStart := time.Now().Add(-time.Minute * 15).UTC()
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes:    "0",
				Schedule: lo.ToPtr(fmt.Sprintf("%d %d * * *", windowStart.Minute(), windowStart.Hour())),
				Duration: &metav1.Duration{Duration: time.Minute * 30},
			}}

			// Hostname anti-affinity to require one pod on each node
			dep.Spec.Template.Spec.Affinity = &v1.Affinity{
				PodAntiAffinity: &v1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
						{
							LabelSelector: dep.Spec.Selector,
							TopologyKey:   v1.LabelHostname,
						},
					},
				},
			}
			env.ExpectCreated(nodeClass, nodePool, dep)

			env.EventuallyExpectCreatedNodeClaimCount("==", 5)
			env.EventuallyExpectCreatedNodeCount("==", 5)
			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			dep.Spec.Replicas = lo.ToPtr[int32](1)
			By("making the nodes empty")
			// Update the deployment to only contain 1 replica.
			env.ExpectUpdated(dep)

			env.ConsistentlyExpectNoDisruptions(5, time.Minute)
		})
	})
	DescribeTable("should consolidate nodes (delete)", Label(debug.NoWatch), Label(debug.NoEvents),
		func(spotToSpot bool) {
			nodePool := test.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Disruption: corev1beta1.Disruption{
						ConsolidationPolicy: corev1beta1.ConsolidationPolicyWhenUnderutilized,
						// Disable Consolidation until we're ready
						ConsolidateAfter: &corev1beta1.NillableDuration{},
					},
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      corev1beta1.CapacityTypeLabelKey,
										Operator: v1.NodeSelectorOpIn,
										Values:   lo.Ternary(spotToSpot, []string{corev1beta1.CapacityTypeSpot}, []string{corev1beta1.CapacityTypeOnDemand}),
									},
								},
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1beta1.LabelInstanceSize,
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"medium", "large", "xlarge"},
									},
								},
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1beta1.LabelInstanceFamily,
										Operator: v1.NodeSelectorOpNotIn,
										// remove some cheap burstable and the odd c1 instance types so we have
										// more control over what gets provisioned
										Values: []string{"t2", "t3", "c1", "t3a", "t4g"},
									},
								},
							},
							NodeClassRef: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
						},
					},
				},
			})

			var numPods int32 = 100
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: numPods,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "large-app"},
					},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
				},
			})

			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			env.ExpectCreatedNodeCount("==", 0)
			env.ExpectCreated(nodePool, nodeClass, dep)

			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			// reduce the number of pods by 60%
			dep.Spec.Replicas = aws.Int32(40)
			env.ExpectUpdated(dep)
			env.EventuallyExpectAvgUtilization(v1.ResourceCPU, "<", 0.5)

			// Enable consolidation as WhenUnderutilized doesn't allow a consolidateAfter value
			nodePool.Spec.Disruption.ConsolidateAfter = nil
			env.ExpectUpdated(nodePool)

			// With consolidation enabled, we now must delete nodes
			env.EventuallyExpectAvgUtilization(v1.ResourceCPU, ">", 0.6)

			env.ExpectDeleted(dep)
		},
		Entry("if the nodes are on-demand nodes", false),
		Entry("if the nodes are spot nodes", true),
	)
	DescribeTable("should consolidate nodes (replace)",
		func(spotToSpot bool) {
			nodePool := test.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Disruption: corev1beta1.Disruption{
						ConsolidationPolicy: corev1beta1.ConsolidationPolicyWhenUnderutilized,
						// Disable Consolidation until we're ready
						ConsolidateAfter: &corev1beta1.NillableDuration{},
					},
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      corev1beta1.CapacityTypeLabelKey,
										Operator: v1.NodeSelectorOpIn,
										Values:   lo.Ternary(spotToSpot, []string{corev1beta1.CapacityTypeSpot}, []string{corev1beta1.CapacityTypeOnDemand}),
									},
								},
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1beta1.LabelInstanceSize,
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"large", "2xlarge"},
									},
								},
							},
							NodeClassRef: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
						},
					},
				},
			})

			var numPods int32 = 3
			largeDep := test.Deployment(test.DeploymentOptions{
				Replicas: numPods,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "large-app"},
					},
					TopologySpreadConstraints: []v1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       v1.LabelHostname,
							WhenUnsatisfiable: v1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "large-app",
								},
							},
						},
					},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4")},
					},
				},
			})
			smallDep := test.Deployment(test.DeploymentOptions{
				Replicas: numPods,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "small-app"},
					},
					TopologySpreadConstraints: []v1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       v1.LabelHostname,
							WhenUnsatisfiable: v1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "small-app",
								},
							},
						},
					},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1.5")},
					},
				},
			})

			selector := labels.SelectorFromSet(largeDep.Spec.Selector.MatchLabels)
			env.ExpectCreatedNodeCount("==", 0)
			env.ExpectCreated(nodePool, nodeClass, largeDep, smallDep)

			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			// 3 nodes due to the anti-affinity rules
			env.ExpectCreatedNodeCount("==", 3)

			// scaling down the large deployment leaves only small pods on each node
			largeDep.Spec.Replicas = aws.Int32(0)
			env.ExpectUpdated(largeDep)
			env.EventuallyExpectAvgUtilization(v1.ResourceCPU, "<", 0.5)

			nodePool.Spec.Disruption.ConsolidateAfter = nil
			env.ExpectUpdated(nodePool)

			// With consolidation enabled, we now must replace each node in turn to consolidate due to the anti-affinity
			// rules on the smaller deployment.  The 2xl nodes should go to a large
			env.EventuallyExpectAvgUtilization(v1.ResourceCPU, ">", 0.8)

			var nodes v1.NodeList
			Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
			numLargeNodes := 0
			numOtherNodes := 0
			for _, n := range nodes.Items {
				// only count the nodes created by the provisoiner
				if n.Labels[corev1beta1.NodePoolLabelKey] != nodePool.Name {
					continue
				}
				if strings.HasSuffix(n.Labels[v1.LabelInstanceTypeStable], ".large") {
					numLargeNodes++
				} else {
					numOtherNodes++
				}
			}

			// all of the 2xlarge nodes should have been replaced with large instance types
			Expect(numLargeNodes).To(Equal(3))
			// and we should have no other nodes
			Expect(numOtherNodes).To(Equal(0))

			env.ExpectDeleted(largeDep, smallDep)
		},
		Entry("if the nodes are on-demand nodes", false),
		Entry("if the nodes are spot nodes", true),
	)
	It("should consolidate on-demand nodes to spot (replace)", func() {
		nodePool := test.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Disruption: corev1beta1.Disruption{
					ConsolidationPolicy: corev1beta1.ConsolidationPolicyWhenUnderutilized,
					// Disable Consolidation until we're ready
					ConsolidateAfter: &corev1beta1.NillableDuration{},
				},
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: v1.NodeSelectorRequirement{
									Key:      corev1beta1.CapacityTypeLabelKey,
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{corev1beta1.CapacityTypeOnDemand},
								},
							},
							{
								NodeSelectorRequirement: v1.NodeSelectorRequirement{
									Key:      v1beta1.LabelInstanceSize,
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"large"},
								},
							},
						},
						NodeClassRef: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
					},
				},
			},
		})

		var numPods int32 = 2
		smallDep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "small-app"},
				},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       v1.LabelHostname,
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "small-app",
							},
						},
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1.5")},
				},
			},
		})

		selector := labels.SelectorFromSet(smallDep.Spec.Selector.MatchLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(nodePool, nodeClass, smallDep)

		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.ExpectCreatedNodeCount("==", int(numPods))

		// Enable spot capacity type after the on-demand node is provisioned
		// Expect the node to consolidate to a spot instance as it will be a cheaper
		// instance than on-demand
		nodePool.Spec.Disruption.ConsolidateAfter = nil
		test.ReplaceRequirements(nodePool,
			corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      corev1beta1.CapacityTypeLabelKey,
					Operator: v1.NodeSelectorOpExists,
				},
			},
			corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1beta1.LabelInstanceSize,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"large"},
				},
			},
		)
		env.ExpectUpdated(nodePool)

		// Eventually expect the on-demand nodes to be consolidated into
		// spot nodes after some time
		Eventually(func(g Gomega) {
			var nodes v1.NodeList
			Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
			var spotNodes []*v1.Node
			var otherNodes []*v1.Node
			for i, n := range nodes.Items {
				// only count the nodes created by the nodePool
				if n.Labels[corev1beta1.NodePoolLabelKey] != nodePool.Name {
					continue
				}
				if n.Labels[corev1beta1.CapacityTypeLabelKey] == corev1beta1.CapacityTypeSpot {
					spotNodes = append(spotNodes, &nodes.Items[i])
				} else {
					otherNodes = append(otherNodes, &nodes.Items[i])
				}
			}
			// all the on-demand nodes should have been replaced with spot nodes
			msg := fmt.Sprintf("node names, spot= %v, other = %v", common.NodeNames(spotNodes), common.NodeNames(otherNodes))
			g.Expect(len(spotNodes)).To(BeNumerically("==", numPods), msg)
			// and we should have no other nodes
			g.Expect(len(otherNodes)).To(BeNumerically("==", 0), msg)
		}, time.Minute*10).Should(Succeed())

		env.ExpectDeleted(smallDep)
	})
})
