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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/apis/v1alpha1"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

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

var nodeClass *v1.EC2NodeClass

var _ = BeforeEach(func() {
	nodeClass = env.DefaultEC2NodeClass()
	env.BeforeEach()
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = DescribeTableSubtree("Consolidation", Ordered, func(minValuesPolicy options.MinValuesPolicy) {
	BeforeEach(func() {
		env.ExpectSettingsOverridden(corev1.EnvVar{Name: "MIN_VALUES_POLICY", Value: string(minValuesPolicy)})
	})
	Context("LastPodEventTime", func() {
		var nodePool *karpv1.NodePool
		BeforeEach(func() {
			nodePool = env.DefaultNodePool(nodeClass)
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("Never")

		})
		It("should update lastPodEventTime when pods are scheduled and removed", func() {
			var numPods int32 = 5
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: numPods,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "regular-app"},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
					},
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{
				{
					Nodes: "0%",
				},
			}
			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 1)
			env.EventuallyExpectCreatedNodeCount("==", 1)
			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			nodeClaim := env.ExpectExists(nodeClaims[0]).(*karpv1.NodeClaim)
			lastPodEventTime := nodeClaim.Status.LastPodEventTime

			// wait 10 seconds so that we don't run into the de-dupe timeout
			time.Sleep(10 * time.Second)

			dep.Spec.Replicas = lo.ToPtr[int32](4)
			By("removing one pod from the node")
			env.ExpectUpdated(dep)

			Eventually(func(g Gomega) {
				nodeClaim = env.ExpectExists(nodeClaim).(*karpv1.NodeClaim)
				g.Expect(nodeClaim.Status.LastPodEventTime.Time).ToNot(BeEquivalentTo(lastPodEventTime.Time))
			}).WithTimeout(5 * time.Second).WithPolling(1 * time.Second).Should(Succeed())
			lastPodEventTime = nodeClaim.Status.LastPodEventTime

			// wait 10 seconds so that we don't run into the de-dupe timeout
			time.Sleep(10 * time.Second)

			dep.Spec.Replicas = lo.ToPtr[int32](5)
			By("adding one pod to the node")
			env.ExpectUpdated(dep)

			Eventually(func(g Gomega) {
				nodeClaim = env.ExpectExists(nodeClaim).(*karpv1.NodeClaim)
				g.Expect(nodeClaim.Status.LastPodEventTime.Time).ToNot(BeEquivalentTo(lastPodEventTime.Time))
			}).WithTimeout(5 * time.Second).WithPolling(1 * time.Second).Should(Succeed())
		})
		It("should update lastPodEventTime when pods go terminal", func() {
			podLabels := map[string]string{"app": "regular-app"}
			pod := coretest.Pod(coretest.PodOptions{
				// use a non-pause image so that we can have a sleep
				Image:   "alpine:3.20.2",
				Command: []string{"/bin/sh", "-c", "sleep 30"},
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			})
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      coretest.RandomName(),
					Namespace: "default",
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: pod.ObjectMeta,
						Spec:       pod.Spec,
					},
				},
			}
			selector := labels.SelectorFromSet(podLabels)
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{
				{
					Nodes: "0%",
				},
			}
			env.ExpectCreated(nodeClass, nodePool, job)

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 1)
			env.EventuallyExpectCreatedNodeCount("==", 1)
			pods := env.EventuallyExpectHealthyPodCount(selector, int(1))

			// pods are healthy, which means the job has started its 30s sleep
			nodeClaim := env.ExpectExists(nodeClaims[0]).(*karpv1.NodeClaim)
			lastPodEventTime := nodeClaim.Status.LastPodEventTime

			// wait a minute for the pod's sleep to finish, and for the nodeclaim to update
			Eventually(func(g Gomega) {
				pod := env.ExpectExists(pods[0]).(*corev1.Pod)
				g.Expect(pod.Status.Phase).To(Equal(corev1.PodSucceeded))
			}).WithTimeout(1 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

			nodeClaim = env.ExpectExists(nodeClaims[0]).(*karpv1.NodeClaim)
			Expect(nodeClaim.Status.LastPodEventTime).ToNot(BeEquivalentTo(lastPodEventTime.Time))
		})

	})
	Context("Budgets", func() {
		var nodePool *karpv1.NodePool
		var dep *appsv1.Deployment
		var selector labels.Selector
		var numPods int32
		BeforeEach(func() {
			nodePool = env.DefaultNodePool(nodeClass)
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")

			numPods = 5
			dep = coretest.Deployment(coretest.DeploymentOptions{
				Replicas: numPods,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "regular-app"},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
					},
				},
			})
			selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		})
		It("should respect budgets for empty delete consolidation", func() {
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{
				{
					Nodes: "40%",
				},
			}

			// Hostname anti-affinity to require one pod on each node
			dep.Spec.Template.Spec.Affinity = &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: dep.Spec.Selector,
							TopologyKey:   corev1.LabelHostname,
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

			env.ConsistentlyExpectDisruptionsUntilNoneLeft(5, 2, 10*time.Minute)
		})
		It("should respect budgets for non-empty delete consolidation", func() {
			// This test will hold consolidation until we are ready to execute it
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("Never")

			nodePool = coretest.ReplaceRequirements(nodePool,
				karpv1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.LabelInstanceSize,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"2xlarge"},
					},
				},
			)
			// We're expecting to create 3 nodes, so we'll expect to see at most 2 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
				Nodes: "50%",
			}}
			numPods = 9
			dep = coretest.Deployment(coretest.DeploymentOptions{
				Replicas: numPods,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "large-app"},
					},
					// Each 2xlarge has 8 cpu, so each node should fit no more than 3 pods.
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2100m"),
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
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
			env.ExpectUpdated(nodePool)

			env.ConsistentlyExpectDisruptionsUntilNoneLeft(3, 2, 10*time.Minute)
		})
		It("should respect budgets for non-empty replace consolidation", func() {
			appLabels := map[string]string{"app": "large-app"}
			// This test will hold consolidation until we are ready to execute it
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("Never")

			nodePool = coretest.ReplaceRequirements(nodePool,
				karpv1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      v1.LabelInstanceSize,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"xlarge", "2xlarge"},
					},
				},
				// Add an Exists operator so that we can select on a fake partition later
				karpv1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      "test-partition",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			)
			nodePool.Labels = appLabels
			// We're expecting to create 5 nodes, so we'll expect to see at most 3 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
				Nodes: "3",
			}}

			ds := coretest.DaemonSet(coretest.DaemonSetOptions{
				Selector: appLabels,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: appLabels,
					},
					// Each 2xlarge has 8 cpu, so each node should fit no more than 1 pod since each node will have.
					// an equivalently sized daemonset
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("3"),
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
				deployments[i] = coretest.Deployment(coretest.DeploymentOptions{
					Replicas: 1,
					PodOptions: coretest.PodOptions{
						ObjectMeta: metav1.ObjectMeta{
							Labels: appLabels,
						},
						NodeSelector: map[string]string{"test-partition": fmt.Sprintf("%d", i)},
						// Each 2xlarge has 8 cpu, so each node should fit no more than 1 pod since each node will have.
						// an equivalently sized daemonset
						ResourceRequirements: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("3"),
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
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
			env.ExpectUpdated(nodePool)

			// Ensure that we get three nodes tainted, and they have overlap during the consolidation
			env.EventuallyExpectTaintedNodeCount("==", 3)
			env.EventuallyExpectLaunchedNodeClaimCount("==", 8)
			env.EventuallyExpectNodeCount("==", 8)

			env.ConsistentlyExpectDisruptionsUntilNoneLeft(5, 3, 10*time.Minute)

			for _, node := range originalNodes {
				Expect(env.ExpectTestingFinalizerRemoved(node)).To(Succeed())
			}
			for _, nodeClaim := range originalNodeClaims {
				Expect(env.ExpectTestingFinalizerRemoved(nodeClaim)).To(Succeed())
			}
			// Eventually expect all the nodes to be rolled and completely removed
			// Since this completes the disruption operation, this also ensures that we aren't leaking nodes into subsequent
			// tests since nodeclaims that are actively replacing but haven't brought-up nodes yet can register nodes later
			env.EventuallyExpectNotFound(lo.Map(originalNodes, func(n *corev1.Node, _ int) client.Object { return n })...)
			env.EventuallyExpectNotFound(lo.Map(originalNodeClaims, func(n *karpv1.NodeClaim, _ int) client.Object { return n })...)
			env.ExpectNodeClaimCount("==", 5)
			env.ExpectNodeCount("==", 5)
		})
		It("should not allow consolidation if the budget is fully blocking", func() {
			// We're going to define a budget that doesn't allow any consolidation to happen
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
				Nodes: "0",
			}}

			// Hostname anti-affinity to require one pod on each node
			dep.Spec.Template.Spec.Affinity = &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: dep.Spec.Selector,
							TopologyKey:   corev1.LabelHostname,
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
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
				Nodes:    "0",
				Schedule: lo.ToPtr(fmt.Sprintf("%d %d * * *", windowStart.Minute(), windowStart.Hour())),
				Duration: &metav1.Duration{Duration: time.Minute * 30},
			}}

			// Hostname anti-affinity to require one pod on each node
			dep.Spec.Template.Spec.Affinity = &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: dep.Spec.Selector,
							TopologyKey:   corev1.LabelHostname,
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
			nodePool := coretest.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
					Disruption: karpv1.Disruption{
						ConsolidationPolicy: karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						// Disable Consolidation until we're ready
						ConsolidateAfter: karpv1.MustParseNillableDuration("Never"),
					},
					Template: karpv1.NodeClaimTemplate{
						Spec: karpv1.NodeClaimTemplateSpec{
							Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      karpv1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   lo.Ternary(spotToSpot, []string{karpv1.CapacityTypeSpot}, []string{karpv1.CapacityTypeOnDemand}),
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceSize,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"medium", "large", "xlarge"},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceFamily,
										Operator: corev1.NodeSelectorOpNotIn,
										// remove some cheap burstable and the odd c1 instance types so we have
										// more control over what gets provisioned
										// TODO: jmdeal@ remove a1 from exclusion list once Karpenter implicitly filters a1 instances for AL2023 AMI family (incompatible)
										Values: []string{"t2", "t3", "c1", "t3a", "t4g", "a1"},
									},
								},
							},
							NodeClassRef: &karpv1.NodeClassReference{
								Group: object.GVK(nodeClass).Group,
								Kind:  object.GVK(nodeClass).Kind,
								Name:  nodeClass.Name,
							},
						},
					},
				},
			})

			var numPods int32 = 100
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: numPods,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "large-app"},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
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
			env.EventuallyExpectAvgUtilization(corev1.ResourceCPU, "<", 0.5)

			// Enable consolidation as WhenEmptyOrUnderutilized doesn't allow a consolidateAfter value
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
			env.ExpectUpdated(nodePool)

			// With consolidation enabled, we now must delete nodes
			env.EventuallyExpectAvgUtilization(corev1.ResourceCPU, ">", 0.6)

			env.ExpectDeleted(dep)
		},
		Entry("if the nodes are on-demand nodes", false),
		Entry("if the nodes are spot nodes", true),
	)
	DescribeTable("should consolidate nodes (replace)",
		func(spotToSpot bool) {
			nodePool := coretest.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
					Disruption: karpv1.Disruption{
						ConsolidationPolicy: karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						// Disable Consolidation until we're ready
						ConsolidateAfter: karpv1.MustParseNillableDuration("Never"),
					},
					Template: karpv1.NodeClaimTemplate{
						Spec: karpv1.NodeClaimTemplateSpec{
							Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      karpv1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   lo.Ternary(spotToSpot, []string{karpv1.CapacityTypeSpot}, []string{karpv1.CapacityTypeOnDemand}),
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceSize,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"large", "2xlarge"},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceFamily,
										Operator: corev1.NodeSelectorOpNotIn,
										// remove some cheap burstable and the odd c1 / a1 instance types so we have
										// more control over what gets provisioned
										Values: []string{"t2", "t3", "c1", "t3a", "t4g", "a1"},
									},
								},
								// Specify Linux in the NodePool to filter out Windows only DS when discovering DS overhead
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelOSStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{string(corev1.Linux)},
									},
								},
							},
							NodeClassRef: &karpv1.NodeClassReference{
								Group: object.GVK(nodeClass).Group,
								Kind:  object.GVK(nodeClass).Kind,
								Name:  nodeClass.Name,
							},
						},
					},
				},
			})

			var numPods int32 = 3
			largeDep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: numPods,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "large-app"},
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       corev1.LabelHostname,
							WhenUnsatisfiable: corev1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "large-app",
								},
							},
						},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")},
					},
				},
			})
			smallDep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: numPods,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "small-app"},
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       corev1.LabelHostname,
							WhenUnsatisfiable: corev1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "small-app",
								},
							},
						},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: func() resource.Quantity {
								dsOverhead := env.GetDaemonSetOverhead(nodePool)
								base := lo.ToPtr(resource.MustParse("1800m"))
								base.Sub(*dsOverhead.Cpu())
								return *base
							}(),
						},
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
			env.EventuallyExpectAvgUtilization(corev1.ResourceCPU, "<", 0.5)

			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
			env.ExpectUpdated(nodePool)

			// With consolidation enabled, we now must replace each node in turn to consolidate due to the anti-affinity
			// rules on the smaller deployment.  The 2xl nodes should go to a large
			env.EventuallyExpectAvgUtilization(corev1.ResourceCPU, ">", 0.8)

			var nodes corev1.NodeList
			Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
			numLargeNodes := 0
			numOtherNodes := 0
			for _, n := range nodes.Items {
				// only count the nodes created by the provisoiner
				if n.Labels[karpv1.NodePoolLabelKey] != nodePool.Name {
					continue
				}
				if strings.HasSuffix(n.Labels[corev1.LabelInstanceTypeStable], ".large") {
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
		nodePool := coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Disruption: karpv1.Disruption{
					ConsolidationPolicy: karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized,
					// Disable Consolidation until we're ready
					ConsolidateAfter: karpv1.MustParseNillableDuration("Never"),
				},
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      karpv1.CapacityTypeLabelKey,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{karpv1.CapacityTypeOnDemand},
								},
							},
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      v1.LabelInstanceSize,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"large"},
								},
							},
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      v1.LabelInstanceFamily,
									Operator: corev1.NodeSelectorOpNotIn,
									// remove some cheap burstable and the odd c1 / a1 instance types so we have
									// more control over what gets provisioned
									Values: []string{"t2", "t3", "c1", "t3a", "t4g", "a1"},
								},
							},
						},
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
					},
				},
			},
		})

		var numPods int32 = 2
		smallDep := coretest.Deployment(coretest.DeploymentOptions{
			Replicas: numPods,
			PodOptions: coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "small-app"},
				},
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       corev1.LabelHostname,
						WhenUnsatisfiable: corev1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "small-app",
							},
						},
					},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: func() resource.Quantity {
						dsOverhead := env.GetDaemonSetOverhead(nodePool)
						base := lo.ToPtr(resource.MustParse("1800m"))
						base.Sub(*dsOverhead.Cpu())
						return *base
					}(),
					},
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
		nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
		coretest.ReplaceRequirements(nodePool,
			karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpExists,
				},
			},
			karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceSize,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"large"},
				},
			},
		)
		env.ExpectUpdated(nodePool)

		// Eventually expect the on-demand nodes to be consolidated into
		// spot nodes after some time
		Eventually(func(g Gomega) {
			var nodes corev1.NodeList
			Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
			var spotNodes []*corev1.Node
			var otherNodes []*corev1.Node
			for i, n := range nodes.Items {
				// only count the nodes created by the nodePool
				if n.Labels[karpv1.NodePoolLabelKey] != nodePool.Name {
					continue
				}
				if n.Labels[karpv1.CapacityTypeLabelKey] == karpv1.CapacityTypeSpot {
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
	Context("Capacity Reservations", func() {
		var largeCapacityReservationID, xlargeCapacityReservationID string
		var nodePool *karpv1.NodePool
		BeforeAll(func() {
			largeCapacityReservationID = environmentaws.ExpectCapacityReservationCreated(
				env.Context,
				env.EC2API,
				ec2types.InstanceTypeM5Large,
				env.ZoneInfo[0].Zone,
				1,
				nil,
				nil,
			)
			xlargeCapacityReservationID = environmentaws.ExpectCapacityReservationCreated(
				env.Context,
				env.EC2API,
				ec2types.InstanceTypeM5Xlarge,
				env.ZoneInfo[0].Zone,
				1,
				nil,
				nil,
			)
		})
		AfterAll(func() {
			environmentaws.ExpectCapacityReservationsCanceled(env.Context, env.EC2API, largeCapacityReservationID, xlargeCapacityReservationID)
		})
		BeforeEach(func() {
			nodePool = coretest.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
					Disruption: karpv1.Disruption{
						ConsolidationPolicy: karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized,
						ConsolidateAfter:    karpv1.MustParseNillableDuration("0s"),
					},
					Template: karpv1.NodeClaimTemplate{
						Spec: karpv1.NodeClaimTemplateSpec{
							Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      karpv1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeReserved},
									},
								},
							},
							NodeClassRef: &karpv1.NodeClassReference{
								Group: object.GVK(nodeClass).Group,
								Kind:  object.GVK(nodeClass).Kind,
								Name:  nodeClass.Name,
							},
						},
					},
				},
			})
		})
		It("should consolidate into a reserved offering", func() {
			dep := coretest.Deployment(coretest.DeploymentOptions{
				PodOptions: coretest.PodOptions{
					NodeRequirements: []corev1.NodeSelectorRequirement{{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values: []string{
							// Should result in an m5.large initially
							string(ec2types.InstanceTypeM5Large),
							// Should consolidate to the m5.xlarge when we add the reservation to the nodeclass
							string(ec2types.InstanceTypeM5Xlarge),
						},
					}},
				},
				Replicas: 1,
			})
			env.ExpectCreated(nodePool, nodeClass, dep)
			env.EventuallyExpectNodeClaimsReady(env.EventuallyExpectLaunchedNodeClaimCount("==", 1)...)
			n := env.EventuallyExpectNodeCount("==", int(1))[0]
			Expect(n.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, string(ec2types.InstanceTypeM5Large)))
			Expect(n.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))

			nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: xlargeCapacityReservationID}}
			env.ExpectUpdated(nodeClass)

			// Eventually expect the m5.large on-demand node to be replaced with an m5.xlarge reserved node. We should prioritize
			// the reserved instance since it's already been paid for.
			Eventually(func(g Gomega) {
				var nodes corev1.NodeList
				g.Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
				filtered := lo.Filter(nodes.Items, func(n corev1.Node, _ int) bool {
					if val, ok := n.Labels[karpv1.NodePoolLabelKey]; !ok || val != nodePool.Name {
						return false
					}
					return true
				})
				g.Expect(filtered).To(HaveLen(1))

				g.Expect(filtered[0].Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, string(ec2types.InstanceTypeM5Xlarge)))
				g.Expect(filtered[0].Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeReserved))
				g.Expect(filtered[0].Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, xlargeCapacityReservationID))
			}, time.Minute*10).Should(Succeed())
		})
		It("should consolidate between reserved offerings", func() {
			dep := coretest.Deployment(coretest.DeploymentOptions{
				PodOptions: coretest.PodOptions{
					NodeRequirements: []corev1.NodeSelectorRequirement{{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values: []string{
							string(ec2types.InstanceTypeM5Large),
							string(ec2types.InstanceTypeM5Xlarge),
						},
					}},
				},
				Replicas: 1,
			})

			// Start by only enabling the m5.xlarge capacity reservation, ensuring it's provisioned
			nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: xlargeCapacityReservationID}}
			env.ExpectCreated(nodePool, nodeClass, dep)
			env.EventuallyExpectNodeClaimsReady(env.EventuallyExpectLaunchedNodeClaimCount("==", 1)...)
			n := env.EventuallyExpectNodeCount("==", int(1))[0]
			Expect(n.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, string(ec2types.InstanceTypeM5Xlarge)))
			Expect(n.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeReserved))
			Expect(n.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, xlargeCapacityReservationID))

			// Add the m5.large capacity reservation to the nodeclass. We should consolidate from the xlarge instance to the large.
			nodeClass.Spec.CapacityReservationSelectorTerms = append(nodeClass.Spec.CapacityReservationSelectorTerms, v1.CapacityReservationSelectorTerm{
				ID: largeCapacityReservationID,
			})
			env.ExpectUpdated(nodeClass)
			Eventually(func(g Gomega) {
				var nodes corev1.NodeList
				g.Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
				filtered := lo.Filter(nodes.Items, func(n corev1.Node, _ int) bool {
					if val, ok := n.Labels[karpv1.NodePoolLabelKey]; !ok || val != nodePool.Name {
						return false
					}
					return true
				})
				g.Expect(filtered).To(HaveLen(1))
				g.Expect(filtered[0].Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, string(ec2types.InstanceTypeM5Large)))
				g.Expect(filtered[0].Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeReserved))
				g.Expect(filtered[0].Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, largeCapacityReservationID))
			}, time.Minute*10).Should(Succeed())
		})
	})
},
	Entry("MinValuesPolicyBestEffort", options.MinValuesPolicyBestEffort),
	Entry("MinValuesPolicyStrict", options.MinValuesPolicyStrict),
)

var _ = Describe("Node Overlay", func() {
	var nodePool *karpv1.NodePool
	BeforeEach(func() {
		nodePool = env.DefaultNodePool(nodeClass)
		nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
	})
	It("should consolidate a instance that is the cheepest based on a price adjustment node overlay applied", func() {
		overlaiedInstanceType := "m7a.32xlarge"
		pod := coretest.Pod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		})
		nodeOverlay := coretest.NodeOverlay(v1alpha1.NodeOverlay{
			Spec: v1alpha1.NodeOverlaySpec{
				PriceAdjustment: lo.ToPtr("-99.99999999999%"),
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{overlaiedInstanceType},
					},
				},
			},
		})

		env.ExpectCreated(nodePool, nodeClass, nodeOverlay, pod)
		env.EventuallyExpectHealthy(pod)
		nodes := env.ExpectCreatedNodeCount("==", 1)

		instanceType, foundInstanceType := nodes[0].Labels[corev1.LabelInstanceTypeStable]
		Expect(foundInstanceType).To(BeTrue())
		Expect(instanceType).To(Equal(overlaiedInstanceType))

		overlaiedInstanceType = "c7a.32xlarge"
		nodeOverlay = coretest.ReplaceOverlayRequirements(nodeOverlay, corev1.NodeSelectorRequirement{
			Key:      corev1.LabelInstanceTypeStable,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{overlaiedInstanceType},
		})
		env.ExpectUpdated(nodeOverlay)

		nodes = env.EventuallyExpectCreatedNodeCount("==", 2)
		nodes = lo.Filter(nodes, func(n *corev1.Node, _ int) bool {
			_, ok := lo.Find(n.Spec.Taints, func(t corev1.Taint) bool {
				return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
			})
			return !ok
		})
		instanceType, foundInstanceType = nodes[0].Labels[corev1.LabelInstanceTypeStable]
		Expect(foundInstanceType).To(BeTrue())
		Expect(instanceType).To(Equal(overlaiedInstanceType))

	})
	It("should consolidate a instance that is the cheepest based on a price override node overlay applied", func() {
		overlaiedInstanceType := "m7a.32xlarge"
		pod := coretest.Pod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		})
		nodeOverlay := coretest.NodeOverlay(v1alpha1.NodeOverlay{
			Spec: v1alpha1.NodeOverlaySpec{
				Price: lo.ToPtr("0.0000000232"),
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{overlaiedInstanceType},
					},
				},
			},
		})

		env.ExpectCreated(nodePool, nodeClass, nodeOverlay, pod)
		env.EventuallyExpectHealthy(pod)
		nodes := env.ExpectCreatedNodeCount("==", 1)

		instanceType, foundInstanceType := nodes[0].Labels[corev1.LabelInstanceTypeStable]
		Expect(foundInstanceType).To(BeTrue())
		Expect(instanceType).To(Equal(overlaiedInstanceType))

		overlaiedInstanceType = "c7a.32xlarge"
		nodeOverlay = coretest.ReplaceOverlayRequirements(nodeOverlay, corev1.NodeSelectorRequirement{
			Key:      corev1.LabelInstanceTypeStable,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{overlaiedInstanceType},
		})
		env.ExpectUpdated(nodeOverlay)

		nodes = env.EventuallyExpectCreatedNodeCount("==", 2)
		nodes = lo.Filter(nodes, func(n *corev1.Node, _ int) bool {
			_, ok := lo.Find(n.Spec.Taints, func(t corev1.Taint) bool {
				return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
			})
			return !ok
		})
		instanceType, foundInstanceType = nodes[0].Labels[corev1.LabelInstanceTypeStable]
		Expect(foundInstanceType).To(BeTrue())
		Expect(instanceType).To(Equal(overlaiedInstanceType))
	})
	It("should consolidate a node that matches hugepages resource requests", func() {
		overlaiedInstanceType := "c7a.8xlarge"
		pod := coretest.Pod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:                   coretest.RandomCPU(),
					corev1.ResourceMemory:                coretest.RandomMemory(),
					corev1.ResourceName("hugepages-2Mi"): resource.MustParse("100Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceName("hugepages-2Mi"): resource.MustParse("100Mi"),
				},
			},
		})
		nodeOverlay := coretest.NodeOverlay(v1alpha1.NodeOverlay{
			Spec: v1alpha1.NodeOverlaySpec{
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{overlaiedInstanceType},
					},
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceName("hugepages-2Mi"): resource.MustParse("4Gi"),
				},
			},
		})

		content, err := os.ReadFile("testdata/hugepage_userdata_input.sh")
		Expect(err).To(BeNil())
		nodeClass.Spec.UserData = lo.ToPtr(string(content))

		env.ExpectCreated(nodePool, nodeClass, nodeOverlay, pod)
		env.EventuallyExpectHealthy(pod)
		nodes := env.ExpectCreatedNodeCount("==", 1)

		instanceType, foundInstanceType := nodes[0].Labels[corev1.LabelInstanceTypeStable]
		Expect(foundInstanceType).To(BeTrue())
		Expect(instanceType).To(Equal(overlaiedInstanceType))

		overlaiedInstanceType = "c7a.2xlarge"
		nodeOverlay = coretest.ReplaceOverlayRequirements(nodeOverlay, corev1.NodeSelectorRequirement{
			Key:      corev1.LabelInstanceTypeStable,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{overlaiedInstanceType},
		})
		env.ExpectUpdated(nodeOverlay)

		nodes = env.EventuallyExpectCreatedNodeCount("==", 2)
		nodes = lo.Filter(nodes, func(n *corev1.Node, _ int) bool {
			_, ok := lo.Find(n.Spec.Taints, func(t corev1.Taint) bool {
				return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
			})
			return !ok
		})
		instanceType, foundInstanceType = nodes[0].Labels[corev1.LabelInstanceTypeStable]
		Expect(foundInstanceType).To(BeTrue())
		Expect(instanceType).To(Equal(overlaiedInstanceType))
	})
})
