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

package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/utils/injection"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"knative.dev/pkg/ptr"
)

var _ = Describe("Instance Types", func() {
	It("should support instance type labels", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		var pods []*v1.Pod
		for key, value := range map[string]string{
			awsv1alpha1.LabelInstanceHypervisor:      "nitro",
			awsv1alpha1.LabelInstanceCategory:        "g",
			awsv1alpha1.LabelInstanceFamily:          "g4dn",
			awsv1alpha1.LabelInstanceGeneration:      "4",
			awsv1alpha1.LabelInstanceSize:            "8xlarge",
			awsv1alpha1.LabelInstanceCPU:             "32",
			awsv1alpha1.LabelInstanceMemory:          "131072",
			awsv1alpha1.LabelInstancePods:            "58",
			awsv1alpha1.LabelInstanceGPUName:         "t4",
			awsv1alpha1.LabelInstanceGPUManufacturer: "nvidia",
			awsv1alpha1.LabelInstanceGPUCount:        "1",
			awsv1alpha1.LabelInstanceGPUMemory:       "16384",
			awsv1alpha1.LabelInstanceLocalNVME:       "900",
		} {
			pods = append(pods, test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{key: value}}))
		}
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller, pods...) {
			ExpectScheduled(ctx, env.Client, pod)
		}
	})
	It("should not launch AWS Pod ENI on a t3", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{
					v1.LabelInstanceTypeStable: "t3.large",
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
				},
			})) {
			ExpectNotScheduled(ctx, env.Client, pod)
		}
	})
	It("should de-prioritize metal", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				},
			})) {
			ExpectScheduled(ctx, env.Client, pod)
		}
		Expect(fakeEC2API.CalledWithCreateFleetInput.Len()).To(Equal(1))
		call := fakeEC2API.CalledWithCreateFleetInput.Pop()
		_ = call
		for _, ltc := range call.LaunchTemplateConfigs {
			for _, ovr := range ltc.Overrides {
				Expect(strings.HasSuffix(aws.StringValue(ovr.InstanceType), "metal")).To(BeFalse())
			}
		}
	})
	It("should launch on metal", func() {
		// add a provisioner requirement for instance type exists to remove our default filter for metal sizes
		provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
			Key:      v1.LabelInstanceTypeStable,
			Operator: v1.NodeSelectorOpExists,
		})
		ExpectApplied(ctx, env.Client, provisioner)
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{
					v1.LabelInstanceTypeStable: "m5.metal",
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
		// ensure the pod ENI option is off
		optsCopy := opts
		optsCopy.AWSEnablePodENI = false
		cancelCtx, cancelFunc := context.WithCancel(injection.WithOptions(ctx, optsCopy))
		// ensure the provisioner is shut down at the end of this test
		defer cancelFunc()

		prov := provisioning.NewProvisioner(cancelCtx, cfg, env.Client, corev1.NewForConfigOrDie(env.Config), recorder, cloudProvider, cluster)
		provisionContoller := provisioning.NewController(env.Client, prov, recorder)
		ExpectApplied(ctx, env.Client, provisioner)
		for _, pod := range ExpectProvisioned(cancelCtx, env.Client, provisionContoller,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
				},
			})) {
			ExpectNotScheduled(cancelCtx, env.Client, pod)
		}
	})
	It("should launch AWS Pod ENI on a compatible instance type", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSPodENI: resource.MustParse("1")},
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
		ExpectApplied(ctx, env.Client, provisioner)
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
				},
			}),
			// Should pack onto same instance
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceNVIDIAGPU: resource.MustParse("2")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceNVIDIAGPU: resource.MustParse("2")},
				},
			}),
			// Should pack onto a separate instance
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceNVIDIAGPU: resource.MustParse("4")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceNVIDIAGPU: resource.MustParse("4")},
				},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "p3.8xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should launch instances for AWS Neuron resource requests", func() {
		nodeNames := sets.NewString()
		ExpectApplied(ctx, env.Client, provisioner)
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
				},
			}),
			// Should pack onto same instance
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("2")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("2")},
				},
			}),
			// Should pack onto a separate instance
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("4")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("4")},
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
		opts.AWSENILimitedPodDensity = false
		instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
		Expect(err).To(BeNil())
		provisioner := test.Provisioner()
		for _, info := range instanceInfo {
			it := NewInstanceType(injection.WithOptions(ctx, opts), info, provisioner.Spec.KubeletConfiguration, 0, "", provider, nil)
			resources := it.Resources()
			Expect(resources.Pods().Value()).To(BeNumerically("==", 110))
		}
	})
	It("should not set pods to 110 if using ENI-based pod density", func() {
		opts.AWSENILimitedPodDensity = true
		instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
		Expect(err).To(BeNil())
		provisioner := test.Provisioner()
		for _, info := range instanceInfo {
			it := NewInstanceType(injection.WithOptions(ctx, opts), info, provisioner.Spec.KubeletConfiguration, 0, "", provider, nil)
			resources := it.Resources()
			Expect(resources.Pods().Value()).ToNot(BeNumerically("==", 110))
		}
	})
	Context("KubeletConfiguration Overrides", func() {
		It("should override system reserved cpus when specified", func() {
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner = test.Provisioner(test.ProvisionerOptions{
				Kubelet: &v1alpha5.KubeletConfiguration{
					SystemReserved: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("2"),
					},
				},
			})
			it := NewInstanceType(injection.WithOptions(ctx, opts), instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, 0, "", provider, nil)
			overhead := it.Overhead()
			Expect(overhead.Cpu().String()).To(Equal("2080m"))
		})
		It("should override system reserved memory when specified", func() {
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner = test.Provisioner(test.ProvisionerOptions{
				Kubelet: &v1alpha5.KubeletConfiguration{
					SystemReserved: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("20Gi"),
					},
				},
			})
			it := NewInstanceType(injection.WithOptions(ctx, opts), instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, 0, "", provider, nil)
			overhead := it.Overhead()
			Expect(overhead.Memory().String()).To(Equal("21473Mi"))
		})
		It("should set max-pods to user-defined value if specified", func() {
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner := test.Provisioner(test.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{MaxPods: ptr.Int32(10)}})
			for _, info := range instanceInfo {
				it := NewInstanceType(injection.WithOptions(ctx, opts), info, provisioner.Spec.KubeletConfiguration, 0, "", provider, nil)
				resources := it.Resources()
				Expect(resources.Pods().Value()).To(BeNumerically("==", 10))
			}
		})
		It("should override max-pods value when AWSENILimitedPodDensity is set", func() {
			opts.AWSENILimitedPodDensity = false
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			provisioner := test.Provisioner(test.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{MaxPods: ptr.Int32(10)}})
			for _, info := range instanceInfo {
				it := NewInstanceType(injection.WithOptions(ctx, opts), info, provisioner.Spec.KubeletConfiguration, 0, "", provider, nil)
				resources := it.Resources()
				Expect(resources.Pods().Value()).To(BeNumerically("==", 10))
			}
		})
	})

	Context("Insufficient Capacity Error Cache", func() {
		It("should launch instances of different type on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: awsv1alpha1.CapacityTypeOnDemand, InstanceType: "inf1.6xlarge", Zone: "test-zone-1a"}})
			ExpectApplied(ctx, env.Client, provisioner)
			pods := ExpectProvisioned(ctx, env.Client, controller,
				test.UnschedulablePod(test.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
						Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
					},
				}),
				test.UnschedulablePod(test.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
						Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("1")},
					},
				}),
			)
			// it should've tried to pack them on a single inf1.6xlarge then hit an insufficient capacity error
			for _, pod := range pods {
				ExpectNotScheduled(ctx, env.Client, pod)
			}
			nodeNames := sets.NewString()
			for _, pod := range ExpectProvisioned(ctx, env.Client, controller, pods...) {
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "inf1.2xlarge"))
				nodeNames.Insert(node.Name)
			}
			Expect(nodeNames.Len()).To(Equal(2))
		})
		It("should launch instances in a different zone on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: awsv1alpha1.CapacityTypeOnDemand, InstanceType: "p3.8xlarge", Zone: "test-zone-1a"}})
			pod := test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "p3.8xlarge"},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{awsv1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{awsv1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
				},
			})
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
					}},
				},
			}}}
			ExpectApplied(ctx, env.Client, provisioner)
			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			// it should've tried to pack them in test-zone-1a on a p3.8xlarge then hit insufficient capacity, the next attempt will try test-zone-1b
			ExpectNotScheduled(ctx, env.Client, pod)

			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(SatisfyAll(
				HaveKeyWithValue(v1.LabelInstanceTypeStable, "p3.8xlarge"),
				HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-1b")))
		})
		It("should launch smaller instances than optimal if larger instance launch results in Insufficient Capacity Error", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
				{CapacityType: awsv1alpha1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			})
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"m5.large", "m5.xlarge"},
			})
			pods := []*v1.Pod{}
			for i := 0; i < 2; i++ {
				pods = append(pods, test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
					NodeSelector: map[string]string{
						v1.LabelTopologyZone: "test-zone-1a",
					},
				}))
			}
			// Provisions 2 m5.large instances since m5.xlarge was ICE'd
			ExpectApplied(ctx, env.Client, provisioner)
			pods = ExpectProvisioned(ctx, env.Client, controller, pods...)
			for _, pod := range pods {
				ExpectNotScheduled(ctx, env.Client, pod)
			}
			pods = ExpectProvisioned(ctx, env.Client, controller, pods...)
			for _, pod := range pods {
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("m5.large"))
			}
		})
		It("should launch instances on later reconciliation attempt with Insufficient Capacity Error Cache expiry", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: awsv1alpha1.CapacityTypeOnDemand, InstanceType: "inf1.6xlarge", Zone: "test-zone-1a"}})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, controller,
				test.UnschedulablePod(test.PodOptions{
					NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "inf1.6xlarge"},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("2")},
						Limits:   v1.ResourceList{awsv1alpha1.ResourceAWSNeuron: resource.MustParse("2")},
					},
				}),
			)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// capacity shortage is over - expire the item from the cache and try again
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{})
			unavailableOfferingsCache.Delete(UnavailableOfferingsCacheKey("inf1.6xlarge", "test-zone-1a", awsv1alpha1.CapacityTypeOnDemand))
			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "inf1.6xlarge"))
		})
		It("should launch on-demand capacity if flexible to both spot and on-demand, but spot is unavailable", func() {
			fakeEC2API.DescribeInstanceTypesPagesWithContext(ctx, &ec2.DescribeInstanceTypesInput{}, func(dito *ec2.DescribeInstanceTypesOutput, b bool) bool {
				for _, it := range dito.InstanceTypes {
					fakeEC2API.InsufficientCapacityPools.Add(fake.CapacityPool{CapacityType: awsv1alpha1.CapacityTypeSpot, InstanceType: aws.StringValue(it.InstanceType), Zone: "test-zone-1a"})
				}
				return true
			})
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{awsv1alpha1.CapacityTypeSpot, awsv1alpha1.CapacityTypeOnDemand}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
			}
			// Spot Unavailable
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// include deprioritized instance types
			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			// Fallback to OD
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, awsv1alpha1.CapacityTypeOnDemand))
		})
		It("should return all instance types, even though with no offerings due to Insufficient Capacity Error", func() {
			fakeEC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
				{CapacityType: awsv1alpha1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
				{CapacityType: awsv1alpha1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
				{CapacityType: awsv1alpha1.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
				{CapacityType: awsv1alpha1.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
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

			ExpectApplied(ctx, env.Client, provisioner)
			for _, ct := range []string{awsv1alpha1.CapacityTypeOnDemand, awsv1alpha1.CapacityTypeSpot} {
				for _, zone := range []string{"test-zone-1a", "test-zone-1b"} {
					ExpectProvisioned(ctx, env.Client, controller,
						test.UnschedulablePod(test.PodOptions{
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
				instanceTypeNames.Insert(it.Name())
				if it.Name() == "m5.xlarge" {
					// should have no valid offerings
					Expect(it.Offerings()).To(HaveLen(0))
				}
			}
			Expect(instanceTypeNames.Has("m5.xlarge"))
		})
	})
	Context("CapacityType", func() {
		It("should default to on-demand", func() {
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, awsv1alpha1.CapacityTypeOnDemand))
		})
		It("should launch spot capacity if flexible to both spot and on demand", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{awsv1alpha1.CapacityTypeSpot, awsv1alpha1.CapacityTypeOnDemand}}}
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, awsv1alpha1.CapacityTypeSpot))
		})
	})
	Context("Metadata Options", func() {
		It("should default metadata options on generated launch template", func() {
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpEndpoint).To(Equal(ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Disabled))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpPutResponseHopLimit).To(Equal(int64(2)))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpTokens).To(Equal(ec2.LaunchTemplateHttpTokensStateRequired))
		})
		It("should set metadata options on generated launch template from provisioner configuration", func() {
			provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			provider.MetadataOptions = &awsv1alpha1.MetadataOptions{
				HTTPEndpoint:            aws.String(ec2.LaunchTemplateInstanceMetadataEndpointStateDisabled),
				HTTPProtocolIPv6:        aws.String(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Enabled),
				HTTPPutResponseHopLimit: aws.Int64(1),
				HTTPTokens:              aws.String(ec2.LaunchTemplateHttpTokensStateOptional),
			}
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
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
