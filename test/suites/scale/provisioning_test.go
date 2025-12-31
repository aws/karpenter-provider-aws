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

package scale_test

import (
	"context"
	"strconv"
	"time"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/pkg/debug"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
)

const testGroup = "provisioning"

var _ = Describe("Provisioning", Label(debug.NoWatch), Label(debug.NoEvents), func() {
	var nodePool *karpv1.NodePool
	var nodeClass *v1.EC2NodeClass
	var deployment *appsv1.Deployment
	var selector labels.Selector
	var dsCount int

	BeforeEach(func() {
		nodeClass = env.DefaultEC2NodeClass()
		nodePool = env.DefaultNodePool(nodeClass)
		nodePool.Spec.Limits = nil
		nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
			Nodes: "70%",
		}}
		test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
			Key:      v1.LabelInstanceHypervisor,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{"nitro"},
		})
		deployment = test.Deployment(test.DeploymentOptions{
			PodOptions: test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector = labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
		// Get the DS pod count and use it to calculate the DS pod overhead
		dsCount = env.GetDaemonSetCount(nodePool)
	})
	It("should scale successfully on a node-dense scale-up", Label(debug.NoEvents), func(_ context.Context) {
		// Disable Prefix Delegation for the node-dense scale-up to not exhaust the IPs
		// This is required because of the number of Warm ENIs that will be created and the number of IPs
		// that will be allocated across this large number of nodes, despite the fact that the ENI CIDR space will
		// be extremely under-utilized
		env.ExpectPrefixDelegationDisabled()
		DeferCleanup(func() {
			env.ExpectPrefixDelegationEnabled()
		})

		replicasPerNode := 1
		expectedNodeCount := 500
		replicas := replicasPerNode * expectedNodeCount

		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		// Hostname anti-affinity to require one pod on each node
		deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: deployment.Spec.Selector,
						TopologyKey:   corev1.LabelHostname,
					},
				},
			},
		}

		By("waiting for the deployment to deploy all of its pods")
		env.ExpectCreated(deployment)
		env.EventuallyExpectPendingPodCount(selector, replicas)

		env.MeasureProvisioningDurationFor(func() {
			By("kicking off provisioning by applying the nodePool and nodeClass")
			env.ExpectCreated(nodePool, nodeClass)

			env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
			env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)
		}, map[string]string{
			aws.TestCategoryDimension:           testGroup,
			aws.TestNameDimension:               "node-dense",
			aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
			aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
			aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
		})
	}, SpecTimeout(time.Minute*30))
	It("should scale successfully on a node-dense scale-up with minValues in the NodePool requirement", Label(debug.NoEvents), func(_ context.Context) {
		// Disable Prefix Delegation for the node-dense scale-up to not exhaust the IPs
		// This is required because of the number of Warm ENIs that will be created and the number of IPs
		// that will be allocated across this large number of nodes, despite the fact that the ENI CIDR space will
		// be extremely under-utilized
		env.ExpectPrefixDelegationDisabled()
		DeferCleanup(func() {
			env.ExpectPrefixDelegationEnabled()
		})

		replicasPerNode := 1
		expectedNodeCount := 500
		replicas := replicasPerNode * expectedNodeCount

		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		// Hostname anti-affinity to require one pod on each node
		deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: deployment.Spec.Selector,
						TopologyKey:   corev1.LabelHostname,
					},
				},
			},
		}

		test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
			// minValues is restricted to 30 to have enough instance types to be sent to launch API and not make this test flaky.
			Key:       corev1.LabelInstanceTypeStable,
			Operator:  corev1.NodeSelectorOpExists,
			MinValues: lo.ToPtr(30),
		})

		By("waiting for the deployment to deploy all of its pods")
		env.ExpectCreated(deployment)
		env.EventuallyExpectPendingPodCount(selector, replicas)

		env.MeasureProvisioningDurationFor(func() {
			By("kicking off provisioning by applying the nodePool and nodeClass")
			env.ExpectCreated(nodePool, nodeClass)

			env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
			env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)
		}, map[string]string{
			aws.TestCategoryDimension:           testGroup,
			aws.TestNameDimension:               "node-dense-min-val",
			aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
			aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
			aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
		})
	}, SpecTimeout(time.Minute*30))
	It("should scale successfully on a pod-dense scale-up", func(_ context.Context) {
		replicasPerNode := 110
		maxPodDensity := replicasPerNode + dsCount
		expectedNodeCount := 60
		replicas := replicasPerNode * expectedNodeCount
		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
			MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
		}
		test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
			// With Prefix Delegation enabled, .large instances can have 434 pods.
			Key:      v1.LabelInstanceSize,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{"large"},
		},
		)

		env.MeasureProvisioningDurationFor(func() {
			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			By("kicking off provisioning by applying the nodePool and nodeClass")
			env.ExpectCreated(nodePool, nodeClass)

			env.EventuallyExpectHealthyPodCount(selector, replicas)
		}, map[string]string{
			aws.TestCategoryDimension:           testGroup,
			aws.TestNameDimension:               "pod-dense",
			aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
			aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
			aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
		})
	}, SpecTimeout(time.Minute*30))
	It("should scale successfully on a pod-dense scale-up with minValues in the NodePool requirement", func(_ context.Context) {
		replicasPerNode := 110
		maxPodDensity := replicasPerNode + dsCount
		expectedNodeCount := 60
		replicas := replicasPerNode * expectedNodeCount
		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
			MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
		}
		test.ReplaceRequirements(nodePool,
			karpv1.NodeSelectorRequirementWithMinValues{
				// With Prefix Delegation enabled, .large instances can have 434 pods.
				Key:      v1.LabelInstanceSize,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"large"},
			},
			karpv1.NodeSelectorRequirementWithMinValues{
				// minValues is restricted to 30 to have enough instance types to be sent to launch API and not make this test flaky.
				Key:       corev1.LabelInstanceTypeStable,
				Operator:  corev1.NodeSelectorOpExists,
				MinValues: lo.ToPtr(30),
			},
		)

		env.MeasureProvisioningDurationFor(func() {
			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			By("kicking off provisioning by applying the nodePool and nodeClass")
			env.ExpectCreated(nodePool, nodeClass)

			env.EventuallyExpectHealthyPodCount(selector, replicas)
		}, map[string]string{
			aws.TestCategoryDimension:           testGroup,
			aws.TestNameDimension:               "pod-dense-min-val",
			aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
			aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
			aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
		})
	}, SpecTimeout(time.Minute*30))
})
