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
	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/test/pkg/debug"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduling", Ordered, ContinueOnFailure, func() {
	var selectors sets.Set[string]

	BeforeEach(func() {
		// Make the NodePool requirements fully flexible, so we can match well-known label keys
		nodePool = test.ReplaceRequirements(nodePool,
			corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1beta1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				},
			},
			corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1beta1.LabelInstanceGeneration,
					Operator: v1.NodeSelectorOpExists,
				},
			},
		)
	})
	BeforeAll(func() {
		selectors = sets.New[string]()
	})
	AfterAll(func() {
		// Ensure that we're exercising all well known labels
		Expect(lo.Keys(selectors)).To(ContainElements(append(corev1beta1.WellKnownLabels.UnsortedList(), lo.Keys(corev1beta1.NormalizedLabels)...)))
	})

	It("should apply annotations to the node", func() {
		nodePool.Spec.Template.Annotations = map[string]string{
			"foo":                                 "bar",
			corev1beta1.DoNotDisruptAnnotationKey: "true",
		}
		pod := test.Pod()
		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		Expect(env.GetNode(pod.Spec.NodeName).Annotations).To(And(HaveKeyWithValue("foo", "bar"), HaveKeyWithValue(corev1beta1.DoNotDisruptAnnotationKey, "true")))
	})

	Context("Labels", func() {
		It("should support well-known labels for instance type selection", func() {
			nodeSelector := map[string]string{
				// Well Known
				corev1beta1.NodePoolLabelKey: nodePool.Name,
				v1.LabelInstanceTypeStable:   "c5.large",
				// Well Known to AWS
				v1beta1.LabelInstanceHypervisor:       "nitro",
				v1beta1.LabelInstanceCategory:         "c",
				v1beta1.LabelInstanceGeneration:       "5",
				v1beta1.LabelInstanceFamily:           "c5",
				v1beta1.LabelInstanceSize:             "large",
				v1beta1.LabelInstanceCPU:              "2",
				v1beta1.LabelInstanceCPUManufacturer:  "intel",
				v1beta1.LabelInstanceMemory:           "4096",
				v1beta1.LabelInstanceEBSBandwidth:     "4750",
				v1beta1.LabelInstanceNetworkBandwidth: "750",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for zone id selection", func() {
			selectors.Insert(v1beta1.LabelTopologyZoneID) // Add node selector keys to selectors used in testing to ensure we test all labels
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1beta1.LabelTopologyZoneID,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{env.GetSubnetInfo(map[string]string{"karpenter.sh/discovery": env.ClusterName})[0].ZoneID},
					},
				},
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for local NVME storage", func() {
			selectors.Insert(v1beta1.LabelInstanceLocalNVME) // Add node selector keys to selectors used in testing to ensure we test all labels
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodePreferences: []v1.NodeSelectorRequirement{
					{
						Key:      v1beta1.LabelInstanceLocalNVME,
						Operator: v1.NodeSelectorOpGt,
						Values:   []string{"0"},
					},
				},
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1beta1.LabelInstanceLocalNVME,
						Operator: v1.NodeSelectorOpGt,
						Values:   []string{"0"},
					},
				},
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for encryption in transit", func() {
			selectors.Insert(v1beta1.LabelInstanceEncryptionInTransitSupported) // Add node selector keys to selectors used in testing to ensure we test all labels
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodePreferences: []v1.NodeSelectorRequirement{
					{
						Key:      v1beta1.LabelInstanceEncryptionInTransitSupported,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"true"},
					},
				},
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1beta1.LabelInstanceEncryptionInTransitSupported,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"true"},
					},
				},
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known deprecated labels", func() {
			nodeSelector := map[string]string{
				// Deprecated Labels
				v1.LabelFailureDomainBetaRegion: env.Region,
				v1.LabelFailureDomainBetaZone:   fmt.Sprintf("%sa", env.Region),
				"topology.ebs.csi.aws.com/zone": fmt.Sprintf("%sa", env.Region),

				"beta.kubernetes.io/arch": "amd64",
				"beta.kubernetes.io/os":   "linux",
				v1.LabelInstanceType:      "c5.large",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for topology and architecture", func() {
			nodeSelector := map[string]string{
				// Well Known
				corev1beta1.NodePoolLabelKey:     nodePool.Name,
				v1.LabelTopologyRegion:           env.Region,
				v1.LabelTopologyZone:             fmt.Sprintf("%sa", env.Region),
				v1.LabelOSStable:                 "linux",
				v1.LabelArchStable:               "amd64",
				corev1beta1.CapacityTypeLabelKey: corev1beta1.CapacityTypeOnDemand,
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for a gpu (nvidia)", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1beta1.AMIFamilyAL2)
			nodeSelector := map[string]string{
				v1beta1.LabelInstanceGPUName:         "t4",
				v1beta1.LabelInstanceGPUMemory:       "16384",
				v1beta1.LabelInstanceGPUManufacturer: "nvidia",
				v1beta1.LabelInstanceGPUCount:        "1",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for an accelerator (inferentia)", func() {
			nodeSelector := map[string]string{
				v1beta1.LabelInstanceAcceleratorName:         "inferentia",
				v1beta1.LabelInstanceAcceleratorManufacturer: "aws",
				v1beta1.LabelInstanceAcceleratorCount:        "1",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for windows-build version", func() {
			env.ExpectWindowsIPAMEnabled()
			DeferCleanup(func() {
				env.ExpectWindowsIPAMDisabled()
			})

			nodeSelector := map[string]string{
				// Well Known
				v1.LabelWindowsBuild: v1beta1.Windows2022Build,
				v1.LabelOSStable:     string(v1.Windows), // Specify the OS to enable vpc-resource-controller to inject the PrivateIPv4Address resource
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
				Image:            aws.WindowsDefaultImage,
			}})
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyWindows2022
			// TODO: remove this requirement once VPC RC rolls out m7a.*, r7a.* ENI data (https://github.com/aws/karpenter-provider-aws/issues/4472)
			test.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceFamily,
						Operator: v1.NodeSelectorOpNotIn,
						Values:   aws.ExcludedInstanceFamilies,
					},
				},
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1.LabelOSStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{string(v1.Windows)},
					},
				},
			)
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCountWithTimeout(time.Minute*15, labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		DescribeTable("should support restricted label domain exceptions", func(domain string) {
			// Assign labels to the nodepool so that it has known values
			test.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: domain + "/team", Operator: v1.NodeSelectorOpExists}},
				corev1beta1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: domain + "/custom-label", Operator: v1.NodeSelectorOpExists}},
				corev1beta1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: "subdomain." + domain + "/custom-label", Operator: v1.NodeSelectorOpExists}},
			)
			nodeSelector := map[string]string{
				domain + "/team":                        "team-1",
				domain + "/custom-label":                "custom-value",
				"subdomain." + domain + "/custom-label": "custom-value",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			node := env.ExpectCreatedNodeCount("==", 1)[0]
			// Ensure that the requirements/labels specified above are propagated onto the node
			for k, v := range nodeSelector {
				Expect(node.Labels).To(HaveKeyWithValue(k, v))
			}
		},
			Entry("node-restriction.kuberentes.io", "node-restriction.kuberentes.io"),
			Entry("node.kubernetes.io", "node.kubernetes.io"),
			Entry("kops.k8s.io", "kops.k8s.io"),
		)
	})

	Context("Provisioning", func() {
		It("should provision a node for naked pods", func() {
			pod := test.Pod()

			env.ExpectCreated(nodeClass, nodePool, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should provision a node for a deployment", Label(debug.NoWatch), Label(debug.NoEvents), func() {
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 50})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("<=", 2) // should probably all land on a single node, but at worst two depending on batching
		})
		It("should provision a node for a self-affinity deployment", func() {
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

			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), 2)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should provision three nodes for a zonal topology spread", func() {
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
							MinDomains:        lo.ToPtr(int32(3)),
						},
					},
				},
			})

			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(podLabels), 3)
			// Karpenter will launch three nodes, however if all three nodes don't get register with the cluster at the same time, two pods will be placed on one node.
			// This can result in a case where all 3 pods are healthy, while there are only two created nodes.
			// In that case, we still expect to eventually have three nodes.
			env.EventuallyExpectNodeCount("==", 3)
		})
		It("should provision a node using a NodePool with higher priority", func() {
			nodePoolLowPri := test.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Weight: lo.ToPtr(int32(10)),
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{
								Name: nodeClass.Name,
							},
							Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1.LabelOSStable,
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{string(v1.Linux)},
									},
								},
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceTypeStable,
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"t3.nano"},
									},
								},
							},
						},
					},
				},
			})
			nodePoolHighPri := test.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Weight: lo.ToPtr(int32(100)),
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{
								Name: nodeClass.Name,
							},
							Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1.LabelOSStable,
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{string(v1.Linux)},
									},
								},
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceTypeStable,
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"c5.large"},
									},
								},
							},
						},
					},
				},
			})
			pod := test.Pod()
			env.ExpectCreated(pod, nodeClass, nodePoolLowPri, nodePoolHighPri)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			Expect(lo.FromPtr(env.GetInstance(pod.Spec.NodeName).InstanceType)).To(Equal("c5.large"))
			Expect(env.GetNode(pod.Spec.NodeName).Labels[corev1beta1.NodePoolLabelKey]).To(Equal(nodePoolHighPri.Name))
		})

		DescribeTable(
			"should provision a right-sized node when a pod has InitContainers (cpu)",
			func(expectedNodeCPU string, containerRequirements v1.ResourceRequirements, initContainers ...v1.Container) {
				if env.K8sMinorVersion() < 29 {
					Skip("native sidecar containers are only enabled on EKS 1.29+")
				}

				labels := map[string]string{"test": test.RandomName()}
				// Create a buffer pod to even out the total resource requests regardless of the daemonsets on the cluster. Assumes
				// CPU is the resource in contention and that total daemonset CPU requests <= 3.
				dsBufferPod := test.Pod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					PodRequirements: []v1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						TopologyKey: v1.LabelHostname,
					}},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU: func() resource.Quantity {
								dsOverhead := env.GetDaemonSetOverhead(nodePool)
								base := lo.ToPtr(resource.MustParse("3"))
								base.Sub(*dsOverhead.Cpu())
								return *base
							}(),
						},
					},
				})

				test.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceCPU,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"4", "8"},
					},
				}, corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpNotIn,
						Values:   []string{"t"},
					},
				})
				pod := test.Pod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					PodRequirements: []v1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						TopologyKey: v1.LabelHostname,
					}},
					InitContainers:       initContainers,
					ResourceRequirements: containerRequirements,
				})
				env.ExpectCreated(nodePool, nodeClass, dsBufferPod, pod)
				env.EventuallyExpectHealthy(pod)
				node := env.ExpectCreatedNodeCount("==", 1)[0]
				Expect(node.ObjectMeta.GetLabels()[v1beta1.LabelInstanceCPU]).To(Equal(expectedNodeCPU))
			},
			Entry("sidecar requirements + later init requirements do exceed container requirements", "8", v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("400m")},
			}, ephemeralInitContainer(v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("300m")},
			}), v1.Container{
				RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("350m")},
				},
			}, ephemeralInitContainer(v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
			})),
			Entry("sidecar requirements + later init requirements do not exceed container requirements", "4", v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("400m")},
			}, ephemeralInitContainer(v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("300m")},
			}), v1.Container{
				RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("350m")},
				},
			}, ephemeralInitContainer(v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("300m")},
			})),
			Entry("init container requirements exceed all later requests", "8", v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("400m")},
			}, v1.Container{
				RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("100m")},
				},
			}, ephemeralInitContainer(v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1500m")},
			}), v1.Container{
				RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("100m")},
				},
			}),
		)
		It("should provision a right-sized node when a pod has InitContainers (mixed resources)", func() {
			if env.K8sMinorVersion() < 29 {
				Skip("native sidecar containers are only enabled on EKS 1.29+")
			}
			test.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1beta1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpNotIn,
					Values:   []string{"t"},
				},
			})
			pod := test.Pod(test.PodOptions{
				InitContainers: []v1.Container{
					{
						RestartPolicy: lo.ToPtr(v1.ContainerRestartPolicyAlways),
						Resources: v1.ResourceRequirements{Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						}},
					},
					ephemeralInitContainer(v1.ResourceRequirements{Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("50m"),
						v1.ResourceMemory: resource.MustParse("4Gi"),
					}}),
				},
				ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("128Mi"),
				}},
			})
			env.ExpectCreated(nodePool, nodeClass, pod)
			env.EventuallyExpectHealthy(pod)
		})

		It("should provision a node for a pod with overlapping zone and zone-id requirements", func() {
			subnetInfo := lo.UniqBy(env.GetSubnetInfo(map[string]string{"karpenter.sh/discovery": env.ClusterName}), func(s aws.SubnetInfo) string {
				return s.Zone
			})
			Expect(len(subnetInfo)).To(BeNumerically(">=", 3))

			// Create a pod with 'overlapping' zone and zone-id requirements. With two options for each label, but only one pair of zone-zoneID that maps to the
			// same AZ, we will always expect the pod to be scheduled to that AZ. In this case, this is the mapping at zone[1].
			pod := test.Pod(test.PodOptions{
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelTopologyZone,
						Operator: v1.NodeSelectorOpIn,
						Values:   lo.Map(subnetInfo[0:2], func(info aws.SubnetInfo, _ int) string { return info.Zone }),
					},
					{
						Key:      v1beta1.LabelTopologyZoneID,
						Operator: v1.NodeSelectorOpIn,
						Values:   lo.Map(subnetInfo[1:3], func(info aws.SubnetInfo, _ int) string { return info.ZoneID }),
					},
				},
			})
			env.ExpectCreated(nodePool, nodeClass, pod)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			Expect(node.Labels[v1.LabelTopologyZone]).To(Equal(subnetInfo[1].Zone))
			Expect(node.Labels[v1beta1.LabelTopologyZoneID]).To(Equal(subnetInfo[1].ZoneID))
		})
		It("should provision nodes for pods with zone-id requirements in the correct zone", func() {
			// Each pod specifies a requirement on this expected zone, where the value is the matching zone for the
			// required zone-id. This allows us to verify that Karpenter launched the node in the correct zone, even if
			// it doesn't add the zone-id label and the label is added by CCM. If we didn't take this approach, we would
			// succeed even if Karpenter doesn't add the label and /or incorrectly generated offerings on k8s 1.30 and
			// above. This is an unlikely scenario, and adding this check is a defense in depth measure.
			const expectedZoneLabel = "expected-zone-label"
			test.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      expectedZoneLabel,
					Operator: v1.NodeSelectorOpExists,
				},
			})

			subnetInfo := lo.UniqBy(env.GetSubnetInfo(map[string]string{"karpenter.sh/discovery": env.ClusterName}), func(s aws.SubnetInfo) string {
				return s.Zone
			})
			pods := lo.Map(subnetInfo, func(info aws.SubnetInfo, _ int) *v1.Pod {
				return test.Pod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{
							Key:      expectedZoneLabel,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{info.Zone},
						},
						{
							Key:      v1beta1.LabelTopologyZoneID,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{info.ZoneID},
						},
					},
				})
			})

			env.ExpectCreated(nodePool, nodeClass)
			for _, pod := range pods {
				env.ExpectCreated(pod)
			}
			nodes := env.EventuallyExpectInitializedNodeCount("==", len(subnetInfo))
			for _, node := range nodes {
				expectedZone, ok := node.Labels[expectedZoneLabel]
				Expect(ok).To(BeTrue())
				Expect(node.Labels[v1.LabelTopologyZone]).To(Equal(expectedZone))
				zoneInfo, ok := lo.Find(subnetInfo, func(info aws.SubnetInfo) bool {
					return info.Zone == expectedZone
				})
				Expect(ok).To(BeTrue())
				Expect(node.Labels[v1beta1.LabelTopologyZoneID]).To(Equal(zoneInfo.ZoneID))
			}
		})
	})
})

func ephemeralInitContainer(requirements v1.ResourceRequirements) v1.Container {
	return v1.Container{
		Image:     aws.EphemeralInitContainerImage,
		Command:   []string{"/bin/sh"},
		Args:      []string{"-c", "sleep 5"},
		Resources: requirements,
	}
}
