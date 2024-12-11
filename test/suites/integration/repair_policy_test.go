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

var _ = Describe("Repair Policy", func() {
	var selector labels.Selector
	var dep *appsv1.Deployment
	var numPods int
	var unhealthyCondition corev1.NodeCondition

	BeforeEach(func() {
		unhealthyCondition = corev1.NodeCondition{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-31 * time.Minute)},
		}
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
						karpenterv1.DoNotDisruptAnnotationKey: "true",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})

	DescribeTable("Conditions", func(unhealthyCondition corev1.NodeCondition) {
		env.ExpectCreated(nodeClass, nodePool, dep)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectInitializedNodeCount("==", 1)

		node = common.ReplaceNodeConditions(node, unhealthyCondition)
		env.ExpectStatusUpdated(node)

		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	},
		Entry("Node Ready False", corev1.NodeCondition{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-31 * time.Minute)},
		}),
		Entry("Node Ready Unknown", corev1.NodeCondition{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionUnknown,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-31 * time.Minute)},
		}),
	)
	It("should ignore disruption budgets", func() {
		nodePool.Spec.Disruption.Budgets = []karpenterv1.Budget{
			{
				Nodes: "0",
			},
		}
		env.ExpectCreated(nodeClass, nodePool, dep)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectInitializedNodeCount("==", 1)

		node = common.ReplaceNodeConditions(node, unhealthyCondition)
		env.ExpectStatusUpdated(node)

		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should ignore do-not-disrupt annotation on node", func() {
		env.ExpectCreated(nodeClass, nodePool, dep)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectInitializedNodeCount("==", 1)

		node.Annotations[karpenterv1.DoNotDisruptAnnotationKey] = "true"
		env.ExpectUpdated(node)

		node = common.ReplaceNodeConditions(node, unhealthyCondition)
		env.ExpectStatusUpdated(node)

		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should ignore terminationGracePeriod on the nodepool", func() {
		nodePool.Spec.Template.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Hour}
		env.ExpectCreated(nodeClass, nodePool, dep)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectInitializedNodeCount("==", 1)

		node = common.ReplaceNodeConditions(node, unhealthyCondition)
		env.ExpectStatusUpdated(node)

		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
})
