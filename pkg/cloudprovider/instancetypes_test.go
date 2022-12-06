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

package cloudprovider

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	coretest "github.com/aws/karpenter-core/pkg/test"

	. "github.com/aws/karpenter-core/pkg/test/expectations"

	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/fake"
)

var _ = Describe("Instance Types", func() {
	It("should support instance type labels", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		var pods []*v1.Pod
		for key, value := range map[string]string{
			v1alpha1.LabelInstanceHypervisor:      "nitro",
			v1alpha1.LabelInstanceCategory:        "g",
			v1alpha1.LabelInstanceFamily:          "g4dn",
			v1alpha1.LabelInstanceGeneration:      "4",
			v1alpha1.LabelInstanceSize:            "8xlarge",
			v1alpha1.LabelInstanceCPU:             "32",
			v1alpha1.LabelInstanceMemory:          "131072",
			v1alpha1.LabelInstancePods:            "58",
			v1alpha1.LabelInstanceGPUName:         "t4",
			v1alpha1.LabelInstanceGPUManufacturer: "nvidia",
			v1alpha1.LabelInstanceGPUCount:        "1",
			v1alpha1.LabelInstanceGPUMemory:       "16384",
			v1alpha1.LabelInstanceLocalNVME:       "900",
		} {
			pods = append(pods, coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{key: value}}))
		}
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pods...) {
			ExpectScheduled(ctx, env.Client, pod)
		}
	})
	It("should not launch AWS Pod ENI on a t3", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				NodeSelector: map[string]string{
					v1.LabelInstanceTypeStable: "t3.large",
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
				},
			})) {
			ExpectNotScheduled(ctx, env.Client, pod)
		}
	})
	It("should order the instance types by price and only consider the cheapest ones", func() {
		instances := makeFakeInstances()
		fakeEC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: makeFakeInstances(),
		})
		fakeEC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: makeFakeInstanceOfferings(instances),
		})
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				},
			})) {
			ExpectScheduled(ctx, env.Client, pod)
		}
		its, err := cloudProvider.GetInstanceTypes(ctx, provisioner)
		Expect(err).To(BeNil())
		// Order all the instances by their price
		// We need some way to deterministically order them if their prices match
		reqs := scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...)
		sort.Slice(its, func(i, j int) bool {
			iPrice := its[i].Offerings.Requirements(reqs).Cheapest().Price
			jPrice := its[j].Offerings.Requirements(reqs).Cheapest().Price
			if iPrice == jPrice {
				return its[i].Name < its[j].Name
			}
			return iPrice < jPrice
		})
		// Expect that the launch template overrides gives the 60 cheapest instance types
		expected := sets.NewString(lo.Map(its[:MaxInstanceTypes], func(i *cloudprovider.InstanceType, _ int) string {
			return i.Name
		})...)
		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		call := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(call.LaunchTemplateConfigs).To(HaveLen(1))

		Expect(call.LaunchTemplateConfigs[0].Overrides).To(HaveLen(MaxInstanceTypes))
		for _, override := range call.LaunchTemplateConfigs[0].Overrides {
			Expect(expected.Has(aws.StringValue(override.InstanceType))).To(BeTrue(), fmt.Sprintf("expected %s to exist in set", aws.StringValue(override.InstanceType)))
		}
	})
	It("should de-prioritize metal", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				},
			})) {
			ExpectScheduled(ctx, env.Client, pod)
		}
		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		call := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		for _, ltc := range call.LaunchTemplateConfigs {
			for _, ovr := range ltc.Overrides {
				Expect(strings.HasSuffix(aws.StringValue(ovr.InstanceType), "metal")).To(BeFalse())
			}
		}
	})
	It("should de-prioritize gpu types", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				},
			})) {
			ExpectScheduled(ctx, env.Client, pod)
		}
		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		call := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		for _, ltc := range call.LaunchTemplateConfigs {
			for _, ovr := range ltc.Overrides {
				Expect(strings.HasPrefix(aws.StringValue(ovr.InstanceType), "g")).To(BeFalse())
			}
		}
	})
	It("should launch on metal", func() {
		// add a provisioner requirement for instance type exists to remove our default filter for metal sizes
		provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
			Key:      v1.LabelInstanceTypeStable,
			Operator: v1.NodeSelectorOpExists,
		})
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				NodeSelector: map[string]string{
					v1alpha1.LabelInstanceSize: "metal",
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				},
			})) {
			ExpectScheduled(ctx, env.Client, pod)
		}
	})
	It("should fail to launch AWS Pod ENI if the command line option enabling it isn't set", func() {
		settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{EnablePodENI: lo.ToPtr(false)})
		ctx = settingsStore.InjectSettings(ctx)
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
				},
			})) {
			ExpectNotScheduled(ctx, env.Client, pod)
		}
	})
	It("should launch AWS Pod ENI on a compatible instance type", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
				},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKey(v1.LabelInstanceTypeStable))
			supportsPodENI := func() bool {
				limits, ok := Limits[node.Labels[v1.LabelInstanceTypeStable]]
				return ok && limits.IsTrunkingCompatible
			}
			Expect(supportsPodENI()).To(Equal(true))
		}
	})
	It("should launch instances for Nvidia GPU resource requests", func() {
		nodeNames := sets.NewString()
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
				},
			}),
			// Should pack onto same instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("2")},
					Limits:   v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("2")},
				},
			}),
			// Should pack onto a separate instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("4")},
					Limits:   v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("4")},
				},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "p3.8xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should launch instances for Habana GPU resource requests", func() {
		nodeNames := sets.NewString()
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceHabanaGaudi: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1alpha1.ResourceHabanaGaudi: resource.MustParse("1")},
				},
			}),
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceHabanaGaudi: resource.MustParse("2")},
					Limits:   v1.ResourceList{v1alpha1.ResourceHabanaGaudi: resource.MustParse("2")},
				},
			}),
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceHabanaGaudi: resource.MustParse("4")},
					Limits:   v1.ResourceList{v1alpha1.ResourceHabanaGaudi: resource.MustParse("4")},
				},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "dl1.24xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(1))
	})
	It("should launch instances for AWS Neuron resource requests", func() {
		nodeNames := sets.NewString()
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
				},
			}),
			// Should pack onto same instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("2")},
					Limits:   v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("2")},
				},
			}),
			// Should pack onto a separate instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("4")},
					Limits:   v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("4")},
				},
			}),
		) {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "inf1.6xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should set pods to 110 if not using ENI-based pod density", func() {
		settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{
			EnableENILimitedPodDensity: lo.ToPtr(false),
		})
		ctx = settingsStore.InjectSettings(ctx)
		instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
		Expect(err).To(BeNil())
		for _, info := range instanceInfo {
			it := NewInstanceType(ctx, info, provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
			Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 110))
		}
	})
	It("should not set pods to 110 if using ENI-based pod density", func() {
		instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
		Expect(err).To(BeNil())
		for _, info := range instanceInfo {
			it := NewInstanceType(ctx, info, provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
			Expect(it.Capacity.Pods().Value()).ToNot(BeNumerically("==", 110))
		}
	})

	Context("KubeletConfiguration Overrides", func() {
		BeforeEach(func() {
			settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{
				VMMemoryOverheadPercent: lo.ToPtr[float64](0),
			})
			ctx = settingsStore.InjectSettings(ctx)
		})
		Context("Reserved Resources", func() {
			It("should override system reserved cpus when specified", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("2"),
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.SystemReserved.Cpu().String()).To(Equal("2"))
			})
			It("should override system reserved memory when specified", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.SystemReserved.Memory().String()).To(Equal("20Gi"))
			})
			It("should override kube reserved when specified", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceCPU:              resource.MustParse("1"),
							v1.ResourceMemory:           resource.MustParse("20Gi"),
							v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceCPU:              resource.MustParse("2"),
							v1.ResourceMemory:           resource.MustParse("10Gi"),
							v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.KubeReserved.Cpu().String()).To(Equal("2"))
				Expect(it.Overhead.KubeReserved.Memory().String()).To(Equal("10Gi"))
				Expect(it.Overhead.KubeReserved.StorageEphemeral().String()).To(Equal("2Gi"))
			})
		})
		Context("Eviction Thresholds", func() {
			BeforeEach(func() {
				settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{
					VMMemoryOverheadPercent: lo.ToPtr[float64](0),
				})
				ctx = settingsStore.InjectSettings(ctx)
			})
			It("should override eviction threshold (hard) when specified as a quantity", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionHard: map[string]string{
							memoryAvailable: "500Mi",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("500Mi"))
			})
			It("should override eviction threshold (hard) when specified as a percentage value", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionHard: map[string]string{
							memoryAvailable: "10%",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().Value()).To(BeNumerically("~", float64(it.Capacity.Memory().Value())*0.1, 10))
			})
			It("should consider the eviction threshold (hard) disabled when specified as 100%", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionHard: map[string]string{
							memoryAvailable: "100%",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("0"))
			})
			It("should used default eviction threshold (hard) for memory when evictionHard not specified", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionSoft: map[string]string{
							memoryAvailable: "50Mi",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("50Mi"))
			})
			It("should override eviction threshold (soft) when specified as a quantity", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionSoft: map[string]string{
							memoryAvailable: "500Mi",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("500Mi"))
			})
			It("should override eviction threshold (soft) when specified as a percentage value", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionHard: map[string]string{
							memoryAvailable: "5%",
						},
						EvictionSoft: map[string]string{
							memoryAvailable: "10%",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().Value()).To(BeNumerically("~", float64(it.Capacity.Memory().Value())*0.1, 10))
			})
			It("should consider the eviction threshold (soft) disabled when specified as 100%", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionSoft: map[string]string{
							memoryAvailable: "100%",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("0"))
			})
			It("should ignore eviction threshold (soft) when using Bottlerocket AMI", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionHard: map[string]string{
							memoryAvailable: "1Gi",
						},
						EvictionSoft: map[string]string{
							memoryAvailable: "10Gi",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("1Gi"))
			})
			It("should take the greater of evictionHard and evictionSoft for overhead as a value", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionSoft: map[string]string{
							memoryAvailable: "3Gi",
						},
						EvictionHard: map[string]string{
							memoryAvailable: "1Gi",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("3Gi"))
			})
			It("should take the greater of evictionHard and evictionSoft for overhead as a value", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionSoft: map[string]string{
							memoryAvailable: "2%",
						},
						EvictionHard: map[string]string{
							memoryAvailable: "5%",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().Value()).To(BeNumerically("~", float64(it.Capacity.Memory().Value())*0.05, 10))
			})
			It("should take the greater of evictionHard and evictionSoft for overhead with mixed percentage/value", func() {
				instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
				Expect(err).To(BeNil())
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("20Gi"),
						},
						KubeReserved: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Gi"),
						},
						EvictionSoft: map[string]string{
							memoryAvailable: "10%",
						},
						EvictionHard: map[string]string{
							memoryAvailable: "1Gi",
						},
					},
				})
				it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Overhead.EvictionThreshold.Memory().Value()).To(BeNumerically("~", float64(it.Capacity.Memory().Value())*0.1, 10))
			})
		})
		It("should set max-pods to user-defined value if specified", func() {
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner = test.Provisioner(coretest.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{MaxPods: ptr.Int32(10)}})
			for _, info := range instanceInfo {
				it := NewInstanceType(ctx, info, provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 10))
			}
		})
		It("should override max-pods value when AWSENILimitedPodDensity is unset", func() {
			settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{
				EnablePodENI: lo.ToPtr(false),
			})
			ctx = settingsStore.InjectSettings(ctx)

			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner = test.Provisioner(coretest.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{MaxPods: ptr.Int32(10)}})
			for _, info := range instanceInfo {
				it := NewInstanceType(ctx, info, provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 10))
			}
		})
		It("should override pods-per-core value", func() {
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner = test.Provisioner(coretest.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{PodsPerCore: ptr.Int32(1)}})
			for _, info := range instanceInfo {
				it := NewInstanceType(ctx, info, provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", ptr.Int64Value(info.VCpuInfo.DefaultVCpus)))
			}
		})
		It("should take the minimum of pods-per-core and max-pods", func() {
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner = test.Provisioner(coretest.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{PodsPerCore: ptr.Int32(4), MaxPods: ptr.Int32(20)}})
			for _, info := range instanceInfo {
				it := NewInstanceType(ctx, info, provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", lo.Min([]int64{20, ptr.Int64Value(info.VCpuInfo.DefaultVCpus) * 4})))
			}
		})
		It("should ignore pods-per-core when using Bottlerocket AMI", func() {
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
			provisioner = test.Provisioner(coretest.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{PodsPerCore: ptr.Int32(1)}})
			for _, info := range instanceInfo {
				it := NewInstanceType(ctx, info, provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", eniLimitedPods(info).Value()))
			}
		})
		It("should take 110 to be the default pods number when pods-per-core is 0 and AWSENILimitedPodDensity is unset", func() {
			settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{
				EnableENILimitedPodDensity: lo.ToPtr(false),
			})
			ctx = settingsStore.InjectSettings(ctx)

			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner = test.Provisioner(coretest.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{PodsPerCore: ptr.Int32(0)}})
			for _, info := range instanceInfo {
				it := NewInstanceType(ctx, info, provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 110))
			}
		})
	})
	Context("Insufficient Capacity Error Cache", func() {
		It("should launch instances of different type on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "inf1.6xlarge", Zone: "test-zone-1a"}})
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pods := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
						Limits:   v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
					},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
						Limits:   v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
					},
				}),
			)
			// it should've tried to pack them on a single inf1.6xlarge then hit an insufficient capacity error
			for _, pod := range pods {
				ExpectNotScheduled(ctx, env.Client, pod)
			}
			nodeNames := sets.NewString()
			for _, pod := range ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pods...) {
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "inf1.2xlarge"))
				nodeNames.Insert(node.Name)
			}
			Expect(nodeNames.Len()).To(Equal(2))
		})
		It("should launch instances in a different zone on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "p3.8xlarge", Zone: "test-zone-1a"}})
			pod := coretest.UnschedulablePod(coretest.PodOptions{
				NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "p3.8xlarge"},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
				},
			})
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
					}},
				},
			}}}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pod)[0]
			// it should've tried to pack them in test-zone-1a on a p3.8xlarge then hit insufficient capacity, the next attempt will try test-zone-1b
			ExpectNotScheduled(ctx, env.Client, pod)

			pod = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(SatisfyAll(
				HaveKeyWithValue(v1.LabelInstanceTypeStable, "p3.8xlarge"),
				HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-1b")))
		})
		It("should launch smaller instances than optimal if larger instance launch results in Insufficient Capacity Error", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
				{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			})
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"m5.large", "m5.xlarge"},
			})
			pods := []*v1.Pod{}
			for i := 0; i < 2; i++ {
				pods = append(pods, coretest.UnschedulablePod(coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
					NodeSelector: map[string]string{
						v1.LabelTopologyZone: "test-zone-1a",
					},
				}))
			}
			// Provisions 2 m5.large instances since m5.xlarge was ICE'd
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pods = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pods...)
			for _, pod := range pods {
				ExpectNotScheduled(ctx, env.Client, pod)
			}
			pods = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pods...)
			for _, pod := range pods {
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("m5.large"))
			}
		})
		It("should launch instances on later reconciliation attempt with Insufficient Capacity Error Cache expiry", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "inf1.6xlarge", Zone: "test-zone-1a"}})
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "inf1.6xlarge"},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("2")},
						Limits:   v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("2")},
					},
				}),
			)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// capacity shortage is over - expire the item from the cache and try again
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{})
			internalUnavailableOfferingsCache.Delete(fmt.Sprintf("%s:%s:%s", v1alpha5.CapacityTypeOnDemand, "inf1.6xlarge", "test-zone-1a"))
			pod = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "inf1.6xlarge"))
		})
		It("should launch instances in a different zone on second reconciliation attempt with Insufficient Capacity Error Cache fallback (Habana)", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "dl1.24xlarge", Zone: "test-zone-1a"}})
			pod := coretest.UnschedulablePod(coretest.PodOptions{
				NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "dl1.24xlarge"},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1alpha1.ResourceHabanaGaudi: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1alpha1.ResourceHabanaGaudi: resource.MustParse("1")},
				},
			})
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
					}},
				},
			}}}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pod)[0]
			// it should've tried to pack them in test-zone-1a on a dl1.24xlarge then hit insufficient capacity, the next attempt will try test-zone-1b
			ExpectNotScheduled(ctx, env.Client, pod)

			pod = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(SatisfyAll(
				HaveKeyWithValue(v1.LabelInstanceTypeStable, "dl1.24xlarge"),
				HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-1b")))
		})
		It("should launch on-demand capacity if flexible to both spot and on-demand, but spot is unavailable", func() {
			Expect(fakeEC2API.DescribeInstanceTypesPagesWithContext(ctx, &ec2.DescribeInstanceTypesInput{}, func(dito *ec2.DescribeInstanceTypesOutput, b bool) bool {
				for _, it := range dito.InstanceTypes {
					fakeEC2API.InsufficientCapacityPools.Add(fake.CapacityPool{CapacityType: v1alpha5.CapacityTypeSpot, InstanceType: aws.StringValue(it.InstanceType), Zone: "test-zone-1a"})
				}
				return true
			})).To(Succeed())
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha5.CapacityTypeSpot, v1alpha5.CapacityTypeOnDemand}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
			}
			// Spot Unavailable
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// include deprioritized instance types
			pod = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, pod)[0]
			// Fallback to OD
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, v1alpha5.CapacityTypeOnDemand))
		})
		It("should return all instance types, even though with no offerings due to Insufficient Capacity Error", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
				{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
				{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
				{CapacityType: v1alpha5.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
				{CapacityType: v1alpha5.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
			})
			provisioner.Spec.Requirements = nil
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"m5.xlarge"},
			})
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"spot", "on-demand"},
			})

			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			for _, ct := range []string{v1alpha5.CapacityTypeOnDemand, v1alpha5.CapacityTypeSpot} {
				for _, zone := range []string{"test-zone-1a", "test-zone-1b"} {
					ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
						coretest.UnschedulablePod(coretest.PodOptions{
							ResourceRequirements: v1.ResourceRequirements{
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
							},
							NodeSelector: map[string]string{
								v1alpha5.LabelCapacityType: ct,
								v1.LabelTopologyZone:       zone,
							},
						}))
				}
			}

			instanceTypeCache.Flush()
			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, provisioner)
			Expect(err).To(BeNil())
			instanceTypeNames := sets.NewString()
			for _, it := range instanceTypes {
				instanceTypeNames.Insert(it.Name)
				if it.Name == "m5.xlarge" {
					// should have no valid offerings
					Expect(it.Offerings.Available()).To(HaveLen(0))
				}
			}
			Expect(instanceTypeNames.Has("m5.xlarge"))
		})
	})
	Context("CapacityType", func() {
		It("should default to on-demand", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, v1alpha5.CapacityTypeOnDemand))
		})
		It("should launch spot capacity if flexible to both spot and on demand", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha5.CapacityTypeSpot, v1alpha5.CapacityTypeOnDemand}}}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, v1alpha5.CapacityTypeSpot))
		})
		It("should fail to launch capacity when there is no zonal availability for spot", func() {
			now := time.Now()
			fakeEC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []*ec2.SpotPrice{
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     aws.String("m5.large"),
						SpotPrice:        aws.String("0.004"),
						Timestamp:        &now,
					},
				},
			})
			Expect(pricingProvider.updateSpotPricing(ctx)).To(Succeed())
			Eventually(func() bool { return pricingProvider.SpotLastUpdated().After(now) }).Should(BeTrue())

			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha5.CapacityTypeSpot}},
				{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"m5.large"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1b"}},
			}

			// Instance type with no zonal availability for spot shouldn't be scheduled
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should succeed to launch spot instance when zonal availability exists", func() {
			now := time.Now()
			fakeEC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []*ec2.SpotPrice{
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     aws.String("m5.large"),
						SpotPrice:        aws.String("0.004"),
						Timestamp:        &now,
					},
				},
			})
			Expect(pricingProvider.updateSpotPricing(ctx)).To(Succeed())
			Eventually(func() bool { return pricingProvider.SpotLastUpdated().After(now) }).Should(BeTrue())

			// not restricting to the zone so we can get any zone
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha5.CapacityTypeSpot}},
				{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"m5.large"}},
			}

			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.ProvisionerNameLabelKey, provisioner.Name))
		})
	})
	Context("Metadata Options", func() {
		It("should default metadata options on generated launch template", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpEndpoint).To(Equal(ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Disabled))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpPutResponseHopLimit).To(Equal(int64(2)))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpTokens).To(Equal(ec2.LaunchTemplateHttpTokensStateRequired))
		})
		It("should set metadata options on generated launch template from provisioner configuration", func() {
			nodeTemplate.Spec.MetadataOptions = &v1alpha1.MetadataOptions{
				HTTPEndpoint:            aws.String(ec2.LaunchTemplateInstanceMetadataEndpointStateDisabled),
				HTTPProtocolIPv6:        aws.String(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Enabled),
				HTTPPutResponseHopLimit: aws.Int64(1),
				HTTPTokens:              aws.String(ec2.LaunchTemplateHttpTokensStateOptional),
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpEndpoint).To(Equal(ec2.LaunchTemplateInstanceMetadataEndpointStateDisabled))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Enabled))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpPutResponseHopLimit).To(Equal(int64(1)))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpTokens).To(Equal(ec2.LaunchTemplateHttpTokensStateOptional))
		})
	})
})

func makeFakeInstances() []*ec2.InstanceTypeInfo {
	var instanceTypes []*ec2.InstanceTypeInfo
	// Use keys from the static pricing data so that we guarantee pricing for the data
	// Create uniform instance data so all of them schedule for a given pod
	for k := range initialOnDemandPrices {
		instanceTypes = append(instanceTypes, &ec2.InstanceTypeInfo{
			InstanceType: aws.String(k),
			ProcessorInfo: &ec2.ProcessorInfo{
				SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
			},
			VCpuInfo: &ec2.VCpuInfo{
				DefaultCores: aws.Int64(1),
				DefaultVCpus: aws.Int64(2),
			},
			MemoryInfo: &ec2.MemoryInfo{
				SizeInMiB: aws.Int64(8192),
			},
			NetworkInfo: &ec2.NetworkInfo{
				MaximumNetworkInterfaces:  aws.Int64(3),
				Ipv4AddressesPerInterface: aws.Int64(10),
			},
			SupportedUsageClasses: fake.DefaultSupportedUsageClasses,
		})
	}
	return instanceTypes
}

func makeFakeInstanceOfferings(instanceTypes []*ec2.InstanceTypeInfo) []*ec2.InstanceTypeOffering {
	var instanceTypeOfferings []*ec2.InstanceTypeOffering

	// Create uniform instance offering data so all of them schedule for a given pod
	for _, instanceType := range instanceTypes {
		instanceTypeOfferings = append(instanceTypeOfferings, &ec2.InstanceTypeOffering{
			InstanceType: instanceType.InstanceType,
			Location:     aws.String("test-zone-1a"),
		})
	}
	return instanceTypeOfferings
}
