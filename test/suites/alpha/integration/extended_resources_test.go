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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awsenv "github.com/aws/karpenter/test/pkg/environment/aws"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Extended Resources", func() {
	It("should provision nodes for a deployment that requests nvidia.com/gpu", func() {
		ExpectNvidiaDevicePluginCreated()

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
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
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests nvidia.com/gpu (Bottlerocket)", func() {
		// For Bottlerocket, we are testing that resources are initialized without needing a device plugin
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			AMIFamily:             &v1alpha1.AMIFamilyBottlerocket,
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
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
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests vpc.amazonaws.com/pod-eni (security groups for pods)", func() {
		env.ExpectPodENIEnabled()
		DeferCleanup(func() {
			env.ExpectPodENIDisabled()
		})
		env.ExpectSettingsOverriddenLegacy(map[string]string{"aws.enablePodENI": "true"})
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				},
				// TODO: remove this requirement once VPC RC rolls out m7a.*, r7a.* ENI data (https://github.com/aws/karpenter/issues/4472)
				{
					Key:      v1alpha1.LabelInstanceFamily,
					Operator: v1.NodeSelectorOpNotIn,
					Values:   awsenv.ExcludedInstanceFamilies,
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
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily:             &v1alpha1.AMIFamilyCustom,
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			},
			AMISelector: map[string]string{
				"aws-ids": customAMI,
			},
		},
		)
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				},
			},
		})
		provider.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle(), provisioner.Name))

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

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			},
			AMISelector: map[string]string{"aws-ids": "ami-0fae925f94979981f"},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeOnDemand},
				},
				{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"c", "m", "r", "p", "g", "dl"},
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
						"habana.ai/gaudi": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"habana.ai/gaudi": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests aws.amazon.com/neuron", func() {
		ExpectNeuronDevicePluginCreated()

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
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
					Labels: map[string]string{"app": "neuron-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"aws.amazon.com/neuron": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"aws.amazon.com/neuron": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests aws.amazon.com/neurondevice", func() {
		ExpectNeuronDevicePluginCreated()

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
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
					Labels: map[string]string{"app": "neuron-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"aws.amazon.com/neurondevice": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"aws.amazon.com/neurondevice": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests aws.amazon.com/neuroncore", func() {
		ExpectNeuronDevicePluginCreated()

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
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
					Labels: map[string]string{"app": "neuron-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"aws.amazon.com/neurondevice": resource.MustParse("1"),
					},
					Limits: v1.ResourceList{
						"aws.amazon.com/neurondevice": resource.MustParse("1"),
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
})

func ExpectNvidiaDevicePluginCreated() {
	GinkgoHelper()
	env.ExpectCreated(&appsv1.DaemonSet{
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

func ExpectAMDDevicePluginCreated() {
	GinkgoHelper()
	env.ExpectCreated(&appsv1.DaemonSet{
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

func ExpectHabanaDevicePluginCreated() {
	GinkgoHelper()
	env.ExpectCreated(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "habana-system",
		},
	})
	env.ExpectCreated(&appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "habanalabs-device-plugin-daemonset",
			Namespace: "habana-system",
		},
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
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"scheduler.alpha.kubernetes.io/critical-pod": "",
					},
					Labels: map[string]string{
						"name": "habanalabs-device-plugin-ds",
					},
				},
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

func ExpectNeuronDevicePluginCreated() {
	GinkgoHelper()
	env.ExpectCreated(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "neuron-device-plugin",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"update", "patch", "get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/status"},
				Verbs:     []string{"update", "patch"},
			},
		},
	})
	env.ExpectCreated(&v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "neuron-device-plugin",
			Namespace: "kube-system",
		},
	})
	env.ExpectCreated(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "neuron-device-plugin",
			Namespace: "kube-system",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "neuron-device-plugin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "neuron-device-plugin",
				Namespace: "kube-system",
			},
		},
	})
	env.ExpectCreated(&appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "neuron-device-plugin-daemonset",
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "neuron-device-plugin-ds",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "neuron-device-plugin-ds",
					},
				},
				Spec: v1.PodSpec{
					ServiceAccountName: "neuron-device-plugin",
					Tolerations: []v1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: v1.TolerationOpExists,
						},
						{
							Key:      "aws.amazon.com/neuron",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "node.kubernetes.io/instance-type",
												Operator: v1.NodeSelectorOpIn,
												Values: []string{
													"inf1.xlarge",
													"inf1.2xlarge",
													"inf1.6xlarge",
													"inf1.24xlarge",
													"inf2.xlarge",
													"inf2.4xlarge",
													"inf2.8xlarge",
													"inf2.24xlarge",
													"inf2.48xlarge",
													"trn1.2xlarge",
													"trn1.32xlarge",
													"trn1n.32xlarge",
												},
											},
										},
									},
								},
							},
						},
					},
					Containers: []v1.Container{
						{
							Image:           "public.ecr.aws/neuron/neuron-device-plugin:2.17.3.0",
							ImagePullPolicy: v1.PullAlways,
							Name:            "neuron-device-plugin",
							Env: []v1.EnvVar{
								{
									Name:  "KUBECONFIG",
									Value: "/etc/kubernetes/kubelet.conf",
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
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
								{
									Name:      "infa-map",
									MountPath: "/run",
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
						{
							Name: "infa-map",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/run",
								},
							},
						},
					},
				},
			},
		},
	})
}
