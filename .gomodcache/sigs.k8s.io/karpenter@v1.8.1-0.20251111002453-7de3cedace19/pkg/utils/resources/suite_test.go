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

package resources_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

func TestResources(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources")
}

var _ = Describe("Resources", func() {
	Context("Resource Calculations", func() {
		It("should calculate resource requests based off of the sum of containers and sidecarContainers", func() {
			pod := test.Pod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
				},
				InitContainers: []v1.Container{
					{
						RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
						Resources: v1.ResourceRequirements{
							Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("2Gi")},
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("2Gi")},
						},
					},
				},
			})
			podResources := resources.Ceiling(pod)
			ExpectResources(podResources.Requests, v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("3"),
				v1.ResourceMemory: resource.MustParse("3Gi"),
			})
			ExpectResources(podResources.Limits, v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("3"),
				v1.ResourceMemory: resource.MustParse("3Gi"),
			})
		})
		It("should calculate resource requests based off of containers, sidecarContainers, initContainers, and overhead", func() {
			pod := test.Pod(test.PodOptions{
				Overhead: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("5"),
					v1.ResourceMemory: resource.MustParse("1Gi"),
				},
				ResourceRequirements: v1.ResourceRequirements{
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
				},
				InitContainers: []v1.Container{
					{
						Resources: v1.ResourceRequirements{
							Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
						},
					},
					{
						RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
						Resources: v1.ResourceRequirements{
							Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
						},
					},
				},
			})
			podResources := resources.Ceiling(pod)
			ExpectResources(podResources.Requests, v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("10"),
				v1.ResourceMemory: resource.MustParse("5Gi"),
			})
			ExpectResources(podResources.Limits, v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("10"),
				v1.ResourceMemory: resource.MustParse("5Gi"),
			})
		})
		It("should calculate resource requests when there is an initContainer after a sidecarContainer that exceeds container resource requests", func() {
			pod := test.Pod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
				},
				InitContainers: []v1.Container{
					{
						RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
						Resources: v1.ResourceRequirements{
							Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
						},
					},
					{
						Resources: v1.ResourceRequirements{
							Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("2Gi")},
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("2Gi")},
						},
					},
				},
			})
			podResources := resources.Ceiling(pod)
			ExpectResources(podResources.Requests, v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("14"),
				v1.ResourceMemory: resource.MustParse("4Gi"),
			})
			ExpectResources(podResources.Limits, v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("14"),
				v1.ResourceMemory: resource.MustParse("4Gi"),
			})
		})
		It("should calculate resource requests when there is an initContainer after a sidecarContainer that doesn't exceed container resource requests", func() {
			pod := test.Pod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
				},

				InitContainers: []v1.Container{
					{
						RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
						Resources: v1.ResourceRequirements{
							Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
						},
					},
					{
						Resources: v1.ResourceRequirements{
							Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
						},
					},
				},
			})
			podResources := resources.Ceiling(pod)
			ExpectResources(podResources.Requests, v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("6"),
				v1.ResourceMemory: resource.MustParse("4Gi"),
			})
			ExpectResources(podResources.Limits, v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("6"),
				v1.ResourceMemory: resource.MustParse("4Gi"),
			})
		})
		Context("Multiple SidecarContainers", func() {
			It("should calculate resource requests when there is an initContainer after multiple sidecarContainers that exceeds container resource requests", func() {
				pod := test.Pod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
					},
					InitContainers: []v1.Container{
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("20"), v1.ResourceMemory: resource.MustParse("20Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("20"), v1.ResourceMemory: resource.MustParse("20Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("31"),
					v1.ResourceMemory: resource.MustParse("31Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("31"),
					v1.ResourceMemory: resource.MustParse("31Gi"),
				})
			})
			It("should calculate resource requests when there is an initContainer after multiple sidecarContainers that doesn't exceed container resource requests", func() {
				pod := test.Pod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
					},
					InitContainers: []v1.Container{
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("14"),
					v1.ResourceMemory: resource.MustParse("14Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("14"),
					v1.ResourceMemory: resource.MustParse("14Gi"),
				})
			})
			It("should calculate resource requests with multiple sidecarContainers when the first initContainer exceeds the sum of all sidecarContainers and container resource requests", func() {
				pod := test.Pod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
					},
					InitContainers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("25"), v1.ResourceMemory: resource.MustParse("25Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("25"), v1.ResourceMemory: resource.MustParse("25Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("25"),
					v1.ResourceMemory: resource.MustParse("25Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("25"),
					v1.ResourceMemory: resource.MustParse("25Gi"),
				})
			})
			It("should calculate resource requests with multiple interspersed sidecarContainers and initContainers", func() {
				pod := test.Pod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
					},

					InitContainers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
							},
						},

						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("10"),
					v1.ResourceMemory: resource.MustParse("10Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("10"),
					v1.ResourceMemory: resource.MustParse("10Gi"),
				})
			})

		})
		Context("Unequal Resource Requests", func() {
			It("should calculate resource requests when the first initContainer exceeds cpu for sidecarContainers and containers but not memory", func() {
				pod := test.Pod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
					},
					InitContainers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("25"), v1.ResourceMemory: resource.MustParse("4Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("25"), v1.ResourceMemory: resource.MustParse("4Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("25"),
					v1.ResourceMemory: resource.MustParse("9Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("25"),
					v1.ResourceMemory: resource.MustParse("9Gi"),
				})
			})
			It("should calculate resource requests when the first initContainer exceeds memory for sidecarContainers and containers but not cpu", func() {
				pod := test.Pod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
					},
					InitContainers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("25Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("25Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("5"), v1.ResourceMemory: resource.MustParse("5Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("9"),
					v1.ResourceMemory: resource.MustParse("25Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("9"),
					v1.ResourceMemory: resource.MustParse("25Gi"),
				})
			})
			It("should calculate resource requests when there is an initContainer after a sidecarContainer that exceeds cpu for containers but not memory", func() {
				pod := test.Pod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("4Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("4Gi")},
					},
					InitContainers: []v1.Container{
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("2Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("2Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("14"),
					v1.ResourceMemory: resource.MustParse("6Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("14"),
					v1.ResourceMemory: resource.MustParse("6Gi"),
				})
			})
			It("should calculate resource requests when there is an initContainer after a sidecarContainer that exceeds memory for containers but not cpu", func() {
				pod := test.Pod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("2Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("2Gi")},
					},
					InitContainers: []v1.Container{
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("2Gi")},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("4Gi")},
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("4Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("14"),
					v1.ResourceMemory: resource.MustParse("6Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("14"),
					v1.ResourceMemory: resource.MustParse("6Gi"),
				})
			})
		})
		Context("Pod Level Resources", func() {
			It("should calculate resource requests when the pod level resources is specified", func() {
				pod := test.Pod(test.PodOptions{
					PodResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("4"), v1.ResourceMemory: resource.MustParse("4Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
					},
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("3Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("2Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("4"),
					v1.ResourceMemory: resource.MustParse("4Gi"),
				})
			})
			It("should calculate resource requests when only the pod level resources request is specified", func() {
				pod := test.Pod(test.PodOptions{
					PodResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")},
					},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
					},
					InitContainers: []v1.Container{
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("2Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("2Gi"),
				})
			})
			It("should calculate resource requests when the pod level resources requests is defaulted from limits", func() {
				pod := test.Pod(test.PodOptions{
					PodResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("10Gi")}, // simulate the API server’s defaulting from limits
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("10"), v1.ResourceMemory: resource.MustParse("10Gi")},
					},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
					},
					InitContainers: []v1.Container{
						{
							RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
								Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")},
							},
						},
					},
				})
				podResources := resources.Ceiling(pod)
				ExpectResources(podResources.Requests, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("10"),
					v1.ResourceMemory: resource.MustParse("10Gi"),
				})
				ExpectResources(podResources.Limits, v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("10"),
					v1.ResourceMemory: resource.MustParse("10Gi"),
				})
			})
		})
	})
})
