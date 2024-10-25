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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/karpenter/pkg/test"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var _ = Describe("Extended Resources", func() {
	BeforeEach(func() {
		if env.PrivateCluster {
			Skip("skipping Extended Resources test for private cluster")
		}
	})
	It("should provision nodes for a deployment that requests nvidia.com/gpu", func() {
		ExpectNvidiaDevicePluginCreated()
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceCategory,
				Operator: corev1.NodeSelectorOpExists,
			},
		})
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests nvidia.com/gpu (Bottlerocket)", func() {
		// For Bottlerocket, we are testing that resources are initialized without needing a device plugin
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceCategory,
				Operator: corev1.NodeSelectorOpExists,
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
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"vpc.amazonaws.com/pod-eni": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
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

		customAMI := env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", env.K8sVersion()))

		// We create custom userData that installs the AMD Radeon driver and then performs the EKS bootstrap script
		// We use a Custom AMI so that we can reboot after we start the kubelet service
		rawContent, err := os.ReadFile("testdata/amd_driver_input.sh")
		Expect(err).ToNot(HaveOccurred())
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: customAMI}}
		nodeClass.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle(), nodePool.Name))

		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"amd.com/gpu": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
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

		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
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
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"habana.ai/gaudi": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
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
		nodePool.Spec.Template.Spec.Taints = []corev1.Taint{
			{
				Key:    "aws.amazon.com/efa",
				Effect: corev1.TaintEffectNoSchedule,
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
				Tolerations: []corev1.Toleration{
					{
						Key:      "aws.amazon.com/efa",
						Operator: corev1.TolerationOpExists,
					},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"vpc.amazonaws.com/efa": resource.MustParse("2"),
					},
					Limits: corev1.ResourceList{
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
			Template: corev1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "nvidia-device-plugin-ds",
					},
				}),
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					Containers: []corev1.Container{
						{
							Name:  "nvidia-device-plugin-ctr",
							Image: "nvcr.io/nvidia/k8s-device-plugin:v0.12.3",
							Env: []corev1.EnvVar{
								{
									Name:  "FAIL_ON_INIT_ERROR",
									Value: "false",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: lo.ToPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "device-plugin",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "device-plugin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
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
			Template: corev1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "amdgpu-dp-ds",
					},
				}),
				Spec: corev1.PodSpec{
					PriorityClassName: "system-node-critical",
					Tolerations: []corev1.Toleration{
						{
							Key:      "amd.com/gpu",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "amdgpu-dp-cntr",
							Image: "rocm/k8s-device-plugin",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: lo.ToPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
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
					Volumes: []corev1.Volume{
						{
							Name: "dp",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/device-plugins",
								},
							},
						},
						{
							Name: "sys",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
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
	env.ExpectCreated(&corev1.Namespace{
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
			Template: corev1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Annotations: map[string]string{
						"scheduler.alpha.kubernetes.io/critical-pod": "",
					},
					Labels: map[string]string{
						"name": "habanalabs-device-plugin-ds",
					},
				}),
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "habana.ai/gaudi",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					Containers: []corev1.Container{
						{
							Name:  "habanalabs-device-plugin-ctr",
							Image: "vault.habana.ai/docker-k8s-device-plugin/docker-k8s-device-plugin:latest",
							SecurityContext: &corev1.SecurityContext{
								Privileged: lo.ToPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "device-plugin",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "device-plugin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
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
			Template: corev1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Annotations: map[string]string{
						"scheduler.alpha.kubernetes.io/critical-pod": "",
					},
					Labels: map[string]string{
						"name": "aws-efa-k8s-device-plugin",
					},
				}),
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"aws.amazon.com/efa": "true",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
						},
						{
							Key:      "aws.amazon.com/efa",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					HostNetwork:       true,
					Containers: []corev1.Container{
						{
							Name:  "aws-efea-k8s-device-plugin",
							Image: "602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-efa-k8s-device-plugin:v0.3.3",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: lo.ToPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								RunAsNonRoot: lo.ToPtr(false),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "device-plugin",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "device-plugin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
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
