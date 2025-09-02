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
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/awslabs/operatorpkg/object"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	opstatus "github.com/awslabs/operatorpkg/status"
	"github.com/imdario/mergo"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
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
	env = coretest.NewEnvironment(
		coretest.WithCRDs(test.DisableCapacityReservationIDValidation(test.RemoveNodeClassTagValidation(apis.CRDs))...),
		coretest.WithCRDs(v1alpha1.CRDs...),
		coretest.WithFieldIndexers(coretest.NodePoolNodeClassRefFieldIndexer(ctx)),
	)
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	fakeClock = clock.NewFakeClock(time.Now())
	recorder = events.NewRecorder(&record.FakeRecorder{})
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, recorder,
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, recorder, cloudProvider, cluster, fakeClock)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())

	cluster.Reset()
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("CloudProvider", func() {
	var nodeClass *v1.EC2NodeClass
	var nodePool *karpv1.NodePool
	var nodeClaim *karpv1.NodeClaim
	var _ = BeforeEach(func() {
		nodeClass = test.EC2NodeClass(
			v1.EC2NodeClass{
				Status: v1.EC2NodeClassStatus{
					InstanceProfile: "test-profile",
					SecurityGroups: []v1.SecurityGroup{
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
					Subnets: []v1.Subnet{
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
		nodePool = coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeOnDemand}}},
						},
					},
				},
			},
		})
		nodeClaim = coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{karpv1.NodePoolLabelKey: nodePool.Name},
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
				Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.CapacityTypeOnDemand},
						},
					},
				},
			},
		})
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass) // Hydrate the subnet cache
		Expect(err).To(BeNil())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())

		// Configure default AMIs so we discover AMIs with the correct requirements in the NodeClass status controller
		awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					Name:         aws.String("amd64-ami"),
					ImageId:      aws.String("amd64-ami-id"),
					CreationDate: aws.String(time.Time{}.Format(time.RFC3339)),
					Architecture: "x86_64",
					State:        ec2types.ImageStateAvailable,
				},
				{
					Name:         aws.String("arm64-ami"),
					ImageId:      aws.String("arm64-ami-id"),
					CreationDate: aws.String(time.Time{}.Add(time.Minute).Format(time.RFC3339)),
					Architecture: "arm64",
					State:        ec2types.ImageStateAvailable,
				},
				{
					Name:         aws.String("amd64-nvidia-ami"),
					ImageId:      aws.String("amd64-nvidia-ami-id"),
					CreationDate: aws.String(time.Time{}.Add(2 * time.Minute).Format(time.RFC3339)),
					Architecture: "x86_64",
					State:        ec2types.ImageStateAvailable,
				},
			},
		})
		version := awsEnv.VersionProvider.Get(ctx)
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", version): "amd64-ami-id",
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/nvidia/recommended/image_id", version):   "amd64-nvidia-ami-id",
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/arm64/standard/recommended/image_id", version):  "arm64-ami-id",
		}
	})
	It("should not proceed with instance creation if NodeClass is unknown", func() {
		nodeClass.StatusConditions().SetUnknown(opstatus.ConditionReady)
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		_, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(HaveOccurred())
		Expect(corecloudprovider.IsNodeClassNotReadyError(err)).To(BeFalse())
	})
	It("should return NodeClassNotReady error on creation if NodeClass is not ready", func() {
		nodeClass.StatusConditions().SetFalse(opstatus.ConditionReady, "NodeClassNotReady", "NodeClass not ready")
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		_, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(HaveOccurred())
		Expect(corecloudprovider.IsNodeClassNotReadyError(err)).To(BeTrue())
	})
	It("should return NodeClassNotReady error on creation if NodeClass tag validation fails", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		nodeClass.Spec.Tags = map[string]string{"kubernetes.io/cluster/thewrongcluster": "owned"}
		ExpectApplied(ctx, env.Client, nodeClass)
		_, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(HaveOccurred())
		Expect(corecloudprovider.IsNodeClassNotReadyError(err)).To(BeTrue())
	})
	It("should return NodeClassNotReady error when observed generation doesn't match", func() {
		nodeClass.Generation = 2
		nodeClass.StatusConditions().SetTrue(opstatus.ConditionReady)
		nodeClass.StatusConditions().Get(opstatus.ConditionReady).ObservedGeneration = 1
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		_, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(HaveOccurred())
		Expect(corecloudprovider.IsNodeClassNotReadyError(err)).To(BeTrue())
	})
	It("should return an ICE error when there are no instance types to launch", func() {
		// Specify no instance types and expect to receive a capacity error
		nodeClaim.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-instance-type"},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
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
		zone, ok := cloudProviderNodeClaim.GetLabels()[corev1.LabelTopologyZone]
		Expect(ok).To(BeTrue())
		zoneID, ok := cloudProviderNodeClaim.GetLabels()[v1.LabelTopologyZoneID]
		Expect(ok).To(BeTrue())
		subnet, ok := lo.Find(nodeClass.Status.Subnets, func(s v1.Subnet) bool {
			return s.Zone == zone
		})
		Expect(ok).To(BeTrue())
		Expect(zoneID).To(Equal(subnet.ZoneID))
	})
	It("should expect a strict set of annotation keys", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		Expect(len(lo.Keys(cloudProviderNodeClaim.Annotations))).To(BeNumerically("==", 3))
		Expect(lo.Keys(cloudProviderNodeClaim.Annotations)).To(ContainElements(v1.AnnotationEC2NodeClassHash, v1.AnnotationEC2NodeClassHashVersion, v1.AnnotationInstanceProfile))
		Expect(cloudProviderNodeClaim.Annotations[v1.AnnotationInstanceProfile]).To(Equal(nodeClass.Status.InstanceProfile))
	})
	It("should return NodeClass Hash on the nodeClaim", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		_, ok := cloudProviderNodeClaim.ObjectMeta.Annotations[v1.AnnotationEC2NodeClassHash]
		Expect(ok).To(BeTrue())
	})
	It("should return NodeClass Hash Version on the nodeClaim", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		v, ok := cloudProviderNodeClaim.ObjectMeta.Annotations[v1.AnnotationEC2NodeClassHashVersion]
		Expect(ok).To(BeTrue())
		Expect(v).To(Equal(v1.EC2NodeClassHashVersion))
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
			Expect(aws.ToString(createFleetInput.Context)).To(Equal(contextID))
		})
		It("should not set context on the CreateFleet request when min values are relaxed even if specified on the NodePool", func() {
			nodeClass.Spec.Context = aws.String(contextID)
			nodeClaimWithRelaxedMinValues := nodeClaim.DeepCopy()
			nodeClaimWithRelaxedMinValues.Annotations = lo.Assign(nodeClaimWithRelaxedMinValues.Annotations, map[string]string{karpv1.NodeClaimMinValuesRelaxedAnnotationKey: "true"})
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaimWithRelaxedMinValues)
			cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaimWithRelaxedMinValues)
			Expect(err).To(BeNil())
			Expect(cloudProviderNodeClaim).ToNot(BeNil())

			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(aws.ToString(createFleetInput.Context)).To(BeEmpty())
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
			instances[0].VCpuInfo = &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(1)}
			instances[1].VCpuInfo = &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(8)}
			awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{InstanceTypes: instances})
			awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: fake.MakeInstanceOfferings(instances)})
			now := time.Now()
			awsEnv.EC2API.DescribeSpotPriceHistoryBehavior.Output.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []ec2types.SpotPrice{
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
			instanceNames := lo.Map(instances, func(info ec2types.InstanceTypeInfo, _ int) string { return string(info.InstanceType) })

			// Define NodePool that has minValues on instance-type requirement.
			nodePool = coretest.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
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
										Key:      karpv1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{karpv1.CapacityTypeSpot},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelInstanceTypeStable,
										Operator: corev1.NodeSelectorOpIn,
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
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			pod2 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("0.9")},
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
					uniqueInstanceTypes.Insert(string(override.InstanceType))
				}
			}
			// This ensures that we have sent the minimum number of requirements defined in the NodePool.
			Expect(len(uniqueInstanceTypes)).To(BeNumerically(">=", 2))
		})
		It("CreateFleet input should respect minValues for Exists Operator in requirement from NodePool", func() {
			// Create fake InstanceTypes where one instances can fit 2 pods and another one can fit only 1 pod.
			instances := fake.MakeInstances()
			instances, _ = fake.MakeUniqueInstancesAndFamilies(instances, 2)
			instances[0].VCpuInfo = &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(1)}
			instances[1].VCpuInfo = &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(8)}
			awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{InstanceTypes: instances})
			awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: fake.MakeInstanceOfferings(instances)})
			now := time.Now()
			awsEnv.EC2API.DescribeSpotPriceHistoryBehavior.Output.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []ec2types.SpotPrice{
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
			instanceNames := lo.Map(instances, func(info ec2types.InstanceTypeInfo, _ int) string { return string(info.InstanceType) })

			// Define NodePool that has minValues on instance-type requirement.
			nodePool = coretest.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
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
										Key:      corev1.LabelInstanceTypeStable,
										Operator: corev1.NodeSelectorOpExists,
									},
									MinValues: lo.ToPtr(2),
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelInstanceTypeStable,
										Operator: corev1.NodeSelectorOpIn,
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
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			pod2 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("0.9")},
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
					uniqueInstanceTypes.Insert(string(override.InstanceType))
				}
			}
			// This ensures that we have sent the minimum number of requirements defined in the NodePool.
			Expect(len(uniqueInstanceTypes)).To(BeNumerically(">=", 2))
		})
		It("CreateFleet input should respect minValues from multiple keys in NodePool", func() {
			// Create fake InstanceTypes where 2 instances can fit 2 pods individually and one can fit only 1 pod.
			instances := fake.MakeInstances()
			uniqInstanceTypes, instanceFamilies := fake.MakeUniqueInstancesAndFamilies(instances, 3)
			uniqInstanceTypes[0].VCpuInfo = &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(1)}
			uniqInstanceTypes[1].VCpuInfo = &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(4)}
			uniqInstanceTypes[2].VCpuInfo = &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(8)}
			awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{InstanceTypes: uniqInstanceTypes})
			awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: fake.MakeInstanceOfferings(uniqInstanceTypes)})
			now := time.Now()
			awsEnv.EC2API.DescribeSpotPriceHistoryBehavior.Output.Set(&ec2.DescribeSpotPriceHistoryOutput{
				SpotPriceHistory: []ec2types.SpotPrice{
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
			instanceNames := lo.Map(uniqInstanceTypes, func(info ec2types.InstanceTypeInfo, _ int) string { return string(info.InstanceType) })

			// Define NodePool that has minValues in multiple requirements.
			nodePool = coretest.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
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
										Key:      corev1.LabelInstanceTypeStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   instanceNames,
									},
									// consider at least 2 unique instance types
									MinValues: lo.ToPtr(2),
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.LabelInstanceFamily,
										Operator: corev1.NodeSelectorOpIn,
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
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("0.9")},
					},
				})
			pod2 := coretest.UnschedulablePod(
				coretest.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("0.9")},
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
					uniqueInstanceTypes.Insert(string(override.InstanceType))
					uniqueInstanceFamilies.Insert(strings.Split(string(override.InstanceType), ".")[0])
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
		var selectedInstanceType *corecloudprovider.InstanceType
		var instance ec2types.Instance
		var validSubnet1 string
		var validSubnet2 string
		BeforeEach(func() {
			armAMIID, amdAMIID = fake.ImageID(), fake.ImageID()
			validSecurityGroup = fake.SecurityGroupID()
			validSubnet1 = fake.SubnetID()
			validSubnet2 = fake.SubnetID()
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String(armAMIID),
						Architecture: "arm64",
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
						Tags: []ec2types.Tag{
							{
								Key:   aws.String("ami-key-1"),
								Value: aws.String("ami-value-1"),
							},
						},
						State: ec2types.ImageStateAvailable,
					},
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String(amdAMIID),
						Architecture: "x86_64",
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
						Tags: []ec2types.Tag{
							{
								Key:   aws.String("ami-key-2"),
								Value: aws.String("ami-value-2"),
							},
						},
						State: ec2types.ImageStateAvailable,
					},
				},
			})
			awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []ec2types.SecurityGroup{
					{
						GroupId:   aws.String(validSecurityGroup),
						GroupName: aws.String("test-securitygroup"),
						Tags: []ec2types.Tag{
							{
								Key:   aws.String("sg-key"),
								Value: aws.String("sg-value"),
							},
						},
					},
				},
			})
			awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{
				Subnets: []ec2types.Subnet{
					{
						SubnetId:         aws.String(validSubnet1),
						AvailabilityZone: aws.String("zone-1"),
						Tags: []ec2types.Tag{
							{
								Key:   aws.String("sn-key-1"),
								Value: aws.String("sn-value-1"),
							},
						},
					},
					{
						SubnetId:         aws.String(validSubnet2),
						AvailabilityZone: aws.String("zone-2"),
						Tags: []ec2types.Tag{
							{
								Key:   aws.String("sn-key-2"),
								Value: aws.String("sn-value-2"),
							},
						},
					},
				},
			})
			nodeClass.Status = v1.EC2NodeClassStatus{
				InstanceProfile: "test-profile",
				Subnets: []v1.Subnet{
					{
						ID:   validSubnet1,
						Zone: "zone-1",
					},
					{
						ID:   validSubnet2,
						Zone: "zone-2",
					},
				},
				SecurityGroups: []v1.SecurityGroup{
					{
						ID: validSecurityGroup,
					},
				},
				AMIs: []v1.AMI{
					{
						ID: armAMIID,
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureArm64}},
						},
					},
					{
						ID: amdAMIID,
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
						},
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).ToNot(HaveOccurred())
			var ok bool
			selectedInstanceType, ok = lo.Find(instanceTypes, func(i *corecloudprovider.InstanceType) bool {
				return i.Requirements.Compatible(scheduling.NewLabelRequirements(map[string]string{
					corev1.LabelArchStable: karpv1.ArchitectureAmd64,
				})) == nil
			})
			Expect(ok).To(BeTrue())

			// Create the instance we want returned from the EC2 API
			instance = ec2types.Instance{
				ImageId:               aws.String(amdAMIID),
				InstanceType:          ec2types.InstanceType(selectedInstanceType.Name),
				SubnetId:              aws.String(validSubnet1),
				SpotInstanceRequestId: aws.String(coretest.RandomName()),
				State: &ec2types.InstanceState{
					Name: ec2types.InstanceStateNameRunning,
				},
				InstanceId: aws.String(fake.InstanceID()),
				Placement: &ec2types.Placement{
					AvailabilityZone: aws.String("test-zone-1a"),
				},
				SecurityGroups: []ec2types.GroupIdentifier{{GroupId: aws.String(validSecurityGroup)}},
			}
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{instance}}},
			})
			nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
				v1.AnnotationEC2NodeClassHash:        nodeClass.Hash(),
				v1.AnnotationEC2NodeClassHashVersion: v1.EC2NodeClassHashVersion,
			})
			nodeClaim.Status.ProviderID = fake.ProviderID(lo.FromPtr(instance.InstanceId))
			nodeClaim.Status.ImageID = amdAMIID
			nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{
				v1.AnnotationEC2NodeClassHash:        nodeClass.Hash(),
				v1.AnnotationEC2NodeClassHashVersion: v1.EC2NodeClassHashVersion,
			})
			nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{corev1.LabelInstanceTypeStable: selectedInstanceType.Name})
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
			nodeClaim.Status.ImageID = fake.ImageID()
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.AMIDrift))
		})
		It("should return drifted if there are multiple drift reasons", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.ImageId = aws.String(fake.ImageID())
			instance.SubnetId = aws.String(fake.SubnetID())
			instance.SecurityGroups = []ec2types.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{instance}}},
			})
			// Assign a fake hash
			nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
				v1.AnnotationEC2NodeClassHash: "abcdefghijkl",
			})
			ExpectApplied(ctx, env.Client, nodeClass)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.NodeClassDrift))
		})
		It("should return drifted if the subnet is not valid", func() {
			instance.SubnetId = aws.String(fake.SubnetID())
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{instance}}},
			})
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SubnetDrift))
		})
		It("should return an error if subnets are empty", func() {
			awsEnv.SubnetCache.Flush()
			nodeClass.Status.Subnets = []v1.Subnet{}
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
			nodeClass.Status.SecurityGroups = []v1.SecurityGroup{}
			ExpectApplied(ctx, env.Client, nodeClass)
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []ec2types.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			_, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
		})
		It("should return drifted if the instance security groups doesn't match the discovered values", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []ec2types.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{instance}}},
			})
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should return drifted if there are more instance security groups present than in the discovered values", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []ec2types.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}, {GroupId: aws.String(validSecurityGroup)}}
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{instance}}},
			})
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should return drifted if more security groups are present than instance security groups then discovered from nodeclass", func() {
			nodeClass.Status.SecurityGroups = []v1.SecurityGroup{
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
		It("should dynamically drift nodeclaims for capacity reservations", func() {
			nodeClass.Status.CapacityReservations = []v1.CapacityReservation{
				{
					AvailabilityZone:      "test-zone-1a",
					ID:                    "cr-foo",
					InstanceMatchCriteria: string(ec2types.InstanceMatchCriteriaTargeted),
					InstanceType:          "m5.large",
					OwnerID:               "012345678901",
					State:                 v1.CapacityReservationStateActive,
					ReservationType:       v1.CapacityReservationTypeDefault,
				},
			}
			setReservationID := func(id string) {
				out := awsEnv.EC2API.DescribeInstancesBehavior.Output.Clone()
				out.Reservations[0].Instances[0].SpotInstanceRequestId = nil
				out.Reservations[0].Instances[0].CapacityReservationId = lo.ToPtr(id)
				out.Reservations[0].Instances[0].CapacityReservationSpecification = &ec2types.CapacityReservationSpecificationResponse{
					CapacityReservationPreference: ec2types.CapacityReservationPreferenceCapacityReservationsOnly,
				}
				awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(out)
			}
			setReservationID("cr-foo")
			ExpectApplied(ctx, env.Client, nodeClass)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(corecloudprovider.DriftReason("")))
			setReservationID("cr-bar")
			awsEnv.InstanceCache.Flush()
			isDrifted, err = cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.CapacityReservationDrift))
		})
		It("should not return drifted if the security groups match", func() {
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should error if the NodeClaim doesn't have the instance-type label", func() {
			delete(nodeClaim.Labels, corev1.LabelInstanceTypeStable)
			_, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
		})
		It("should error if the NodeClaim doesn't have ImageID", func() {
			nodeClaim.Status.ImageID = ""
			_, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
		})
		It("should error drift if NodeClaim doesn't have provider id", func() {
			nodeClaim.Status = karpv1.NodeClaimStatus{}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should error if the underlying NodeClaim doesn't exist", func() {
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{}}},
			})
			_, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).To(HaveOccurred())
		})
		It("should return drifted if the AMI no longer matches the existing NodeClaims instance type", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amdAMIID}}
			nodeClass.Status.AMIs = []v1.AMI{
				{
					ID: amdAMIID,
					Requirements: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
					},
				},
			}
			nodeClaim.Status.ImageID = armAMIID
			ExpectApplied(ctx, env.Client, nodeClass)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.AMIDrift))
		})
		Context("Static Drift Detection", func() {
			BeforeEach(func() {
				armRequirements := []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureArm64}},
				}
				amdRequirements := []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
				}
				nodeClass = &v1.EC2NodeClass{
					ObjectMeta: nodeClass.ObjectMeta,
					Spec: v1.EC2NodeClassSpec{
						SubnetSelectorTerms:        nodeClass.Spec.SubnetSelectorTerms,
						SecurityGroupSelectorTerms: nodeClass.Spec.SecurityGroupSelectorTerms,
						Role:                       nodeClass.Spec.Role,
						UserData:                   lo.ToPtr("Fake Userdata"),
						Tags: map[string]string{
							"fakeKey": "fakeValue",
						},
						Context:            lo.ToPtr("fake-context"),
						DetailedMonitoring: lo.ToPtr(false),
						AMISelectorTerms: []v1.AMISelectorTerm{{
							Alias: "al2023@latest",
						}},
						AssociatePublicIPAddress: lo.ToPtr(false),
						MetadataOptions: &v1.MetadataOptions{
							HTTPEndpoint:            lo.ToPtr("disabled"),
							HTTPProtocolIPv6:        lo.ToPtr("disabled"),
							HTTPPutResponseHopLimit: lo.ToPtr(int64(1)),
							HTTPTokens:              lo.ToPtr("optional"),
						},
						BlockDeviceMappings: []*v1.BlockDeviceMapping{
							{
								DeviceName: lo.ToPtr("fakeName"),
								RootVolume: false,
								EBS: &v1.BlockDevice{
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
					Status: v1.EC2NodeClassStatus{
						InstanceProfile: "test-profile",
						Subnets: []v1.Subnet{
							{
								ID:   validSubnet1,
								Zone: "zone-1",
							},
							{
								ID:   validSubnet2,
								Zone: "zone-2",
							},
						},
						SecurityGroups: []v1.SecurityGroup{
							{
								ID: validSecurityGroup,
							},
						},
						AMIs: []v1.AMI{
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
				nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1.AnnotationEC2NodeClassHash: nodeClass.Hash()})
				nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{v1.AnnotationEC2NodeClassHash: nodeClass.Hash()})
			})
			DescribeTable("should return drifted if a statically drifted EC2NodeClass.Spec field is updated",
				func(changes v1.EC2NodeClass) {
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())

					Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride, mergo.WithSliceDeepCopy)).To(Succeed())
					nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1.AnnotationEC2NodeClassHash: nodeClass.Hash()})

					ExpectApplied(ctx, env.Client, nodeClass)
					isDrifted, err = cloudProvider.IsDrifted(ctx, nodeClaim)
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(Equal(cloudprovider.NodeClassDrift))
				},
				Entry("UserData", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{UserData: lo.ToPtr("userdata-test-2")}}),
				Entry("Tags", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
				Entry("Context", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Context: lo.ToPtr("context-2")}}),
				Entry("DetailedMonitoring", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
				Entry("InstanceStorePolicy", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{InstanceStorePolicy: lo.ToPtr(v1.InstanceStorePolicyRAID0)}}),
				Entry("AssociatePublicIPAddress", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{AssociatePublicIPAddress: lo.ToPtr(true)}}),
				Entry("MetadataOptions HTTPEndpoint", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPEndpoint: lo.ToPtr("enabled")}}}),
				Entry("MetadataOptions HTTPProtocolIPv6", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPProtocolIPv6: lo.ToPtr("enabled")}}}),
				Entry("MetadataOptions HTTPPutResponseHopLimit", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPPutResponseHopLimit: lo.ToPtr(int64(10))}}}),
				Entry("MetadataOptions HTTPTokens", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPTokens: lo.ToPtr("required")}}}),
				Entry("BlockDeviceMapping DeviceName", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{DeviceName: lo.ToPtr("map-device-test-3")}}}}),
				Entry("BlockDeviceMapping RootVolume", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{RootVolume: true}}}}),
				Entry("BlockDeviceMapping DeleteOnTermination", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{DeleteOnTermination: lo.ToPtr(true)}}}}}),
				Entry("BlockDeviceMapping Encrypted", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{Encrypted: lo.ToPtr(true)}}}}}),
				Entry("BlockDeviceMapping IOPS", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{IOPS: lo.ToPtr(int64(10))}}}}}),
				Entry("BlockDeviceMapping KMSKeyID", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{KMSKeyID: lo.ToPtr("test")}}}}}),
				Entry("BlockDeviceMapping SnapshotID", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{SnapshotID: lo.ToPtr("test")}}}}}),
				Entry("BlockDeviceMapping Throughput", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{Throughput: lo.ToPtr(int64(10))}}}}}),
				Entry("BlockDeviceMapping VolumeType", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{VolumeType: lo.ToPtr("io1")}}}}}),
			)
			// We create a separate test for updating blockDeviceMapping volumeSize, since resource.Quantity is a struct, and mergo.WithSliceDeepCopy
			// doesn't work well with unexported fields, like the ones that are present in resource.Quantity
			It("should return drifted when updating blockDeviceMapping volumeSize", func() {
				nodeClass.Spec.BlockDeviceMappings[0].EBS.VolumeSize = resource.NewScaledQuantity(10, resource.Giga)
				nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1.AnnotationEC2NodeClassHash: nodeClass.Hash()})

				ExpectApplied(ctx, env.Client, nodeClass)
				isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDrifted).To(Equal(cloudprovider.NodeClassDrift))
			})
			DescribeTable("should not return drifted if dynamic fields are updated",
				func(changes v1.EC2NodeClass) {
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())

					Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride))
					nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1.AnnotationEC2NodeClassHash: nodeClass.Hash()})

					ExpectApplied(ctx, env.Client, nodeClass)
					isDrifted, err = cloudProvider.IsDrifted(ctx, nodeClaim)
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())
				},
				Entry("AMI Drift", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{
					AMIFamily:        lo.ToPtr(v1.AMIFamilyAL2023),
					AMISelectorTerms: []v1.AMISelectorTerm{{Tags: map[string]string{"ami-key-1": "ami-value-1"}}},
				}}),
				Entry("Subnet Drift", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{SubnetSelectorTerms: []v1.SubnetSelectorTerm{{Tags: map[string]string{"sn-key-1": "sn-value-1"}}}}}),
				Entry("SecurityGroup Drift", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"sg-key": "sg-value"}}}}}),
			)
			It("should not return drifted if karpenter.k8s.aws/ec2nodeclass-hash annotation is not present on the NodeClaim", func() {
				nodeClaim.Annotations = map[string]string{
					v1.AnnotationEC2NodeClassHashVersion: v1.EC2NodeClassHashVersion,
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
					v1.AnnotationEC2NodeClassHash:        "test-hash-111111",
					v1.AnnotationEC2NodeClassHashVersion: "test-hash-version-1",
				}
				nodeClaim.ObjectMeta.Annotations = map[string]string{
					v1.AnnotationEC2NodeClassHash:        "test-hash-222222",
					v1.AnnotationEC2NodeClassHashVersion: "test-hash-version-2",
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDrifted).To(BeEmpty())
			})
			It("should not return drifted if karpenter.k8s.aws/ec2nodeclass-hash-version annotation is not present on the NodeClass", func() {
				nodeClass.ObjectMeta.Annotations = map[string]string{
					v1.AnnotationEC2NodeClassHash: "test-hash-111111",
				}
				nodeClaim.ObjectMeta.Annotations = map[string]string{
					v1.AnnotationEC2NodeClassHash:        "test-hash-222222",
					v1.AnnotationEC2NodeClassHashVersion: "test-hash-version-2",
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
					v1.AnnotationEC2NodeClassHash:        "test-hash-111111",
					v1.AnnotationEC2NodeClassHashVersion: "test-hash-version-1",
				}
				nodeClaim.ObjectMeta.Annotations = map[string]string{
					v1.AnnotationEC2NodeClassHash: "test-hash-222222",
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
				coretest.PodOptions{NodeSelector: map[string]string{corev1.LabelArchStable: karpv1.ArchitectureAmd64}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			input := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(len(input.LaunchTemplateConfigs)).To(BeNumerically(">=", 1))

			foundNonGPULT := false
			for _, v := range input.LaunchTemplateConfigs {
				for _, ov := range v.Overrides {
					if ov.InstanceType == "m5.large" {
						foundNonGPULT = true
						Expect(v.Overrides).To(ContainElements(
							ec2types.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test1"), ImageId: ov.ImageId, InstanceType: "m5.large", AvailabilityZone: aws.String("test-zone-1a")},
							ec2types.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test2"), ImageId: ov.ImageId, InstanceType: "m5.large", AvailabilityZone: aws.String("test-zone-1b")},
							ec2types.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test3"), ImageId: ov.ImageId, InstanceType: "m5.large", AvailabilityZone: aws.String("test-zone-1c")},
						))
					}
				}
			}
			Expect(foundNonGPULT).To(BeTrue())
		})
		It("should launch instances into subnet with the most available IP addresses", func() {
			awsEnv.SubnetCache.Flush()
			awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(100),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			controller := nodeclass.NewController(awsEnv.Clock, env.Client, cloudProvider, recorder, fake.DefaultRegion, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider, awsEnv.InstanceTypesProvider, awsEnv.LaunchTemplateProvider, awsEnv.CapacityReservationProvider, awsEnv.EC2API, awsEnv.ValidationCache, awsEnv.RecreationCache, awsEnv.AMIResolver, options.FromContext(ctx).DisableDryRun)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1a"}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
		})
		It("should launch instances into subnet with the most available IP addresses in-between cache refreshes", func() {
			awsEnv.SubnetCache.Flush()
			awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(11),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			controller := nodeclass.NewController(awsEnv.Clock, env.Client, cloudProvider, recorder, fake.DefaultRegion, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider, awsEnv.InstanceTypesProvider, awsEnv.LaunchTemplateProvider, awsEnv.CapacityReservationProvider, awsEnv.EC2API, awsEnv.ValidationCache, awsEnv.RecreationCache, awsEnv.AMIResolver, options.FromContext(ctx).DisableDryRun)
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				MaxPods: aws.Int32(1),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			pod1 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1a"}})
			pod2 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1a"}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			ExpectScheduled(ctx, env.Client, pod1)
			ExpectScheduled(ctx, env.Client, pod2)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
			// Provision for another pod that should now use the other subnet since we've consumed some from the first launch.
			pod3 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1a"}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod3)
			ExpectScheduled(ctx, env.Client, pod3)
			createFleetInput = awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-1"))
		})
		It("should update in-flight IPs when a CreateFleet error occurs", func() {
			awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int32(10),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
			}})
			pod1 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1a"}})
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, pod1)
			awsEnv.EC2API.CreateFleetBehavior.Error.Set(fmt.Errorf("CreateFleet synthetic error"))
			bindings := ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1)
			Expect(len(bindings)).To(Equal(0))
		})
		It("should launch instances into subnets that are excluded by another NodePool", func() {
			awsEnv.EC2API.Subnets.Store("test-zone-1a", ec2types.Subnet{
				SubnetId:                aws.String("test-subnet-1"),
				AvailabilityZone:        aws.String("test-zone-1a"),
				AvailabilityZoneId:      aws.String("tstz1-1a"),
				AvailableIpAddressCount: aws.Int32(10),
				Tags:                    []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}},
			})
			awsEnv.EC2API.Subnets.Store("test-zone-1b", ec2types.Subnet{
				SubnetId:                aws.String("test-subnet-2"),
				AvailabilityZone:        aws.String("test-zone-1b"),
				AvailabilityZoneId:      aws.String("tstz1-1a"),
				AvailableIpAddressCount: aws.Int32(100),
				Tags:                    []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}},
			})
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{Tags: map[string]string{"Name": "test-subnet-1"}}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			controller := nodeclass.NewController(awsEnv.Clock, env.Client, cloudProvider, recorder, fake.DefaultRegion, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider, awsEnv.InstanceTypesProvider, awsEnv.LaunchTemplateProvider, awsEnv.CapacityReservationProvider, awsEnv.EC2API, awsEnv.ValidationCache, awsEnv.RecreationCache, awsEnv.AMIResolver, options.FromContext(ctx).DisableDryRun)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			podSubnet1 := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, podSubnet1)
			ExpectScheduled(ctx, env.Client, podSubnet1)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-1"))

			nodeClass2 := test.EC2NodeClass(v1.EC2NodeClass{
				Spec: v1.EC2NodeClassSpec{
					SubnetSelectorTerms: []v1.SubnetSelectorTerm{
						{
							Tags: map[string]string{"Name": "test-subnet-2"},
						},
					},
					SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
						{
							Tags: map[string]string{"*": "*"},
						},
					},
				},
				Status: v1.EC2NodeClassStatus{
					AMIs: nodeClass.Status.AMIs,
					SecurityGroups: []v1.SecurityGroup{
						{
							ID: "sg-test1",
						},
					},
				},
			})
			nodePool2 := coretest.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
					Template: karpv1.NodeClaimTemplate{
						Spec: karpv1.NodeClaimTemplateSpec{
							NodeClassRef: &karpv1.NodeClassReference{
								Group: object.GVK(nodeClass2).Group,
								Kind:  object.GVK(nodeClass2).Kind,
								Name:  nodeClass2.Name,
							},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool2, nodeClass2)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass2)
			podSubnet2 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{karpv1.NodePoolLabelKey: nodePool2.Name}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, podSubnet2)
			ExpectScheduled(ctx, env.Client, podSubnet2)
			createFleetInput = awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
		})
		It("should launch instances with an alternate NodePool when a NodeClass selects 0 subnets, security groups, or amis", func() {
			misconfiguredNodeClass := test.EC2NodeClass(v1.EC2NodeClass{
				Spec: v1.EC2NodeClassSpec{
					// select nothing!
					SubnetSelectorTerms: []v1.SubnetSelectorTerm{
						{
							Tags: map[string]string{"Name": "nothing"},
						},
					},
					// select nothing!
					SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
						{
							Tags: map[string]string{"Name": "nothing"},
						},
					},
					AMIFamily: lo.ToPtr(v1.AMIFamilyCustom),
					// select nothing!
					AMISelectorTerms: []v1.AMISelectorTerm{
						{
							Tags: map[string]string{"Name": "nothing"},
						},
					},
				},
			})
			nodePool2 := coretest.NodePool(karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
					Template: karpv1.NodeClaimTemplate{
						Spec: karpv1.NodeClaimTemplateSpec{
							NodeClassRef: &karpv1.NodeClassReference{
								Group: object.GVK(misconfiguredNodeClass).Group,
								Kind:  object.GVK(misconfiguredNodeClass).Kind,
								Name:  misconfiguredNodeClass.Name,
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
			nodeClaim.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"dl1.24xlarge"},
					},
				},
			}
			nodeClaim.Spec.Resources.Requests = corev1.ResourceList{v1.ResourceEFA: resource.MustParse("1")}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
			cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
			Expect(err).To(BeNil())
			Expect(lo.Keys(cloudProviderNodeClaim.Status.Allocatable)).To(ContainElement(v1.ResourceEFA))
		})
		It("shouldn't include vpc.amazonaws.com/efa on a nodeclaim if it doesn't request it", func() {
			nodeClaim.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"dl1.24xlarge"},
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
			cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
			Expect(err).To(BeNil())
			Expect(lo.Keys(cloudProviderNodeClaim.Status.Allocatable)).ToNot(ContainElement(v1.ResourceEFA))
		})
	})
	Context("Capacity Reservations", func() {
		const reservationCapacity = 10
		var crs []ec2types.CapacityReservation
		BeforeEach(func() {
			crs = lo.Map(v1.CapacityReservationType("").Values(), func(crt v1.CapacityReservationType, _ int) ec2types.CapacityReservation {
				return ec2types.CapacityReservation{
					AvailabilityZone:       lo.ToPtr("test-zone-1a"),
					InstanceType:           lo.ToPtr("m5.large"),
					OwnerId:                lo.ToPtr("012345678901"),
					InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
					CapacityReservationId:  lo.ToPtr(fmt.Sprintf("cr-m5.large-1a-%s", string(crt))),
					AvailableInstanceCount: lo.ToPtr[int32](reservationCapacity),
					State:                  ec2types.CapacityReservationStateActive,
					ReservationType:        ec2types.CapacityReservationType(crt),
				}
			})
			for _, cr := range crs {
				awsEnv.CapacityReservationProvider.SetAvailableInstanceCount(*cr.CapacityReservationId, 10)
			}
			awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(&ec2.DescribeCapacityReservationsOutput{
				CapacityReservations: crs,
			})
			nodeClass.Status.CapacityReservations = lo.Map(crs, func(cr ec2types.CapacityReservation, _ int) v1.CapacityReservation {
				return lo.Must(v1.CapacityReservationFromEC2(awsEnv.Clock, &cr))
			})
			nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{{NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      karpv1.CapacityTypeLabelKey,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{karpv1.CapacityTypeReserved},
			}}}
		})
		It("should mark capacity reservations as launched", func() {
			pod := coretest.UnschedulablePod()
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, pod)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ncs := ExpectNodeClaims(ctx, env.Client)
			Expect(ncs).To(HaveLen(1))
			Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount(ncs[0].Labels[corecloudprovider.ReservationIDLabel])).To(Equal(9))
		})
		It("should mark capacity reservations as terminated", func() {
			pod := coretest.UnschedulablePod()
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, pod)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ncs := ExpectNodeClaims(ctx, env.Client)
			Expect(ncs).To(HaveLen(1))

			// Attempt the first delete - since the instance still exists we shouldn't increment the availability count
			err := cloudProvider.Delete(ctx, ncs[0])
			Expect(corecloudprovider.IsNodeClaimNotFoundError(err)).To(BeFalse())
			Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount(ncs[0].Labels[corecloudprovider.ReservationIDLabel])).To(Equal(9))

			// Attempt again after clearing the instance from the EC2 output. Now that we get a NotFound error, expect
			// availability to be incremented.
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{})
			err = cloudProvider.Delete(ctx, ncs[0])
			Expect(corecloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
			Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount(ncs[0].Labels[corecloudprovider.ReservationIDLabel])).To(Equal(10))
		})
		DescribeTable(
			"should include capacity reservation labels",
			func(crt v1.CapacityReservationType) {
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{
						v1.LabelCapacityReservationType: string(crt),
					},
				})
				ExpectApplied(ctx, env.Client, nodePool, nodeClass, pod)
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				ncs := ExpectNodeClaims(ctx, env.Client)
				Expect(ncs).To(HaveLen(1))
				Expect(ncs[0].Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeReserved))
				Expect(ncs[0].Labels).To(HaveKeyWithValue(corecloudprovider.ReservationIDLabel, *lo.Must(lo.Find(crs, func(cr ec2types.CapacityReservation) bool {
					return string(cr.ReservationType) == string(crt)

				})).CapacityReservationId))
				Expect(ncs[0].Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationType, string(crt)))
			},
			lo.Map(v1.CapacityReservationType("").Values(), func(crt v1.CapacityReservationType, _ int) TableEntry {
				return Entry(fmt.Sprintf("when the capacity reservation type is %q", string(crt)), crt)
			}),
		)
	})
})
