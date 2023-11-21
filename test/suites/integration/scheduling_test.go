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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/ptr"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/test/pkg/debug"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var _ = Describe("Scheduling", Ordered, ContinueOnFailure, func() {
	var selectors sets.Set[string]

	BeforeEach(func() {
		// Make the NodePool requirements fully flexible, so we can match well-known label keys
		nodePool = test.ReplaceRequirements(nodePool,
			v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceCategory,
				Operator: v1.NodeSelectorOpExists,
			},
			v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceGeneration,
				Operator: v1.NodeSelectorOpExists,
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
			v1beta1.LabelInstanceMemory:           "4096",
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
		// TODO: remove this requirement once VPC RC rolls out m7a.*, r7a.* ENI data (https://github.com/aws/karpenter/issues/4472)
		test.ReplaceRequirements(nodePool,
			v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceFamily,
				Operator: v1.NodeSelectorOpNotIn,
				Values:   aws.ExcludedInstanceFamilies,
			},
			v1.NodeSelectorRequirement{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{string(v1.Windows)},
			},
		)
		env.ExpectCreated(nodeClass, nodePool, deployment)
		env.EventuallyExpectHealthyPodCountWithTimeout(time.Minute*15, labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	DescribeTable("should support restricted label domain exceptions", func(domain string) {
		// Assign labels to the nodepool so that it has known values
		test.ReplaceRequirements(nodePool,
			v1.NodeSelectorRequirement{Key: domain + "/team", Operator: v1.NodeSelectorOpExists},
			v1.NodeSelectorRequirement{Key: domain + "/custom-label", Operator: v1.NodeSelectorOpExists},
			v1.NodeSelectorRequirement{Key: "subdomain." + domain + "/custom-label", Operator: v1.NodeSelectorOpExists},
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
					},
				},
			},
		})

		env.ExpectCreated(nodeClass, nodePool, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(podLabels), 3)
		env.ExpectCreatedNodeCount("==", 3)
	})
	It("should provision a node using a NodePool with higher priority", func() {
		nodePoolLowPri := test.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Weight: ptr.Int32(10),
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
						Requirements: []v1.NodeSelectorRequirement{
							{
								Key:      v1.LabelOSStable,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{string(v1.Linux)},
							},
							{
								Key:      v1.LabelInstanceTypeStable,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{"t3.nano"},
							},
						},
					},
				},
			},
		})
		nodePoolHighPri := test.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Weight: ptr.Int32(100),
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
						Requirements: []v1.NodeSelectorRequirement{
							{
								Key:      v1.LabelOSStable,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{string(v1.Linux)},
							},
							{
								Key:      v1.LabelInstanceTypeStable,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{"c4.large"},
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
		Expect(ptr.StringValue(env.GetInstance(pod.Spec.NodeName).InstanceType)).To(Equal("c4.large"))
		Expect(env.GetNode(pod.Spec.NodeName).Labels[corev1beta1.NodePoolLabelKey]).To(Equal(nodePoolHighPri.Name))
	})
})
