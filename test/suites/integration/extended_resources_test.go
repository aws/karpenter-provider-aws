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
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Extended Resources", func() {
	It("should provision nodes for a deployment that requests nvidia.com/gpu", func() {
		ExpectNvidiaDevicePluginCreated()
		DeferCleanup(func() {
			ExpectNvidiaDevicePluginDeleted()
		})

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				},
			},
		})
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
		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectCreatedNodesInitialized()
	})
	It("should provision nodes for a deployment that requests nvidia.com/gpu (Bottlerocket)", func() {
		// For Bottlerocket, we are testing that resources are initialized without needing a device plugin
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			AMIFamily:             &v1alpha1.AMIFamilyBottlerocket,
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				},
			},
		})
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
		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectCreatedNodesInitialized()
	})
	It("should provision nodes for a deployment that requests vpc.amazonaws.com/pod-eni (security groups for pods)", func() {
		ExpectPodENIEnabled()
		DeferCleanup(func() {
			ExpectPodENIDisabled()
		})
		env.ExpectSettingsOverridden(map[string]string{
			"aws.enablePodENI": "true",
		})
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				},
			},
		})
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
		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectCreatedNodesInitialized()
	})
	It("should provision nodes for a deployment that requests amd.com/gpu", func() {
		ExpectAMDDevicePluginCreated()
		DeferCleanup(func() {
			ExpectAMDDevicePluginDeleted()
		})

		content, err := os.ReadFile("testdata/amd_driver_input.golden")
		Expect(err).ToNot(HaveOccurred())
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		},
			UserData: aws.String(string(content)),
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				},
			},
		})
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
		env.ExpectCreated(provisioner, provider, dep)
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Monitor.RunningPodsCount(selector)).To(Equal(numPods))
		}).WithTimeout(10 * time.Minute).Should(Succeed()) // The node needs additional time to install the AMD GPU driver
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectCreatedNodesInitialized()
	})
})

func ExpectNvidiaDevicePluginCreated() {
	env.ExpectCreatedWithOffset(1, &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nvidia-device-plugin-daemonset",
			Namespace: "kube-system",
		},
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
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "nvidia-device-plugin-ds",
					},
				},
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

func ExpectNvidiaDevicePluginDeleted() {
	env.ExpectDeletedWithOffset(1, &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nvidia-device-plugin-daemonset",
			Namespace: "kube-system",
		},
	})
}

func ExpectAMDDevicePluginCreated() {
	env.ExpectCreatedWithOffset(1, &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "amdgpu-device-plugin-daemonset",
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "amdgpu-dp-ds",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "amdgpu-dp-ds",
					},
				},
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

func ExpectAMDDevicePluginDeleted() {
	env.ExpectDeletedWithOffset(1, &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "amdgpu-device-plugin-daemonset",
			Namespace: "kube-system",
		},
	})
}

func ExpectPodENIEnabled() {
	env.ExpectDaemonSetEnvironmentVariableUpdatedWithOffset(1, types.NamespacedName{Namespace: "kube-system", Name: "aws-node"},
		"ENABLE_POD_ENI", "true")
}

func ExpectPodENIDisabled() {
	env.ExpectDaemonSetEnvironmentVariableUpdatedWithOffset(1, types.NamespacedName{Namespace: "kube-system", Name: "aws-node"},
		"ENABLE_POD_ENI", "false")
}
