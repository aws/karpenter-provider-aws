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

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	. "github.com/onsi/ginkgo/v2"

	autoscalingv1alpha1 "sigs.k8s.io/karpenter/pkg/apis/autoscaling/v1alpha1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("CapacityBuffer", func() {
	var bufferTemplate *corev1.PodTemplate

	BeforeEach(func() {
		bufferTemplate = &corev1.PodTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "buffer-template",
				Namespace: "default",
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "pause",
						Image: "public.ecr.aws/eks-distro/kubernetes/pause:3.2",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					}},
				},
			},
		}
	})

	It("should provision buffer capacity with podTemplateRef", func() {
		// Create nodeClass and nodePool — no nodes yet
		env.ExpectCreated(nodeClass, nodePool)

		// Record baseline (0 nodes)
		nodeClaimsBefore := env.EventuallyExpectCreatedNodeClaimCount("==", 0)
		countBefore := len(nodeClaimsBefore)

		// Apply buffer — should drive node provisioning from zero
		buffer := test.CapacityBuffer(autoscalingv1alpha1.CapacityBuffer{
			Spec: autoscalingv1alpha1.CapacityBufferSpec{
				PodTemplateRef: &autoscalingv1alpha1.LocalObjectRef{Name: "buffer-template"},
				Replicas:       lo.ToPtr(int32(3)),
			},
		})
		env.ExpectCreated(bufferTemplate, buffer)

		EventuallyExpectCapacityBufferReplicas(env, env.Client, buffer, 3)

		// Buffer must create at least 1 new node from the zero baseline
		env.EventuallyExpectCreatedNodeClaimCount(">=", countBefore+1)
		env.EventuallyExpectInitializedNodeCount(">=", countBefore+1)
		EventuallyExpectCapacityBufferProvisioned(env, env.Client, buffer)
	})

	It("should allow consumer pods to schedule on buffer capacity", func() {
		buffer := test.CapacityBuffer(autoscalingv1alpha1.CapacityBuffer{
			Spec: autoscalingv1alpha1.CapacityBufferSpec{
				PodTemplateRef: &autoscalingv1alpha1.LocalObjectRef{Name: "buffer-template"},
				Replicas:       lo.ToPtr(int32(2)),
			},
		})

		env.ExpectCreated(nodeClass, nodePool, bufferTemplate, buffer)

		env.EventuallyExpectInitializedNodeCount(">=", 1)
		EventuallyExpectCapacityBufferProvisioned(env, env.Client, buffer)

		// Deploy consumer pods that match buffer shape
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: 1,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "consumer"}},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
		})
		env.ExpectCreated(dep)

		// Consumer should schedule quickly on pre-existing capacity
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.EventuallyExpectHealthyPodCountWithTimeout(2*time.Minute, selector, 1)
	})

	It("should not disrupt buffer nodes when empty", func() {
		nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
		nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")

		buffer := test.CapacityBuffer(autoscalingv1alpha1.CapacityBuffer{
			Spec: autoscalingv1alpha1.CapacityBufferSpec{
				PodTemplateRef: &autoscalingv1alpha1.LocalObjectRef{Name: "buffer-template"},
				Replicas:       lo.ToPtr(int32(1)),
			},
		})

		env.ExpectCreated(nodeClass, nodePool, bufferTemplate, buffer)

		nodes := env.EventuallyExpectInitializedNodeCount(">=", 1)
		EventuallyExpectCapacityBufferProvisioned(env, env.Client, buffer)

		// Buffer node should NOT be consolidated as empty
		env.ConsistentlyExpectNoDisruptions(len(nodes), 60*time.Second)
	})

	It("should clean up buffer nodes after buffer deletion", func() {
		nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
		nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")

		buffer := test.CapacityBuffer(autoscalingv1alpha1.CapacityBuffer{
			Spec: autoscalingv1alpha1.CapacityBufferSpec{
				PodTemplateRef: &autoscalingv1alpha1.LocalObjectRef{Name: "buffer-template"},
				Replicas:       lo.ToPtr(int32(1)),
			},
		})

		env.ExpectCreated(nodeClass, nodePool, bufferTemplate, buffer)

		nodeClaims := env.EventuallyExpectCreatedNodeClaimCount(">=", 1)
		env.EventuallyExpectInitializedNodeCount(">=", 1)
		EventuallyExpectCapacityBufferProvisioned(env, env.Client, buffer)

		// Delete buffer — nodes should be cleaned up
		env.ExpectDeleted(buffer)
		env.EventuallyExpectNotFound(nodeClaims[0])
	})

	It("should provision buffer with scalableRef", func() {
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: 10,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "scalable-app"}},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
		})

		// Create Deployment first, wait for pods to schedule
		env.ExpectCreated(nodeClass, nodePool, dep)
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.EventuallyExpectHealthyPodCountWithTimeout(5*time.Minute, selector, 10)

		// Record node count before buffer
		nodeClaimsBefore := env.EventuallyExpectCreatedNodeClaimCount(">=", 1)
		countBefore := len(nodeClaimsBefore)

		// Apply buffer — 20% of 10 = 2 replicas
		buffer := test.CapacityBuffer(autoscalingv1alpha1.CapacityBuffer{
			Spec: autoscalingv1alpha1.CapacityBufferSpec{
				ScalableRef: &autoscalingv1alpha1.ScalableRef{
					APIGroup: "apps",
					Kind:     "Deployment",
					Name:     dep.Name,
				},
				Percentage: lo.ToPtr(int32(20)),
			},
		})
		env.ExpectCreated(buffer)

		// Buffer resolves correctly: 20% of 10 = 2
		EventuallyExpectCapacityBufferReplicas(env, env.Client, buffer, 2)

		// Buffer reaches Provisioning=True (virtual pods fit on existing or new capacity)
		EventuallyExpectCapacityBufferProvisioned(env, env.Client, buffer)

		// At minimum, node count must not have decreased
		env.EventuallyExpectCreatedNodeClaimCount(">=", countBefore)
	})
})
