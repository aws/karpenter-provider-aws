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
		buffer := test.CapacityBuffer(autoscalingv1alpha1.CapacityBuffer{
			Spec: autoscalingv1alpha1.CapacityBufferSpec{
				PodTemplateRef: &autoscalingv1alpha1.LocalObjectRef{Name: "buffer-template"},
				Replicas:       lo.ToPtr(int32(2)),
			},
		})

		env.ExpectCreated(nodeClass, nodePool, bufferTemplate, buffer)

		env.EventuallyExpectCapacityBufferReplicas(buffer, 2)
		env.EventuallyExpectCreatedNodeClaimCount(">=", 1)
		env.EventuallyExpectInitializedNodeCount(">=", 1)
		env.EventuallyExpectCapacityBufferProvisioned(buffer)
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
		env.EventuallyExpectCapacityBufferProvisioned(buffer)

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
		env.EventuallyExpectCapacityBufferProvisioned(buffer)

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
		env.EventuallyExpectCapacityBufferProvisioned(buffer)

		// Delete buffer — nodes should be cleaned up
		env.ExpectDeleted(buffer)
		env.EventuallyExpectNotFound(nodeClaims[0])
	})

	It("should provision buffer with scalableRef", func() {
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: 5,
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

		env.ExpectCreated(nodeClass, nodePool, dep, buffer)

		// Wait for Deployment pods to schedule
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.EventuallyExpectHealthyPodCountWithTimeout(5*time.Minute, selector, 5)

		// Buffer should resolve: 20% of 5 = 1
		env.EventuallyExpectCapacityBufferReplicas(buffer, 1)
		env.EventuallyExpectCapacityBufferProvisioned(buffer)
	})
})
