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

package expiration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"

	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *aws.Environment
var nodeClass *v1beta1.EC2NodeClass
var nodePool *corev1beta1.NodePool

func TestExpiration(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Expiration")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
	nodePool.Spec.Disruption.ExpireAfter = corev1beta1.NillableDuration{Duration: lo.ToPtr(time.Second * 30)}
})

var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Expiration", func() {
	var dep *appsv1.Deployment
	var selector labels.Selector
	var numPods int
	BeforeEach(func() {
		numPods = 1
		// Add pods with a do-not-disrupt annotation so that we can check node metadata before we disrupt
		dep = coretest.Deployment(coretest.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "my-app",
					},
					Annotations: map[string]string{
						corev1beta1.DoNotDisruptAnnotationKey: "true",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})
	Context("Budgets", func() {
		// Two nodes, both expired or both drifted, the more drifted one with a pre-stop pod that sleeps for 300 seconds,
		// and we consistently ensure that the second node is not tainted == disrupted.
		It("should not continue to disrupt nodes that have been the target of pod nomination", func() {
			coretest.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceSize,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"2xlarge"},
					},
				},
			)
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes: "100%",
			}}
			nodePool.Spec.Disruption.ExpireAfter = corev1beta1.NillableDuration{}

			// Create a deployment with one pod to create one node.
			dep = coretest.Deployment(coretest.DeploymentOptions{
				Replicas: 1,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							corev1beta1.DoNotDisruptAnnotationKey: "true",
						},
						Labels: map[string]string{"app": "large-app"},
					},
					// Each 2xlarge has 8 cpu, so each node should fit 2 pods.
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("3"),
						},
					},
					Command:                       []string{"sh", "-c", "sleep 3600"},
					Image:                         "alpine:latest",
					PreStopSleep:                  lo.ToPtr(int64(300)),
					TerminationGracePeriodSeconds: lo.ToPtr(int64(500)),
				},
			})
			selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			env.ExpectCreated(nodeClass, nodePool, dep)

			env.EventuallyExpectCreatedNodeClaimCount("==", 1)
			env.EventuallyExpectCreatedNodeCount("==", 1)
			env.EventuallyExpectHealthyPodCount(selector, 1)

			// Set the node to unschedulable so that we can create another node with one pod.
			node := env.EventuallyExpectNodeCount("==", 1)[0]
			node.Spec.Unschedulable = true
			env.ExpectUpdated(node)

			dep.Spec.Replicas = lo.ToPtr(int32(2))
			env.ExpectUpdated(dep)

			ncs := env.EventuallyExpectCreatedNodeClaimCount("==", 2)
			env.EventuallyExpectCreatedNodeCount("==", 2)
			pods := env.EventuallyExpectHealthyPodCount(selector, 2)
			env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

			node = env.ExpectExists(node).(*v1.Node)
			node.Spec.Unschedulable = false
			env.ExpectUpdated(node)

			By("enabling expiration")
			nodePool.Spec.Disruption.ExpireAfter = corev1beta1.NillableDuration{Duration: lo.ToPtr(time.Second * 30)}
			env.ExpectUpdated(nodePool)

			// Expect that both of the nodes are expired, but not being disrupted
			env.EventuallyExpectExpired(ncs...)
			env.ConsistentlyExpectNoDisruptions(2, 30*time.Second)

			By("removing the do not disrupt annotations")
			// Remove the do not disrupt annotation from the two pods
			for _, p := range pods {
				p := env.ExpectExists(p).(*v1.Pod)
				delete(p.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
				env.ExpectUpdated(p)
			}
			env.EventuallyExpectTaintedNodeCount("==", 1)

			By("expecting only one disruption for 60s")
			// Expect only one node being disrupted as the other node should continue to be nominated.
			// As the pod has a 300s pre-stop sleep.
			env.ConsistentlyExpectDisruptionsWithNodeCount(1, 2, time.Minute)
		})
		It("should respect budgets for empty expiration", func() {
			coretest.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceSize,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"2xlarge"},
					},
				},
			)
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes: "50%",
			}}
			nodePool.Spec.Disruption.ExpireAfter = corev1beta1.NillableDuration{}

			numPods = 6
			dep = coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							corev1beta1.DoNotDisruptAnnotationKey: "true",
						},
						Labels: map[string]string{"app": "large-app"},
					},
					// Each 2xlarge has 8 cpu, so each node should fit 2 pods.
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("3"),
						},
					},
				},
			})
			selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			nodes := env.EventuallyExpectCreatedNodeCount("==", 3)
			env.EventuallyExpectHealthyPodCount(selector, numPods)
			env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

			By("adding finalizers to the nodes to prevent termination")
			// Add a finalizer to each node so that we can stop termination disruptions
			for _, node := range nodes {
				Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				env.ExpectUpdated(node)
			}

			By("making the nodes empty")
			// Delete the deployment to make all nodes empty.
			env.ExpectDeleted(dep)

			By("enabling expiration")
			nodePool.Spec.Disruption.ExpireAfter = corev1beta1.NillableDuration{Duration: lo.ToPtr(time.Second * 30)}
			env.ExpectUpdated(nodePool)

			env.EventuallyExpectExpired(nodeClaims...)

			// Expect that two nodes are tainted.
			env.EventuallyExpectTaintedNodeCount("==", 2)
			nodes = env.ConsistentlyExpectDisruptionsWithNodeCount(2, 3, 5*time.Second)

			// Remove finalizers
			for _, node := range nodes {
				Expect(env.ExpectTestingFinalizerRemoved(node)).To(Succeed())
			}

			// After the deletion timestamp is set and all pods are drained
			// the node should be gone
			env.EventuallyExpectNotFound(nodes[0], nodes[1])

			// Expect that only one node is tainted, even considering the new node that was just created.
			nodes = env.EventuallyExpectTaintedNodeCount("==", 1)

			// Expect the finalizers to be removed and deleted.
			Expect(env.ExpectTestingFinalizerRemoved(nodes[0])).To(Succeed())
			env.EventuallyExpectNotFound(nodes[0])
		})
		It("should respect budgets for non-empty delete expiration", func() {
			nodePool = coretest.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceSize,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"2xlarge"},
					},
				},
			)
			// We're expecting to create 3 nodes, so we'll expect to see at most 2 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes: "50%",
			}}
			// disable expiration so that we can enable it later when we want.
			nodePool.Spec.Disruption.ExpireAfter = corev1beta1.NillableDuration{}
			numPods = 9
			dep = coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							corev1beta1.DoNotDisruptAnnotationKey: "true",
						},
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

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			nodes := env.EventuallyExpectCreatedNodeCount("==", 3)
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			By("scaling down the deployment")
			// Update the deployment to a third of the replicas.
			dep.Spec.Replicas = lo.ToPtr[int32](3)
			env.ExpectUpdated(dep)

			// First expect there to be 3 pods, then try to spread the pods.
			env.EventuallyExpectHealthyPodCount(selector, 3)
			env.ForcePodsToSpread(nodes...)
			env.EventuallyExpectHealthyPodCount(selector, 3)

			By("cordoning and adding finalizer to the nodes")
			// Add a finalizer to each node so that we can stop termination disruptions
			for _, node := range nodes {
				Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				env.ExpectUpdated(node)
			}

			By("expiring the nodes")
			// expire the nodeclaims
			nodePool.Spec.Disruption.ExpireAfter = corev1beta1.NillableDuration{Duration: lo.ToPtr(time.Second * 30)}
			env.ExpectUpdated(nodePool)

			env.EventuallyExpectExpired(nodeClaims...)

			By("enabling disruption by removing the do not disrupt annotation")
			pods := env.EventuallyExpectHealthyPodCount(selector, 3)
			// Remove the do-not-disrupt annotation so that the nodes are now disruptable
			for _, pod := range pods {
				delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
				env.ExpectUpdated(pod)
			}

			// Ensure that we get two nodes tainted, and they have overlap during the expiration
			env.EventuallyExpectTaintedNodeCount("==", 2)
			nodes = env.ConsistentlyExpectDisruptionsWithNodeCount(2, 3, 5*time.Second)

			By("removing the finalizer from the nodes")
			Expect(env.ExpectTestingFinalizerRemoved(nodes[0])).To(Succeed())
			Expect(env.ExpectTestingFinalizerRemoved(nodes[1])).To(Succeed())

			// After the deletion timestamp is set and all pods are drained
			// the node should be gone
			env.EventuallyExpectNotFound(nodes[0], nodes[1])
		})
		It("should respect budgets for non-empty replace expiration", func() {
			appLabels := map[string]string{"app": "large-app"}
			nodePool.Labels = appLabels
			// We're expecting to create 5 nodes, so we'll expect to see at most 3 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes: "3",
			}}

			// Create a 5 pod deployment with hostname inter-pod anti-affinity to ensure each pod is placed on a unique node
			selector = labels.SelectorFromSet(appLabels)
			numPods = 5
			deployment := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: appLabels,
					},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: appLabels,
						},
					}},
				},
			})

			env.ExpectCreated(nodeClass, nodePool, deployment)

			env.EventuallyExpectCreatedNodeClaimCount("==", numPods)
			nodes := env.EventuallyExpectCreatedNodeCount("==", numPods)

			// Check that all daemonsets and deployment pods are online
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			By("cordoning and adding finalizer to the nodes")
			// Add a finalizer to each node so that we can stop termination disruptions
			for _, node := range nodes {
				Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				env.ExpectUpdated(node)
			}

			By("enabling expiration")
			nodePool.Spec.Disruption.ExpireAfter = corev1beta1.NillableDuration{Duration: lo.ToPtr(30 * time.Second)}
			env.ExpectUpdated(nodePool)

			// Ensure that we get two nodes tainted, and they have overlap during the expiration
			env.EventuallyExpectTaintedNodeCount("==", 3)
			env.EventuallyExpectNodeClaimCount("==", 8)
			env.EventuallyExpectNodeCount("==", 8)
			nodes = env.ConsistentlyExpectDisruptionsWithNodeCount(3, 8, 5*time.Second)

			// Set the expireAfter to "Never" to make sure new node isn't deleted
			// This is CRITICAL since it prevents nodes that are immediately spun up from immediately being expired and
			// racing at the end of the E2E test, leaking node resources into subsequent tests
			nodePool.Spec.Disruption.ExpireAfter.Duration = nil
			env.ExpectUpdated(nodePool)

			for _, node := range nodes {
				Expect(env.ExpectTestingFinalizerRemoved(node)).To(Succeed())
			}

			env.EventuallyExpectNotFound(nodes[0], nodes[1], nodes[2])
			env.ExpectNodeCount("==", 5)
		})
		It("should not allow expiration if the budget is fully blocking", func() {
			// We're going to define a budget that doesn't allow any expirations to happen
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes: "0",
			}}

			dep.Spec.Template.Annotations = nil
			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			env.EventuallyExpectCreatedNodeCount("==", 1)
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			env.EventuallyExpectExpired(nodeClaim)
			env.ConsistentlyExpectNoDisruptions(1, time.Minute)
		})
		It("should not allow expiration if the budget is fully blocking during a scheduled time", func() {
			// We're going to define a budget that doesn't allow any expirations to happen
			// This is going to be on a schedule that only lasts 30 minutes, whose window starts 15 minutes before
			// the current time and extends 15 minutes past the current time
			// Times need to be in UTC since the karpenter containers were built in UTC time
			windowStart := time.Now().Add(-time.Minute * 15).UTC()
			nodePool.Spec.Disruption.Budgets = []corev1beta1.Budget{{
				Nodes:    "0",
				Schedule: lo.ToPtr(fmt.Sprintf("%d %d * * *", windowStart.Minute(), windowStart.Hour())),
				Duration: &metav1.Duration{Duration: time.Minute * 30},
			}}

			dep.Spec.Template.Annotations = nil
			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			env.EventuallyExpectCreatedNodeCount("==", 1)
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			env.EventuallyExpectExpired(nodeClaim)
			env.ConsistentlyExpectNoDisruptions(1, time.Minute)
		})
	})
	It("should expire the node after the expiration is reached", func() {
		env.ExpectCreated(nodeClass, nodePool, dep)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

		env.EventuallyExpectExpired(nodeClaim)

		// Remove the do-not-disrupt annotation so that the Nodes are now deprovisionable
		for _, pod := range env.ExpectPodsMatchingSelector(selector) {
			delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
			env.ExpectUpdated(pod)
		}

		// Eventually the node will be tainted, which means its actively being disrupted
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).Should(Succeed())
			_, ok := lo.Find(node.Spec.Taints, func(t v1.Taint) bool {
				return corev1beta1.IsDisruptingTaint(t)
			})
			g.Expect(ok).To(BeTrue())
		}).Should(Succeed())

		// Set the expireAfter to "Never" to make sure new node isn't deleted
		// This is CRITICAL since it prevents nodes that are immediately spun up from immediately being expired and
		// racing at the end of the E2E test, leaking node resources into subsequent tests
		nodePool.Spec.Disruption.ExpireAfter.Duration = nil
		env.ExpectUpdated(nodePool)

		// After the deletion timestamp is set and all pods are drained
		// the node should be gone
		env.EventuallyExpectNotFound(nodeClaim, node)

		env.EventuallyExpectCreatedNodeClaimCount("==", 1)
		env.EventuallyExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should replace expired node with a single node and schedule all pods", func() {
		var numPods int32 = 5
		// We should setup a PDB that will only allow a minimum of 1 pod to be pending at a time
		minAvailable := intstr.FromInt32(numPods - 1)
		pdb := coretest.PodDisruptionBudget(coretest.PDBOptions{
			Labels: map[string]string{
				"app": "large-app",
			},
			MinAvailable: &minAvailable,
		})
		dep := coretest.Deployment(coretest.DeploymentOptions{
			Replicas: numPods,
			PodOptions: coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						corev1beta1.DoNotDisruptAnnotationKey: "true",
					},
					Labels: map[string]string{"app": "large-app"},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, pdb, dep)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

		// Set the expireAfter value to get the node deleted
		nodePool.Spec.Disruption.ExpireAfter.Duration = lo.ToPtr(time.Minute)
		env.ExpectUpdated(nodePool)

		env.EventuallyExpectExpired(nodeClaim)

		// Remove the do-not-disruption annotation so that the Nodes are now deprovisionable
		for _, pod := range env.ExpectPodsMatchingSelector(selector) {
			delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
			env.ExpectUpdated(pod)
		}

		// Eventually the node will be tainted, which means its actively being disrupted
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).Should(Succeed())
			_, ok := lo.Find(node.Spec.Taints, func(t v1.Taint) bool {
				return corev1beta1.IsDisruptingTaint(t)
			})
			g.Expect(ok).To(BeTrue())
		}).Should(Succeed())

		// Set the expireAfter to "Never" to make sure new node isn't deleted
		// This is CRITICAL since it prevents nodes that are immediately spun up from immediately being expired and
		// racing at the end of the E2E test, leaking node resources into subsequent tests
		nodePool.Spec.Disruption.ExpireAfter.Duration = nil
		env.ExpectUpdated(nodePool)

		// After the deletion timestamp is set and all pods are drained
		// the node should be gone
		env.EventuallyExpectNotFound(nodeClaim, node)

		env.EventuallyExpectCreatedNodeClaimCount("==", 1)
		env.EventuallyExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
	})
	Context("Failure", func() {
		It("should not continue to expire if a node never registers", func() {
			// Launch a new NodeClaim
			var numPods int32 = 2
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: 2,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
				},
			})
			env.ExpectCreated(dep, nodeClass, nodePool)

			startingNodeClaimState := env.EventuallyExpectCreatedNodeClaimCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Set a configuration that will not register a NodeClaim
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID: env.GetAMIBySSMPath("/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-ebs"),
				},
			}
			env.ExpectCreatedOrUpdated(nodeClass)

			env.EventuallyExpectExpired(startingNodeClaimState...)

			// Expect nodes To get tainted
			taintedNodes := env.EventuallyExpectTaintedNodeCount("==", 1)

			// Expire should fail and the original node should be untainted
			// TODO: reduce timeouts when deprovisioning waits are factored out
			env.EventuallyExpectNodesUntaintedWithTimeout(11*time.Minute, taintedNodes...)

			// Expect all the NodeClaims that existed on the initial provisioning loop are not removed
			Consistently(func(g Gomega) {
				nodeClaims := &corev1beta1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())

				startingNodeClaimUIDs := lo.Map(startingNodeClaimState, func(nc *corev1beta1.NodeClaim, _ int) types.UID { return nc.UID })
				nodeClaimUIDs := lo.Map(nodeClaims.Items, func(nc corev1beta1.NodeClaim, _ int) types.UID { return nc.UID })
				g.Expect(sets.New(nodeClaimUIDs...).IsSuperset(sets.New(startingNodeClaimUIDs...))).To(BeTrue())
			}, "2m").Should(Succeed())
		})
		It("should not continue to expire if a node registers but never becomes initialized", func() {
			// Set a configuration that will allow us to make a NodeClaim not be initialized
			nodePool.Spec.Template.Spec.StartupTaints = []v1.Taint{{Key: "example.com/taint", Effect: v1.TaintEffectPreferNoSchedule}}

			// Launch a new NodeClaim
			var numPods int32 = 2
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: 2,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
				},
			})
			env.ExpectCreated(dep, nodeClass, nodePool)

			startingNodeClaimState := env.EventuallyExpectCreatedNodeClaimCount("==", int(numPods))
			nodes := env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Remove the startup taints from these nodes to initialize them
			Eventually(func(g Gomega) {
				for _, node := range nodes {
					g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
					stored := node.DeepCopy()
					node.Spec.Taints = lo.Reject(node.Spec.Taints, func(t v1.Taint, _ int) bool { return t.Key == "example.com/taint" })
					g.Expect(env.Client.Patch(env.Context, node, client.StrategicMergeFrom(stored))).To(Succeed())
				}
			}).Should(Succeed())

			env.EventuallyExpectExpired(startingNodeClaimState...)

			// Expect nodes To be tainted
			taintedNodes := env.EventuallyExpectTaintedNodeCount("==", 1)

			// Expire should fail and original node should be untainted and no NodeClaims should be removed
			// TODO: reduce timeouts when deprovisioning waits are factored out
			env.EventuallyExpectNodesUntaintedWithTimeout(11*time.Minute, taintedNodes...)

			// Expect that the new NodeClaim/Node is kept around after the un-cordon
			nodeList := &v1.NodeList{}
			Expect(env.Client.List(env, nodeList, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())
			Expect(nodeList.Items).To(HaveLen(int(numPods) + 1))

			nodeClaimList := &corev1beta1.NodeClaimList{}
			Expect(env.Client.List(env, nodeClaimList, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())
			Expect(nodeClaimList.Items).To(HaveLen(int(numPods) + 1))

			// Expect all the NodeClaims that existed on the initial provisioning loop are not removed
			Consistently(func(g Gomega) {
				nodeClaims := &corev1beta1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())

				startingNodeClaimUIDs := lo.Map(startingNodeClaimState, func(nc *corev1beta1.NodeClaim, _ int) types.UID { return nc.UID })
				nodeClaimUIDs := lo.Map(nodeClaims.Items, func(nc corev1beta1.NodeClaim, _ int) types.UID { return nc.UID })
				g.Expect(sets.New(nodeClaimUIDs...).IsSuperset(sets.New(startingNodeClaimUIDs...))).To(BeTrue())
			}, "2m").Should(Succeed())
		})
		It("should not expire any nodes if their PodDisruptionBudgets are unhealthy", func() {
			// Create a deployment that contains a readiness probe that will never succeed
			// This way, the pod will bind to the node, but the PodDisruptionBudget will never go healthy
			var numPods int32 = 2
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: 2,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
					ReadinessProbe: &v1.Probe{
						ProbeHandler: v1.ProbeHandler{
							HTTPGet: &v1.HTTPGetAction{
								Port: intstr.FromInt32(80),
							},
						},
					},
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			minAvailable := intstr.FromInt32(numPods - 1)
			pdb := coretest.PodDisruptionBudget(coretest.PDBOptions{
				Labels:       dep.Spec.Template.Labels,
				MinAvailable: &minAvailable,
			})
			env.ExpectCreated(dep, nodeClass, nodePool, pdb)

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Expect pods to be bound but not to be ready since we are intentionally failing the readiness check
			env.EventuallyExpectBoundPodCount(selector, int(numPods))

			env.EventuallyExpectExpired(nodeClaims...)
			env.ConsistentlyExpectNoDisruptions(int(numPods), time.Minute)
		})
	})
})
