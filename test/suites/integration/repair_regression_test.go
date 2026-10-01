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

package integration_test

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	karpenterv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"

	. "github.com/onsi/ginkgo/v2"
	"github.com/samber/lo"
)

// Budgeted-breaker regression coverage for node repair, complementing the single-node cases in repair_policy_test.go.
// These exercise the pool-level 20%-unhealthy breaker, its reset, the do-not-repair veto, and replace-then-terminate.
// The fault is injected as a node-monitoring-agent condition (StorageReady=False, backdated past the toleration): unlike
// NodeReady it is not kubelet-managed, so it holds across a multi-node pool without being reverted.
var _ = Describe("Node Repair Budgeted Breaker", func() {
	var dep *appsv1.Deployment
	var selector labels.Selector

	unhealthy := func(status corev1.ConditionStatus) corev1.NodeCondition {
		return corev1.NodeCondition{
			Type:               "StorageReady",
			Status:             status,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-31 * time.Minute)},
		}
	}
	injectFault := func(node *corev1.Node) {
		node = common.ReplaceNodeConditions(node, unhealthy(corev1.ConditionFalse))
		env.ExpectStatusUpdated(node)
	}
	healFault := func(node *corev1.Node) {
		node = common.ReplaceNodeConditions(node, unhealthy(corev1.ConditionTrue))
		env.ExpectStatusUpdated(node)
	}

	BeforeEach(func() {
		dep = coretest.Deployment(coretest.DeploymentOptions{
			Replicas: 5,
			PodOptions: coretest.PodOptions{
				ObjectMeta:                    metav1.ObjectMeta{Labels: map[string]string{"app": "repair-breaker"}},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](30),
			},
		})
		// Hostname anti-affinity so each of the 5 pods lands on its own node — a fault on K of 5 is exactly K/5 of the pool.
		dep.Spec.Template.Spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{LabelSelector: dep.Spec.Selector, TopologyKey: corev1.LabelHostname},
				},
			},
		}
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})

	It("does not repair when more than 20% of the pool is unhealthy", func() {
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, 5)
		nodes := env.EventuallyExpectCreatedNodeCount("==", 5)
		env.EventuallyExpectInitializedNodeCount("==", 5)

		// 2 of 5 unhealthy (40% > 20%) trips the breaker for the pool: repair must not disrupt any node.
		injectFault(nodes[0])
		injectFault(nodes[1])
		env.ConsistentlyExpectNoDisruptions(5, 2*time.Minute)
	})

	It("resumes repair once the pool drops back under the breaker threshold", func() {
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, 5)
		nodes := env.EventuallyExpectCreatedNodeCount("==", 5)
		env.EventuallyExpectInitializedNodeCount("==", 5)

		// Trip the breaker (2/5 unhealthy) -> repair frozen.
		injectFault(nodes[0])
		injectFault(nodes[1])
		env.ConsistentlyExpectNoDisruptions(5, time.Minute)

		// Heal one -> 1/5 unhealthy is under the threshold -> the breaker resets and the remaining unhealthy node is repaired.
		healFault(nodes[0])
		env.EventuallyExpectNotFound(nodes[1])
		env.EventuallyExpectHealthyPodCount(selector, 5)
	})

	It("does not repair a node carrying the do-not-repair annotation", func() {
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, 5)
		nodes := env.EventuallyExpectCreatedNodeCount("==", 5)
		env.EventuallyExpectInitializedNodeCount("==", 5)

		nodes[0].Annotations = lo.Assign(nodes[0].Annotations, map[string]string{karpenterv1.DoNotRepairAnnotationKey: "true"})
		env.ExpectUpdated(nodes[0])
		injectFault(nodes[0]) // 1 of 5 (under the breaker threshold) — would be repaired, but do-not-repair vetoes it
		env.ConsistentlyExpectNoDisruptions(5, time.Minute)
	})

	It("repairs replace-first: a replacement joins before the unhealthy node is removed", func() {
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, 5)
		nodes := env.EventuallyExpectCreatedNodeCount("==", 5)
		env.EventuallyExpectInitializedNodeCount("==", 5)

		injectFault(nodes[0])                  // 1 of 5 (under the breaker threshold)
		env.EventuallyExpectNodeCount("==", 6) // replacement is up while the original is still terminating (replace-first)
		env.EventuallyExpectNotFound(nodes[0]) // original removed only after the replacement joined
		env.EventuallyExpectHealthyPodCount(selector, 5)
	})
})
