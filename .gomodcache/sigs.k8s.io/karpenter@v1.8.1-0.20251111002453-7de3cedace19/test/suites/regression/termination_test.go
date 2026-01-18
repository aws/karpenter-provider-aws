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
	"time"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/karpenter/pkg/test"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

var _ = Describe("Termination", func() {
	Context("Emptiness", func() {
		var dep *appsv1.Deployment
		var selector labels.Selector
		var numPods int
		BeforeEach(func() {
			nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmpty
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")

			numPods = 1
			dep = test.Deployment(test.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "large-app"},
					},
				},
			})
			selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		})
		Context("Budgets", func() {
			It("should not allow emptiness if the budget is fully blocking", func() {
				// We're going to define a budget that doesn't allow any emptiness disruption to happen
				nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
					Nodes: "0",
				}}

				env.ExpectCreated(nodeClass, nodePool, dep)

				nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
				env.EventuallyExpectCreatedNodeCount("==", 1)
				env.EventuallyExpectHealthyPodCount(selector, numPods)

				// Delete the deployment so there is nothing running on the node
				env.ExpectDeleted(dep)

				env.EventuallyExpectConsolidatable(nodeClaim)
				env.ConsistentlyExpectNoDisruptions(1, time.Minute)
			})
			It("should not allow emptiness if the budget is fully blocking during a scheduled time", func() {
				// We're going to define a budget that doesn't allow any emptiness disruption to happen
				// This is going to be on a schedule that only lasts 30 minutes, whose window starts 15 minutes before
				// the current time and extends 15 minutes past the current time
				// Times need to be in UTC since the karpenter containers were built in UTC time
				windowStart := time.Now().Add(-time.Minute * 15).UTC()
				nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
					Nodes:    "0",
					Schedule: lo.ToPtr(fmt.Sprintf("%d %d * * *", windowStart.Minute(), windowStart.Hour())),
					Duration: &metav1.Duration{Duration: time.Minute * 30},
				}}

				env.ExpectCreated(nodeClass, nodePool, dep)

				nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
				env.EventuallyExpectCreatedNodeCount("==", 1)
				env.EventuallyExpectHealthyPodCount(selector, numPods)

				// Delete the deployment so there is nothing running on the node
				env.ExpectDeleted(dep)

				env.EventuallyExpectConsolidatable(nodeClaim)
				env.ConsistentlyExpectNoDisruptions(1, time.Minute)
			})
		})
		It("should terminate an empty node", func() {
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("10s")

			const numPods = 1
			deployment := test.Deployment(test.DeploymentOptions{Replicas: numPods})

			By("kicking off provisioning for a deployment")
			env.ExpectCreated(nodeClass, nodePool, deployment)
			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), numPods)

			By("making the nodeclaim empty")
			persisted := deployment.DeepCopy()
			deployment.Spec.Replicas = lo.ToPtr(int32(0))
			Expect(env.Client.Patch(env, deployment, client.StrategicMergeFrom(persisted))).To(Succeed())

			env.EventuallyExpectConsolidatable(nodeClaim)

			By("waiting for the nodeclaim to deprovision when past its ConsolidateAfter timeout of 0")
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
			env.ExpectUpdated(nodePool)

			env.EventuallyExpectNotFound(nodeClaim, node)
		})
	})
	Describe("TerminationGracePeriod", func() {
		BeforeEach(func() {
			nodePool.Spec.Template.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 60}
		})
		It("should delete pod with do-not-disrupt when it reaches its terminationGracePeriodSeconds", func() {
			pod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				karpv1.DoNotDisruptAnnotationKey: "true",
			}}, TerminationGracePeriodSeconds: lo.ToPtr(int64(30))})
			env.ExpectCreated(nodeClass, nodePool, pod)

			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
			env.EventuallyExpectHealthy(pod)

			// Delete the nodeclaim to start the TerminationGracePeriod
			env.ExpectDeleted(nodeClaim)

			// Eventually the node will be tainted
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).Should(Succeed())
				_, ok := lo.Find(node.Spec.Taints, func(t corev1.Taint) bool {
					return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
				})
				g.Expect(ok).To(BeTrue())
				//Reduced polling time from 100 to 50 to mitigate flakes
				//TODO Investigate root cause of timing sensitivity and restructure test
			}).WithTimeout(3 * time.Second).WithPolling(50 * time.Millisecond).Should(Succeed())

			// Check that pod remains healthy until termination grace period
			// subtracting 5s is close enough to say that we waited for the entire terminationGracePeriod
			// and to stop us flaking from tricky timing bugs
			env.ConsistentlyExpectHealthyPods(time.Duration(lo.FromPtr(pod.Spec.TerminationGracePeriodSeconds)-5)*time.Second, pod)

			// Both nodeClaim and node should be gone once terminationGracePeriod is reached
			env.EventuallyExpectNotFound(nodeClaim, node, pod)
		})
		It("should delete pod that has a pre-stop hook after termination grace period seconds", func() {
			pod := test.UnschedulablePod(test.PodOptions{
				PreStopSleep:                  lo.ToPtr(int64(300)),
				TerminationGracePeriodSeconds: lo.ToPtr(int64(30)),
				Image:                         "alpine:3.20.2",
				Command:                       []string{"/bin/sh", "-c", "sleep 30"},
			})
			env.ExpectCreated(nodeClass, nodePool, pod)

			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
			env.EventuallyExpectHealthy(pod)

			// Delete the nodeclaim to start the TerminationGracePeriod
			env.ExpectDeleted(nodeClaim)

			// Eventually the node will be tainted
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).Should(Succeed())
				_, ok := lo.Find(node.Spec.Taints, func(t corev1.Taint) bool {
					return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
				})
				g.Expect(ok).To(BeTrue())
			}).WithTimeout(3 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

			env.EventuallyExpectTerminating(pod)

			// Check that pod remains healthy until termination grace period
			// subtracting 5s is close enough to say that we waited for the entire terminationGracePeriod
			// and to stop us flaking from tricky timing bugs
			env.ConsistentlyExpectHealthyPods(time.Duration(lo.FromPtr(pod.Spec.TerminationGracePeriodSeconds)-5)*time.Second, pod)

			// Both nodeClaim and node should be gone once terminationGracePeriod is reached
			env.EventuallyExpectNotFound(nodeClaim, node, pod)
		})
	})
	It("should terminate the node and the instance on deletion", func() {
		pod := test.Pod()
		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		nodes := env.Monitor.CreatedNodes()

		// Pod is deleted so that we don't re-provision after node deletion
		// NOTE: We have to do this right now to deal with a race condition in nodepool ownership
		// This can be removed once this race is resolved with the NodePool
		env.ExpectDeleted(pod)

		// Node is deleted and now should be not found
		env.ExpectDeleted(nodes[0])
		env.EventuallyExpectNotFound(nodes[0])
	})
	// Pods from Karpenter nodes are expected to drain in the following order:
	//   1. Non-Critical Non-Daemonset pods
	//   2. Non-Critical Daemonset pods
	//   3. Critical Non-Daemonset pods
	//   4. Critical Daemonset pods
	// Pods in one group are expected to be fully removed before the next group is executed
	It("should drain pods on a node in order", func() {
		daemonSet := test.DaemonSet(test.DaemonSetOptions{
			Selector: map[string]string{"app": "non-critical-daemonset"},
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"drain-test": "true",
						"app":        "daemonset",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr(int64(60)),
				Image:                         "alpine:3.20.2",
				Command:                       []string{"/bin/sh", "-c", "sleep 1000"},
				PreStopSleep:                  lo.ToPtr(int64(60)),
				ResourceRequirements:          corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}},
			},
		})
		nodeCriticalDaemonSet := test.DaemonSet(test.DaemonSetOptions{
			Selector: map[string]string{"app": "critical-daemonset"},
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"drain-test": "true",
						"app":        "node-critical-daemonset",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr(int64(10)), // shorter terminationGracePeriod since it's the last pod
				Image:                         "alpine:3.20.2",
				Command:                       []string{"/bin/sh", "-c", "sleep 1000"},
				PreStopSleep:                  lo.ToPtr(int64(10)), // shorter preStopSleep since it's the last pod
				PriorityClassName:             "system-node-critical",
				ResourceRequirements:          corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}},
			},
		})
		clusterCriticalDaemonSet := test.DaemonSet(test.DaemonSetOptions{
			Selector: map[string]string{"app": "critical-daemonset"},
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"drain-test": "true",
						"app":        "cluster-critical-daemonset",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr(int64(10)), // shorter terminationGracePeriod since it's the last pod
				Image:                         "alpine:3.20.2",
				Command:                       []string{"/bin/sh", "-c", "sleep 1000"},
				PreStopSleep:                  lo.ToPtr(int64(10)), // shorter preStopSleep since it's the last pod
				PriorityClassName:             "system-cluster-critical",
				ResourceRequirements:          corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}},
			},
		})
		deployment := test.Deployment(test.DeploymentOptions{
			Replicas: int32(1),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"drain-test": "true",
						"app":        "deployment",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr(int64(60)),
				Image:                         "alpine:3.20.2",
				Command:                       []string{"/bin/sh", "-c", "sleep 1000"},
				PreStopSleep:                  lo.ToPtr(int64(60)),
				ResourceRequirements:          corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}},
			},
		})
		nodeCriticalDeployment := test.Deployment(test.DeploymentOptions{
			Replicas: int32(1),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"drain-test": "true",
						"app":        "node-critical-deployment",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr(int64(60)),
				Image:                         "alpine:3.20.2",
				Command:                       []string{"/bin/sh", "-c", "sleep 1000"},
				PreStopSleep:                  lo.ToPtr(int64(60)),
				PriorityClassName:             "system-node-critical",
				ResourceRequirements:          corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}},
			},
		})
		clusterCriticalDeployment := test.Deployment(test.DeploymentOptions{
			Replicas: int32(1),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"drain-test": "true",
						"app":        "cluster-critical-deployment",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr(int64(60)),
				Image:                         "alpine:3.20.2",
				Command:                       []string{"/bin/sh", "-c", "sleep 1000"},
				PreStopSleep:                  lo.ToPtr(int64(60)),
				PriorityClassName:             "system-cluster-critical",
				ResourceRequirements:          corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}},
			},
		})
		env.ExpectCreated(nodeClass, nodePool, daemonSet, nodeCriticalDaemonSet, clusterCriticalDaemonSet, deployment, nodeCriticalDeployment, clusterCriticalDeployment)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		_ = env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(map[string]string{"drain-test": "true"}), 6)

		daemonsetPod := env.ExpectPodsMatchingSelector(labels.SelectorFromSet(map[string]string{"app": "daemonset"}))[0]
		nodeCriticalDaemonsetPod := env.ExpectPodsMatchingSelector(labels.SelectorFromSet(map[string]string{"app": "node-critical-daemonset"}))[0]
		clusterCriticalDaemonsetPod := env.ExpectPodsMatchingSelector(labels.SelectorFromSet(map[string]string{"app": "cluster-critical-daemonset"}))[0]
		deploymentPod := env.ExpectPodsMatchingSelector(labels.SelectorFromSet(map[string]string{"app": "deployment"}))[0]
		nodeCriticalDeploymentPod := env.ExpectPodsMatchingSelector(labels.SelectorFromSet(map[string]string{"app": "node-critical-deployment"}))[0]
		clusterCriticalDeploymentPod := env.ExpectPodsMatchingSelector(labels.SelectorFromSet(map[string]string{"app": "cluster-critical-deployment"}))[0]

		env.ExpectDeleted(nodeClaim)

		// Wait for non-critical deployment pod to drain and delete
		env.EventuallyExpectTerminating(deploymentPod)
		// We check that other pods are live for 30s since pre-stop sleep and terminationGracePeriod are 60s
		env.ConsistentlyExpectActivePods(time.Second*30, daemonsetPod, nodeCriticalDeploymentPod, nodeCriticalDaemonsetPod, clusterCriticalDeploymentPod, clusterCriticalDaemonsetPod)
		env.EventuallyExpectNotFound(deploymentPod)

		// Wait for non-critical daemonset pod to drain and delete
		env.EventuallyExpectTerminating(daemonsetPod)
		// We check that other pods are live for 30s since pre-stop sleep and terminationGracePeriod are 60s
		env.ConsistentlyExpectActivePods(time.Second*30, nodeCriticalDeploymentPod, nodeCriticalDaemonsetPod, clusterCriticalDeploymentPod, clusterCriticalDaemonsetPod)
		env.EventuallyExpectNotFound(daemonsetPod)

		// Wait for critical deployment pod to drain and delete
		env.EventuallyExpectTerminating(nodeCriticalDeploymentPod, clusterCriticalDeploymentPod)
		// We check that other pods are live for 30s since pre-stop sleep and terminationGracePeriod are 60s
		env.ConsistentlyExpectActivePods(time.Second*30, nodeCriticalDaemonsetPod, clusterCriticalDaemonsetPod)
		env.EventuallyExpectNotFound(nodeCriticalDeploymentPod, clusterCriticalDeploymentPod)

		// Wait for critical daemonset pod to drain and delete
		env.EventuallyExpectTerminating(nodeCriticalDaemonsetPod, clusterCriticalDaemonsetPod)
		env.EventuallyExpectNotFound(nodeCriticalDaemonsetPod, clusterCriticalDaemonsetPod)
	})
})
