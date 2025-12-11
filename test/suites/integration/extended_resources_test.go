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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/karpenter/pkg/test"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/amazon-vpc-resource-controller-k8s/apis/vpcresources/v1beta1"

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
	It("should provision nodes for a deployment that requests aws.amazon.com/neuron", func() {
		ExpectNeuronDevicePluginCreated()
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						// Only 1 is requested to avoid the use of the Neuron scheduler
						// TODO: bryantbiggs@ add the ability to specify the scheduler name to test.PodOptions in order to use the Neuron scheduler
						"aws.amazon.com/neuron": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						"aws.amazon.com/neuron": resource.MustParse("1"),
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
		test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceGeneration,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"1", "2"},
			},
		})
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectInitializedNodeCount("==", 1)
	})
	It("should provision nodes for a deployment that requests aws.amazon.com/neuroncore", func() {
		ExpectNeuronDevicePluginCreated()
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						// Only 1 is requested to avoid the use of the Neuron scheduler
						// TODO: bryantbiggs@ add the ability to specify the scheduler name to test.PodOptions in order to use the Neuron scheduler
						"aws.amazon.com/neuroncore": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						"aws.amazon.com/neuroncore": resource.MustParse("1"),
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
		test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceGeneration,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"1", "2"},
			},
		})
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
		env.ExpectCreated(nodeClass) // Creating the nodeclass first to discover the security groups

		// evenutally expect the status on the nodeclass to be hydrated
		Eventually(func(g Gomega) {
			nodeClass = env.ExpectExists(nodeClass).(*v1.EC2NodeClass)
			g.Expect(len(nodeClass.Status.SecurityGroups)).To(BeNumerically(">", 0))
		}).Should(Succeed())
		securityGroupIDs := lo.Map(nodeClass.Status.SecurityGroups, func(sg v1.SecurityGroup, _ int) string {
			return sg.ID
		})

		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		sgp := &v1beta1.SecurityGroupPolicy{
			ObjectMeta: test.NamespacedObjectMeta(),
			Spec: v1beta1.SecurityGroupPolicySpec{
				PodSelector: metav1.SetAsLabelSelector(dep.Spec.Selector.MatchLabels),
				SecurityGroups: v1beta1.GroupIds{
					Groups: securityGroupIDs,
				},
			},
		}

		env.ExpectCreated(nodePool, dep, sgp)
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

		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
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
						"vpc.amazonaws.com/efa": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						"vpc.amazonaws.com/efa": resource.MustParse("1"),
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

// https://github.com/aws-neuron/aws-neuron-sdk/blob/master/src/k8/k8s-neuron-device-plugin.yml
func ExpectNeuronDevicePluginCreated() {
	GinkgoHelper()

	// When selecting more than 1 neuron/neuroncore but less than ALL of the neuron/neuroncores on the instance,
	// you must use the Neuron scheduler to schedule neuron/neuroncores in a contiguous manner.
	// https://awsdocs-neuron.readthedocs-hosted.com/en/latest/containers/kubernetes-getting-started.html#neuron-scheduler-extension
	ExpectK8sNeuronSchedulerCreated()
	ExpectNeuronSchedulerExtensionCreated()

	neuronDevicePlugin := "neuron-device-plugin"

	env.ExpectCreatedOrUpdated(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: neuronDevicePlugin,
		},
		Rules: []rbacv1.PolicyRule{
			// Device plugin
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
			// Scheduler
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"create", "get", "list", "update"},
			},
		},
	})

	env.ExpectCreatedOrUpdated(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: neuronDevicePlugin,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     neuronDevicePlugin,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      neuronDevicePlugin,
				Namespace: "kube-system",
			},
		},
	})

	env.ExpectCreatedOrUpdated(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      neuronDevicePlugin,
			Namespace: "kube-system",
		},
	})

	env.ExpectCreated(&appsv1.DaemonSet{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      neuronDevicePlugin,
			Namespace: "kube-system",
		}),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": neuronDevicePlugin,
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{
						"name": neuronDevicePlugin,
					},
				}),
				Spec: corev1.PodSpec{
					ServiceAccountName: neuronDevicePlugin,
					Tolerations: []corev1.Toleration{
						{
							Key:      "aws.amazon.com/neuron",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					Containers: []corev1.Container{
						{
							Name:  neuronDevicePlugin,
							Image: "public.ecr.aws/neuron/neuron-device-plugin:2.22.4.0",
							Env: []corev1.EnvVar{
								{
									Name:  "KUBECONFIG",
									Value: "/etc/kubernetes/kubelet.conf",
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
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
								{
									Name:      "infa-map",
									MountPath: "/run",
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
						{
							Name: "infa-map",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
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

// https://github.com/aws-neuron/aws-neuron-sdk/blob/master/src/k8/k8s-neuron-scheduler-eks.yml
func ExpectK8sNeuronSchedulerCreated() {
	GinkgoHelper()

	k8sNeuronScheduler := "k8s-neuron-scheduler"

	env.ExpectCreatedOrUpdated(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sNeuronScheduler,
			Namespace: "kube-system",
		},
	})

	env.ExpectCreatedOrUpdated(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sNeuronScheduler,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"node/status"},
				Verbs:     []string{"update", "patch", "get", "list", "watch"},
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
				Resources: []string{"bindings", "pods/bindings"},
				Verbs:     []string{"create"},
			},
		},
	})

	env.ExpectCreatedOrUpdated(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sNeuronScheduler,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     k8sNeuronScheduler,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      k8sNeuronScheduler,
				Namespace: "kube-system",
			},
		},
	})

	env.ExpectCreatedOrUpdated(&corev1.Service{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      k8sNeuronScheduler,
			Namespace: "kube-system",
		}),
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": k8sNeuronScheduler,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       12345,
					TargetPort: intstr.FromInt(12345),
				},
			},
		},
	})

	replicas := int32(1)

	env.ExpectCreatedOrUpdated(&appsv1.Deployment{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      k8sNeuronScheduler,
			Namespace: "kube-system",
		}),
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": k8sNeuronScheduler,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{
						"app": k8sNeuronScheduler,
					},
					Annotations: map[string]string{
						"scheduler.alpha.kubernetes.io/critical-pod": "",
					},
				}),
				Spec: corev1.PodSpec{
					ServiceAccountName: k8sNeuronScheduler,
					PriorityClassName:  "system-node-critical",
					SchedulerName:      k8sNeuronScheduler,
					Tolerations: []corev1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  k8sNeuronScheduler,
							Image: "public.ecr.aws/neuron/neuron-scheduler:2.22.4.0",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 12345,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "PORT",
									Value: "12345",
								},
							},
						},
					},
				},
			},
		},
	})
}

// https://github.com/aws-neuron/aws-neuron-sdk/blob/master/src/k8/my-scheduler.yml
func ExpectNeuronSchedulerExtensionCreated() {
	GinkgoHelper()

	neuronSchedulerExtension := "neuron-scheduler-ext"

	env.ExpectCreatedOrUpdated(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      neuronSchedulerExtension,
			Namespace: "kube-system",
		},
	})

	env.ExpectCreatedOrUpdated(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: neuronSchedulerExtension,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"create", "get", "list", "update"},
			},
		},
	})

	env.ExpectCreatedOrUpdated(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-kube-scheduler", neuronSchedulerExtension),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      neuronSchedulerExtension,
				Namespace: "kube-system",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:kube-scheduler",
		},
	})
	env.ExpectCreatedOrUpdated(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-volume-scheduler", neuronSchedulerExtension),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      neuronSchedulerExtension,
				Namespace: "kube-system",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:volume-scheduler",
		},
	})
	env.ExpectCreatedOrUpdated(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: neuronSchedulerExtension,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      neuronSchedulerExtension,
				Namespace: "kube-system",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     neuronSchedulerExtension,
		},
	})

	env.ExpectCreatedOrUpdated(&corev1.ConfigMap{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", neuronSchedulerExtension),
			Namespace: "kube-system",
		}),
		Data: map[string]string{
			fmt.Sprintf("%s-config.yaml", neuronSchedulerExtension): fmt.Sprintf(`apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
profiles:
  - schedulerName: %[1]v
extenders:
  - urlPrefix: 'http://k8s-neuron-scheduler.kube-system.svc.cluster.local:12345'
    filterVerb: filter
    bindVerb: bind
    enableHTTPS: false
    nodeCacheCapable: true
    managedResources:
      - name: 'aws.amazon.com/neuron'
        ignoredByScheduler: false
      - name: 'aws.amazon.com/neuroncore'
        ignoredByScheduler: false
      - name: 'aws.amazon.com/neurondevice'
        ignoredByScheduler: false
    ignorable: false
leaderElection:
  leaderElect: true
  resourceNamespace: kube-system
  resourceName: %[1]v`, neuronSchedulerExtension),
		},
	})

	replicas := int32(1)

	env.ExpectCreatedOrUpdated(&appsv1.Deployment{
		ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
			Name:      neuronSchedulerExtension,
			Namespace: "kube-system",
			Labels: map[string]string{
				"tier": "control-plane",
			},
		}),
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"tier": "control-plane",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{
					Labels: map[string]string{
						"tier": "control-plane",
					},
				}),
				Spec: corev1.PodSpec{
					ServiceAccountName: neuronSchedulerExtension,
					Tolerations: []corev1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Containers: []corev1.Container{
						{
							Name:    neuronSchedulerExtension,
							Args:    []string{fmt.Sprintf("--config=/etc/kubernetes/%[1]v/%[1]v-config.yaml", neuronSchedulerExtension), "--leader-elect=true", "--v=2"},
							Command: []string{"/usr/local/bin/kube-scheduler"},
							Image:   fmt.Sprintf("public.ecr.aws/eks-distro/kubernetes/kube-scheduler:v1.%[1]v.0-eks-1-%[1]v-latest", env.K8sMinorVersion()),
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: 15,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(10259),
										Scheme: corev1.URISchemeHTTPS,
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(10259),
										Scheme: corev1.URISchemeHTTPS,
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: lo.ToPtr(false),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: fmt.Sprintf("/etc/kubernetes/%s", neuronSchedulerExtension),
									ReadOnly:  true,
								},
							},
						},
					},
					HostNetwork: false,
					HostPID:     false,
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: fmt.Sprintf("%s-config", neuronSchedulerExtension),
									},
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
