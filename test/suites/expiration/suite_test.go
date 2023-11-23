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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/service/ssm"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/test/pkg/environment/aws"

	coretest "github.com/aws/karpenter-core/pkg/test"
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
	It("should expire the node after the expiration is reached", func() {
		var numPods int32 = 1
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
		env.ExpectCreated(nodeClass, nodePool, dep)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

		// Expect that the NodeClaim will get an expired status condition
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Expired).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		// Remove the do-not-disrupt annotation so that the Nodes are now deprovisionable
		for _, pod := range env.ExpectPodsMatchingSelector(selector) {
			delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
			env.ExpectUpdated(pod)
		}

		// Eventually the node will be set as unschedulable, which means its actively being deprovisioned
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

		// Expect that the NodeClaim will get an expired status condition
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Expired).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		// Remove the do-not-disruption annotation so that the Nodes are now deprovisionable
		for _, pod := range env.ExpectPodsMatchingSelector(selector) {
			delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
			env.ExpectUpdated(pod)
		}

		// Eventually the node will be set as unschedulable, which means its actively being deprovisioned
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
	Context("Expiration Failure", func() {
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
			parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
				Name: lo.ToPtr("/aws/service/ami-amazon-linux-latest/amzn-ami-hvm-x86_64-ebs"),
			})
			Expect(err).ToNot(HaveOccurred())
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID: *parameter.Parameter.Value,
				},
			}
			env.ExpectCreatedOrUpdated(nodeClass)

			// Should see the NodeClaim has expired
			Eventually(func(g Gomega) {
				for _, nc := range startingNodeClaimState {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nc), nc)).To(Succeed())
					g.Expect(nc.StatusConditions().GetCondition(corev1beta1.Expired).IsTrue()).To(BeTrue())
				}
			}).Should(Succeed())

			// Expect nodes To get tainted
			taintedNodes := env.EventuallyExpectTaintedNodeCount("==", 1)

			// Expire should fail and the original node should be untainted
			// TODO: reduce timeouts when deprovisioning waits are factored out
			env.EventuallyExpectNodesUntaintedWithTimeout(11*time.Minute, taintedNodes...)

			// The nodeclaims that never registers will be removed
			Eventually(func(g Gomega) {
				nodeClaims := &corev1beta1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())
				g.Expect(len(nodeClaims.Items)).To(BeNumerically("==", int(numPods)))
			}).WithTimeout(6 * time.Minute).Should(Succeed())

			// Expect all the NodeClaims that existed on the initial provisioning loop are not removed
			Consistently(func(g Gomega) {
				nodeClaims := &corev1beta1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())

				startingNodeClaimUIDs := lo.Map(startingNodeClaimState, func(nc *corev1beta1.NodeClaim, _ int) types.UID { return nc.UID })
				nodeClaimUIDs := lo.Map(nodeClaims.Items, func(nc corev1beta1.NodeClaim, _ int) types.UID { return nc.UID })
				g.Expect(sets.New(nodeClaimUIDs...).IsSuperset(sets.New(startingNodeClaimUIDs...))).To(BeTrue())
			}, "2m").Should(Succeed())
		})
		It("should not continue to expiration if a node registers but never becomes initialized", func() {
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
					g.Expect(env.Client.Patch(env.Context, node, client.MergeFrom(stored))).To(Succeed())
				}
			}).Should(Succeed())

			// Should see the NodeClaim has expired
			Eventually(func(g Gomega) {
				for _, nc := range startingNodeClaimState {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nc), nc)).To(Succeed())
					g.Expect(nc.StatusConditions().GetCondition(corev1beta1.Expired).IsTrue()).To(BeTrue())
				}
			}).Should(Succeed())

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
	})
})
