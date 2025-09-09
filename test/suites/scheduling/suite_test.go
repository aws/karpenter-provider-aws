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
	"fmt"
	"testing"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator/options"
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
var _ = DescribeTableSubtree("Scheduling", Ordered, ContinueOnFailure, func(minValuesPolicy options.MinValuesPolicy) {
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
		env.ExpectSettingsOverridden(corev1.EnvVar{Name: "MIN_VALUES_POLICY", Value: string(minValuesPolicy)})
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

	Context("Labels", func() {
		It("should support well-known labels for instance type selection", func() {
			nodeSelector := map[string]string{
				// Well Known
				karpv1.NodePoolLabelKey:        nodePool.Name,
				corev1.LabelInstanceTypeStable: "c5.large",
				// Well Known to AWS
				v1.LabelInstanceHypervisor:                "nitro",
				v1.LabelInstanceCategory:                  "c",
				v1.LabelInstanceGeneration:                "5",
				v1.LabelInstanceFamily:                    "c5",
				v1.LabelInstanceSize:                      "large",
				v1.LabelInstanceCPU:                       "2",
				v1.LabelInstanceCPUManufacturer:           "intel",
				v1.LabelInstanceCPUSustainedClockSpeedMhz: "3400",
				v1.LabelInstanceMemory:                    "4096",
				v1.LabelInstanceEBSBandwidth:              "4750",
				v1.LabelInstanceNetworkBandwidth:          "750",
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
			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support well-known labels for an accelerator (inferentia2)", func() {
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

			nodePoolWithMinValues := test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceCategory,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"c"},
				},
			})

			env.ExpectCreated(nodeClass, nodePoolWithMinValues, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should honor minValuesPolicy when provisioning a node", func() {
			eventClient := debug.NewEventClient(env.Client)
			pod := test.Pod()
			nodePoolWithMinValues := test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"c5.large", "invalid-instance-type-1", "invalid-instance-type-2"},
				},
				MinValues: lo.ToPtr(3),
			})
			env.ExpectCreated(nodeClass, nodePoolWithMinValues, pod)

			// minValues should only be relaxed when policy is set to BestEffort
			if minValuesPolicy == options.MinValuesPolicyBestEffort {
				env.EventuallyExpectHealthy(pod)
				env.ExpectCreatedNodeCount("==", 1)
				nodeClaim := env.ExpectNodeClaimCount("==", 1)
				Expect(nodeClaim[0].Annotations).To(HaveKeyWithValue(karpv1.NodeClaimMinValuesRelaxedAnnotationKey, "true"))
				Expect(nodeClaim[0].Spec.Requirements).To(ContainElement(karpv1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"c5.large"},
					},
					MinValues: lo.ToPtr(1),
				}))
			} else {
				env.ExpectExists(pod)
				// Give a min for the scheduling decision to be done.
				env.ConsistentlyExpectPendingPods(time.Minute, pod)
				env.EventuallyExpectNodeCount("==", 0)
				env.ExpectNodeClaimCount("==", 0)
				events, err := eventClient.GetEvents(env.Context, "NodePool")
				Expect(err).ToNot(HaveOccurred())
				key, found := lo.FindKeyBy(events, func(k corev1.ObjectReference, v *corev1.EventList) bool {
					return k.Name == nodePoolWithMinValues.Name &&
						k.Namespace == nodePoolWithMinValues.Namespace
				})
				Expect(found).To(BeTrue())
				_, found = lo.Find(events[key].Items, func(e corev1.Event) bool {
					return e.InvolvedObject.Name == nodePoolWithMinValues.Name &&
						e.InvolvedObject.Namespace == nodePoolWithMinValues.Namespace &&
						e.Message == "NodePool requirements filtered out all compatible available instance types due to minValues incompatibility"
				})
				Expect(found).To(BeTrue())
			}
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
			Expect(env.GetInstance(pod.Spec.NodeName).InstanceType).To(Equal(ec2types.InstanceType("c5.large")))
			Expect(env.GetNode(pod.Spec.NodeName).Labels[karpv1.NodePoolLabelKey]).To(Equal(nodePoolHighPri.Name))
		})
		It("should provision a flex node for a pod", func() {
			selectors.Insert(v1.LabelInstanceCapacityFlex)
			pod := test.Pod()
			nodePoolWithMinValues := test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceCapacityFlex,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"true"},
				},
			})
			env.ExpectCreated(nodeClass, nodePoolWithMinValues, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			Expect(env.GetNode(pod.Spec.NodeName).Labels).To(And(HaveKeyWithValue(corev1.LabelInstanceType, ContainSubstring("flex"))))
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

	Context("Capacity Reservations", func() {
		var largeCapacityReservationID, xlargeCapacityReservationID string
		BeforeAll(func() {
			largeCapacityReservationID = environmentaws.ExpectCapacityReservationCreated(
				env.Context,
				env.EC2API,
				ec2types.InstanceTypeM5Large,
				env.ZoneInfo[0].Zone,
				1,
				nil,
				nil,
			)
			xlargeCapacityReservationID = environmentaws.ExpectCapacityReservationCreated(
				env.Context,
				env.EC2API,
				ec2types.InstanceTypeM5Xlarge,
				env.ZoneInfo[0].Zone,
				2,
				nil,
				nil,
			)
		})
		AfterAll(func() {
			environmentaws.ExpectCapacityReservationsCanceled(env.Context, env.EC2API, largeCapacityReservationID, xlargeCapacityReservationID)
		})
		BeforeEach(func() {
			nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{
				{
					ID: largeCapacityReservationID,
				},
				{
					ID: xlargeCapacityReservationID,
				},
			}
			nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      karpv1.CapacityTypeLabelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeReserved},
					},
				},
				// We need to specify the OS label to prevent a daemonset with a Windows specific resource from scheduling against
				// the node. Omitting this requirement will result in scheduling failures.
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelOSStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{string(corev1.Linux)},
					},
				},
			}
		})
		It("should schedule against a specific reservation ID", func() {
			selectors.Insert(v1.LabelCapacityReservationID)
			pod := test.Pod(test.PodOptions{
				NodeRequirements: []corev1.NodeSelectorRequirement{{
					Key:      v1.LabelCapacityReservationID,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{xlargeCapacityReservationID},
				}},
			})
			env.ExpectCreated(nodePool, nodeClass, pod)

			nc := env.EventuallyExpectLaunchedNodeClaimCount("==", 1)[0]
			req, ok := lo.Find(nc.Spec.Requirements, func(req karpv1.NodeSelectorRequirementWithMinValues) bool {
				return req.Key == v1.LabelCapacityReservationID
			})
			Expect(ok).To(BeTrue())
			Expect(req.Values).To(ConsistOf(xlargeCapacityReservationID))

			env.EventuallyExpectNodeClaimsReady(nc)
			n := env.EventuallyExpectNodeCount("==", 1)[0]
			Expect(n.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeReserved))
			Expect(n.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationType, string(v1.CapacityReservationTypeDefault)))
			Expect(n.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, xlargeCapacityReservationID))
		})
		// NOTE: We're not exercising capacity blocks because it isn't possible to provision them ad-hoc for the use in an
		// integration test.
		It("should schedule against a specific reservation type", func() {
			selectors.Insert(v1.LabelCapacityReservationType)
			pod := test.Pod(test.PodOptions{
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      v1.LabelCapacityReservationType,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{string(v1.CapacityReservationTypeDefault)},
					},
					// NOTE: Continue to select the xlarge instance to ensure we can use the large instance for the fallback test. ODCR
					// capacity eventual consistency is inconsistent between different services (e.g. DescribeCapacityReservations and
					// RunInstances) so we've allocated enough to ensure that each test can make use of them without overlapping.
					{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{string(ec2types.InstanceTypeM5Xlarge)},
					},
				},
			})
			env.ExpectCreated(nodePool, nodeClass, pod)

			nc := env.EventuallyExpectLaunchedNodeClaimCount("==", 1)[0]
			req, ok := lo.Find(nc.Spec.Requirements, func(req karpv1.NodeSelectorRequirementWithMinValues) bool {
				return req.Key == v1.LabelCapacityReservationType
			})
			Expect(ok).To(BeTrue())
			Expect(req.Values).To(ConsistOf(string(v1.CapacityReservationTypeDefault)))

			env.EventuallyExpectNodeClaimsReady(nc)
			n := env.EventuallyExpectNodeCount("==", 1)[0]
			Expect(n.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeReserved))
			Expect(n.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationType, string(v1.CapacityReservationTypeDefault)))
			Expect(n.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, xlargeCapacityReservationID))
		})
		It("should fall back when compatible capacity reservations are exhausted", func() {
			// We create two pods with self anti-affinity and a node selector on a specific instance type. The anti-affinity term
			// ensures that we must provision 2 nodes, and the node selector selects upon an instance type with a single reserved
			// instance available. As such, we should create a reserved NodeClaim for one pod, and an on-demand NodeClaim for the
			// other.
			podLabels := map[string]string{"foo": "bar"}
			pods := test.Pods(2, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{string(ec2types.InstanceTypeM5Large)},
				}},
				PodAntiRequirements: []corev1.PodAffinityTerm{{
					TopologyKey: corev1.LabelHostname,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: podLabels,
					},
				}},
			})
			env.ExpectCreated(nodePool, nodeClass, pods[0], pods[1])

			reservedCount := 0
			for _, nc := range env.EventuallyExpectLaunchedNodeClaimCount("==", 2) {
				req, ok := lo.Find(nc.Spec.Requirements, func(req karpv1.NodeSelectorRequirementWithMinValues) bool {
					return req.Key == v1.LabelCapacityReservationID
				})
				if ok {
					reservedCount += 1
					Expect(req.Values).To(ConsistOf(largeCapacityReservationID))
				}
			}
			Expect(reservedCount).To(Equal(1))
			env.EventuallyExpectNodeCount("==", 2)
		})
	})
},
	Entry("MinValuesPolicyBestEffort", options.MinValuesPolicyBestEffort),
	Entry("MinValuesPolicyStrict", options.MinValuesPolicyStrict),
)

func ephemeralInitContainer(requirements corev1.ResourceRequirements) corev1.Container {
	return corev1.Container{
		Image:     environmentaws.EphemeralInitContainerImage,
		Command:   []string{"/bin/sh"},
		Args:      []string{"-c", "sleep 5"},
		Resources: requirements,
	}
}
