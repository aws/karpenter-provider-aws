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

package cloudprovider_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	opstatus "github.com/awslabs/operatorpkg/status"
	"github.com/imdario/mergo"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/status"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	corecloudproivder "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var prov *provisioning.Provisioner
var cloudProvider *cloudprovider.CloudProvider
var cluster *state.Cluster
var fakeClock *clock.FakeClock
var recorder events.Recorder

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "cloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	fakeClock = clock.NewFakeClock(time.Now())
	recorder = events.NewRecorder(&record.FakeRecorder{})
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, recorder,
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, recorder, cloudProvider, cluster, fakeClock)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())

	cluster.Reset()
	awsEnv.Reset()

	awsEnv.LaunchTemplateProvider.KubeDNSIP = net.ParseIP("10.0.100.10")
	awsEnv.LaunchTemplateProvider.ClusterEndpoint = "https://test-cluster"
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("CloudProvider", func() {
	var nodeClass *v1beta1.EC2NodeClass
	var nodePool *corev1beta1.NodePool
	var nodeClaim *corev1beta1.NodeClaim
	var _ = BeforeEach(func() {
		nodeClass = test.EC2NodeClass(
			v1beta1.EC2NodeClass{
				Status: v1beta1.EC2NodeClassStatus{
					InstanceProfile: "test-profile",
					SecurityGroups: []v1beta1.SecurityGroup{
						{
							ID:   "sg-test1",
							Name: "securityGroup-test1",
						},
						{
							ID:   "sg-test2",
							Name: "securityGroup-test2",
						},
						{
							ID:   "sg-test3",
							Name: "securityGroup-test3",
						},
					},
					Subnets: []v1beta1.Subnet{
						{
							ID:     "subnet-test1",
							Zone:   "test-zone-1a",
							ZoneID: "tstz1-1a",
						},
						{
							ID:     "subnet-test2",
							Zone:   "test-zone-1b",
							ZoneID: "tstz1-1b",
						},
						{
							ID:     "subnet-test3",
							Zone:   "test-zone-1c",
							ZoneID: "tstz1-1c",
						},
					},
				},
			},
		)
		nodeClass.StatusConditions().SetTrue(opstatus.ConditionReady)
		nodePool = coretest.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
						Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
							{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: corev1beta1.CapacityTypeLabelKey, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.CapacityTypeOnDemand}}},
						},
					},
				},
			},
		})
		nodeClaim = coretest.NodeClaim(corev1beta1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{corev1beta1.NodePoolLabelKey: nodePool.Name},
			},
			Spec: corev1beta1.NodeClaimSpec{
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
				},
			},
		})
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass) // Hydrate the subnet cache
		Expect(err).To(BeNil())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
	})
	It("should not proceed with instance creation of nodeClass in not ready", func() {
		nodeClass.StatusConditions().SetFalse(opstatus.ConditionReady, "NodeClassNotReady", "NodeClass not ready")
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		_, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(HaveOccurred())
	})
	It("should return an ICE error when there are no instance types to launch", func() {
		// Specify no instance types and expect to receive a capacity error
		nodeClaim.Spec.Requirements = []corev1beta1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"test-instance-type"},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(corecloudproivder.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(cloudProviderNodeClaim).To(BeNil())
	})
	It("should set ImageID in the status field of the nodeClaim", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		Expect(cloudProviderNodeClaim.Status.ImageID).ToNot(BeEmpty())
	})
	It("should return availability zone ID as a label on the nodeClaim", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).ToNot(HaveOccurred())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		zone, ok := cloudProviderNodeClaim.GetLabels()[v1.LabelTopologyZone]
		Expect(ok).To(BeTrue())
		zoneID, ok := cloudProviderNodeClaim.GetLabels()[v1beta1.LabelTopologyZoneID]
		Expect(ok).To(BeTrue())
		subnet, ok := lo.Find(nodeClass.Status.Subnets, func(s v1beta1.Subnet) bool {
			return s.Zone == zone
		})
		Expect(ok).To(BeTrue())
		Expect(zoneID).To(Equal(subnet.ZoneID))
	})
	It("should return NodeClass Hash on the nodeClaim", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		_, ok := cloudProviderNodeClaim.ObjectMeta.Annotations[v1beta1.AnnotationEC2NodeClassHash]
		Expect(ok).To(BeTrue())
	})
	It("should return NodeClass Hash Version on the nodeClaim", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		v, ok := cloudProviderNodeClaim.ObjectMeta.Annotations[v1beta1.AnnotationEC2NodeClassHashVersion]
		Expect(ok).To(BeTrue())
		Expect(v).To(Equal(v1beta1.EC2NodeClassHashVersion))
	})
	Context("EC2 Context", func() {
		contextID := "context-1234"
		It("should set context on the CreateFleet request if specified on the NodePool", func() {
			nodeClass.Spec.Context = aws.String(contextID)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(aws.StringValue(createFleetInput.Context)).To(Equal(contextID))
		})
		It("should default to no EC2 Context", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.Context).To(BeNil())
		})
	})
	Context("MinValues", func() {
		It("CreateFleet input should respect minValues for In operator requirement from NodePool", func() {
			// Create fake InstanceTypes where one instances can fit 2 pods and another one can fit only 1 pod.
			// This specific type of inputs will help us differentiate the scenario we are trying to test where ideally
			// 1 instance launch would have been sufficient to fit the pods and was cheaper but we would launch 2 separate
			// instances to meet the minimum requirement.
			instances := fake.MakeInstances()
			instances, _ = fake.MakeUniqueInstancesAndFamilies(instances, 2)
			instances[0].VCpuInfo = &ec2.VCpuInfo{DefaultVCpus: aws.Int64(1)}
			instances[1].VCpuInfo = &ec2.VCpuInfo{DefaultVCpus: aws.Int64(8)}
			awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{InstanceTypes: instances})
			awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: fake.MakeInstanceOfferings(instances)})
			now := time.Now()
			awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []*ec2.SpotPrice{
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     instances[0].InstanceType,
						SpotPrice:        aws.String("0.002"),
						Timestamp:        &now,
					},
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     instances[1].InstanceType,
						SpotPrice:        aws.String("0.003"),
						Timestamp:        &now,
					},
				},
			})
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
			Expect(awsEnv.PricingProvider.UpdateSpotPricing(ctx)).To(Succeed())
			instanceNames := lo.Map(instances, func(info *ec2.InstanceTypeInfo, _ int) string { return *info.InstanceType })

			// Define NodePool that has minValues on instance-type requirement.
			nodePool = coretest.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{
								Name: nodeClass.Name,
							},
							Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      corev1beta1.CapacityTypeLabelKey,
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{corev1beta1.CapacityTypeSpot},
									},
								},
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceTypeStable,
										Operator: v1.NodeSelectorOpIn,
										Values:   instanceNames,
									},
									MinValues: lo.ToPtr(2),
								},
							},
						},
					},
				},
			})

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)

			// 2 pods are created with resources such that both fit together only in one of the 2 InstanceTypes created above.
			pod1 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			pod2 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)

			// Under normal circumstances 1 node would have been created that fits both the pods but
			// here minValue enforces to include both the instances. And since one of the instances can
			// only fit 1 pod, only 1 pod is scheduled to run in the node to be launched by CreateFleet.
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(2))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			uniqueInstanceTypes := sets.Set[string]{}
			for _, launchTemplateConfig := range createFleetInput.LaunchTemplateConfigs {
				for _, override := range launchTemplateConfig.Overrides {
					uniqueInstanceTypes.Insert(*override.InstanceType)
				}
			}
			// This ensures that we have sent the minimum number of requirements defined in the NodePool.
			Expect(len(uniqueInstanceTypes)).To(BeNumerically(">=", 2))
		})
		It("CreateFleet input should respect minValues for Exists Operator in requirement from NodePool", func() {
			// Create fake InstanceTypes where one instances can fit 2 pods and another one can fit only 1 pod.
			instances := fake.MakeInstances()
			instances, _ = fake.MakeUniqueInstancesAndFamilies(instances, 2)
			instances[0].VCpuInfo = &ec2.VCpuInfo{DefaultVCpus: aws.Int64(1)}
			instances[1].VCpuInfo = &ec2.VCpuInfo{DefaultVCpus: aws.Int64(8)}
			awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{InstanceTypes: instances})
			awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: fake.MakeInstanceOfferings(instances)})
			now := time.Now()
			awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []*ec2.SpotPrice{
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     instances[0].InstanceType,
						SpotPrice:        aws.String("0.002"),
						Timestamp:        &now,
					},
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     instances[1].InstanceType,
						SpotPrice:        aws.String("0.003"),
						Timestamp:        &now,
					},
				},
			})
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
			Expect(awsEnv.PricingProvider.UpdateSpotPricing(ctx)).To(Succeed())
			instanceNames := lo.Map(instances, func(info *ec2.InstanceTypeInfo, _ int) string { return *info.InstanceType })

			// Define NodePool that has minValues on instance-type requirement.
			nodePool = coretest.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{
								Name: nodeClass.Name,
							},
							Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceTypeStable,
										Operator: v1.NodeSelectorOpExists,
									},
									MinValues: lo.ToPtr(2),
								},
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceTypeStable,
										Operator: v1.NodeSelectorOpIn,
										Values:   instanceNames,
									},
									MinValues: lo.ToPtr(1),
								},
							},
						},
					},
				},
			})

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)

			// 2 pods are created with resources such that both fit together only in one of the 2 InstanceTypes created above.
			pod1 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			pod2 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)

			// Under normal circumstances 1 node would have been created that fits both the pods but
			// here minValue enforces to include both the instances. And since one of the instances can
			// only fit 1 pod, only 1 pod is scheduled to run in the node to be launched by CreateFleet.
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(2))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			uniqueInstanceTypes := sets.Set[string]{}
			for _, launchTemplateConfig := range createFleetInput.LaunchTemplateConfigs {
				for _, override := range launchTemplateConfig.Overrides {
					uniqueInstanceTypes.Insert(*override.InstanceType)
				}
			}
			// This ensures that we have sent the minimum number of requirements defined in the NodePool.
			Expect(len(uniqueInstanceTypes)).To(BeNumerically(">=", 2))
		})
		It("CreateFleet input should respect minValues from multiple keys in NodePool", func() {
			// Create fake InstanceTypes where 2 instances can fit 2 pods individually and one can fit only 1 pod.
			instances := fake.MakeInstances()
			uniqInstanceTypes, instanceFamilies := fake.MakeUniqueInstancesAndFamilies(instances, 3)
			uniqInstanceTypes[0].VCpuInfo = &ec2.VCpuInfo{DefaultVCpus: aws.Int64(1)}
			uniqInstanceTypes[1].VCpuInfo = &ec2.VCpuInfo{DefaultVCpus: aws.Int64(4)}
			uniqInstanceTypes[2].VCpuInfo = &ec2.VCpuInfo{DefaultVCpus: aws.Int64(8)}
			awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{InstanceTypes: uniqInstanceTypes})
			awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: fake.MakeInstanceOfferings(uniqInstanceTypes)})
			now := time.Now()
			awsEnv.EC2API.DescribeSpotPriceHistoryOutput.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []*ec2.SpotPrice{
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     uniqInstanceTypes[0].InstanceType,
						SpotPrice:        aws.String("0.002"),
						Timestamp:        &now,
					},
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     uniqInstanceTypes[1].InstanceType,
						SpotPrice:        aws.String("0.003"),
						Timestamp:        &now,
					},
					{
						AvailabilityZone: aws.String("test-zone-1a"),
						InstanceType:     uniqInstanceTypes[2].InstanceType,
						SpotPrice:        aws.String("0.004"),
						Timestamp:        &now,
					},
				},
			})
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
			Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
			Expect(awsEnv.PricingProvider.UpdateSpotPricing(ctx)).To(Succeed())
			instanceNames := lo.Map(uniqInstanceTypes, func(info *ec2.InstanceTypeInfo, _ int) string { return *info.InstanceType })

			// Define NodePool that has minValues in multiple requirements.
			nodePool = coretest.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{
								Name: nodeClass.Name,
							},
							Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceTypeStable,
										Operator: v1.NodeSelectorOpIn,
										Values:   instanceNames,
									},
									// consider at least 2 unique instance types
									MinValues: lo.ToPtr(2),
								},
								{
									NodeSelectorRequirement: v1.NodeSelectorRequirement{
										Key:      v1beta1.LabelInstanceFamily,
										Operator: v1.NodeSelectorOpIn,
										Values:   instanceFamilies.UnsortedList(),
									},
									// consider at least 3 unique instance families
									MinValues: lo.ToPtr(3),
								},
							},
						},
					},
				},
			})

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod1 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			pod2 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)

			// Under normal circumstances 1 node would have been created that fits both the pods but
			// here minValue enforces to include all the 3 instances to satisfy both the instance-type and instance-family requirements.
			// And since one of the instances can only fit 1 pod, only 1 pod is scheduled to run in the node to be launched by CreateFleet.
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)
			Expect(node1.Name).ToNot(Equal(node2.Name))
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(2))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			uniqueInstanceTypes, uniqueInstanceFamilies := sets.Set[string]{}, sets.Set[string]{}
			for _, launchTemplateConfig := range createFleetInput.LaunchTemplateConfigs {
				for _, override := range launchTemplateConfig.Overrides {
					uniqueInstanceTypes.Insert(*override.InstanceType)
					uniqueInstanceFamilies.Insert(strings.Split(*override.InstanceType, ".")[0])
				}
			}
			// Ensure that there are at least minimum number of unique instance types as per the requirement in the CreateFleet request.
			Expect(len(uniqueInstanceTypes)).To(BeNumerically("==", 3))
			// Ensure that there are at least minimum number of unique instance families as per the requirement in the CreateFleet request.
			Expect(len(uniqueInstanceFamilies)).To(BeNumerically("==", 3))
		})
	})
	Context("NodeClaim Drift", func() {
		var armAMIID, amdAMIID string
		var validSecurityGroup string
		var selectedInstanceType *corecloudproivder.InstanceType
		var instance *ec2.Instance
		var validSubnet1 string
		var validSubnet2 string
		BeforeEach(func() {
			armAMIID, amdAMIID = fake.ImageID(), fake.ImageID()
			validSecurityGroup = fake.SecurityGroupID()
			validSubnet1 = fake.SubnetID()
			validSubnet2 = fake.SubnetID()
			awsEnv.SSMAPI.GetParameterOutput = &ssm.GetParameterOutput{
				Parameter: &ssm.Parameter{Value: aws.String(armAMIID)},
			}
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String(armAMIID),
						Architecture: aws.String("arm64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("ami-key-1"),
								Value: aws.String("ami-value-1"),
							},
						},
					},
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String(amdAMIID),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("ami-key-2"),
								Value: aws.String("ami-value-2"),
							},
						},
					},
				},
			})
			awsEnv.EC2API.DescribeSecurityGroupsOutput.Set(&ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []*ec2.SecurityGroup{
					{
						GroupId:   aws.String(validSecurityGroup),
						GroupName: aws.String("test-securitygroup"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("sg-key"),
								Value: aws.String("sg-value"),
							},
						},
					},
				},
			})
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{
				Subnets: []*ec2.Subnet{
					{
						SubnetId:         aws.String(validSubnet1),
						AvailabilityZone: aws.String("zone-1"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("sn-key-1"),
								Value: aws.String("sn-value-1"),
							},
						},
					},
					{
						SubnetId:         aws.String(validSubnet2),
						AvailabilityZone: aws.String("zone-2"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("sn-key-2"),
								Value: aws.String("sn-value-2"),
							},
						},
					},
				},
			})
			nodeClass.Status = v1beta1.EC2NodeClassStatus{
				InstanceProfile: "test-profile",
				Subnets: []v1beta1.Subnet{
					{
						ID:   validSubnet1,
						Zone: "zone-1",
					},
					{
						ID:   validSubnet2,
						Zone: "zone-2",
					},
				},
				SecurityGroups: []v1beta1.SecurityGroup{
					{
						ID: validSecurityGroup,
					},
				},
				AMIs: []v1beta1.AMI{
					{
						ID: armAMIID,
						Requirements: []v1.NodeSelectorRequirement{
							{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureArm64}},
						},
					},
					{
						ID: amdAMIID,
						Requirements: []v1.NodeSelectorRequirement{
							{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureAmd64}},
						},
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			selectedInstanceType = instanceTypes[0]

			// Create the instance we want returned from the EC2 API
			instance = &ec2.Instance{
				ImageId:               aws.String(armAMIID),
				InstanceType:          aws.String(selectedInstanceType.Name),
				SubnetId:              aws.String(validSubnet1),
				SpotInstanceRequestId: aws.String(coretest.RandomName()),
				State: &ec2.InstanceState{
					Name: aws.String(ec2.InstanceStateNameRunning),
				},
				InstanceId: aws.String(fake.InstanceID()),
				Placement: &ec2.Placement{
					AvailabilityZone: aws.String("test-zone-1a"),
				},
				SecurityGroups: []*ec2.GroupIdentifier{{GroupId: aws.String(validSecurityGroup)}},
			}
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{instance}}},
			})
			nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
				v1beta1.AnnotationEC2NodeClassHash:        nodeClass.Hash(),
				v1beta1.AnnotationEC2NodeClassHashVersion: v1beta1.EC2NodeClassHashVersion,
			})
			nodeClaim.Status.ProviderID = fake.ProviderID(lo.FromPtr(instance.InstanceId))
			nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{
				v1beta1.AnnotationEC2NodeClassHash:        nodeClass.Hash(),
				v1beta1.AnnotationEC2NodeClassHashVersion: v1beta1.EC2NodeClassHashVersion,
			})
			nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{v1.LabelInstanceTypeStable: selectedInstanceType.Name})
		})
		It("should not fail if NodeClass does not exist", func() {
			ExpectDeleted(ctx, env.Client, nodeClass)
			drifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(drifted).To(BeEmpty())
		})
		It("should not fail if NodePool does not exist", func() {
			ExpectDeleted(ctx, env.Client, nodePool)
			drifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(drifted).To(BeEmpty())
		})
		It("should return drifted if the AMI is not valid", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.ImageId = aws.String(fake.ImageID())
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.AMIDrift))
		})
		It("should return drifted if there are multiple drift reasons", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.ImageId = aws.String(fake.ImageID())
			instance.SubnetId = aws.String(fake.SubnetID())
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			// Assign a fake hash
			nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
				v1beta1.AnnotationEC2NodeClassHash: "abcdefghijkl",
			})
			ExpectApplied(ctx, env.Client, nodeClass)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.NodeClassDrift))
		})
		It("should return drifted if the subnet is not valid", func() {
			instance.SubnetId = aws.String(fake.SubnetID())
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SubnetDrift))
		})
		It("should return an error if subnets are empty", func() {
			awsEnv.SubnetCache.Flush()
			nodeClass.Status.Subnets = []v1beta1.Subnet{}
			ExpectApplied(ctx, env.Client, nodeClass)
			_, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
		})
		It("should not return drifted if the NodeClaim is valid", func() {
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should return an error if the security groups are empty", func() {
			nodeClass.Status.SecurityGroups = []v1beta1.SecurityGroup{}
			ExpectApplied(ctx, env.Client, nodeClass)
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			_, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
		})
		It("should return drifted if the instance security groups doesn't match the discovered values", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should return drifted if there are more instance security groups present than in the discovered values", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}, {GroupId: aws.String(validSecurityGroup)}}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should return drifted if more security groups are present than instance security groups then discovered from nodeclass", func() {
			nodeClass.Status.SecurityGroups = []v1beta1.SecurityGroup{
				{
					ID:   validSecurityGroup,
					Name: "test-securitygroup",
				},
				{
					ID:   fake.SecurityGroupID(),
					Name: "test-securitygroup",
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should not return drifted if the security groups match", func() {
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should error if the NodeClaim doesn't have the instance-type label", func() {
			delete(nodeClaim.Labels, v1.LabelInstanceTypeStable)
			_, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
		})
		It("should error drift if NodeClaim doesn't have provider id", func() {
			nodeClaim.Status = corev1beta1.NodeClaimStatus{}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should error if the underlying NodeClaim doesn't exist", func() {
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{}}},
			})
			_, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
		})
		It("should return drifted if the AMI no longer matches the existing NodeClaims instance type", func() {
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: amdAMIID}}
			nodeClass.Status.AMIs = []v1beta1.AMI{
				{
					ID: amdAMIID,
					Requirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureAmd64}},
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.AMIDrift))
		})
		Context("Static Drift Detection", func() {
			BeforeEach(func() {
				armRequirements := []v1.NodeSelectorRequirement{
					{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureArm64}},
				}
				amdRequirements := []v1.NodeSelectorRequirement{
					{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureAmd64}},
				}
				nodeClass = &v1beta1.EC2NodeClass{
					ObjectMeta: nodeClass.ObjectMeta,
					Spec: v1beta1.EC2NodeClassSpec{
						SubnetSelectorTerms:        nodeClass.Spec.SubnetSelectorTerms,
						SecurityGroupSelectorTerms: nodeClass.Spec.SecurityGroupSelectorTerms,
						Role:                       nodeClass.Spec.Role,
						UserData:                   lo.ToPtr("Fake Userdata"),
						Tags: map[string]string{
							"fakeKey": "fakeValue",
						},
						Context:                  lo.ToPtr("fake-context"),
						DetailedMonitoring:       lo.ToPtr(false),
						AMIFamily:                lo.ToPtr(v1beta1.AMIFamilyAL2023),
						AssociatePublicIPAddress: lo.ToPtr(false),
						MetadataOptions: &v1beta1.MetadataOptions{
							HTTPEndpoint:            lo.ToPtr("disabled"),
							HTTPProtocolIPv6:        lo.ToPtr("disabled"),
							HTTPPutResponseHopLimit: lo.ToPtr(int64(1)),
							HTTPTokens:              lo.ToPtr("optional"),
						},
						BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{
							{
								DeviceName: lo.ToPtr("fakeName"),
								RootVolume: false,
								EBS: &v1beta1.BlockDevice{
									DeleteOnTermination: lo.ToPtr(false),
									Encrypted:           lo.ToPtr(false),
									IOPS:                lo.ToPtr(int64(0)),
									KMSKeyID:            lo.ToPtr("fakeKMSKeyID"),
									SnapshotID:          lo.ToPtr("fakeSnapshot"),
									Throughput:          lo.ToPtr(int64(0)),
									VolumeSize:          resource.NewScaledQuantity(2, resource.Giga),
									VolumeType:          lo.ToPtr("standard"),
								},
							},
						},
					},
					Status: v1beta1.EC2NodeClassStatus{
						InstanceProfile: "test-profile",
						Subnets: []v1beta1.Subnet{
							{
								ID:   validSubnet1,
								Zone: "zone-1",
							},
							{
								ID:   validSubnet2,
								Zone: "zone-2",
							},
						},
						SecurityGroups: []v1beta1.SecurityGroup{
							{
								ID: validSecurityGroup,
							},
						},
						AMIs: []v1beta1.AMI{
							{
								ID:           armAMIID,
								Requirements: armRequirements,
							},
							{
								ID:           amdAMIID,
								Requirements: amdRequirements,
							},
						},
					},
				}
				nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1beta1.AnnotationEC2NodeClassHash: nodeClass.Hash()})
				nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{v1beta1.AnnotationEC2NodeClassHash: nodeClass.Hash()})
			})
			DescribeTable("should return drifted if a statically drifted EC2NodeClass.Spec field is updated",
				func(changes v1beta1.EC2NodeClass) {
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())

					Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride, mergo.WithSliceDeepCopy)).To(Succeed())
					nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1beta1.AnnotationEC2NodeClassHash: nodeClass.Hash()})

					ExpectApplied(ctx, env.Client, nodeClass)
					isDrifted, err = cloudProvider.IsDrifted(ctx, nodeClaim)
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(Equal(cloudprovider.NodeClassDrift))
				},
				Entry("UserData", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{UserData: lo.ToPtr("userdata-test-2")}}),
				Entry("Tags", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
				Entry("Context", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Context: lo.ToPtr("context-2")}}),
				Entry("DetailedMonitoring", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
				Entry("AMIFamily", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMIFamily: lo.ToPtr(v1beta1.AMIFamilyBottlerocket)}}),
				Entry("InstanceStorePolicy", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{InstanceStorePolicy: lo.ToPtr(v1beta1.InstanceStorePolicyRAID0)}}),
				Entry("AssociatePublicIPAddress", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AssociatePublicIPAddress: lo.ToPtr(true)}}),
				Entry("MetadataOptions HTTPEndpoint", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPEndpoint: lo.ToPtr("enabled")}}}),
				Entry("MetadataOptions HTTPProtocolIPv6", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPProtocolIPv6: lo.ToPtr("enabled")}}}),
				Entry("MetadataOptions HTTPPutResponseHopLimit", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPPutResponseHopLimit: lo.ToPtr(int64(10))}}}),
				Entry("MetadataOptions HTTPTokens", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPTokens: lo.ToPtr("required")}}}),
				Entry("BlockDeviceMapping DeviceName", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: lo.ToPtr("map-device-test-3")}}}}),
				Entry("BlockDeviceMapping RootVolume", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{RootVolume: true}}}}),
				Entry("BlockDeviceMapping DeleteOnTermination", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{DeleteOnTermination: lo.ToPtr(true)}}}}}),
				Entry("BlockDeviceMapping Encrypted", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{Encrypted: lo.ToPtr(true)}}}}}),
				Entry("BlockDeviceMapping IOPS", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{IOPS: lo.ToPtr(int64(10))}}}}}),
				Entry("BlockDeviceMapping KMSKeyID", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{KMSKeyID: lo.ToPtr("test")}}}}}),
				Entry("BlockDeviceMapping SnapshotID", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{SnapshotID: lo.ToPtr("test")}}}}}),
				Entry("BlockDeviceMapping Throughput", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{Throughput: lo.ToPtr(int64(10))}}}}}),
				Entry("BlockDeviceMapping VolumeType", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{VolumeType: lo.ToPtr("io1")}}}}}),
			)
			// We create a separate test for updating blockDeviceMapping volumeSize, since resource.Quantity is a struct, and mergo.WithSliceDeepCopy
			// doesn't work well with unexported fields, like the ones that are present in resource.Quantity
			It("should return drifted when updating blockDeviceMapping volumeSize", func() {
				nodeClass.Spec.BlockDeviceMappings[0].EBS.VolumeSize = resource.NewScaledQuantity(10, resource.Giga)
				nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1beta1.AnnotationEC2NodeClassHash: nodeClass.Hash()})

				ExpectApplied(ctx, env.Client, nodeClass)
				isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDrifted).To(Equal(cloudprovider.NodeClassDrift))
			})
			DescribeTable("should not return drifted if dynamic fields are updated",
				func(changes v1beta1.EC2NodeClass) {
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())

					Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride))
					nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1beta1.AnnotationEC2NodeClassHash: nodeClass.Hash()})

					ExpectApplied(ctx, env.Client, nodeClass)
					isDrifted, err = cloudProvider.IsDrifted(ctx, nodeClaim)
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())
				},
				Entry("AMI Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMISelectorTerms: []v1beta1.AMISelectorTerm{{Tags: map[string]string{"ami-key-1": "ami-value-1"}}}}}),
				Entry("Subnet Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{{Tags: map[string]string{"sn-key-1": "sn-value-1"}}}}}),
				Entry("SecurityGroup Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{{Tags: map[string]string{"sg-key": "sg-value"}}}}}),
			)
			It("should not return drifted if karpenter.k8s.aws/ec2nodeclass-hash annotation is not present on the NodeClaim", func() {
				nodeClaim.Annotations = map[string]string{
					v1beta1.AnnotationEC2NodeClassHashVersion: v1beta1.EC2NodeClassHashVersion,
				}
				nodeClass.Spec.Tags = map[string]string{
					"Test Key": "Test Value",
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDrifted).To(BeEmpty())
			})
			It("should not return drifted if the NodeClaim's karpenter.k8s.aws/ec2nodeclass-hash-version annotation does not match the EC2NodeClass's", func() {
				nodeClass.ObjectMeta.Annotations = map[string]string{
					v1beta1.AnnotationEC2NodeClassHash:        "test-hash-111111",
					v1beta1.AnnotationEC2NodeClassHashVersion: "test-hash-version-1",
				}
				nodeClaim.ObjectMeta.Annotations = map[string]string{
					v1beta1.AnnotationEC2NodeClassHash:        "test-hash-222222",
					v1beta1.AnnotationEC2NodeClassHashVersion: "test-hash-version-2",
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDrifted).To(BeEmpty())
			})
			It("should not return drifted if karpenter.k8s.aws/ec2nodeclass-hash-version annotation is not present on the NodeClass", func() {
				nodeClass.ObjectMeta.Annotations = map[string]string{
					v1beta1.AnnotationEC2NodeClassHash: "test-hash-111111",
				}
				nodeClaim.ObjectMeta.Annotations = map[string]string{
					v1beta1.AnnotationEC2NodeClassHash:        "test-hash-222222",
					v1beta1.AnnotationEC2NodeClassHashVersion: "test-hash-version-2",
				}
				// should trigger drift
				nodeClass.Spec.Tags = map[string]string{
					"Test Key": "Test Value",
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDrifted).To(BeEmpty())
			})
			It("should not return drifted if karpenter.k8s.aws/ec2nodeclass-hash-version annotation is not present on the NodeClaim", func() {
				nodeClass.ObjectMeta.Annotations = map[string]string{
					v1beta1.AnnotationEC2NodeClassHash:        "test-hash-111111",
					v1beta1.AnnotationEC2NodeClassHashVersion: "test-hash-version-1",
				}
				nodeClaim.ObjectMeta.Annotations = map[string]string{
					v1beta1.AnnotationEC2NodeClassHash: "test-hash-222222",
				}
				// should trigger drift
				nodeClass.Spec.Tags = map[string]string{
					"Test Key": "Test Value",
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDrifted).To(BeEmpty())
			})
		})
	})
	Context("Subnet Compatibility", func() {
		// Note when debugging these tests -
		// hard coded fixture data (ex. what the aws api will return) is maintained in fake/ec2api.go
		It("should default to the cluster's subnets", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(
				coretest.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: corev1beta1.ArchitectureAmd64}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			input := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(len(input.LaunchTemplateConfigs)).To(BeNumerically(">=", 1))

			foundNonGPULT := false
			for _, v := range input.LaunchTemplateConfigs {
				for _, ov := range v.Overrides {
					if *ov.InstanceType == "m5.large" {
						foundNonGPULT = true
						Expect(v.Overrides).To(ContainElements(
							&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test1"), ImageId: ov.ImageId, InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1a")},
							&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test2"), ImageId: ov.ImageId, InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1b")},
							&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test3"), ImageId: ov.ImageId, InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1c")},
						))
					}
				}
			}
			Expect(foundNonGPULT).To(BeTrue())
		})
		It("should launch instances into subnet with the most available IP addresses", func() {
			awsEnv.SubnetCache.Flush()
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int64(10),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int64(100),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			controller := status.NewController(env.Client, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider, awsEnv.LaunchTemplateProvider)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
		})
		It("should launch instances into subnet with the most available IP addresses in-between cache refreshes", func() {
			awsEnv.SubnetCache.Flush()
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int64(10),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int64(11),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			controller := status.NewController(env.Client, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider, awsEnv.LaunchTemplateProvider)
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{MaxPods: aws.Int32(1)}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			pod1 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"}})
			pod2 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			ExpectScheduled(ctx, env.Client, pod1)
			ExpectScheduled(ctx, env.Client, pod2)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
			// Provision for another pod that should now use the other subnet since we've consumed some from the first launch.
			pod3 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod3)
			ExpectScheduled(ctx, env.Client, pod3)
			createFleetInput = awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-1"))
		})
		It("should update in-flight IPs when a CreateFleet error occurs", func() {
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(10),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
			}})
			pod1 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"}})
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, pod1)
			awsEnv.EC2API.CreateFleetBehavior.Error.Set(fmt.Errorf("CreateFleet synthetic error"))
			bindings := ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1)
			Expect(len(bindings)).To(Equal(0))
		})
		It("should launch instances into subnets that are excluded by another NodePool", func() {
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int64(10),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int64(100),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{{Tags: map[string]string{"Name": "test-subnet-1"}}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			controller := status.NewController(env.Client, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider, awsEnv.LaunchTemplateProvider)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			podSubnet1 := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, podSubnet1)
			ExpectScheduled(ctx, env.Client, podSubnet1)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-1"))

			nodeClass2 := test.EC2NodeClass(v1beta1.EC2NodeClass{
				Spec: v1beta1.EC2NodeClassSpec{
					SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{
						{
							Tags: map[string]string{"Name": "test-subnet-2"},
						},
					},
					SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{
						{
							Tags: map[string]string{"*": "*"},
						},
					},
				},
				Status: v1beta1.EC2NodeClassStatus{
					AMIs: nodeClass.Status.AMIs,
					SecurityGroups: []v1beta1.SecurityGroup{
						{
							ID: "sg-test1",
						},
					},
				},
			})
			nodePool2 := coretest.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{
								Name: nodeClass2.Name,
							},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool2, nodeClass2)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass2)
			podSubnet2 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{corev1beta1.NodePoolLabelKey: nodePool2.Name}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, podSubnet2)
			ExpectScheduled(ctx, env.Client, podSubnet2)
			createFleetInput = awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
		})
		It("should launch instances with an alternate NodePool when a NodeClass selects 0 subnets, security groups, or amis", func() {
			misconfiguredNodeClass := test.EC2NodeClass(v1beta1.EC2NodeClass{
				Spec: v1beta1.EC2NodeClassSpec{
					// select nothing!
					SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{
						{
							Tags: map[string]string{"Name": "nothing"},
						},
					},
					// select nothing!
					SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{
						{
							Tags: map[string]string{"Name": "nothing"},
						},
					},
					// select nothing!
					AMISelectorTerms: []v1beta1.AMISelectorTerm{
						{
							Tags: map[string]string{"Name": "nothing"},
						},
					},
				},
			})
			nodePool2 := coretest.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{
								Name: misconfiguredNodeClass.Name,
							},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodePool2, nodeClass, misconfiguredNodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
	})
	Context("EFA", func() {
		It("should include vpc.amazonaws.com/efa on a nodeclaim if it requests it", func() {
			nodeClaim.Spec.Requirements = []corev1beta1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1.LabelInstanceTypeStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"dl1.24xlarge"},
					},
				},
			}
			nodeClaim.Spec.Resources.Requests = v1.ResourceList{v1beta1.ResourceEFA: resource.MustParse("1")}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
			cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
			Expect(err).To(BeNil())
			Expect(lo.Keys(cloudProviderNodeClaim.Status.Allocatable)).To(ContainElement(v1beta1.ResourceEFA))
		})
		It("shouldn't include vpc.amazonaws.com/efa on a nodeclaim if it doesn't request it", func() {
			nodeClaim.Spec.Requirements = []corev1beta1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1.LabelInstanceTypeStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"dl1.24xlarge"},
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
			cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
			Expect(err).To(BeNil())
			Expect(lo.Keys(cloudProviderNodeClaim.Status.Allocatable)).ToNot(ContainElement(v1beta1.ResourceEFA))
		})
	})
})
