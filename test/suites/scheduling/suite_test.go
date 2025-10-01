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

package scheduling_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/pkg/debug"
	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *environmentaws.Environment
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestScheduling(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Scheduling")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })
var _ = Describe("Scheduling", Ordered, ContinueOnFailure, func() {
	var selectors sets.Set[string]

	BeforeEach(func() {
		// Make the NodePool requirements fully flexible, so we can match well-known label keys
		nodePool = test.ReplaceRequirements(nodePool,
			karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceCategory,
					Operator: corev1.NodeSelectorOpExists,
				},
			},
			karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceGeneration,
					Operator: corev1.NodeSelectorOpExists,
				},
			},
		)
	})
	BeforeAll(func() {
		selectors = sets.New[string]()
	})
	AfterAll(func() {
		// Ensure that we're exercising all well known labels
		Expect(lo.Keys(selectors)).To(ContainElements(append(karpv1.WellKnownLabels.UnsortedList(), lo.Keys(karpv1.NormalizedLabels)...)))
	})

	It("should apply annotations to the node", func() {
		nodePool.Spec.Template.Annotations = map[string]string{
			"foo":                            "bar",
			karpv1.DoNotDisruptAnnotationKey: "true",
		}
		pod := test.Pod()
		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		Expect(env.GetNode(pod.Spec.NodeName).Annotations).To(And(HaveKeyWithValue("foo", "bar"), HaveKeyWithValue(karpv1.DoNotDisruptAnnotationKey, "true")))
	})
	It("should ensure the NodePool and the NodeClaim use the same view of the v1beta1 kubelet configuration", func() {
		v1beta1NodePool := &v1beta1.NodePool{}
		Expect(nodePool.ConvertTo(env.Context, v1beta1NodePool)).To(Succeed())

		// Allow a reasonable number of pods to schedule
		v1beta1NodePool.Spec.Template.Spec.Kubelet = &v1beta1.KubeletConfiguration{
			PodsPerCore: lo.ToPtr(int32(2)),
		}
		kubeletHash, err := json.Marshal(v1beta1NodePool.Spec.Template.Spec.Kubelet)
		Expect(err).ToNot(HaveOccurred())
		// Ensure that the v1 version of the NodeClass has a configuration that won't let pods schedule
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
			MaxPods: lo.ToPtr(int32(0)),
		}
		pod := test.Pod()
		env.ExpectCreated(nodeClass, v1beta1NodePool, pod)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(karpv1.KubeletCompatibilityAnnotationKey, string(kubeletHash)))
		env.EventuallyExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectHealthy(pod)
	})

	Context("Labels", func() {
		It("should support well-known labels for instance type selection", func() {
			nodeSelector := map[string]string{
				// Well Known
				karpv1.NodePoolLabelKey:        nodePool.Name,
				corev1.LabelInstanceTypeStable: "c5.large",
				// Well Known to AWS
				v1.LabelInstanceHypervisor:       "nitro",
				v1.LabelInstanceCategory:         "c",
				v1.LabelInstanceGeneration:       "5",
				v1.LabelInstanceFamily:           "c5",
				v1.LabelInstanceSize:             "large",
				v1.LabelInstanceCPU:              "2",
				v1.LabelInstanceCPUManufacturer:  "intel",
				v1.LabelInstanceMemory:           "4096",
				v1.LabelInstanceEBSBandwidth:     "4750",
				v1.LabelInstanceNetworkBandwidth: "750",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) corev1.NodeSelectorRequirement {
				return corev1.NodeSelectorRequirement{Key: key, Operator: corev1.NodeSelectorOpIn, Values: []string{value}}
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
			selectors.Insert(v1.LabelTopologyZoneID) // Add node selector keys to selectors used in testing to ensure we test all labels
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      v1.LabelTopologyZoneID,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{env.GetSubnetInfo(map[string]string{"karpenter.sh/discovery": env.ClusterName})[0].ZoneID},
					},
				},
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for local NVME storage", func() {
			selectors.Insert(v1.LabelInstanceLocalNVME) // Add node selector keys to selectors used in testing to ensure we test all labels
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodePreferences: []corev1.NodeSelectorRequirement{
					{
						Key:      v1.LabelInstanceLocalNVME,
						Operator: corev1.NodeSelectorOpGt,
						Values:   []string{"0"},
					},
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      v1.LabelInstanceLocalNVME,
						Operator: corev1.NodeSelectorOpGt,
						Values:   []string{"0"},
					},
				},
			}})
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for encryption in transit", func() {
			selectors.Insert(v1.LabelInstanceEncryptionInTransitSupported) // Add node selector keys to selectors used in testing to ensure we test all labels
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodePreferences: []corev1.NodeSelectorRequirement{
					{
						Key:      v1.LabelInstanceEncryptionInTransitSupported,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"true"},
					},
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      v1.LabelInstanceEncryptionInTransitSupported,
						Operator: corev1.NodeSelectorOpIn,
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
				corev1.LabelFailureDomainBetaRegion: env.Region,
				corev1.LabelFailureDomainBetaZone:   fmt.Sprintf("%sa", env.Region),
				"topology.ebs.csi.aws.com/zone":     fmt.Sprintf("%sa", env.Region),

				"beta.kubernetes.io/arch": "amd64",
				"beta.kubernetes.io/os":   "linux",
				corev1.LabelInstanceType:  "c5.large",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) corev1.NodeSelectorRequirement {
				return corev1.NodeSelectorRequirement{Key: key, Operator: corev1.NodeSelectorOpIn, Values: []string{value}}
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
				karpv1.NodePoolLabelKey:     nodePool.Name,
				corev1.LabelTopologyRegion:  env.Region,
				corev1.LabelTopologyZone:    fmt.Sprintf("%sa", env.Region),
				corev1.LabelOSStable:        "linux",
				corev1.LabelArchStable:      "amd64",
				karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand,
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) corev1.NodeSelectorRequirement {
				return corev1.NodeSelectorRequirement{Key: key, Operator: corev1.NodeSelectorOpIn, Values: []string{value}}
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
			nodeSelector := map[string]string{
				v1.LabelInstanceGPUName:         "t4",
				v1.LabelInstanceGPUMemory:       "16384",
				v1.LabelInstanceGPUManufacturer: "nvidia",
				v1.LabelInstanceGPUCount:        "1",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) corev1.NodeSelectorRequirement {
				return corev1.NodeSelectorRequirement{Key: key, Operator: corev1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
			}})
			// Use AL2 AMIs instead of AL2023 since accelerated AMIs aren't yet available
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for an accelerator (inferentia)", func() {
			nodeSelector := map[string]string{
				v1.LabelInstanceAcceleratorName:         "inferentia",
				v1.LabelInstanceAcceleratorManufacturer: "aws",
				v1.LabelInstanceAcceleratorCount:        "1",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) corev1.NodeSelectorRequirement {
				return corev1.NodeSelectorRequirement{Key: key, Operator: corev1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
			}})
			// Use AL2 AMIs instead of AL2023 since accelerated AMIs aren't yet available
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		// Windows tests are can flake due to the instance types that are used in testing.
		// The VPC Resource controller will need to support the instance types that are used.
		// If the instance type is not supported by the controller resource `vpc.amazonaws.com/PrivateIPv4Address` will not register.
		// Issue: https://github.com/aws/karpenter-provider-aws/issues/4472
		// See: https://github.com/aws/amazon-vpc-resource-controller-k8s/blob/master/pkg/aws/vpc/limits.go
		It("should support well-known labels for windows-build version", func() {
			env.ExpectWindowsIPAMEnabled()
			DeferCleanup(func() {
				env.ExpectWindowsIPAMDisabled()
			})

			nodeSelector := map[string]string{
				// Well Known
				corev1.LabelWindowsBuild: v1.Windows2022Build,
				corev1.LabelOSStable:     string(corev1.Windows), // Specify the OS to enable vpc-resource-controller to inject the PrivateIPv4Address resource
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) corev1.NodeSelectorRequirement {
				return corev1.NodeSelectorRequirement{Key: key, Operator: corev1.NodeSelectorOpIn, Values: []string{value}}
			})
			deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
				NodeSelector:     nodeSelector,
				NodePreferences:  requirements,
				NodeRequirements: requirements,
				Image:            environmentaws.WindowsDefaultImage,
			}})
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
			test.ReplaceRequirements(nodePool,
				karpv1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelOSStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{string(corev1.Windows)},
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
				karpv1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: domain + "/team", Operator: corev1.NodeSelectorOpExists}},
				karpv1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: domain + "/custom-label", Operator: corev1.NodeSelectorOpExists}},
				karpv1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "subdomain." + domain + "/custom-label", Operator: corev1.NodeSelectorOpExists}},
			)
			nodeSelector := map[string]string{
				domain + "/team":                        "team-1",
				domain + "/custom-label":                "custom-value",
				"subdomain." + domain + "/custom-label": "custom-value",
			}
			selectors.Insert(lo.Keys(nodeSelector)...) // Add node selector keys to selectors used in testing to ensure we test all labels
			requirements := lo.MapToSlice(nodeSelector, func(key string, value string) corev1.NodeSelectorRequirement {
				return corev1.NodeSelectorRequirement{Key: key, Operator: corev1.NodeSelectorOpIn, Values: []string{value}}
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
					PodRequirements: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{MatchLabels: podLabels},
							TopologyKey:   corev1.LabelHostname,
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
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       corev1.LabelTopologyZone,
							WhenUnsatisfiable: corev1.DoNotSchedule,
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
			nodePoolLowPri := test.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
					Weight: lo.ToPtr(int32(10)),
					Template: karpv1.NodeClaimTemplate{
						Spec: karpv1.NodeClaimTemplateSpec{
							NodeClassRef: &karpv1.NodeClassReference{
								Group: object.GVK(nodeClass).Group,
								Kind:  object.GVK(nodeClass).Kind,
								Name:  nodeClass.Name,
							},
							Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelOSStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{string(corev1.Linux)},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelInstanceTypeStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"t3.nano"},
									},
								},
							},
						},
					},
				},
			})
			nodePoolHighPri := test.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
					Weight: lo.ToPtr(int32(100)),
					Template: karpv1.NodeClaimTemplate{
						Spec: karpv1.NodeClaimTemplateSpec{
							NodeClassRef: &karpv1.NodeClassReference{
								Group: object.GVK(nodeClass).Group,
								Kind:  object.GVK(nodeClass).Kind,
								Name:  nodeClass.Name,
							},
							Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelOSStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{string(corev1.Linux)},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelInstanceTypeStable,
										Operator: corev1.NodeSelectorOpIn,
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
			Expect(env.GetNode(pod.Spec.NodeName).Labels[karpv1.NodePoolLabelKey]).To(Equal(nodePoolHighPri.Name))
		})
		DescribeTable(
			"should provision a right-sized node when a pod has InitContainers (cpu)",
			func(expectedNodeCPU string, containerRequirements corev1.ResourceRequirements, initContainers ...corev1.Container) {
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
					PodRequirements: []corev1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						TopologyKey: corev1.LabelHostname,
					}},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: func() resource.Quantity {
								dsOverhead := env.GetDaemonSetOverhead(nodePool)
								base := lo.ToPtr(resource.MustParse("3"))
								base.Sub(*dsOverhead.Cpu())
								return *base
							}(),
						},
					},
				})

				test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      v1.LabelInstanceCPU,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"4", "8"},
					},
				}, karpv1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      v1.LabelInstanceCategory,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{"t"},
					},
				})
				pod := test.Pod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					PodRequirements: []corev1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						TopologyKey: corev1.LabelHostname,
					}},
					InitContainers:       initContainers,
					ResourceRequirements: containerRequirements,
				})
				env.ExpectCreated(nodePool, nodeClass, dsBufferPod, pod)
				env.EventuallyExpectHealthy(pod)
				node := env.ExpectCreatedNodeCount("==", 1)[0]
				Expect(node.ObjectMeta.GetLabels()[v1.LabelInstanceCPU]).To(Equal(expectedNodeCPU))
			},
			Entry("sidecar requirements + later init requirements do exceed container requirements", "8", corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("400m")},
			}, ephemeralInitContainer(corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("300m")},
			}), corev1.Container{
				RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("350m")},
				},
			}, ephemeralInitContainer(corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			})),
			Entry("sidecar requirements + later init requirements do not exceed container requirements", "4", corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("400m")},
			}, ephemeralInitContainer(corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("300m")},
			}), corev1.Container{
				RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("350m")},
				},
			}, ephemeralInitContainer(corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("300m")},
			})),
			Entry("init container requirements exceed all later requests", "8", corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("400m")},
			}, corev1.Container{
				RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
				},
			}, ephemeralInitContainer(corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1500m")},
			}), corev1.Container{
				RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
				},
			}),
		)
		It("should provision a right-sized node when a pod has InitContainers (mixed resources)", func() {
			if env.K8sMinorVersion() < 29 {
				Skip("native sidecar containers are only enabled on EKS 1.29+")
			}
			test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceCategory,
					Operator: corev1.NodeSelectorOpNotIn,
					Values:   []string{"t"},
				},
			})
			pod := test.Pod(test.PodOptions{
				InitContainers: []corev1.Container{
					{
						RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
						Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						}},
					},
					ephemeralInitContainer(corev1.ResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					}}),
				},
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				}},
			})
			env.ExpectCreated(nodePool, nodeClass, pod)
			env.EventuallyExpectHealthy(pod)
		})
		It("should provision a node for a pod with overlapping zone and zone-id requirements", func() {
			subnetInfo := lo.UniqBy(env.GetSubnetInfo(map[string]string{"karpenter.sh/discovery": env.ClusterName}), func(s environmentaws.SubnetInfo) string {
				return s.Zone
			})
			Expect(len(subnetInfo)).To(BeNumerically(">=", 3))

			// Create a pod with 'overlapping' zone and zone-id requirements. With two options for each label, but only one pair of zone-zoneID that maps to the
			// same AZ, we will always expect the pod to be scheduled to that AZ. In this case, this is the mapping at zone[1].
			pod := test.Pod(test.PodOptions{
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelTopologyZone,
						Operator: corev1.NodeSelectorOpIn,
						Values:   lo.Map(subnetInfo[0:2], func(info environmentaws.SubnetInfo, _ int) string { return info.Zone }),
					},
					{
						Key:      v1.LabelTopologyZoneID,
						Operator: corev1.NodeSelectorOpIn,
						Values:   lo.Map(subnetInfo[1:3], func(info environmentaws.SubnetInfo, _ int) string { return info.ZoneID }),
					},
				},
			})
			env.ExpectCreated(nodePool, nodeClass, pod)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			Expect(node.Labels[corev1.LabelTopologyZone]).To(Equal(subnetInfo[1].Zone))
			Expect(node.Labels[v1.LabelTopologyZoneID]).To(Equal(subnetInfo[1].ZoneID))
		})
		It("should provision nodes for pods with zone-id requirements in the correct zone", func() {
			// Each pod specifies a requirement on this expected zone, where the value is the matching zone for the
			// required zone-id. This allows us to verify that Karpenter launched the node in the correct zone, even if
			// it doesn't add the zone-id label and the label is added by CCM. If we didn't take this approach, we would
			// succeed even if Karpenter doesn't add the label and /or incorrectly generated offerings on k8s 1.30 and
			// above. This is an unlikely scenario, and adding this check is a defense in depth measure.
			const expectedZoneLabel = "expected-zone-label"
			test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      expectedZoneLabel,
					Operator: corev1.NodeSelectorOpExists,
				},
			})

			subnetInfo := lo.UniqBy(env.GetSubnetInfo(map[string]string{"karpenter.sh/discovery": env.ClusterName}), func(s environmentaws.SubnetInfo) string {
				return s.Zone
			})
			pods := lo.Map(subnetInfo, func(info environmentaws.SubnetInfo, _ int) *corev1.Pod {
				return test.Pod(test.PodOptions{
					NodeRequirements: []corev1.NodeSelectorRequirement{
						{
							Key:      expectedZoneLabel,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{info.Zone},
						},
						{
							Key:      v1.LabelTopologyZoneID,
							Operator: corev1.NodeSelectorOpIn,
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
				Expect(node.Labels[corev1.LabelTopologyZone]).To(Equal(expectedZone))
				zoneInfo, ok := lo.Find(subnetInfo, func(info environmentaws.SubnetInfo) bool {
					return info.Zone == expectedZone
				})
				Expect(ok).To(BeTrue())
				Expect(node.Labels[v1.LabelTopologyZoneID]).To(Equal(zoneInfo.ZoneID))
			}
		})
	})
})

func ephemeralInitContainer(requirements corev1.ResourceRequirements) corev1.Container {
	return corev1.Container{
		Image:     environmentaws.EphemeralInitContainerImage,
		Command:   []string{"/bin/sh"},
		Args:      []string{"-c", "sleep 5"},
		Resources: requirements,
	}
}
