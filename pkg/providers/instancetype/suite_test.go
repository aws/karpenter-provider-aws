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

package instancetype_test

import (
	"context"
	"fmt"
	"math"
	"net"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/status"
	"github.com/imdario/mergo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"sigs.k8s.io/karpenter/pkg/utils/resources"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var fakeClock *clock.FakeClock
var prov *provisioning.Provisioner
var cluster *state.Cluster
var cloudProvider *cloudprovider.CloudProvider

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceTypeProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
	fakeClock = &clock.FakeClock{}
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, events.NewRecorder(&record.FakeRecorder{}), cloudProvider, cluster, fakeClock)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	cluster.Reset()
	awsEnv.Reset()
	awsEnv.LaunchTemplateProvider.KubeDNSIP = net.ParseIP("10.0.100.10")
	awsEnv.LaunchTemplateProvider.ClusterEndpoint = "https://test-cluster"
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("InstanceTypeProvider", func() {
	var nodeClass, windowsNodeClass *v1.EC2NodeClass
	var nodePool, windowsNodePool *karpv1.NodePool
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(
			v1.EC2NodeClass{
				Status: v1.EC2NodeClassStatus{
					InstanceProfile: "test-profile",
					SecurityGroups: []v1.SecurityGroup{
						{
							ID: "sg-test1",
						},
						{
							ID: "sg-test2",
						},
						{
							ID: "sg-test3",
						},
					},
					Subnets: []v1.Subnet{
						{
							ID:   "subnet-test1",
							Zone: "test-zone-1a",
						},
						{
							ID:   "subnet-test2",
							Zone: "test-zone-1b",
						},
						{
							ID:   "subnet-test3",
							Zone: "test-zone-1c",
						},
					},
				},
			},
		)
		nodeClass.StatusConditions().SetTrue(status.ConditionReady)
		nodePool = coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      karpv1.CapacityTypeLabelKey,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{karpv1.CapacityTypeOnDemand},
								},
							},
						},
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
					},
				},
			},
		})
		windowsNodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				AMISelectorTerms: []v1.AMISelectorTerm{{
					Alias: "windows2022@latest",
				}},
			},
			Status: v1.EC2NodeClassStatus{
				InstanceProfile: "test-profile",
				SecurityGroups:  nodeClass.Status.SecurityGroups,
				Subnets:         nodeClass.Status.Subnets,
				AMIs: []v1.AMI{
					{
						ID: "ami-window-test1",
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
							{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(corev1.Windows)}},
							{Key: corev1.LabelWindowsBuild, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.Windows2022Build}},
						},
					},
				},
			},
		})
		windowsNodeClass.StatusConditions().SetTrue(status.ConditionReady)
		windowsNodePool = coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      karpv1.CapacityTypeLabelKey,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{karpv1.CapacityTypeOnDemand},
								},
							},
						},
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(windowsNodeClass).Group,
							Kind:  object.GVK(windowsNodeClass).Kind,
							Name:  windowsNodeClass.Name,
						},
					},
				},
			},
		})
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass) // Hydrate the subnet cache
		Expect(err).To(BeNil())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
	})

	It("should support individual instance type labels", func() {
		ExpectApplied(ctx, env.Client, nodePool, windowsNodePool, nodeClass, windowsNodeClass)

		nodeSelector := map[string]string{
			// Well known
			karpv1.NodePoolLabelKey:        nodePool.Name,
			corev1.LabelTopologyRegion:     fake.DefaultRegion,
			corev1.LabelTopologyZone:       "test-zone-1a",
			corev1.LabelInstanceTypeStable: "g4dn.8xlarge",
			corev1.LabelOSStable:           "linux",
			corev1.LabelArchStable:         "amd64",
			karpv1.CapacityTypeLabelKey:    "on-demand",
			// Well Known to AWS
			v1.LabelInstanceHypervisor:                   "nitro",
			v1.LabelInstanceEncryptionInTransitSupported: "true",
			v1.LabelInstanceCategory:                     "g",
			v1.LabelInstanceCapacityFlex:                 "false",
			v1.LabelInstanceGeneration:                   "4",
			v1.LabelInstanceFamily:                       "g4dn",
			v1.LabelInstanceSize:                         "8xlarge",
			v1.LabelInstanceCPU:                          "32",
			v1.LabelInstanceCPUManufacturer:              "intel",
			v1.LabelInstanceCPUSustainedClockSpeedMhz:    "2500",
			v1.LabelInstanceMemory:                       "131072",
			v1.LabelInstanceEBSBandwidth:                 "9500",
			v1.LabelInstanceNetworkBandwidth:             "50000",
			v1.LabelInstanceGPUName:                      "t4",
			v1.LabelInstanceGPUManufacturer:              "nvidia",
			v1.LabelInstanceGPUCount:                     "1",
			v1.LabelInstanceGPUMemory:                    "16384",
			v1.LabelInstanceLocalNVME:                    "900",
			// TODO - NVIDIA/GPU instances should not have Neuron/accelerator labels
			v1.LabelInstanceAcceleratorName:         "inferentia2",
			v1.LabelInstanceAcceleratorManufacturer: "aws",
			v1.LabelInstanceAcceleratorCount:        "1",
			v1.LabelTopologyZoneID:                  "tstz1-1a",
			// Deprecated Labels
			corev1.LabelFailureDomainBetaRegion: fake.DefaultRegion,
			corev1.LabelFailureDomainBetaZone:   "test-zone-1a",
			"beta.kubernetes.io/arch":           "amd64",
			"beta.kubernetes.io/os":             "linux",
			corev1.LabelInstanceType:            "g4dn.8xlarge",
			"topology.ebs.csi.aws.com/zone":     "test-zone-1a",
			corev1.LabelWindowsBuild:            v1.Windows2022Build,
		}

		// Ensure that we're exercising all well known labels
		Expect(lo.Keys(nodeSelector)).To(ContainElements(append(karpv1.WellKnownLabels.Difference(sets.New(
			// TODO: add back to test with a preconfigured reserved instance type
			v1.LabelCapacityReservationID,
			v1.LabelCapacityReservationType,
		)).UnsortedList(), lo.Keys(karpv1.NormalizedLabels)...)))

		var pods []*corev1.Pod
		for key, value := range nodeSelector {
			pods = append(pods, coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{key: value}}))
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			ExpectScheduled(ctx, env.Client, pod)
		}
	})
	It("should support combined instance type labels", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)

		nodeSelector := map[string]string{
			// Well known
			karpv1.NodePoolLabelKey:        nodePool.Name,
			corev1.LabelTopologyRegion:     fake.DefaultRegion,
			corev1.LabelTopologyZone:       "test-zone-1a",
			corev1.LabelInstanceTypeStable: "g4dn.8xlarge",
			corev1.LabelOSStable:           "linux",
			corev1.LabelArchStable:         "amd64",
			karpv1.CapacityTypeLabelKey:    "on-demand",
			// Well Known to AWS
			v1.LabelInstanceHypervisor:                   "nitro",
			v1.LabelInstanceEncryptionInTransitSupported: "true",
			v1.LabelInstanceCategory:                     "g",
			v1.LabelInstanceCapacityFlex:                 "false",
			v1.LabelInstanceGeneration:                   "4",
			v1.LabelInstanceFamily:                       "g4dn",
			v1.LabelInstanceSize:                         "8xlarge",
			v1.LabelInstanceCPU:                          "32",
			v1.LabelInstanceCPUManufacturer:              "intel",
			v1.LabelInstanceCPUSustainedClockSpeedMhz:    "2500",
			v1.LabelInstanceMemory:                       "131072",
			v1.LabelInstanceEBSBandwidth:                 "9500",
			v1.LabelInstanceNetworkBandwidth:             "50000",
			v1.LabelInstanceGPUName:                      "t4",
			v1.LabelInstanceGPUManufacturer:              "nvidia",
			v1.LabelInstanceGPUCount:                     "1",
			v1.LabelInstanceGPUMemory:                    "16384",
			v1.LabelInstanceLocalNVME:                    "900",
			v1.LabelTopologyZoneID:                       "tstz1-1a",
			// Deprecated Labels
			corev1.LabelFailureDomainBetaRegion: fake.DefaultRegion,
			corev1.LabelFailureDomainBetaZone:   "test-zone-1a",
			"beta.kubernetes.io/arch":           "amd64",
			"beta.kubernetes.io/os":             "linux",
			corev1.LabelInstanceType:            "g4dn.8xlarge",
			"topology.ebs.csi.aws.com/zone":     "test-zone-1a",
		}

		// Ensure that we're exercising all well known labels except for the accelerator and capacity reservation labels
		Expect(lo.Keys(nodeSelector)).To(ContainElements(
			append(
				karpv1.WellKnownLabels.Difference(sets.New(
					v1.LabelCapacityReservationID,
					v1.LabelCapacityReservationType,
					v1.LabelInstanceAcceleratorCount,
					v1.LabelInstanceAcceleratorName,
					v1.LabelInstanceAcceleratorManufacturer,
					corev1.LabelWindowsBuild,
				)).UnsortedList(), lo.Keys(karpv1.NormalizedLabels)...)))

		pod := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: nodeSelector})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
	})
	It("should support instance type labels with accelerator", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)

		nodeSelector := map[string]string{
			// Well known
			karpv1.NodePoolLabelKey:        nodePool.Name,
			corev1.LabelTopologyRegion:     fake.DefaultRegion,
			corev1.LabelTopologyZone:       "test-zone-1a",
			corev1.LabelInstanceTypeStable: "inf2.xlarge",
			corev1.LabelOSStable:           "linux",
			corev1.LabelArchStable:         "amd64",
			karpv1.CapacityTypeLabelKey:    "on-demand",
			// Well Known to AWS
			v1.LabelInstanceHypervisor:                   "nitro",
			v1.LabelInstanceEncryptionInTransitSupported: "true",
			v1.LabelInstanceCategory:                     "inf",
			v1.LabelInstanceCapacityFlex:                 "false",
			v1.LabelInstanceGeneration:                   "2",
			v1.LabelInstanceFamily:                       "inf2",
			v1.LabelInstanceSize:                         "xlarge",
			v1.LabelInstanceCPU:                          "4",
			v1.LabelInstanceCPUSustainedClockSpeedMhz:    "3600",
			v1.LabelInstanceCPUManufacturer:              "amd",
			v1.LabelInstanceMemory:                       "16384",
			v1.LabelInstanceEBSBandwidth:                 "10000",
			v1.LabelInstanceNetworkBandwidth:             "2083",
			v1.LabelInstanceAcceleratorName:              "inferentia2",
			v1.LabelInstanceAcceleratorManufacturer:      "aws",
			v1.LabelInstanceAcceleratorCount:             "1",
			v1.LabelTopologyZoneID:                       "tstz1-1a",
			// Deprecated Labels
			corev1.LabelFailureDomainBetaRegion: fake.DefaultRegion,
			corev1.LabelFailureDomainBetaZone:   "test-zone-1a",
			"beta.kubernetes.io/arch":           "amd64",
			"beta.kubernetes.io/os":             "linux",
			corev1.LabelInstanceType:            "inf2.xlarge",
			"topology.ebs.csi.aws.com/zone":     "test-zone-1a",
		}

		// Ensure that we're exercising all well known labels except for the gpu, nvme and capacity reservation id labels
		expectedLabels := append(karpv1.WellKnownLabels.Difference(sets.New(
			v1.LabelCapacityReservationID,
			v1.LabelCapacityReservationType,
			v1.LabelInstanceGPUCount,
			v1.LabelInstanceGPUName,
			v1.LabelInstanceGPUManufacturer,
			v1.LabelInstanceGPUMemory,
			v1.LabelInstanceLocalNVME,
			corev1.LabelWindowsBuild,
		)).UnsortedList(), lo.Keys(karpv1.NormalizedLabels)...)
		Expect(lo.Keys(nodeSelector)).To(ContainElements(expectedLabels))

		pod := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: nodeSelector})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
	})
	It("should not launch AWS Pod ENI on a t3", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			NodeSelector: map[string]string{
				corev1.LabelInstanceTypeStable: "t3.large",
			},
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{v1.ResourceAWSPodENI: resource.MustParse("1")},
				Limits:   corev1.ResourceList{v1.ResourceAWSPodENI: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should order the instance types by price and only consider the cheapest ones", func() {
		instances := fake.MakeInstances()
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: fake.MakeInstances(),
		})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: fake.MakeInstanceOfferings(instances),
		})
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
		its, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).To(BeNil())
		// Order all the instances by their price
		// We need some way to deterministically order them if their prices match
		reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(nodePool.Spec.Template.Spec.Requirements...)
		sort.Slice(its, func(i, j int) bool {
			iPrice := its[i].Offerings.Compatible(reqs).Cheapest().Price
			jPrice := its[j].Offerings.Compatible(reqs).Cheapest().Price
			if iPrice == jPrice {
				return its[i].Name < its[j].Name
			}
			return iPrice < jPrice
		})
		// Expect that the launch template overrides gives the 100 cheapest instance types
		expected := sets.New(lo.Map(its[:100], func(i *corecloudprovider.InstanceType, _ int) string {
			return i.Name
		})...)
		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		call := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(call.LaunchTemplateConfigs).To(HaveLen(1))

		Expect(call.LaunchTemplateConfigs[0].Overrides).To(HaveLen(60))
		for _, override := range call.LaunchTemplateConfigs[0].Overrides {
			Expect(expected.Has(string(override.InstanceType))).To(BeTrue(), fmt.Sprintf("expected %s to exist in set", string(override.InstanceType)))
		}
	})
	It("should order the instance types by price and only consider the spot types that are cheaper than the cheapest on-demand", func() {
		instances := fake.MakeInstances()
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: fake.MakeInstances(),
		})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: fake.MakeInstanceOfferings(instances),
		})
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())

		nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values: []string{
						karpv1.CapacityTypeSpot,
						karpv1.CapacityTypeOnDemand,
					},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		awsEnv.EC2API.DescribeSpotPriceHistoryBehavior.Output.Set(generateSpotPricing(cloudProvider, nodePool))
		Expect(awsEnv.PricingProvider.UpdateSpotPricing(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())

		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)

		its, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).To(BeNil())
		// Order all the instances by their price
		// We need some way to deterministically order them if their prices match
		reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(nodePool.Spec.Template.Spec.Requirements...)
		sort.Slice(its, func(i, j int) bool {
			iPrice := its[i].Offerings.Compatible(reqs).Cheapest().Price
			jPrice := its[j].Offerings.Compatible(reqs).Cheapest().Price
			if iPrice == jPrice {
				return its[i].Name < its[j].Name
			}
			return iPrice < jPrice
		})

		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		call := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(call.LaunchTemplateConfigs).To(HaveLen(1))

		// find the cheapest OD price that works
		cheapestODPrice := math.MaxFloat64
		for _, override := range call.LaunchTemplateConfigs[0].Overrides {
			odPrice, ok := awsEnv.PricingProvider.OnDemandPrice(override.InstanceType)
			Expect(ok).To(BeTrue())
			if odPrice < cheapestODPrice {
				cheapestODPrice = odPrice
			}
		}
		// and our spot prices should be cheaper than the OD price
		for _, override := range call.LaunchTemplateConfigs[0].Overrides {
			spotPrice, ok := awsEnv.PricingProvider.SpotPrice(override.InstanceType, *override.AvailabilityZone)
			Expect(ok).To(BeTrue())
			Expect(spotPrice).To(BeNumerically("<", cheapestODPrice))
		}
	})
	It("should not remove expensive metal instanceTypeOptions if any of the requirement with minValues is provided", func() {
		// Construct requirements with minValues for capacityType requirement.
		nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.CapacityTypeSpot},
				},
				MinValues: lo.ToPtr(1),
			},
		}

		// Apply requirements and schedule pods.
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		})

		// Check if pods are scheduled and if CreateFleet has the expensive instance-types.
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		call := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		var expensiveInstanceType bool
		for _, ltc := range call.LaunchTemplateConfigs {
			for _, ovr := range ltc.Overrides {
				if strings.Contains(string(ovr.InstanceType), "metal") {
					expensiveInstanceType = true
				}
			}
		}
		Expect(expensiveInstanceType).To(BeTrue())
	})
	It("should de-prioritize metal", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)

		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		call := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		for _, ltc := range call.LaunchTemplateConfigs {
			for _, ovr := range ltc.Overrides {
				Expect(strings.Contains(string(ovr.InstanceType), "metal")).To(BeFalse())
			}
		}
	})
	It("should de-prioritize gpu types", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)

		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		call := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		for _, ltc := range call.LaunchTemplateConfigs {
			for _, ovr := range ltc.Overrides {
				Expect(strings.HasPrefix(string(ovr.InstanceType), "g")).To(BeFalse())
			}
		}
	})
	It("should launch on metal", func() {
		// add a nodePool requirement for instance type exists to remove our default filter for metal sizes
		nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      corev1.LabelInstanceTypeStable,
				Operator: corev1.NodeSelectorOpExists,
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			NodeSelector: map[string]string{
				v1.LabelInstanceSize: "metal",
			},
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
	})
	It("should launch vpc.amazonaws.com/pod-eni on a compatible instance type", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{v1.ResourceAWSPodENI: resource.MustParse("1")},
				Limits:   corev1.ResourceList{v1.ResourceAWSPodENI: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels).To(HaveKey(corev1.LabelInstanceTypeStable))
		supportsPodENI := func() bool {
			limits, ok := instancetype.Limits[node.Labels[corev1.LabelInstanceTypeStable]]
			return ok && limits.IsTrunkingCompatible
		}
		Expect(supportsPodENI()).To(Equal(true))
	})
	It("should launch pod in flex instance type", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			NodeSelector: map[string]string{
				v1.LabelInstanceCapacityFlex: "true",
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceCapacityFlex, "true"))
		Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, MatchRegexp(".*flex.*")))
	})
	It("should not launch pod when flex instances are disallowed", func() {
		nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceCapacityFlex,
				Operator: corev1.NodeSelectorOpNotIn,
				Values:   []string{"true"},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			NodeSelector: map[string]string{
				v1.LabelInstanceCapacityFlex: "true",
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should launch pod in non-flex instances", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			NodeSelector: map[string]string{
				v1.LabelInstanceCapacityFlex: "false",
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceCapacityFlex, "false"))
		Expect(node.Labels).ToNot(HaveKeyWithValue(corev1.LabelInstanceTypeStable, MatchRegexp("^.*flex.*")))
	})
	It("should launch vpc.amazonaws.com/PrivateIPv4Address on a compatible instance type", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{v1.ResourcePrivateIPv4Address: resource.MustParse("1")},
				Limits:   corev1.ResourceList{v1.ResourcePrivateIPv4Address: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels).To(HaveKey(corev1.LabelInstanceTypeStable))
		limits, ok := instancetype.Limits[node.Labels[corev1.LabelInstanceTypeStable]]
		Expect(ok).To(BeTrue())
		Expect(limits.IPv4PerInterface).ToNot(BeZero())
	})
	It("should not launch instance type for vpc.amazonaws.com/PrivateIPv4Address if VPC resource controller doesn't advertise it", func() {
		// Create a "test" instance type that has PrivateIPv4Addresses but isn't advertised in the VPC limits config
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: []ec2types.InstanceTypeInfo{
				{
					InstanceType: "test",
					ProcessorInfo: &ec2types.ProcessorInfo{
						SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
					},
					VCpuInfo: &ec2types.VCpuInfo{
						DefaultCores: aws.Int32(1),
						DefaultVCpus: aws.Int32(2),
					},
					MemoryInfo: &ec2types.MemoryInfo{
						SizeInMiB: aws.Int64(8192),
					},
					NetworkInfo: &ec2types.NetworkInfo{
						Ipv4AddressesPerInterface: aws.Int32(10),
						DefaultNetworkCardIndex:   aws.Int32(0),
						NetworkCards: []ec2types.NetworkCardInfo{{
							NetworkCardIndex:         lo.ToPtr(int32(0)),
							MaximumNetworkInterfaces: aws.Int32(3),
						}},
					},
					SupportedUsageClasses: fake.DefaultSupportedUsageClasses,
				},
			},
		})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
				{
					InstanceType: "test",
					Location:     aws.String("test-zone-1a"),
				},
			},
		})
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())

		nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      corev1.LabelInstanceTypeStable,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"test"},
			},
		})
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{v1.ResourcePrivateIPv4Address: resource.MustParse("1")},
				Limits:   corev1.ResourceList{v1.ResourcePrivateIPv4Address: resource.MustParse("1")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should launch instances for nvidia.com/gpu resource requests", func() {
		nodeNames := sets.NewString()
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pods := []*corev1.Pod{
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("1")},
					Limits:   corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("1")},
				},
			}),
			// Should pack onto same instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("2")},
					Limits:   corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("2")},
				},
			}),
			// Should pack onto a separate instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("4")},
					Limits:   corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("4")},
				},
			}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, "p3.8xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should launch instances for habana.ai/gaudi resource requests", func() {
		nodeNames := sets.NewString()
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pods := []*corev1.Pod{
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceHabanaGaudi: resource.MustParse("1")},
					Limits:   corev1.ResourceList{v1.ResourceHabanaGaudi: resource.MustParse("1")},
				},
			}),
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceHabanaGaudi: resource.MustParse("2")},
					Limits:   corev1.ResourceList{v1.ResourceHabanaGaudi: resource.MustParse("2")},
				},
			}),
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceHabanaGaudi: resource.MustParse("4")},
					Limits:   corev1.ResourceList{v1.ResourceHabanaGaudi: resource.MustParse("4")},
				},
			}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, "dl1.24xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(1))
	})
	It("should launch instances for aws.amazon.com/neuron resource requests", func() {
		nodeNames := sets.NewString()
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pods := []*corev1.Pod{
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("2")},
					Limits:   corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("2")},
				},
			}),
			// Should pack onto same instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("2")},
					Limits:   corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("2")},
				},
			}),
			// Should pack onto a separate instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("4")},
					Limits:   corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("4")},
				},
			}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, "inf2.24xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should launch trn1 instances for aws.amazon.com/neuron resource requests", func() {
		nodeNames := sets.NewString()
		nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"trn1.2xlarge"},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pods := []*corev1.Pod{
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("1")},
					Limits:   corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("1")},
				},
			}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, "trn1.2xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(1))
	})
	It("should launch inf2 instances for aws.amazon.com/neuroncore resource requests", func() {
		nodeNames := sets.NewString()
		nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"inf2.xlarge"},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pods := []*corev1.Pod{
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAWSNeuronCore: resource.MustParse("2")},
					Limits:   corev1.ResourceList{v1.ResourceAWSNeuronCore: resource.MustParse("2")},
				},
			}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, "inf2.xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(1))
	})
	It("should launch instances for vpc.amazonaws.com/efa resource requests", func() {
		nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"dl1.24xlarge"},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pods := []*corev1.Pod{
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceEFA: resource.MustParse("1")},
					Limits:   corev1.ResourceList{v1.ResourceEFA: resource.MustParse("1")},
				},
			}),
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceEFA: resource.MustParse("2")},
					Limits:   corev1.ResourceList{v1.ResourceEFA: resource.MustParse("2")},
				},
			}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		nodes := sets.NewString()
		for _, pod := range pods {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, "dl1.24xlarge"))
			nodes.Insert(node.Name)
		}
		Expect(nodes.Len()).To(Equal(1))
	})
	It("should launch instances for amd.com/gpu resource requests", func() {
		nodeNames := sets.NewString()
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pods := []*corev1.Pod{
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAMDGPU: resource.MustParse("1")},
					Limits:   corev1.ResourceList{v1.ResourceAMDGPU: resource.MustParse("1")},
				},
			}),
			// Should pack onto same instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAMDGPU: resource.MustParse("2")},
					Limits:   corev1.ResourceList{v1.ResourceAMDGPU: resource.MustParse("2")},
				},
			}),
			// Should pack onto a separate instance
			coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAMDGPU: resource.MustParse("4")},
					Limits:   corev1.ResourceList{v1.ResourceAMDGPU: resource.MustParse("4")},
				},
			}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, "g4ad.16xlarge"))
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should not launch instances w/ instance storage for ephemeral storage resource requests when exceeding blockDeviceMapping", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceEphemeralStorage: resource.MustParse("5000Gi")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should launch instances w/ instance storage for ephemeral storage resource requests when disks are mounted for ephemeral-storage", func() {
		nodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1.InstanceStorePolicyRAID0)
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceEphemeralStorage: resource.MustParse("5000Gi")},
			},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("m6idn.32xlarge"))
		Expect(*node.Status.Capacity.StorageEphemeral()).To(Equal(resource.MustParse("7600G")))
	})
	It("should not set pods to 110 if using ENI-based pod density", func() {
		instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
		Expect(err).To(BeNil())
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
		for _, info := range instanceInfo.InstanceTypes {
			it := instancetype.NewInstanceType(ctx,
				info,
				fake.DefaultRegion,
				nil,
				nil,
				nodeClass.Spec.BlockDeviceMappings,
				nodeClass.Spec.InstanceStorePolicy,
				nodeClass.Spec.Kubelet.MaxPods,
				nodeClass.Spec.Kubelet.PodsPerCore,
				nodeClass.Spec.Kubelet.KubeReserved,
				nodeClass.Spec.Kubelet.SystemReserved,
				nodeClass.Spec.Kubelet.EvictionHard,
				nodeClass.Spec.Kubelet.EvictionSoft,
				nodeClass.AMIFamily(),
				nil,
			)
			Expect(it.Capacity.Pods().Value()).ToNot(BeNumerically("==", 110))
		}
	})
	It("should set pods to 110 if AMI Family doesn't support", func() {
		instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
		Expect(err).To(BeNil())
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
		for _, info := range instanceInfo.InstanceTypes {
			it := instancetype.NewInstanceType(ctx,
				info,
				fake.DefaultRegion,
				nil,
				nil,
				windowsNodeClass.Spec.BlockDeviceMappings,
				windowsNodeClass.Spec.InstanceStorePolicy,
				nodeClass.Spec.Kubelet.MaxPods,
				nodeClass.Spec.Kubelet.PodsPerCore,
				nodeClass.Spec.Kubelet.KubeReserved,
				nodeClass.Spec.Kubelet.SystemReserved,
				nodeClass.Spec.Kubelet.EvictionHard,
				nodeClass.Spec.Kubelet.EvictionSoft,
				windowsNodeClass.AMIFamily(),
				nil,
			)
			Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 110))
		}
	})
	Context("Metrics", func() {
		It("should expose vcpu metrics for instance types", func() {
			instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))
			for _, it := range instanceTypes {
				metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_cpu_cores", map[string]string{
					"instance_type": it.Name,
				})
				Expect(ok).To(BeTrue())
				Expect(metric).To(Not(BeNil()))
				value := metric.GetGauge().Value
				Expect(aws.ToFloat64(value)).To(BeNumerically(">", 0))
			}
		})
		It("should expose memory metrics for instance types", func() {
			instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))
			for _, it := range instanceTypes {
				metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_memory_bytes", map[string]string{
					"instance_type": it.Name,
				})
				Expect(ok).To(BeTrue())
				Expect(metric).To(Not(BeNil()))
				value := metric.GetGauge().Value
				Expect(aws.ToFloat64(value)).To(BeNumerically(">", 0))
			}
		})
	})
	It("should launch instances in local zones", func() {
		nodeClass.Status.Subnets = []v1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a-local",
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      corev1.LabelTopologyZone,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"test-zone-1a-local"},
			}},
		})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
	})
	Context("Overhead", func() {
		var info ec2types.InstanceTypeInfo
		BeforeEach(func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				ClusterName: lo.ToPtr("karpenter-cluster"),
			}))

			var ok bool
			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			info, ok = lo.Find(instanceInfo.InstanceTypes, func(i ec2types.InstanceTypeInfo) bool {
				return i.InstanceType == "m5.xlarge"
			})
			Expect(ok).To(BeTrue())
		})
		Context("System Reserved Resources", func() {
			It("should use defaults when no kubelet is specified", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Overhead.SystemReserved.Cpu().String()).To(Equal("0"))
				Expect(it.Overhead.SystemReserved.Memory().String()).To(Equal("0"))
				Expect(it.Overhead.SystemReserved.StorageEphemeral().String()).To(Equal("0"))
			})
			It("should override system reserved cpus when specified", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					SystemReserved: map[string]string{
						string(corev1.ResourceCPU):              "2",
						string(corev1.ResourceMemory):           "20Gi",
						string(corev1.ResourceEphemeralStorage): "10Gi",
					},
				}
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Overhead.SystemReserved.Cpu().String()).To(Equal("2"))
				Expect(it.Overhead.SystemReserved.Memory().String()).To(Equal("20Gi"))
				Expect(it.Overhead.SystemReserved.StorageEphemeral().String()).To(Equal("10Gi"))
			})
		})
		Context("Kube Reserved Resources", func() {
			It("should use defaults when no kubelet is specified", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Overhead.KubeReserved.Cpu().String()).To(Equal("80m"))
				Expect(it.Overhead.KubeReserved.Memory().String()).To(Equal("893Mi"))
				Expect(it.Overhead.KubeReserved.StorageEphemeral().String()).To(Equal("1Gi"))
			})
			It("should override kube reserved when specified", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					SystemReserved: map[string]string{
						string(corev1.ResourceCPU):              "1",
						string(corev1.ResourceMemory):           "20Gi",
						string(corev1.ResourceEphemeralStorage): "1Gi",
					},
					KubeReserved: map[string]string{
						string(corev1.ResourceCPU):              "2",
						string(corev1.ResourceMemory):           "10Gi",
						string(corev1.ResourceEphemeralStorage): "2Gi",
					},
				}
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Overhead.KubeReserved.Cpu().String()).To(Equal("2"))
				Expect(it.Overhead.KubeReserved.Memory().String()).To(Equal("10Gi"))
				Expect(it.Overhead.KubeReserved.StorageEphemeral().String()).To(Equal("2Gi"))
			})
		})
		Context("Eviction Thresholds", func() {
			BeforeEach(func() {
				ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
					VMMemoryOverheadPercent: lo.ToPtr[float64](0),
				}))
			})
			Context("Eviction Hard", func() {
				It("should override eviction threshold when specified as a quantity", func() {
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceMemory): "20Gi",
						},
						KubeReserved: map[string]string{
							string(corev1.ResourceMemory): "10Gi",
						},
						EvictionHard: map[string]string{
							instancetype.MemoryAvailable: "500Mi",
						},
					}
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("500Mi"))
				})
				It("should override eviction threshold when specified as a percentage value", func() {
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceMemory): "20Gi",
						},
						KubeReserved: map[string]string{
							string(corev1.ResourceMemory): "10Gi",
						},
						EvictionHard: map[string]string{
							instancetype.MemoryAvailable: "10%",
						},
					}
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Overhead.EvictionThreshold.Memory().Value()).To(BeNumerically("~", float64(it.Capacity.Memory().Value())*0.1, 10))
				})
				It("should consider the eviction threshold disabled when specified as 100%", func() {
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceMemory): "20Gi",
						},
						KubeReserved: map[string]string{
							string(corev1.ResourceMemory): "10Gi",
						},
						EvictionHard: map[string]string{
							instancetype.MemoryAvailable: "100%",
						},
					}
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("0"))
				})
				It("should used default eviction threshold for memory when evictionHard not specified", func() {
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceMemory): "20Gi",
						},
						KubeReserved: map[string]string{
							string(corev1.ResourceMemory): "10Gi",
						},
						EvictionSoft: map[string]string{
							instancetype.MemoryAvailable: "50Mi",
						},
					}
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("50Mi"))
				})
			})
			Context("Eviction Soft", func() {
				It("should override eviction threshold when specified as a quantity", func() {
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceMemory): "20Gi",
						},
						KubeReserved: map[string]string{
							string(corev1.ResourceMemory): "10Gi",
						},
						EvictionSoft: map[string]string{
							instancetype.MemoryAvailable: "500Mi",
						},
					}
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("500Mi"))
				})
				It("should override eviction threshold when specified as a percentage value", func() {
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceMemory): "20Gi",
						},
						KubeReserved: map[string]string{
							string(corev1.ResourceMemory): "10Gi",
						},
						EvictionHard: map[string]string{
							instancetype.MemoryAvailable: "5%",
						},
						EvictionSoft: map[string]string{
							instancetype.MemoryAvailable: "10%",
						},
					}
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Overhead.EvictionThreshold.Memory().Value()).To(BeNumerically("~", float64(it.Capacity.Memory().Value())*0.1, 10))
				})
				It("should consider the eviction threshold disabled when specified as 100%", func() {
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceMemory): "20Gi",
						},
						KubeReserved: map[string]string{
							string(corev1.ResourceMemory): "10Gi",
						},
						EvictionSoft: map[string]string{
							instancetype.MemoryAvailable: "100%",
						},
					}
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("0"))
				})
				It("should ignore eviction threshold when using Bottlerocket AMI", func() {
					nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceMemory): "20Gi",
						},
						KubeReserved: map[string]string{
							string(corev1.ResourceMemory): "10Gi",
						},
						EvictionHard: map[string]string{
							instancetype.MemoryAvailable: "1Gi",
						},
						EvictionSoft: map[string]string{
							instancetype.MemoryAvailable: "10Gi",
						},
					}
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("1Gi"))
				})
			})
			It("should take the default eviction threshold when none is specified", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Overhead.EvictionThreshold.Cpu().String()).To(Equal("0"))
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("100Mi"))
				Expect(it.Overhead.EvictionThreshold.StorageEphemeral().AsApproximateFloat64()).To(BeNumerically("~", resources.Quantity("2Gi").AsApproximateFloat64()))
			})
			It("should take the greater of evictionHard and evictionSoft for overhead as a value", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					SystemReserved: map[string]string{
						string(corev1.ResourceMemory): "20Gi",
					},
					KubeReserved: map[string]string{
						string(corev1.ResourceMemory): "10Gi",
					},
					EvictionSoft: map[string]string{
						instancetype.MemoryAvailable: "3Gi",
					},
					EvictionHard: map[string]string{
						instancetype.MemoryAvailable: "1Gi",
					},
				}
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("3Gi"))
			})
			It("should take the greater of evictionHard and evictionSoft for overhead as a value", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					SystemReserved: map[string]string{
						string(corev1.ResourceMemory): "20Gi",
					},
					KubeReserved: map[string]string{
						string(corev1.ResourceMemory): "10Gi",
					},
					EvictionSoft: map[string]string{
						instancetype.MemoryAvailable: "2%",
					},
					EvictionHard: map[string]string{
						instancetype.MemoryAvailable: "5%",
					},
				}
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Overhead.EvictionThreshold.Memory().Value()).To(BeNumerically("~", float64(it.Capacity.Memory().Value())*0.05, 10))
			})
			It("should take the greater of evictionHard and evictionSoft for overhead with mixed percentage/value", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					SystemReserved: map[string]string{
						string(corev1.ResourceMemory): "20Gi",
					},
					KubeReserved: map[string]string{
						string(corev1.ResourceMemory): "10Gi",
					},
					EvictionSoft: map[string]string{
						instancetype.MemoryAvailable: "10%",
					},
					EvictionHard: map[string]string{
						instancetype.MemoryAvailable: "1Gi",
					},
				}
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Overhead.EvictionThreshold.Memory().Value()).To(BeNumerically("~", float64(it.Capacity.Memory().Value())*0.1, 10))
			})
		})
		It("should default max pods based off of network interfaces", func() {
			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
			for _, info := range instanceInfo.InstanceTypes {
				if info.InstanceType == "t3.large" {
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 35))
				}
				if info.InstanceType == "m6idn.32xlarge" {
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 394))
				}
			}
		})
		It("should set max-pods to user-defined value if specified", func() {
			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				MaxPods: lo.ToPtr(int32(10)),
			}
			for _, info := range instanceInfo.InstanceTypes {
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 10))
			}
		})
		DescribeTable(
			"should set max-pods to user-defined value if specified",
			func(alias string, family string, maxPods int, memory string) {
				instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
				Expect(err).To(BeNil())
				t3Large, ok := lo.Find(instanceInfo.InstanceTypes, func(info ec2types.InstanceTypeInfo) bool {
					return info.InstanceType == "t3.large"
				})
				Expect(ok).To(Equal(true))

				nodeClass.Spec.AMIFamily = lo.ToPtr(family)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					MaxPods: lo.ToPtr(int32(10)),
				}
				it := instancetype.NewInstanceType(ctx,
					t3Large,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				// t3.large
				// maxInterfaces = 3
				// maxIPv4PerInterface = 12
				// reservedENIs = 1
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", maxPods))
				// 11 * pods + 255
				Expect(it.Overhead.KubeReserved.Memory().String()).To(Equal(memory))
			},
			Entry("al2 (latest)", "al2@latest", v1.AMIFamilyAL2, 10, "640Mi"),                            // 11 * 35 + 255
			Entry("al2023 (latest)", "al2023@latest", v1.AMIFamilyAL2023, 10, "640Mi"),                   // 11 * 35 + 255
			Entry("bottlerocket (latest)", "bottlerocket@latest", v1.AMIFamilyBottlerocket, 10, "365Mi"), // 11 * 10 + 255
			Entry("windows2019 (latest)", "windows2019@latest", v1.AMIFamilyWindows2019, 10, "365Mi"),    // 11 * 10 + 255
			Entry("windows2022 (latest)", "windows2022@latest", v1.AMIFamilyWindows2022, 10, "365Mi"),    // 11 * 10 + 255
			Entry("custom", fake.ImageID(), v1.AMIFamilyCustom, 10, "640Mi"),                             // 11 * 35 + 255
		)
		It("should override max-pods value", func() {
			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				MaxPods: lo.ToPtr(int32(10)),
			}
			for _, info := range instanceInfo.InstanceTypes {
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 10))
			}
		})
		DescribeTable(
			"should reserve ENIs when aws.reservedENIs is set and use it in max-pods calculation",
			func(alias string, family string, maxPods int, memory string) {
				ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
					ReservedENIs: lo.ToPtr(1),
				}))
				instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
				Expect(err).To(BeNil())
				t3Large, ok := lo.Find(instanceInfo.InstanceTypes, func(info ec2types.InstanceTypeInfo) bool {
					return info.InstanceType == "t3.large"
				})
				Expect(ok).To(Equal(true))

				nodeClass.Spec.AMIFamily = lo.ToPtr(family)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}

				it := instancetype.NewInstanceType(ctx,
					t3Large,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				// t3.large
				// maxInterfaces = 3
				// maxIPv4PerInterface = 12
				// reservedENIs = 1
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", maxPods))
				// 11 * pods + 255
				Expect(it.Overhead.KubeReserved.Memory().String()).To(Equal(memory))
			},
			Entry("al2 (latest)", "al2@latest", v1.AMIFamilyAL2, 24, "640Mi"),                            // 11 * 35 + 255
			Entry("al2023 (latest)", "al2023@latest", v1.AMIFamilyAL2023, 24, "640Mi"),                   // 11 * 35 + 255
			Entry("bottlerocket (latest)", "bottlerocket@latest", v1.AMIFamilyBottlerocket, 24, "519Mi"), // 11 * 24 + 255
			Entry("windows2019 (latest)", "windows2019@latest", v1.AMIFamilyWindows2019, 110, "1465Mi"),  // 11 * 110 + 255
			Entry("windows2022 (latest)", "windows2022@latest", v1.AMIFamilyWindows2022, 110, "1465Mi"),  // 11 * 110 + 255
			Entry("custom", fake.ImageID(), v1.AMIFamilyCustom, 24, "640Mi"),                             // 11 * 35 + 255
		)
		It("should reserve ENIs when aws.reservedENIs is set and not go below 0 ENIs in max-pods calculation", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				ReservedENIs: lo.ToPtr(1_000_000),
			}))

			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			t3Large, ok := lo.Find(instanceInfo.InstanceTypes, func(info ec2types.InstanceTypeInfo) bool {
				return info.InstanceType == "t3.large"
			})
			Expect(ok).To(Equal(true))
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
			it := instancetype.NewInstanceType(ctx,
				t3Large,
				fake.DefaultRegion,
				nil,
				nil,
				nodeClass.Spec.BlockDeviceMappings,
				nodeClass.Spec.InstanceStorePolicy,
				nodeClass.Spec.Kubelet.MaxPods,
				nodeClass.Spec.Kubelet.PodsPerCore,
				nodeClass.Spec.Kubelet.KubeReserved,
				nodeClass.Spec.Kubelet.SystemReserved,
				nodeClass.Spec.Kubelet.EvictionHard,
				nodeClass.Spec.Kubelet.EvictionSoft,
				nodeClass.AMIFamily(),
				nil,
			)
			// t3.large
			// maxInterfaces = 3
			// maxIPv4PerInterface = 12
			// reservedENIs = 1,000,000
			// max(3 - 1,000,000, 0) * (12 - 1) + 2 = 2
			// if max-pods is 2, we output 0
			maxPods := 0
			Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", maxPods))
		})
		It("should override pods-per-core value", func() {
			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				PodsPerCore: lo.ToPtr(int32(1)),
			}
			for _, info := range instanceInfo.InstanceTypes {
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", lo.FromPtr(info.VCpuInfo.DefaultVCpus)))
			}
		})
		It("should take the minimum of pods-per-core and max-pods", func() {
			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				PodsPerCore: lo.ToPtr(int32(4)),
				MaxPods:     lo.ToPtr(int32(20)),
			}
			for _, info := range instanceInfo.InstanceTypes {
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", lo.Min([]int32{20, lo.FromPtr(info.VCpuInfo.DefaultVCpus) * 4})))
			}
		})
		It("should ignore pods-per-core when using Bottlerocket AMI", func() {
			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				PodsPerCore: lo.ToPtr(int32(1)),
			}
			for _, info := range instanceInfo.InstanceTypes {
				it := instancetype.NewInstanceType(ctx,
					info,
					fake.DefaultRegion,
					nil,
					nil,
					nodeClass.Spec.BlockDeviceMappings,
					nodeClass.Spec.InstanceStorePolicy,
					nodeClass.Spec.Kubelet.MaxPods,
					nodeClass.Spec.Kubelet.PodsPerCore,
					nodeClass.Spec.Kubelet.KubeReserved,
					nodeClass.Spec.Kubelet.SystemReserved,
					nodeClass.Spec.Kubelet.EvictionHard,
					nodeClass.Spec.Kubelet.EvictionSoft,
					nodeClass.AMIFamily(),
					nil,
				)
				limitedPods := instancetype.ENILimitedPods(ctx, info, 0)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", limitedPods.Value()))
			}
		})
		It("should take limited pod density to be the default pods number when pods-per-core is 0", func() {
			instanceInfo, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).To(BeNil())
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				PodsPerCore: lo.ToPtr(int32(0)),
			}
			for _, info := range instanceInfo.InstanceTypes {
				if info.InstanceType == "t3.large" {
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 35))
				}
				if info.InstanceType == "m6idn.32xlarge" {
					it := instancetype.NewInstanceType(ctx,
						info,
						fake.DefaultRegion,
						nil,
						nil,
						nodeClass.Spec.BlockDeviceMappings,
						nodeClass.Spec.InstanceStorePolicy,
						nodeClass.Spec.Kubelet.MaxPods,
						nodeClass.Spec.Kubelet.PodsPerCore,
						nodeClass.Spec.Kubelet.KubeReserved,
						nodeClass.Spec.Kubelet.SystemReserved,
						nodeClass.Spec.Kubelet.EvictionHard,
						nodeClass.Spec.Kubelet.EvictionSoft,
						nodeClass.AMIFamily(),
						nil,
					)
					Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 394))
				}
			}
		})
		It("shouldn't report more resources than are actually available on instances", func() {
			awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{
				Subnets: []ec2types.Subnet{
					{
						AvailabilityZone: aws.String("us-west-2a"),
						SubnetId:         aws.String("subnet-12345"),
					},
				},
			})
			awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
				InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
					{
						InstanceType: "t4g.small",
						Location:     aws.String("us-west-2a"),
					},
					{
						InstanceType: "t4g.medium",
						Location:     aws.String("us-west-2a"),
					},
					{
						InstanceType: "t4g.xlarge",
						Location:     aws.String("us-west-2a"),
					},
					{
						InstanceType: "m5.large",
						Location:     aws.String("us-west-2a"),
					},
				},
			})
			awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
				InstanceTypes: []ec2types.InstanceTypeInfo{
					{InstanceType: "t4g.small"},
					{InstanceType: "t4g.medium"},
					{InstanceType: "t4g.xlarge"},
					{InstanceType: "m5.large"},
				},
			})

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			its, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).To(BeNil())

			instanceTypes := map[string]*corecloudprovider.InstanceType{}
			for _, it := range its {
				instanceTypes[it.Name] = it
			}

			for _, tc := range []struct {
				InstanceType string
				// Actual allocatable values as reported by the node from kubelet. You find these
				// by launching the node and inspecting the node status allocatable.
				Memory resource.Quantity
				CPU    resource.Quantity
			}{
				{
					InstanceType: "t4g.small",
					Memory:       resource.MustParse("1408312Ki"),
					CPU:          resource.MustParse("1930m"),
				},
				{
					InstanceType: "t4g.medium",
					Memory:       resource.MustParse("3377496Ki"),
					CPU:          resource.MustParse("1930m"),
				},
				{
					InstanceType: "t4g.xlarge",
					Memory:       resource.MustParse("15136012Ki"),
					CPU:          resource.MustParse("3920m"),
				},
				{
					InstanceType: "m5.large",
					Memory:       resource.MustParse("7220184Ki"),
					CPU:          resource.MustParse("1930m"),
				},
			} {
				it, ok := instanceTypes[tc.InstanceType]
				Expect(ok).To(BeTrue(), fmt.Sprintf("didn't find instance type %q, add to instanceTypeTestData in ./hack/codegen.sh", tc.InstanceType))

				allocatable := it.Allocatable()
				// We need to ensure that our estimate of the allocatable resources <= the value that kubelet reports.  If it's greater,
				// we can launch nodes that can't actually run the pods.
				Expect(allocatable.Memory().AsApproximateFloat64()).To(BeNumerically("<=", tc.Memory.AsApproximateFloat64()),
					fmt.Sprintf("memory estimate for %s was too large, had %s vs %s", tc.InstanceType, allocatable.Memory().String(), tc.Memory.String()))
				Expect(allocatable.Cpu().AsApproximateFloat64()).To(BeNumerically("<=", tc.CPU.AsApproximateFloat64()),
					fmt.Sprintf("CPU estimate for %s was too large, had %s vs %s", tc.InstanceType, allocatable.Cpu().String(), tc.CPU.String()))
			}
		})
	})
	Context("Insufficient Capacity Error Cache", func() {
		It("should launch instances of different type on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
			awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "inf2.24xlarge", Zone: "test-zone-1a"}})
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pods := []*corev1.Pod{
				coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1a"},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("1")},
						Limits:   corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("1")},
					},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1a"},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("1")},
						Limits:   corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("1")},
					},
				}),
			}
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			// it should've tried to pack them on a single inf2.24xlarge then hit an insufficient capacity error
			for _, pod := range pods {
				ExpectNotScheduled(ctx, env.Client, pod)
			}
			nodeNames := sets.NewString()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceAcceleratorName, "inferentia2"))
				nodeNames.Insert(node.Name)
			}
			Expect(nodeNames.Len()).To(Equal(2))
		})
		It("should launch instances in a different zone on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
			awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "p3.8xlarge", Zone: "test-zone-1a"}})
			pod := coretest.UnschedulablePod(coretest.PodOptions{
				NodeSelector: map[string]string{corev1.LabelInstanceTypeStable: "p3.8xlarge"},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("1")},
					Limits:   corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("1")},
				},
			})
			pod.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
					}},
				},
			}}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// it should've tried to pack them in test-zone-1a on a p3.8xlarge then hit insufficient capacity, the next attempt will try test-zone-1b
			ExpectNotScheduled(ctx, env.Client, pod)

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(SatisfyAll(
				HaveKeyWithValue(corev1.LabelInstanceTypeStable, "p3.8xlarge"),
				HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-1b")))
		})
		It("should launch smaller instances than optimal if larger instance launch results in Insufficient Capacity Error", func() {
			awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
				{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			})
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceType,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"m5.large", "m5.xlarge"},
				},
			})
			pods := []*corev1.Pod{}
			for i := 0; i < 2; i++ {
				pods = append(pods, coretest.UnschedulablePod(coretest.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
					},
					NodeSelector: map[string]string{
						corev1.LabelTopologyZone: "test-zone-1a",
					},
				}))
			}
			// Provisions 2 m5.large instances since m5.xlarge was ICE'd
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				ExpectNotScheduled(ctx, env.Client, pod)
			}
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("m5.large"))
			}
		})
		It("should launch instances on later reconciliation attempt with Insufficient Capacity Error Cache expiry", func() {
			awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "inf2.24xlarge", Zone: "test-zone-1a"}})
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{
				NodeSelector: map[string]string{corev1.LabelInstanceTypeStable: "inf2.24xlarge"},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("2")},
					Limits:   corev1.ResourceList{v1.ResourceAWSNeuron: resource.MustParse("2")},
				},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
			// capacity shortage is over - expire the item from the cache and try again
			awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{})
			awsEnv.UnavailableOfferingsCache.Delete("inf2.24xlarge", "test-zone-1a", karpv1.CapacityTypeOnDemand)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, "inf2.24xlarge"))
		})
		It("should launch instances in a different zone on second reconciliation attempt with Insufficient Capacity Error Cache fallback (Habana)", func() {
			awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "dl1.24xlarge", Zone: "test-zone-1a"}})
			pod := coretest.UnschedulablePod(coretest.PodOptions{
				NodeSelector: map[string]string{corev1.LabelInstanceTypeStable: "dl1.24xlarge"},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{v1.ResourceHabanaGaudi: resource.MustParse("1")},
					Limits:   corev1.ResourceList{v1.ResourceHabanaGaudi: resource.MustParse("1")},
				},
			})
			pod.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
					}},
				},
			}}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// it should've tried to pack them in test-zone-1a on a dl1.24xlarge then hit insufficient capacity, the next attempt will try test-zone-1b
			ExpectNotScheduled(ctx, env.Client, pod)

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(SatisfyAll(
				HaveKeyWithValue(corev1.LabelInstanceTypeStable, "dl1.24xlarge"),
				HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-1b")))
		})
		It("should launch on-demand capacity if flexible to both spot and on-demand, but spot is unavailable", func() {
			Expect(func() error {
				output, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
				if err != nil {
					return err
				}

				// Call the function with the output and false (indicating it's not the last page)
				_ = func(dito *ec2.DescribeInstanceTypesOutput) bool {
					for _, it := range dito.InstanceTypes {
						awsEnv.EC2API.InsufficientCapacityPools.Add(fake.CapacityPool{
							CapacityType: karpv1.CapacityTypeSpot,
							InstanceType: string(it.InstanceType),
							Zone:         "test-zone-1a",
						})
					}
					return true
				}(output)

				return nil
			}()).To(Succeed())
			nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeSpot, karpv1.CapacityTypeOnDemand}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}}},
			}
			// Spot Unavailable
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
			// include deprioritized instance types
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// Fallback to OD
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
		})
		It("should return all instance types, even though with no offerings due to Insufficient Capacity Error", func() {
			awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
				{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
				{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
				{CapacityType: karpv1.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
				{CapacityType: karpv1.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
			})
			nodePool.Spec.Template.Spec.Requirements = nil
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceType,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"m5.xlarge"},
				},
			},
			)
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"spot", "on-demand"},
				},
			})

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			for _, ct := range []string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeSpot} {
				for _, zone := range []string{"test-zone-1a", "test-zone-1b"} {
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
						coretest.UnschedulablePod(coretest.PodOptions{
							ResourceRequirements: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
							},
							NodeSelector: map[string]string{
								karpv1.CapacityTypeLabelKey: ct,
								corev1.LabelTopologyZone:    zone,
							},
						}))
				}
			}

			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
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
		It("should not return an offering when marking an offering as unavailable in the Insufficient Capacity Error Cache", func() {
			ExpectApplied(ctx, env.Client, nodeClass)

			// Initial list of GetInstanceTypes
			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			m5InstanceType, ok := lo.Find(instanceTypes, func(it *corecloudprovider.InstanceType) bool {
				return it.Name == string(ec2types.InstanceTypeM5Large)
			})
			Expect(ok).To(BeTrue())
			Expect(m5InstanceType.Offerings.Available()).To(HaveLen(6))

			// Mark spot m5.xlarge instance as unavailable in a few zones, nothing should change
			awsEnv.UnavailableOfferingsCache.MarkUnavailable(ctx, "test", ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeSpot)
			awsEnv.UnavailableOfferingsCache.MarkUnavailable(ctx, "test", ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)
			Expect(err).ToNot(HaveOccurred())
			m5InstanceType, ok = lo.Find(instanceTypes, func(it *corecloudprovider.InstanceType) bool {
				return it.Name == string(ec2types.InstanceTypeM5Large)
			})
			Expect(ok).To(BeTrue())
			Expect(m5InstanceType.Offerings.Available()).To(HaveLen(6))

			// Mark spot m5.large instance in test-zone-1a as unavailable
			awsEnv.UnavailableOfferingsCache.MarkUnavailable(ctx, "test", ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeSpot)
			instanceTypes, err = cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			m5InstanceType, ok = lo.Find(instanceTypes, func(it *corecloudprovider.InstanceType) bool {
				return it.Name == string(ec2types.InstanceTypeM5Large)
			})
			Expect(ok).To(BeTrue())
			Expect(m5InstanceType.Offerings.Available()).To(HaveLen(5))
			Expect(m5InstanceType.Offerings.Compatible(scheduling.NewLabelRequirements(map[string]string{
				corev1.LabelTopologyZone:    "test-zone-1a",
				karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeSpot,
			}))[0].Available).To(BeFalse())

			// Mark on-demand m5.large instance in test-zone-1b and test-zone-1c as unavailable
			awsEnv.UnavailableOfferingsCache.MarkUnavailable(ctx, "test", ec2types.InstanceTypeM5Large, "test-zone-1b", karpv1.CapacityTypeOnDemand)
			awsEnv.UnavailableOfferingsCache.MarkUnavailable(ctx, "test", ec2types.InstanceTypeM5Large, "test-zone-1c", karpv1.CapacityTypeOnDemand)

			instanceTypes, err = cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			m5InstanceType, ok = lo.Find(instanceTypes, func(it *corecloudprovider.InstanceType) bool {
				return it.Name == string(ec2types.InstanceTypeM5Large)
			})
			Expect(ok).To(BeTrue())
			Expect(m5InstanceType.Offerings.Available()).To(HaveLen(3))
			Expect(m5InstanceType.Offerings.Compatible(scheduling.NewLabelRequirements(map[string]string{
				corev1.LabelTopologyZone:    "test-zone-1a",
				karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeSpot,
			}))[0].Available).To(BeFalse())
			Expect(m5InstanceType.Offerings.Compatible(scheduling.NewLabelRequirements(map[string]string{
				corev1.LabelTopologyZone:    "test-zone-1b",
				karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand,
			}))[0].Available).To(BeFalse())
			Expect(m5InstanceType.Offerings.Compatible(scheduling.NewLabelRequirements(map[string]string{
				corev1.LabelTopologyZone:    "test-zone-1c",
				karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand,
			}))[0].Available).To(BeFalse())
		})
		It("should not return a capacity type when marking a capacity type as unavailable in the Insufficient Capacity Error Cache", func() {
			ExpectApplied(ctx, env.Client, nodeClass)

			// Initial list of GetInstanceTypes
			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			capacityTypes := sets.New[string]()
			for _, it := range instanceTypes {
				for _, of := range it.Offerings.Available() {
					capacityTypes.Insert(of.CapacityType())
				}
			}

			Expect(capacityTypes).To(HaveLen(2))
			Expect(capacityTypes.UnsortedList()).To(ConsistOf([]string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeSpot}))

			// Mark one of the zones as unavailable
			awsEnv.UnavailableOfferingsCache.MarkCapacityTypeUnavailable(karpv1.CapacityTypeSpot)

			// Initial list of GetInstanceTypes
			instanceTypes, err = cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			capacityTypes = sets.New[string]()
			for _, it := range instanceTypes {
				for _, of := range it.Offerings.Available() {
					capacityTypes.Insert(of.CapacityType())
				}
			}
			Expect(capacityTypes).To(HaveLen(1))
			Expect(capacityTypes.UnsortedList()).To(ConsistOf([]string{karpv1.CapacityTypeOnDemand}))
		})
		It("should not return a zone when marking a zone as unavailable in the Insufficient Capacity Error Cache", func() {
			ExpectApplied(ctx, env.Client, nodeClass)

			// Initial list of GetInstanceTypes
			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			zones := sets.New[string]()
			for _, it := range instanceTypes {
				for _, of := range it.Offerings.Available() {
					zones.Insert(of.Zone())
				}
			}

			Expect(zones).To(HaveLen(3))
			Expect(zones.UnsortedList()).To(ConsistOf([]string{"test-zone-1a", "test-zone-1b", "test-zone-1c"}))

			// Mark one of the zones as unavailable
			awsEnv.UnavailableOfferingsCache.MarkAZUnavailable("test-zone-1a")

			// Initial list of GetInstanceTypes
			instanceTypes, err = cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			zones = sets.New[string]()
			for _, it := range instanceTypes {
				for _, of := range it.Offerings.Available() {
					zones.Insert(of.Zone())
				}
			}
			Expect(zones).To(HaveLen(2))
			Expect(zones.UnsortedList()).To(ConsistOf([]string{"test-zone-1b", "test-zone-1c"}))
		})
	})
	Context("CapacityType", func() {
		It("should default to on-demand", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
		})
		It("should launch spot capacity if flexible to both spot and on demand", func() {
			nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeSpot, karpv1.CapacityTypeOnDemand}}}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeSpot))
		})
		It("should fail to launch capacity when there is no zonal availability for spot", func() {
			now := time.Now()
			awsEnv.EC2API.DescribeSpotPriceHistoryBehavior.Output.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []ec2types.SpotPrice{
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     "m5.large",
						SpotPrice:        aws.String("0.004"),
						Timestamp:        &now,
					},
				},
			})
			Expect(awsEnv.PricingProvider.UpdateSpotPricing(ctx)).To(Succeed())

			nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeSpot}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelInstanceTypeStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"m5.large"}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1b"}}},
			}

			// Instance type with no zonal availability for spot shouldn't be scheduled
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should succeed to launch spot instance when zonal availability exists", func() {
			now := time.Now()
			awsEnv.EC2API.DescribeSpotPriceHistoryBehavior.Output.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []ec2types.SpotPrice{
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     "m5.large",
						SpotPrice:        aws.String("0.004"),
						Timestamp:        &now,
					},
				},
			})
			Expect(awsEnv.PricingProvider.UpdateSpotPricing(ctx)).To(Succeed())

			// not restricting to the zone so we can get any zone
			nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeSpot}}},
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelInstanceTypeStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"m5.large"}}},
			}

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(karpv1.NodePoolLabelKey, nodePool.Name))
		})
	})
	Context("Ephemeral Storage", func() {
		BeforeEach(func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						SnapshotID:               aws.String("snap-xxxxxxxx"),
						VolumeInitializationRate: aws.Int32(100),
					},
				},
			}
		})
		It("should default to EBS defaults when volumeSize is not defined in blockDeviceMappings for custom AMIs", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"*": "*",
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Capacity.StorageEphemeral()).To(Equal(resource.MustParse("20Gi")))
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings).To(HaveLen(1))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].DeviceName).To(Equal("/dev/xvda"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.SnapshotId).To(Equal("snap-xxxxxxxx"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeInitializationRate).To(Equal(int32(100)))
			})
		})
		It("should default to EBS defaults when volumeSize is not defined in blockDeviceMappings for AL2 Root volume", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Capacity.StorageEphemeral()).To(Equal(resource.MustParse("20Gi")))
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings).To(HaveLen(1))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].DeviceName).To(Equal("/dev/xvda"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.SnapshotId).To(Equal("snap-xxxxxxxx"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeInitializationRate).To(Equal(int32(100)))
			})
		})
		It("should default to EBS defaults when volumeSize is not defined in blockDeviceMappings for AL2023 Root volume", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
			awsEnv.LaunchTemplateProvider.CABundle = lo.ToPtr("Y2EtYnVuZGxlCg==")
			awsEnv.LaunchTemplateProvider.ClusterCIDR.Store(lo.ToPtr("10.100.0.0/16"))
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Capacity.StorageEphemeral()).To(Equal(resource.MustParse("20Gi")))
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings).To(HaveLen(1))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].DeviceName).To(Equal("/dev/xvda"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.SnapshotId).To(Equal("snap-xxxxxxxx"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeInitializationRate).To(Equal(int32(100)))
			})
		})
		It("should default to EBS defaults when volumeSize is not defined in blockDeviceMappings for Bottlerocket Root volume", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			nodeClass.Spec.BlockDeviceMappings[0].DeviceName = aws.String("/dev/xvdb")
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Capacity.StorageEphemeral()).To(Equal(resource.MustParse("20Gi")))
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings).To(HaveLen(1))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].DeviceName).To(Equal("/dev/xvdb"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.SnapshotId).To(Equal("snap-xxxxxxxx"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeInitializationRate).To(Equal(int32(100)))
			})
		})
	})
	Context("Metadata Options", func() {
		It("should default metadata options on generated launch template", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpEndpoint).To(Equal(ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled))
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Disabled))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.MetadataOptions.HttpPutResponseHopLimit)).To(Equal(int32(1)))
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpTokens).To(Equal(ec2types.LaunchTemplateHttpTokensStateRequired))
			})
		})
		It("should set metadata options on generated launch template from nodePool configuration", func() {
			nodeClass.Spec.MetadataOptions = &v1.MetadataOptions{
				HTTPEndpoint:            aws.String(string(ec2types.LaunchTemplateInstanceMetadataEndpointStateDisabled)),
				HTTPProtocolIPv6:        aws.String(string(ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Enabled)),
				HTTPPutResponseHopLimit: aws.Int64(1),
				HTTPTokens:              aws.String(string(ec2types.LaunchTemplateHttpTokensStateOptional)),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpEndpoint).To(Equal(ec2types.LaunchTemplateInstanceMetadataEndpointStateDisabled))
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Enabled))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.MetadataOptions.HttpPutResponseHopLimit)).To(Equal(int32(1)))
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpTokens).To(Equal(ec2types.LaunchTemplateHttpTokensStateOptional))
			})
		})
	})
	Context("Offerings", func() {
		It("should create a single offering per instance pool", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			// Test twice to ensure we get the same result for a cache miss and hit
			for range 2 {
				instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
				Expect(err).ToNot(HaveOccurred())
				type instancePool struct {
					instanceType string
					zone         string
					capacityType string
				}
				pools := sets.New[instancePool]()
				for _, it := range instanceTypes {
					for _, o := range it.Offerings {
						pool := instancePool{
							instanceType: it.Name,
							zone:         o.Zone(),
							capacityType: o.CapacityType(),
						}
						Expect(pools.Has(pool)).To(BeFalse())
						pools.Insert(pool)
					}
				}
			}
		})
	})
	Context("Provider Cache", func() {
		It("should return the same set of instance types from the cache when no changes are made", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			list1, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			list2, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			ExpectSameInstanceTypeLists(list1, list2)
		})
		// Keeping the Cache testing in one IT block to validate the combinatorial expansion of instance types generated by different configs
		It("changes to kubelet configuration fields should result in a different set of instances types", func() {
			// We should expect these kubelet configuration fields to change the result of the instance type call
			// kubelet.kubeReserved
			// kubelet.systemReserved
			// kubelet.evictionHard
			// kubelet.evictionSoft
			// kubelet.maxPods
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				KubeReserved:   map[string]string{string(corev1.ResourceCPU): "1"},
				SystemReserved: map[string]string{string(corev1.ResourceCPU): "1"},
				EvictionHard:   map[string]string{"memory.available": "5%"},
				EvictionSoft:   map[string]string{"nodefs.available": "10%"},
				EvictionSoftGracePeriod: map[string]metav1.Duration{
					"nodefs.available": {Duration: time.Minute},
				},
				MaxPods: aws.Int32(10),
			}
			kubeletChanges := []*v1.KubeletConfiguration{
				{KubeReserved: map[string]string{string(corev1.ResourceCPU): "20"}},
				{SystemReserved: map[string]string{string(corev1.ResourceMemory): "10Gi"}},
				{EvictionHard: map[string]string{"memory.available": "52%"}},
				{EvictionSoft: map[string]string{"nodefs.available": "132%"}},
				{MaxPods: aws.Int32(20)},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			// Adding the general set of to the instancetype into the cache
			fullInstanceTypeResult, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).To(BeNil())

			instanceTypeResults := [][]*corecloudprovider.InstanceType{fullInstanceTypeResult}
			sorted := nodePool.DeepCopy()
			for _, change := range kubeletChanges {
				nodePool = sorted.DeepCopy()
				Expect(mergo.Merge(nodeClass.Spec.Kubelet, change, mergo.WithOverride, mergo.WithSliceDeepCopy)).To(BeNil())
				// Calling the provider and storing the instance type list to the instancetype provider cache
				_, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
				// We are making sure to pull from the cache
				instancetypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
				instanceTypeResults = append(instanceTypeResults, instancetypes)
			}

			// Based on the nodeclass configuration, we expect to have 5 unique set of instance types
			ExpectUniqueInstanceTypeLists(instanceTypeResults...)
		})
		It("changes to nodeclass fields should result in a different set of instances types", func() {
			// We should expect these nodeclass fields to change the result of the instance type
			// nodeClass.instanceStorePolicy
			// nodeClass.amiSelectorTerms (alias)
			// nodeClass.blockDeviceMapping.rootVolume
			// nodeClass.blockDeviceMapping.volumeSize
			// nodeClass.blockDeviceMapping.deviceName
			nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: lo.ToPtr("/dev/xvda"),
					EBS:        &v1.BlockDevice{VolumeSize: resource.NewScaledQuantity(20, resource.Giga)},
					RootVolume: false,
				},
			}
			nodeClassChanges := []*v1.EC2NodeClass{
				{Spec: v1.EC2NodeClassSpec{InstanceStorePolicy: lo.ToPtr(v1.InstanceStorePolicyRAID0)}},
				{Spec: v1.EC2NodeClassSpec{AMISelectorTerms: []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}}},
				{
					Spec: v1.EC2NodeClassSpec{
						BlockDeviceMappings: []*v1.BlockDeviceMapping{
							{
								DeviceName: lo.ToPtr("/dev/xvda"),
								EBS:        &v1.BlockDevice{VolumeSize: resource.NewScaledQuantity(20, resource.Giga)},
								RootVolume: false,
							},
							{
								DeviceName: lo.ToPtr("/dev/sda1"),
								EBS:        &v1.BlockDevice{VolumeSize: resource.NewScaledQuantity(10, resource.Giga)},
								RootVolume: true,
							},
						},
					},
				},
				{
					Spec: v1.EC2NodeClassSpec{
						BlockDeviceMappings: []*v1.BlockDeviceMapping{{
							DeviceName: lo.ToPtr("/dev/xvda"),
							EBS:        &v1.BlockDevice{VolumeSize: resource.NewScaledQuantity(15, resource.Giga)},
							RootVolume: false,
						}},
					},
				},
				{
					Spec: v1.EC2NodeClassSpec{
						BlockDeviceMappings: []*v1.BlockDeviceMapping{{
							DeviceName: lo.ToPtr("/dev/yvda"),
							EBS:        &v1.BlockDevice{VolumeSize: resource.NewScaledQuantity(25, resource.Giga)},
							RootVolume: true,
						}},
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			nodePool.Spec.Template.Spec.NodeClassRef.Name = nodeClass.Name
			// Adding the general set of to the instancetype into the cache
			fullInstanceTypeResult, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).To(BeNil())

			instanceTypeResults := [][]*corecloudprovider.InstanceType{fullInstanceTypeResult}
			sorted := nodeClass.DeepCopy()
			for _, change := range nodeClassChanges {
				nodeClass = sorted.DeepCopy()
				Expect(mergo.Merge(nodeClass, change, mergo.WithOverride)).To(BeNil())
				// Calling the provider and storing the instance type list to the instancetype provider cache
				_, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
				// We are making sure to pull from the cache
				its, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
				instanceTypeResults = append(instanceTypeResults, its)
			}

			// Based on the nodeclass configuration, we expect to have 5 unique set of instance types
			ExpectUniqueInstanceTypeLists(instanceTypeResults...)
		})
		It("updates to the instanceTypeInfo should result in a different set of instance types", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			// Initial list of GetInstanceTypes
			list1, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			// Update the instance types but don't change anything -- everything should remain cached
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx))
			list2, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			ExpectSameInstanceTypeLists(list1, list2)

			// Now, update the instance types, we should refresh the cache
			out, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
			Expect(err).ToNot(HaveOccurred())
			awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
				InstanceTypes: append([]ec2types.InstanceTypeInfo{}, out.InstanceTypes[1:]...), // Drop one of the instance types from the response
			})
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx))

			list3, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			ExpectUniqueInstanceTypeLists(list2, list3)
		})
		It("updates to the instanceTypeOfferingInfo should result in a different set of instance types", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			// Initial list of GetInstanceTypes
			list1, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			// Update the instance types but don't change anything -- everything should remain cached
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx))
			list2, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			ExpectSameInstanceTypeLists(list1, list2)

			// Now, update the instance types, we should refresh the cache
			out, err := awsEnv.EC2API.DescribeInstanceTypeOfferings(ctx, &ec2.DescribeInstanceTypeOfferingsInput{})
			Expect(err).ToNot(HaveOccurred())

			awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: append(
				[]ec2types.InstanceTypeOffering{
					{
						InstanceType: "m5.xlarge",
						Location:     lo.ToPtr("test-zone-1c"),
					},
				},
				out.InstanceTypeOfferings..., // Add an instance type offering option
			)})
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx))

			list3, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			ExpectUniqueInstanceTypeLists(list2, list3)
		})
		It("returning an ICE error only results in a cache miss for that instance type", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			// Initial list of GetInstanceTypes
			list1, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			awsEnv.UnavailableOfferingsCache.MarkUnavailable(ctx, "test", "m5.xlarge", "test-zone-1a", karpv1.CapacityTypeSpot)
			list2, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			// Only the m5.xlarge instance type should cache miss
			Expect(list2).To(HaveLen(len(list1)))
			for _, it2 := range list2 {
				it, ok := lo.Find(list1, func(it *corecloudprovider.InstanceType) bool {
					return it.Name == it2.Name
				})
				Expect(ok).To(BeTrue())
				if it2.Name == "m5.xlarge" {
					Expect(it2).ToNot(Equal(it))
					for _, of2 := range it2.Offerings {
						Expect(it.Offerings).ToNot(ContainElement(BeIdenticalTo(of2)))
					}
				} else {
					Expect(it2).To(Equal(it))
					Expect(it2.Offerings).To(ContainElements(lo.Map(it.Offerings, func(of *corecloudprovider.Offering, _ int) any { return BeIdenticalTo(of) })...))
				}
			}
		})
		It("returning an ICE error for capacity type results in a cache miss for every instance type", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			// Initial list of GetInstanceTypes
			list1, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			awsEnv.UnavailableOfferingsCache.MarkCapacityTypeUnavailable(karpv1.CapacityTypeSpot)
			list2, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			Expect(list2).To(HaveLen(len(list1)))
			for _, it2 := range list2 {
				it, ok := lo.Find(list1, func(it *corecloudprovider.InstanceType) bool {
					return it.Name == it2.Name
				})
				Expect(ok).To(BeTrue())
				for _, of2 := range it2.Offerings {
					Expect(it.Offerings).ToNot(ContainElement(BeIdenticalTo(of2)))
				}
			}

			awsEnv.UnavailableOfferingsCache.MarkAZUnavailable("test-zone-1a")
			list3, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())

			Expect(list3).To(HaveLen(len(list2)))
			for _, it3 := range list3 {
				it2, ok := lo.Find(list2, func(it2 *corecloudprovider.InstanceType) bool {
					return it2.Name == it3.Name
				})
				Expect(ok).To(BeTrue())
				for _, of3 := range it3.Offerings {
					Expect(it2.Offerings).ToNot(ContainElement(BeIdenticalTo(of3)))
				}
			}
		})
	})
	It("should not cause data races when calling List() simultaneously", func() {
		mu := sync.RWMutex{}
		var instanceTypeOrder []string
		wg := sync.WaitGroup{}
		for i := 0; i < 10000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())

				// Sort everything in parallel and ensure that we don't get data races
				sort.Slice(instanceTypes, func(i, j int) bool {
					return instanceTypes[i].Name < instanceTypes[j].Name
				})
				// Get the ordering of the instance types based on name
				tempInstanceTypeOrder := lo.Map(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) string {
					return i.Name
				})
				// Expect that all the elements in the instance type list are unique
				Expect(lo.Uniq(tempInstanceTypeOrder)).To(HaveLen(len(tempInstanceTypeOrder)))

				// We have to lock since we are doing simultaneous access to this value
				mu.Lock()
				if len(instanceTypeOrder) == 0 {
					instanceTypeOrder = tempInstanceTypeOrder
				} else {
					Expect(tempInstanceTypeOrder).To(BeEquivalentTo(instanceTypeOrder))
				}
				mu.Unlock()
			}()
		}
		wg.Wait()
	})
	Context("Capacity Blocks", func() {
		const crInstanceType = "c6g.large"
		const crZone = "test-zone-1a"
		const crID = "cr-123"
		const crCapacity = 1
		BeforeEach(func() {
			awsEnv.CapacityReservationProvider.SetAvailableInstanceCount(crID, crCapacity)
			nodeClass.Status.CapacityReservations = []v1.CapacityReservation{{
				AvailabilityZone: crZone,
				ID:               crID,
				InstanceType:     crInstanceType,
				ReservationType:  v1.CapacityReservationTypeCapacityBlock,
			}}
		})
		DescribeTable(
			"should create an offering for a capacity block",
			func(state v1.CapacityReservationState) {
				nodeClass.Status.CapacityReservations[0].State = state
				instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())

				var instanceType *corecloudprovider.InstanceType
				for _, it := range instanceTypes {
					if it.Name == crInstanceType {
						instanceType = it
						break
					}
				}
				Expect(instanceType).ToNot(BeNil())

				var offering *corecloudprovider.Offering
				for _, o := range instanceType.Offerings {
					if o.CapacityType() == karpv1.CapacityTypeReserved {
						if offering != nil {
							Fail("only a single reserved offering should exist")
						}
						offering = o
					}
				}
				Expect(offering).ToNot(BeNil())

				Expect(offering.Requirements.Has(karpv1.CapacityTypeLabelKey)).To(BeTrue())
				Expect(offering.Requirements.Get(karpv1.CapacityTypeLabelKey).Any()).To(Equal(karpv1.CapacityTypeReserved))
				Expect(offering.Requirements.Has(corev1.LabelTopologyZone)).To(BeTrue())
				Expect(offering.Requirements.Get(corev1.LabelTopologyZone).Any()).To(Equal(crZone))
				Expect(offering.Requirements.Has(v1.LabelCapacityReservationType)).To(BeTrue())
				Expect(offering.Requirements.Get(v1.LabelCapacityReservationType).Any()).To(Equal(string(v1.CapacityReservationTypeCapacityBlock)))
				Expect(offering.Requirements.Has(v1.LabelCapacityReservationID)).To(BeTrue())
				Expect(offering.Requirements.Get(v1.LabelCapacityReservationID).Any()).To(Equal(crID))
				Expect(offering.Available).To(Equal(state != v1.CapacityReservationStateExpiring))
				Expect(offering.ReservationCapacity).To(Equal(crCapacity))
			},
			Entry("when the capacity block is active", v1.CapacityReservationStateActive),
			Entry("when the capacity block is expiring", v1.CapacityReservationStateExpiring),
		)
	})
})

func ExpectSameInstanceTypeLists(instanceTypesLists ...[]*corecloudprovider.InstanceType) {
	GinkgoHelper()

	for _, itList := range instanceTypesLists {
		sort.Slice(itList, func(x int, y int) bool {
			return itList[x].Name < itList[y].Name
		})
	}
	for x := range instanceTypesLists {
		for y := range instanceTypesLists {
			if x == y {
				continue
			}
			Expect(instanceTypesLists[x]).To(HaveLen(len(instanceTypesLists[y])))
			for i := range instanceTypesLists[x] {
				ExpectIdenticalInstanceTypes(instanceTypesLists[x][i], instanceTypesLists[y][i])
			}
		}
	}
}

func ExpectUniqueInstanceTypeLists(instanceTypesLists ...[]*corecloudprovider.InstanceType) {
	GinkgoHelper()

	for _, itList := range instanceTypesLists {
		sort.Slice(itList, func(x int, y int) bool {
			return itList[x].Name < itList[y].Name
		})
	}
	for x := range instanceTypesLists {
		for y := range instanceTypesLists {
			if x == y {
				continue
			}
			Expect(reflect.DeepEqual(instanceTypesLists[x], instanceTypesLists[y])).To(BeFalse())
		}
	}
}

func ExpectIdenticalInstanceTypes(instanceTypes ...*corecloudprovider.InstanceType) {
	GinkgoHelper()

	for x := range instanceTypes {
		for y := range instanceTypes {
			if x == y {
				continue
			}
			// We have to compare a subset of the instance type fields because we create a new pointer for the instance type each time we call GetInstanceTypes()
			Expect(instanceTypes[x].Name).To(BeIdenticalTo(instanceTypes[y].Name))
			Expect(reflect.ValueOf(instanceTypes[x].Requirements).Pointer()).To(BeIdenticalTo(reflect.ValueOf(instanceTypes[y].Requirements).Pointer()))
			Expect(reflect.ValueOf(instanceTypes[x].Capacity).Pointer()).To(BeIdenticalTo(reflect.ValueOf(instanceTypes[y].Capacity).Pointer()))
			Expect(reflect.ValueOf(instanceTypes[x].Overhead).Pointer()).To(BeIdenticalTo(reflect.ValueOf(instanceTypes[y].Overhead).Pointer()))
		}
	}
}

// generateSpotPricing creates a spot price history output for use in a mock that has all spot offerings discounted by 50%
// vs the on-demand offering.
func generateSpotPricing(cp *cloudprovider.CloudProvider, nodePool *karpv1.NodePool) *ec2.DescribeSpotPriceHistoryOutput {
	rsp := &ec2.DescribeSpotPriceHistoryOutput{}
	instanceTypes, err := cp.GetInstanceTypes(ctx, nodePool)
	awsEnv.InstanceTypesProvider.Reset()
	Expect(err).To(Succeed())
	t := fakeClock.Now()

	for _, it := range instanceTypes {
		instanceType := it
		onDemandPrice := 1.00
		for _, o := range it.Offerings {
			if o.Requirements.Get(karpv1.CapacityTypeLabelKey).Any() == karpv1.CapacityTypeOnDemand {
				onDemandPrice = o.Price
			}
		}
		for _, o := range instanceType.Offerings {
			if o.Requirements.Get(karpv1.CapacityTypeLabelKey).Any() != karpv1.CapacityTypeSpot {
				continue
			}
			zone := o.Requirements.Get(corev1.LabelTopologyZone).Any()
			spotPrice := fmt.Sprintf("%0.3f", onDemandPrice*0.5)
			rsp.SpotPriceHistory = append(rsp.SpotPriceHistory, ec2types.SpotPrice{
				AvailabilityZone: lo.ToPtr(zone),
				InstanceType:     ec2types.InstanceType(instanceType.Name),
				SpotPrice:        lo.ToPtr(spotPrice),
				Timestamp:        lo.ToPtr(t),
			})
		}
	}
	return rsp
}
