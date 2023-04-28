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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/test/pkg/debug"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Scheduling", Ordered, ContinueOnFailure, func() {
	var provider *v1alpha1.AWSNodeTemplate
	var provisioner *v1alpha5.Provisioner
	var selectors sets.String

	BeforeEach(func() {
		provider = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner = test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{Key: v1alpha1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists},
				{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpExists},
			},
		})
	})
	BeforeAll(func() {
		selectors = sets.NewString()
	})
	AfterAll(func() {
		// Ensure that we're exercising all well known labels
		Expect(lo.Keys(selectors)).To(ContainElements(append(v1alpha5.WellKnownLabels.UnsortedList(), lo.Keys(v1alpha5.NormalizedLabels)...)))
	})
	It("should apply annotations to the node", func() {
		provisioner.Spec.Annotations = map[string]string{
			"foo": "bar",
			v1alpha5.DoNotConsolidateNodeAnnotationKey: "true",
		}
		pod := test.Pod()
		env.ExpectCreated(provisioner, provider, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		Expect(env.GetNode(pod.Spec.NodeName).Annotations).To(And(HaveKeyWithValue("foo", "bar"), HaveKeyWithValue(v1alpha5.DoNotConsolidateNodeAnnotationKey, "true")))
	})
	It("should support well-known labels for instance type selection", func() {
		nodeSelector := map[string]string{
			// Well Known
			v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
			v1.LabelInstanceTypeStable:       "c5.large",
			// Well Known to AWS
			v1alpha1.LabelInstanceHypervisor:       "nitro",
			v1alpha1.LabelInstanceCategory:         "c",
			v1alpha1.LabelInstanceGeneration:       "5",
			v1alpha1.LabelInstanceFamily:           "c5",
			v1alpha1.LabelInstanceSize:             "large",
			v1alpha1.LabelInstanceCPU:              "2",
			v1alpha1.LabelInstanceMemory:           "4096",
			v1alpha1.LabelInstanceNetworkBandwidth: "750",
			v1alpha1.LabelInstancePods:             "29",
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
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should support well-known labels for local NVME storage", func() {
		selectors.Insert(v1alpha1.LabelInstanceLocalNVME) // Add node selector keys to selectors used in testing to ensure we test all labels
		deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
			NodePreferences: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceLocalNVME,
					Operator: v1.NodeSelectorOpGt,
					Values:   []string{"0"},
				},
			},
			NodeRequirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceLocalNVME,
					Operator: v1.NodeSelectorOpGt,
					Values:   []string{"0"},
				},
			},
		}})
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should support well-known labels for encryption in transit", func() {
		selectors.Insert(v1alpha1.LabelInstanceEncryptionInTransitSupported) // Add node selector keys to selectors used in testing to ensure we test all labels
		deployment := test.Deployment(test.DeploymentOptions{Replicas: 1, PodOptions: test.PodOptions{
			NodePreferences: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceEncryptionInTransitSupported,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"true"},
				},
			},
			NodeRequirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceEncryptionInTransitSupported,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"true"},
				},
			},
		}})
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should support well-known deprecated labels", func() {
		nodeSelector := map[string]string{
			// Deprecated Labels
			v1.LabelFailureDomainBetaRegion: env.Region,
			v1.LabelFailureDomainBetaZone:   fmt.Sprintf("%sa", env.Region),
			"beta.kubernetes.io/arch":       "amd64",
			"beta.kubernetes.io/os":         "linux",
			v1.LabelInstanceType:            "c5.large",
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
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should support well-known labels for topology and architecture", func() {
		nodeSelector := map[string]string{
			// Well Known
			v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
			v1.LabelTopologyRegion:           env.Region,
			v1.LabelTopologyZone:             fmt.Sprintf("%sa", env.Region),
			v1.LabelOSStable:                 "linux",
			v1.LabelArchStable:               "amd64",
			v1alpha5.LabelCapacityType:       "on-demand",
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
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should support well-known labels for an accelerator (nvidia)", func() {
		nodeSelector := map[string]string{
			v1alpha1.LabelInstanceGPUName:                 "t4",
			v1alpha1.LabelInstanceGPUMemory:               "16384",
			v1alpha1.LabelInstanceGPUManufacturer:         "nvidia",
			v1alpha1.LabelInstanceGPUCount:                "1",
			v1alpha1.LabelInstanceAcceleratorName:         "t4",
			v1alpha1.LabelInstanceAcceleratorMemory:       "16384",
			v1alpha1.LabelInstanceAcceleratorManufacturer: "nvidia",
			v1alpha1.LabelInstanceAcceleratorCount:        "1",
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
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should support well-known labels for an accelerator (inferentia)", func() {
		nodeSelector := map[string]string{
			v1alpha1.LabelInstanceAcceleratorName:         "inferentia",
			v1alpha1.LabelInstanceAcceleratorManufacturer: "aws",
			v1alpha1.LabelInstanceAcceleratorCount:        "1",
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
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should provision a node for naked pods", func() {
		pod := test.Pod()

		env.ExpectCreated(provisioner, provider, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should provision a node for a deployment", Label(debug.NoWatch), Label(debug.NoEvents), func() {
		deployment := test.Deployment(test.DeploymentOptions{Replicas: 50})
		env.ExpectCreated(provisioner, provider, deployment)
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

		env.ExpectCreated(provisioner, provider, deployment)
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

		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(podLabels), 3)
		env.ExpectCreatedNodeCount("==", 3)
	})
	It("should provision a node using a provisioner with higher priority", func() {
		provisionerLowPri := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
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
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
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
})
