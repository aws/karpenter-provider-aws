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

package termination_test

import (
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("Termination", func() {
	It("should terminate the node and the instance on deletion", func() {
		pod := test.Pod()
		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		nodes := env.Monitor.CreatedNodes()
		instanceID := env.ExpectParsedProviderID(nodes[0].Spec.ProviderID)
		env.GetInstance(nodes[0].Name)

		// Pod is deleted so that we don't re-provision after node deletion
		// NOTE: We have to do this right now to deal with a race condition in nodepool ownership
		// This can be removed once this race is resolved with the NodePool
		env.ExpectDeleted(pod)

		// Node is deleted and now should be not found
		env.ExpectDeleted(nodes[0])
		env.EventuallyExpectNotFound(nodes[0])
		Eventually(func(g Gomega) {
			g.Expect(env.GetInstanceByID(instanceID).State.Name).To(BeElementOf(ec2types.InstanceStateNameTerminated, ec2types.InstanceStateNameShuttingDown))
		}, time.Second*10).Should(Succeed())
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
