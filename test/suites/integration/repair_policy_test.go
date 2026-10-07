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
	"strings"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpenterv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

var _ = Describe("Repair Policy", func() {
	var selector labels.Selector
	var dep *appsv1.Deployment
	var numPods int

	BeforeEach(func() {
		numPods = 1
		// Repair drains like other voluntary disruption, bounded by the NodePool's TerminationGracePeriod (unset here),
		// so these pods must not carry do-not-disrupt or the drain would never finish
		dep = coretest.Deployment(coretest.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "my-app",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})

	// Core clamps an unhealthy condition's LastTransitionTime to the Node's creation time, so no policy fires until the
	// node is older than the policy's toleration, however far the injected condition is backdated.
	DescribeTable("Conditions", func(unhealthyCondition corev1.NodeCondition) {
		env.ExpectCreated(nodeClass, nodePool, dep)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		// Use the initialized node, otherwise the status update strips the initialized label
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]

		node = common.ReplaceNodeConditions(node, unhealthyCondition)
		env.ExpectStatusUpdated(node)

		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	},
		// Kubelet Ready False/Unknown aren't tested here: the kubelet owns Ready and re-patches it within its status
		// update loop, so an injected value reverts before the disruption loop can act. Core's KWOK suite covers them.
		// Node Monitoring Agent Supported Conditions
		Entry("Node AcceleratedHardwareReady False", corev1.NodeCondition{
			Type:               "AcceleratedHardwareReady",
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-11 * time.Minute)},
		}),
	)
	It("should repair the Node Monitoring Agent conditions with a 30 minute toleration", func() {
		// Each condition gets its own NodePool so the conditions share one 30 minute wait without tripping the repair
		// breaker, which stops repair in a pool once more than 20% (rounded up) of its nodes are unhealthy
		conditionTypes := []corev1.NodeConditionType{"StorageReady", "NetworkingReady", "KernelReady", "ContainerRuntimeReady"}
		var selectors []labels.Selector
		var pods []*corev1.Pod
		var repaired []client.Object
		env.ExpectCreated(nodeClass)
		for _, conditionType := range conditionTypes {
			pool := env.DefaultNodePool(nodeClass)
			d := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: 1,
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "repair-" + strings.ToLower(string(conditionType))},
					},
					NodeSelector:                  map[string]string{karpenterv1.NodePoolLabelKey: pool.Name},
					TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
				},
			})
			env.ExpectCreated(pool, d)
			selectors = append(selectors, labels.SelectorFromSet(d.Spec.Selector.MatchLabels))
			pods = append(pods, env.EventuallyExpectHealthyPodCount(selectors[len(selectors)-1], 1)[0])
		}
		env.EventuallyExpectInitializedNodeCount("==", len(conditionTypes))
		for i, conditionType := range conditionTypes {
			node := env.GetNode(pods[i].Spec.NodeName)
			node = *common.ReplaceNodeConditions(&node, corev1.NodeCondition{
				Type:               conditionType,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Time{Time: time.Now().Add(-31 * time.Minute)},
			})
			env.ExpectStatusUpdated(&node)
			repaired = append(repaired, pods[i], &node)
		}

		// The default timeout is shorter than the toleration
		Eventually(func(g Gomega) {
			for _, object := range repaired {
				g.Expect(errors.IsNotFound(env.Client.Get(env, client.ObjectKeyFromObject(object), object))).To(BeTrue())
			}
		}).WithTimeout(45 * time.Minute).Should(Succeed())
		for _, s := range selectors {
			env.EventuallyExpectHealthyPodCount(s, 1)
		}
	})
	It("should terminate the unhealthy nodeclaim before launching its replacement when the reservation is full", func() {
		capacityReservationID := aws.ExpectCapacityReservationCreated(
			env.Context,
			env.EC2API,
			ec2types.InstanceTypeM5Large,
			env.ZoneInfo[0].Zone,
			1,
			nil,
			nil,
		)
		DeferCleanup(func() {
			aws.ExpectCapacityReservationsCanceled(env.Context, env.EC2API, capacityReservationID)
		})

		nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: capacityReservationID}}
		nodePool = coretest.ReplaceRequirements(nodePool, karpenterv1.NodeSelectorRequirementWithMinValues{
			Key:      karpenterv1.CapacityTypeLabelKey,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{karpenterv1.CapacityTypeReserved},
		})
		env.ExpectCreated(nodeClass, nodePool, dep)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		// Use the initialized node, otherwise the status update strips the initialized label
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, capacityReservationID))

		// Use a Node Monitoring Agent condition, the kubelet reverts Ready before repair observes it. AcceleratedHardwareReady
		// has the shortest toleration, which keeps the spec inside the default timeout.
		node = common.ReplaceNodeConditions(node, corev1.NodeCondition{
			Type:               "AcceleratedHardwareReady",
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-11 * time.Minute)},
		})
		env.ExpectStatusUpdated(node)

		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
})
