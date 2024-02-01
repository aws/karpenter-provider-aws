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
	"fmt"
	"os"
	"time"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	awsenv "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Extended Resources", func() {
	It("should provision nodes for a deployment that requests nvidia.com/gpu", func() {
		ExpectNvidiaDevicePluginCreated()

		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		test.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceCategory,
				Operator: v1.NodeSelectorOpExists,
			},
		})
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests nvidia.com/gpu (Bottlerocket)", func() {
		// For Bottlerocket, we are testing that resources are initialized without needing a device plugin
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		test.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceCategory,
				Operator: v1.NodeSelectorOpExists,
			}})
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests vpc.amazonaws.com/pod-eni (security groups for pods)", func() {
		env.ExpectPodENIEnabled()
		DeferCleanup(func() {
			env.ExpectPodENIDisabled()
		})
		// TODO: remove this requirement once VPC RC rolls out m7a.*, r7a.* ENI data (https://github.com/aws/karpenter-provider-aws/issues/4472)
		test.ReplaceRequirements(nodePool,
			corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1beta1.LabelInstanceFamily,
					Operator: v1.NodeSelectorOpNotIn,
					Values:   awsenv.ExcludedInstanceFamilies,
				},
			},
		)
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"vpc.amazonaws.com/pod-eni": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"vpc.amazonaws.com/pod-eni": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests amd.com/gpu", func() {
		Skip("skipping test on AMD instance types")
		ExpectAMDDevicePluginCreated()

		customAMI := env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 0)

		// We create custom userData that installs the AMD Radeon driver and then performs the EKS bootstrap script
		// We use a Custom AMI so that we can reboot after we start the kubelet service
		rawContent, err := os.ReadFile("testdata/amd_driver_input.sh")
		Expect(err).ToNot(HaveOccurred())
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
			{
				ID: customAMI,
			},
		}
		nodeClass.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle(), nodePool.Name))

		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"amd.com/gpu": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"amd.com/gpu": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, dep)
		Eventually(func(g Gomega) {
			g.Expect(env.Monitor.RunningPodsCount(selector)).To(Equal(numPods))
		}).WithTimeout(15 * time.Minute).Should(Succeed()) // The node needs additional time to install the AMD GPU driver
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	// Need to subscribe to the AMI to run the test successfully
	// https://aws.amazon.com/marketplace/pp/prodview-st5jc2rk3phr2?sr=0-2&ref_=beagle&applicationId=AWSMPContessa
	It("should provision nodes for a deployment that requests habana.ai/gaudi", func() {
		Skip("skipping test on an exotic instance type")
		ExpectHabanaDevicePluginCreated()

		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
			{
				ID: "ami-0fae925f94979981f",
			},
		}
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"habana.ai/gaudi": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"habana.ai/gaudi": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})

	It("should provision nodes for a deployment that requests vpc.amazonaws.com/efa", func() {
		ExpectEFADevicePluginCreated()

		nodePool.Spec.Template.Labels = map[string]string{
			"aws.amazon.com/efa": "true",
		}
		nodePool.Spec.Template.Spec.Taints = []v1.Taint{
			{
				Key:    "aws.amazon.com/efa",
				Effect: v1.TaintEffectNoSchedule,
			},
		}
		// Only select private subnets since instances with multiple network instances at launch won't get a public IP.
		nodeClass.Spec.SubnetSelectorTerms[0].Tags["Name"] = "*Private*"

		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "efa-app"},
				},
				Tolerations: []v1.Toleration{
					{
						Key:      "aws.amazon.com/efa",
						Operator: v1.TolerationOpExists,
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"vpc.amazonaws.com/efa": resource.MustParse("2"),
					},
					Limits: v1.ResourceList{
						"vpc.amazonaws.com/efa": resource.MustParse("2"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
})

func ExpectNvidiaDevicePluginCreated() {
	GinkgoHelper()
	env.ExpectCreated(&appsv1.DaemonSet{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      "nvidia-device-plugin-daemonset",
			Namespace: "kube-system",
		}),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "nvidia-device-plugin-ds",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "nvidia-device-plugin-ds",
					},
				}),
				Spec: v1.PodSpec{
					Tolerations: []v1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					Containers: []v1.Container{
						{
							Name:  "nvidia-device-plugin-ctr",
							Image: "nvcr.io/nvidia/k8s-device-plugin:v0.12.3",
							Env: []v1.EnvVar{
								{
									Name:  "FAIL_ON_INIT_ERROR",
									Value: "false",
								},
							},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: lo.ToPtr(false),
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{"ALL"},
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "device-plugin",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "device-plugin",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
				},
			},
		},
	})
}

func ExpectAMDDevicePluginCreated() {
	GinkgoHelper()
	env.ExpectCreated(&appsv1.DaemonSet{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      "amdgpu-device-plugin-daemonset",
			Namespace: "kube-system",
		}),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "amdgpu-dp-ds",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "amdgpu-dp-ds",
					},
				}),
				Spec: v1.PodSpec{
					PriorityClassName: "system-node-critical",
					Tolerations: []v1.Toleration{
						{
							Key:      "amd.com/gpu",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
					Containers: []v1.Container{
						{
							Name:  "amdgpu-dp-cntr",
							Image: "rocm/k8s-device-plugin",
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: lo.ToPtr(false),
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{"ALL"},
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "dp",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
								{
									Name:      "sys",
									MountPath: "/sys",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "dp",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/device-plugins",
								},
							},
						},
						{
							Name: "sys",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/sys",
								},
							},
						},
					},
				},
			},
		},
	})
}

func ExpectHabanaDevicePluginCreated() {
	GinkgoHelper()
	env.ExpectCreated(&v1.Namespace{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name: "habana-system",
		}),
	})
	env.ExpectCreated(&appsv1.DaemonSet{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      "habanalabs-device-plugin-daemonset",
			Namespace: "habana-system",
		}),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "habanalabs-device-plugin-ds",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Annotations: map[string]string{
						"scheduler.alpha.kubernetes.io/critical-pod": "",
					},
					Labels: map[string]string{
						"name": "habanalabs-device-plugin-ds",
					},
				}),
				Spec: v1.PodSpec{
					Tolerations: []v1.Toleration{
						{
							Key:      "habana.ai/gaudi",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					Containers: []v1.Container{
						{
							Name:  "habanalabs-device-plugin-ctr",
							Image: "vault.habana.ai/docker-k8s-device-plugin/docker-k8s-device-plugin:latest",
							SecurityContext: &v1.SecurityContext{
								Privileged: lo.ToPtr(true),
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "device-plugin",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "device-plugin",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
				},
			},
		},
	})
}

func ExpectEFADevicePluginCreated() {
	GinkgoHelper()
	env.ExpectCreated(&appsv1.DaemonSet{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      "aws-efa-k8s-device-plugin-daemonset",
			Namespace: "kube-system",
		}),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "aws-efa-k8s-device-plugin",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Annotations: map[string]string{
						"scheduler.alpha.kubernetes.io/critical-pod": "",
					},
					Labels: map[string]string{
						"name": "aws-efa-k8s-device-plugin",
					},
				}),
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"aws.amazon.com/efa": "true",
					},
					Tolerations: []v1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: v1.TolerationOpExists,
						},
						{
							Key:      "aws.amazon.com/efa",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					HostNetwork:       true,
					Containers: []v1.Container{
						{
							Name:  "aws-efea-k8s-device-plugin",
							Image: "602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-efa-k8s-device-plugin:v0.3.3",
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: lo.ToPtr(false),
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{"ALL"},
								},
								RunAsNonRoot: lo.ToPtr(false),
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "device-plugin",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "device-plugin",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
				},
			},
		},
	})
}
