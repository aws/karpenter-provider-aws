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

package integration_test

import (
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/test/pkg/environment/common"
)

var _ = Describe("Drift", Ordered, func() {
	var dep *appsv1.Deployment
	var selector labels.Selector
	var numPods int
	var label map[string]string
	BeforeEach(func() {
		numPods = 1
		label = map[string]string{"app": "large-app"}
		// Add pods with a do-not-disrupt annotation so that we can check node metadata before we disrupt
		dep = test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
					Annotations: map[string]string{
						v1.DoNotDisruptAnnotationKey: "true",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})
	Context("Budgets", func() {
		It("should respect budgets for empty drift", func() {
			// We're expecting to create 3 nodes, so we'll expect to see 2 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{
				Nodes: "50%",
			}}
			var numPods int32 = 3
			dep = test.Deployment(test.DeploymentOptions{
				Replicas: numPods,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1.DoNotDisruptAnnotationKey: "true",
						},
						Labels: label,
					},
					PodAntiRequirements: []corev1.PodAffinityTerm{{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: label,
						},
					}},
				},
			})
			selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			nodes := env.EventuallyExpectCreatedNodeCount("==", 3)
			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

			// List nodes so that we get any updated information on the nodes. If we don't
			// we have the potential to over-write any changes Karpenter makes to the nodes.
			// Add a finalizer to each node so that we can stop termination disruptions
			By("adding finalizers to the nodes to prevent termination")
			for _, node := range nodes {
				Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				env.ExpectUpdated(node)
			}

			By("making the nodes empty")
			// Delete the deployment to make all nodes empty.
			env.ExpectDeleted(dep)

			// Drift the nodeclaims
			By("drift the nodeclaims")
			nodePool.Spec.Template.Annotations = map[string]string{"test": "annotation"}
			env.ExpectUpdated(nodePool)

			env.EventuallyExpectDrifted(nodeClaims...)

			env.ConsistentlyExpectDisruptionsUntilNoneLeft(3, 2, 5*time.Minute)
		})
		It("should respect budgets for non-empty delete drift", func() {
			// We're expecting to create 3 nodes, so we'll expect to see at most 2 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{
				Nodes: "50%",
			}}
			var numPods int32 = 9
			dep = test.Deployment(test.DeploymentOptions{
				Replicas: numPods,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1.DoNotDisruptAnnotationKey: "true",
						},
						Labels: label,
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           3,
							TopologyKey:       corev1.LabelHostname,
							WhenUnsatisfiable: corev1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: label,
							},
						},
					},
				},
			})
			selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 3)
			nodes := env.EventuallyExpectCreatedNodeCount("==", 3)
			env.EventuallyExpectHealthyPodCount(selector, int(numPods))

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

			By("drifting the nodes")
			// Drift the nodeclaims
			nodePool.Spec.Template.Annotations = map[string]string{"test": "annotation"}
			env.ExpectUpdated(nodePool)

			env.EventuallyExpectDrifted(nodeClaims...)

			By("enabling disruption by removing the do not disrupt annotation")
			pods := env.EventuallyExpectHealthyPodCount(selector, 3)
			// Remove the do-not-disrupt annotation so that the nodes are now disruptable
			for _, pod := range pods {
				delete(pod.Annotations, v1.DoNotDisruptAnnotationKey)
				env.ExpectUpdated(pod)
			}

			env.ConsistentlyExpectDisruptionsUntilNoneLeft(3, 2, 5*time.Minute)
		})
		It("should respect budgets for non-empty replace drift", func() {
			appLabels := map[string]string{"app": "large-app"}
			nodePool.Labels = appLabels
			// We're expecting to create 5 nodes, so we'll expect to see at most 3 nodes deleting at one time.
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{
				Nodes: "3",
			}}
			// Create a 5 pod deployment with hostname inter-pod anti-affinity to ensure each pod is placed on a unique node
			numPods = 5
			selector = labels.SelectorFromSet(appLabels)
			deployment := test.Deployment(test.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: appLabels,
					},
					PodAntiRequirements: []corev1.PodAffinityTerm{{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: appLabels,
						},
					}},
				},
			})

			env.ExpectCreated(nodeClass, nodePool, deployment)

			originalNodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", numPods)
			originalNodes := env.EventuallyExpectCreatedNodeCount("==", numPods)

			// Check that all deployment pods are online
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			By("cordoning and adding finalizer to the nodes")
			// Add a finalizer to each node so that we can stop termination disruptions
			for _, node := range originalNodes {
				Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				env.ExpectUpdated(node)
			}

			By("drifting the nodepool")
			nodePool.Spec.Template.Annotations = lo.Assign(nodePool.Spec.Template.Annotations, map[string]string{"test-annotation": "drift"})
			env.ExpectUpdated(nodePool)
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
			env.EventuallyExpectNotFound(lo.Map(originalNodeClaims, func(n *v1.NodeClaim, _ int) client.Object { return n })...)
			env.ExpectNodeClaimCount("==", 5)
			env.ExpectNodeCount("==", 5)
		})
		It("should not allow drift if the budget is fully blocking", func() {
			// We're going to define a budget that doesn't allow any drift to happen
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{
				Nodes: "0",
			}}

			dep.Spec.Template.Annotations = nil
			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			env.EventuallyExpectCreatedNodeCount("==", 1)
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			By("drifting the nodes")
			// Drift the nodeclaims
			nodePool.Spec.Template.Annotations = map[string]string{"test": "annotation"}
			env.ExpectUpdated(nodePool)

			env.EventuallyExpectDrifted(nodeClaim)
			env.ConsistentlyExpectNoDisruptions(1, time.Minute)
		})
		It("should not allow drift if the budget is fully blocking during a scheduled time", func() {
			// We're going to define a budget that doesn't allow any drift to happen
			// This is going to be on a schedule that only lasts 30 minutes, whose window starts 15 minutes before
			// the current time and extends 15 minutes past the current time
			// Times need to be in UTC since the karpenter containers were built in UTC time
			windowStart := time.Now().Add(-time.Minute * 15).UTC()
			nodePool.Spec.Disruption.Budgets = []v1.Budget{{
				Nodes:    "0",
				Schedule: lo.ToPtr(fmt.Sprintf("%d %d * * *", windowStart.Minute(), windowStart.Hour())),
				Duration: &metav1.Duration{Duration: time.Minute * 30},
			}}

			dep.Spec.Template.Annotations = nil
			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			env.EventuallyExpectCreatedNodeCount("==", 1)
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			By("drifting the nodes")
			// Drift the nodeclaims
			nodePool.Spec.Template.Annotations = map[string]string{"test": "annotation"}
			env.ExpectUpdated(nodePool)

			env.EventuallyExpectDrifted(nodeClaim)
			env.ConsistentlyExpectNoDisruptions(1, time.Minute)
		})
	})
	DescribeTable("NodePool Drift", func(nodeClaimTemplate v1.NodeClaimTemplate) {
		updatedNodePool := test.NodePool(
			lo.FromPtr(nodePool),
			v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: nodeClaimTemplate,
				},
			},
		)
		updatedNodePool.ObjectMeta = nodePool.ObjectMeta

		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		env.ExpectCreatedOrUpdated(updatedNodePool)

		env.EventuallyExpectDrifted(nodeClaim)

		delete(pod.Annotations, v1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)

		// Nodes will need to have the start-up taint removed before the node can be considered as initialized
		fmt.Println(CurrentSpecReport().LeafNodeText)
		if CurrentSpecReport().LeafNodeText == "Start-up Taints" {
			nodes := env.EventuallyExpectCreatedNodeCount("==", 2)
			sort.Slice(nodes, func(i int, j int) bool {
				return nodes[i].CreationTimestamp.Before(&nodes[j].CreationTimestamp)
			})
			nodeTwo := nodes[1]
			// Remove the startup taints from the new nodes to initialize them
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeTwo), nodeTwo)).To(Succeed())
				g.Expect(len(nodeTwo.Spec.Taints)).To(BeNumerically("==", 1))
				_, found := lo.Find(nodeTwo.Spec.Taints, func(t corev1.Taint) bool {
					return t.MatchTaint(&corev1.Taint{Key: "example.com/another-taint-2", Effect: corev1.TaintEffectPreferNoSchedule})
				})
				g.Expect(found).To(BeTrue())
				stored := nodeTwo.DeepCopy()
				nodeTwo.Spec.Taints = lo.Reject(nodeTwo.Spec.Taints, func(t corev1.Taint, _ int) bool { return t.Key == "example.com/another-taint-2" })
				g.Expect(env.Client.Patch(env.Context, nodeTwo, client.StrategicMergeFrom(stored))).To(Succeed())
			}).Should(Succeed())
		}
		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	},
		Entry("Annotations", v1.NodeClaimTemplate{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{"keyAnnotationTest": "valueAnnotationTest"},
			},
		}),
		Entry("Labels", v1.NodeClaimTemplate{
			ObjectMeta: v1.ObjectMeta{
				Labels: map[string]string{"keyLabelTest": "valueLabelTest"},
			},
		}),
		Entry("Taints", v1.NodeClaimTemplate{
			Spec: v1.NodeClaimTemplateSpec{
				Taints: []corev1.Taint{{Key: "example.com/another-taint-2", Effect: corev1.TaintEffectPreferNoSchedule}},
			},
		}),
		Entry("Start-up Taints", v1.NodeClaimTemplate{
			Spec: v1.NodeClaimTemplateSpec{
				StartupTaints: []corev1.Taint{{Key: "example.com/another-taint-2", Effect: corev1.TaintEffectPreferNoSchedule}},
			},
		}),
		Entry("NodeRequirements", v1.NodeClaimTemplate{
			Spec: v1.NodeClaimTemplateSpec{
				// since this will overwrite the default requirements, add instance category and family selectors back into requirements
				Requirements: []v1.NodeSelectorRequirementWithMinValues{
					{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.CapacityTypeSpot}}},
				},
			},
		}),
	)
	It("should update the nodepool-hash annotation on the nodepool and nodeclaim when the nodepool's nodepool-hash-version annotation does not match the controller hash version", func() {
		env.ExpectCreated(dep, nodeClass, nodePool)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		nodePool = env.ExpectExists(nodePool).(*v1.NodePool)
		expectedHash := nodePool.Hash()

		By(fmt.Sprintf("expect nodepool %s and nodeclaim %s to contain %s and %s annotations", nodePool.Name, nodeClaim.Name, v1.NodePoolHashAnnotationKey, v1.NodePoolHashVersionAnnotationKey))
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())

			g.Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
			g.Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
			g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
			g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
		}).WithTimeout(30 * time.Second).Should(Succeed())

		nodePool.Annotations = lo.Assign(nodePool.Annotations, map[string]string{
			v1.NodePoolHashAnnotationKey:        "test-hash-1",
			v1.NodePoolHashVersionAnnotationKey: "test-hash-version-1",
		})
		// Updating `nodePool.Spec.Template.Annotations` would normally trigger drift on all nodeclaims owned by the
		// nodepool. However, the nodepool-hash-version does not match the controller hash version, so we will see that
		// none of the nodeclaims will be drifted and all nodeclaims will have an updated `nodepool-hash` and `nodepool-hash-version` annotation
		nodePool.Spec.Template.Annotations = lo.Assign(nodePool.Spec.Template.Annotations, map[string]string{
			"test-key": "test-value",
		})
		nodeClaim.Annotations = lo.Assign(nodePool.Annotations, map[string]string{
			v1.NodePoolHashAnnotationKey:        "test-hash-2",
			v1.NodePoolHashVersionAnnotationKey: "test-hash-version-2",
		})

		// The nodeclaim will need to be updated first, as the hash controller will only be triggered on changes to the nodepool
		env.ExpectUpdated(nodeClaim, nodePool)
		expectedHash = nodePool.Hash()

		// Expect all nodeclaims not to be drifted and contain an updated `nodepool-hash` and `nodepool-hash-version` annotation
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())

			g.Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
			g.Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
			g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
			g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
		})
	})
	Context("Failure", func() {
		It("should not disrupt a drifted node if the replacement node never registers", func() {
			env.ExpectBlockNodeRegistration()

			// launch a new nodeClaim
			var numPods int32 = 2
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: 2,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []corev1.PodAffinityTerm{{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
				},
			})
			// Expect only a single node to get tainted due to default disruption budgets
			nodePool.Spec.Disruption = v1.Disruption{}
			env.ExpectCreated(dep, nodeClass, nodePool)

			startingNodeClaimState := env.EventuallyExpectCreatedNodeClaimCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Drift the nodeClaim with bad configuration that will not register a NodeClaim
			nodePool.Spec.Template.Labels = lo.Assign(nodePool.Spec.Template.Labels, map[string]string{
				"registration": "fail",
			})
			env.ExpectCreatedOrUpdated(nodePool)

			env.EventuallyExpectDrifted(startingNodeClaimState...)

			// Expect only a single node to be tainted due to default disruption budgets
			taintedNodes := env.EventuallyExpectTaintedNodeCount("==", 1)

			// Drift should fail and the original node should be untainted
			// TODO: reduce timeouts when disruption waits are factored out
			env.EventuallyExpectNodesUntaintedWithTimeout(11*time.Minute, taintedNodes...)

			// Expect all the NodeClaims that existed on the initial provisioning loop are not removed.
			// Assert this over several minutes to ensure a subsequent disruption controller pass doesn't
			// successfully schedule the evicted pods to the in-flight nodeclaim and disrupt the original node
			Consistently(func(g Gomega) {
				nodeClaims := &v1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
				startingNodeClaimUIDs := sets.New(lo.Map(startingNodeClaimState, func(nc *v1.NodeClaim, _ int) types.UID { return nc.UID })...)
				nodeClaimUIDs := sets.New(lo.Map(nodeClaims.Items, func(nc v1.NodeClaim, _ int) types.UID { return nc.UID })...)
				g.Expect(nodeClaimUIDs.IsSuperset(startingNodeClaimUIDs)).To(BeTrue())
			}, "2m").Should(Succeed())
		})
		It("should not disrupt a drifted node if the replacement node registers but never initialized", func() {
			// launch a new nodeClaim
			var numPods int32 = 2
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: 2,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []corev1.PodAffinityTerm{{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
				},
			})
			// Expect only a single node to get tainted due to default disruption budgets
			nodePool.Spec.Disruption = v1.Disruption{}
			env.ExpectCreated(dep, nodeClass, nodePool)

			startingNodeClaimState := env.EventuallyExpectCreatedNodeClaimCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Drift the nodeClaim with bad configuration that never initializes
			nodePool.Spec.Template.Spec.StartupTaints = []corev1.Taint{{Key: "example.com/taint", Effect: corev1.TaintEffectPreferNoSchedule}}
			env.ExpectCreatedOrUpdated(nodePool)

			env.EventuallyExpectDrifted(startingNodeClaimState...)

			// Expect only a single node to get tainted due to default disruption budgets
			taintedNodes := env.EventuallyExpectTaintedNodeCount("==", 1)

			// Drift should fail and original node should be untainted
			// TODO: reduce timeouts when disruption waits are factored out
			env.EventuallyExpectNodesUntaintedWithTimeout(11*time.Minute, taintedNodes...)

			// Expect that the new nodeClaim/node is kept around after the un-cordon
			nodeList := &corev1.NodeList{}
			Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
			Expect(nodeList.Items).To(HaveLen(int(numPods) + 1))

			nodeClaimList := &v1.NodeClaimList{}
			Expect(env.Client.List(env, nodeClaimList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
			Expect(nodeClaimList.Items).To(HaveLen(int(numPods) + 1))

			// Expect all the NodeClaims that existed on the initial provisioning loop are not removed
			// Assert this over several minutes to ensure a subsequent disruption controller pass doesn't
			// successfully schedule the evicted pods to the in-flight nodeclaim and disrupt the original node
			Consistently(func(g Gomega) {
				nodeClaims := &v1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
				startingNodeClaimUIDs := sets.New(lo.Map(startingNodeClaimState, func(m *v1.NodeClaim, _ int) types.UID { return m.UID })...)
				nodeClaimUIDs := sets.New(lo.Map(nodeClaims.Items, func(m v1.NodeClaim, _ int) types.UID { return m.UID })...)
				g.Expect(nodeClaimUIDs.IsSuperset(startingNodeClaimUIDs)).To(BeTrue())
			}, "2m").Should(Succeed())
		})
		It("should not drift any nodes if their PodDisruptionBudgets are unhealthy", func() {
			// Create a deployment that contains a readiness probe that will never succeed
			// This way, the pod will bind to the node, but the PodDisruptionBudget will never go healthy
			var numPods int32 = 2
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: 2,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                 "inflate",
							"kwok.x-k8s.io/stage": "unhealthy",
						},
					},
					PodAntiRequirements: []corev1.PodAffinityTerm{{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Port: intstr.FromInt32(80),
							},
						},
					},
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			minAvailable := intstr.FromInt32(numPods - 1)
			pdb := test.PodDisruptionBudget(test.PDBOptions{
				Labels:       dep.Spec.Template.Labels,
				MinAvailable: &minAvailable,
			})
			env.ExpectCreated(dep, nodeClass, nodePool, pdb)

			nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Expect pods to be bound but not to be ready since we are intentionally failing the readiness check
			env.EventuallyExpectBoundPodCount(selector, int(numPods))

			// Drift the nodeclaims
			nodePool.Spec.Template.Annotations = map[string]string{"test": "annotation"}
			env.ExpectUpdated(nodePool)

			env.EventuallyExpectDrifted(nodeClaims...)
			env.ConsistentlyExpectNoDisruptions(int(numPods), time.Minute)
		})
	})
})
