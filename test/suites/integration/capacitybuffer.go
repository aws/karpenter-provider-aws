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

package integration

import (
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	. "github.com/onsi/ginkgo/v2"

	autoscalingv1beta1 "sigs.k8s.io/karpenter/pkg/apis/autoscaling/v1beta1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations" //nolint:staticcheck
)

func registerCapacitybuffer() bool {
	return Describe("CapacityBuffer", func() {
		var bufferTemplate *corev1.PodTemplate

		BeforeEach(func() {
			Env.RequireCRD(&autoscalingv1beta1.CapacityBuffer{})
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
			Env.ExpectCreated(NodeClass, NodePool)

			// Record baseline (0 nodes)
			nodeClaimsBefore := Env.EventuallyExpectCreatedNodeClaimCount("==", 0)
			countBefore := len(nodeClaimsBefore)

			// Apply buffer — should drive node provisioning from zero
			buffer := test.CapacityBuffer(autoscalingv1beta1.CapacityBuffer{
				Spec: autoscalingv1beta1.CapacityBufferSpec{
					PodTemplateRef: &autoscalingv1beta1.LocalObjectRef{Name: "buffer-template"},
					Replicas:       lo.ToPtr(int32(3)),
				},
			})
			Env.ExpectCreated(bufferTemplate, buffer)

			EventuallyExpectCapacityBufferReplicas(Env, Env.Client, buffer, 3)

			// Buffer must create at least 1 new node from the zero baseline
			Env.EventuallyExpectCreatedNodeClaimCount(">=", countBefore+1)
			Env.EventuallyExpectInitializedNodeCount(">=", countBefore+1)
			EventuallyExpectCapacityBufferProvisioned(Env, Env.Client, buffer)
		})

		It("should allow consumer pods to schedule on buffer capacity", func() {
			buffer := test.CapacityBuffer(autoscalingv1beta1.CapacityBuffer{
				Spec: autoscalingv1beta1.CapacityBufferSpec{
					PodTemplateRef: &autoscalingv1beta1.LocalObjectRef{Name: "buffer-template"},
					Replicas:       lo.ToPtr(int32(2)),
				},
			})

			Env.ExpectCreated(NodeClass, NodePool, bufferTemplate, buffer)

			Env.EventuallyExpectInitializedNodeCount(">=", 1)
			EventuallyExpectCapacityBufferProvisioned(Env, Env.Client, buffer)

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
			Env.ExpectCreated(dep)

			// Consumer should schedule quickly on pre-existing capacity
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			Env.EventuallyExpectHealthyPodCountWithTimeout(2*time.Minute, selector, 1)
		})

		It("should not disrupt buffer nodes when empty", func() {
			NodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
			NodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")

			buffer := test.CapacityBuffer(autoscalingv1beta1.CapacityBuffer{
				Spec: autoscalingv1beta1.CapacityBufferSpec{
					PodTemplateRef: &autoscalingv1beta1.LocalObjectRef{Name: "buffer-template"},
					Replicas:       lo.ToPtr(int32(1)),
				},
			})

			Env.ExpectCreated(NodeClass, NodePool, bufferTemplate, buffer)

			nodes := Env.EventuallyExpectInitializedNodeCount(">=", 1)
			EventuallyExpectCapacityBufferProvisioned(Env, Env.Client, buffer)

			// Buffer node should NOT be consolidated as empty
			Env.ConsistentlyExpectNoDisruptions(len(nodes), 60*time.Second)
		})

		It("should clean up buffer nodes after buffer deletion", func() {
			NodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
			NodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")

			buffer := test.CapacityBuffer(autoscalingv1beta1.CapacityBuffer{
				Spec: autoscalingv1beta1.CapacityBufferSpec{
					PodTemplateRef: &autoscalingv1beta1.LocalObjectRef{Name: "buffer-template"},
					Replicas:       lo.ToPtr(int32(1)),
				},
			})

			Env.ExpectCreated(NodeClass, NodePool, bufferTemplate, buffer)

			nodeClaims := Env.EventuallyExpectCreatedNodeClaimCount(">=", 1)
			Env.EventuallyExpectInitializedNodeCount(">=", 1)
			EventuallyExpectCapacityBufferProvisioned(Env, Env.Client, buffer)

			// Delete buffer — nodes should be cleaned up
			Env.ExpectDeleted(buffer)
			Env.EventuallyExpectNotFound(nodeClaims[0])
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
			Env.ExpectCreated(NodeClass, NodePool, dep)
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			Env.EventuallyExpectHealthyPodCountWithTimeout(5*time.Minute, selector, 10)

			// Record node count before buffer
			nodeClaimsBefore := Env.EventuallyExpectCreatedNodeClaimCount(">=", 1)
			countBefore := len(nodeClaimsBefore)

			// Apply buffer — 20% of 10 = 2 replicas
			buffer := test.CapacityBuffer(autoscalingv1beta1.CapacityBuffer{
				Spec: autoscalingv1beta1.CapacityBufferSpec{
					ScalableRef: &autoscalingv1beta1.ScalableRef{
						APIGroup: "apps",
						Kind:     "Deployment",
						Name:     dep.Name,
					},
					Percentage: lo.ToPtr(int32(20)),
				},
			})
			Env.ExpectCreated(buffer)

			// Buffer resolves correctly: 20% of 10 = 2
			EventuallyExpectCapacityBufferReplicas(Env, Env.Client, buffer, 2)

			// Buffer reaches Provisioning=True (virtual pods fit on existing or new capacity)
			EventuallyExpectCapacityBufferProvisioned(Env, Env.Client, buffer)

			// At minimum, node count must not have decreased
			Env.EventuallyExpectCreatedNodeClaimCount(">=", countBefore)
		})
	})
}
