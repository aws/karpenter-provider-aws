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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/common"
)

var _ = Describe("Scheduling", func() {
	It("should support well known labels", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef:  &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{{Key: v1alpha1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists}},
		})
		nodeSelector := map[string]string{
			// Well Known
			v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
			v1.LabelTopologyRegion:           env.Region,
			v1.LabelTopologyZone:             fmt.Sprintf("%sa", env.Region),
			v1.LabelInstanceTypeStable:       "g4dn.8xlarge",
			v1.LabelOSStable:                 "linux",
			v1.LabelArchStable:               "amd64",
			v1alpha5.LabelCapacityType:       "on-demand",
			// Well Known to AWS
			v1alpha1.LabelInstanceHypervisor:      "nitro",
			v1alpha1.LabelInstanceCategory:        "g",
			v1alpha1.LabelInstanceGeneration:      "4",
			v1alpha1.LabelInstanceFamily:          "g4dn",
			v1alpha1.LabelInstanceSize:            "8xlarge",
			v1alpha1.LabelInstanceCPU:             "32",
			v1alpha1.LabelInstanceMemory:          "131072",
			v1alpha1.LabelInstancePods:            "58", // May vary w/ environment
			v1alpha1.LabelInstanceGPUName:         "t4",
			v1alpha1.LabelInstanceGPUManufacturer: "nvidia",
			v1alpha1.LabelInstanceGPUCount:        "1",
			v1alpha1.LabelInstanceGPUMemory:       "16384",
			v1alpha1.LabelInstanceLocalNVME:       "900",
			// Deprecated Labels
			v1.LabelFailureDomainBetaZone:   fmt.Sprintf("%sa", env.Region),
			v1.LabelFailureDomainBetaRegion: env.Region,
			"beta.kubernetes.io/arch":       "amd64",
			"beta.kubernetes.io/os":         "linux",
			v1.LabelInstanceType:            "g4dn.8xlarge",
		}
		requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
			return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
		})
		deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
			NodeSelector:     nodeSelector,
			NodePreferences:  requirements,
			NodeRequirements: requirements,
		}})
		// Ensure that we're exercising all well known labels
		Expect(lo.Keys(nodeSelector)).To(ContainElements(append(v1alpha5.WellKnownLabels.UnsortedList(), lo.Keys(v1alpha5.NormalizedLabels)...)))
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should provision a node for naked pods", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(provisioner, provider, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should provision a node for a deployment", Label(common.NoWatch), Label(common.NoEvents), func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})

		deployment := test.Deployment(test.DeploymentOptions{Replicas: 50})
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("<=", 2) // should probably all land on a single node, but at worst two depending on batching
	})
	It("should provision a node for a self-affinity deployment", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		// just two pods as they all need to land on the same node
		podLabels := map[string]string{"test": "self-affinity"}
		deployment := test.Deployment(test.DeploymentOptions{
			Replicas: 2,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				PodRequirements: []v1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: podLabels},
						TopologyKey:   v1.LabelHostname,
					},
				},
			},
		})

		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), 2)
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should provision three nodes for a zonal topology spread", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})

		// one pod per zone
		podLabels := map[string]string{"test": "zonal-spread"}
		deployment := test.Deployment(test.DeploymentOptions{
			Replicas: 3,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       v1.LabelTopologyZone,
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector:     &metav1.LabelSelector{MatchLabels: podLabels},
					},
				},
			},
		})

		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(podLabels), 3)
		env.ExpectCreatedNodeCount("==", 3)
	})
	It("should provision a node using a provisioner with higher priority", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})

		provisionerLowPri := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Weight:      ptr.Int32(10),
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"t3.nano"},
				},
			},
		})
		provisionerHighPri := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Weight:      ptr.Int32(100),
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"c4.large"},
				},
			},
		})

		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisionerLowPri, provisionerHighPri)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		Expect(ptr.StringValue(env.GetInstance(pod.Spec.NodeName).InstanceType)).To(Equal("c4.large"))
		Expect(env.GetNode(pod.Spec.NodeName).Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisionerHighPri.Name))
	})
	Context("Extended Resources", func() {
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

func ExpectPodENIEnabled() {
	env.ExpectDaemonSetEnvironmentVariableUpdatedWithOffset(1, types.NamespacedName{Namespace: "kube-system", Name: "aws-node"},
		"ENABLE_POD_ENI", "true")
}

func ExpectPodENIDisabled() {
	env.ExpectDaemonSetEnvironmentVariableUpdatedWithOffset(1, types.NamespacedName{Namespace: "kube-system", Name: "aws-node"},
		"ENABLE_POD_ENI", "false")
}
