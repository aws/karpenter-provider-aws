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
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	clock "k8s.io/utils/clock/testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/imdario/mergo"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"

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
	. "knative.dev/pkg/logging/testing"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
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
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.SubnetProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, env.KubernetesInterface.CoreV1(), recorder, cloudProvider, cluster)
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
		nodeClass = test.EC2NodeClass()
		nodePool = coretest.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
						Requirements: []v1.NodeSelectorRequirement{
							{Key: corev1beta1.CapacityTypeLabelKey, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.CapacityTypeOnDemand}},
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
	})
	It("should return an ICE error when there are no instance types to launch", func() {
		// Specify no instance types and expect to receive a capacity error
		nodeClaim.Spec.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelInstanceTypeStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-instance-type"},
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
	It("should return NodeClass Hash on the nodeClaim", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		_, ok := cloudProviderNodeClaim.ObjectMeta.Annotations[v1beta1.AnnotationEC2NodeClassHash]
		Expect(ok).To(BeTrue())
	})
	Context("EC2 Context", func() {
		It("should set context on the CreateFleet request if specified on the NodePool", func() {
			nodeClass.Spec.Context = aws.String("context-1234")
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(aws.StringValue(createFleetInput.Context)).To(Equal("context-1234"))
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
					},
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String(amdAMIID),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
				},
			})
			nodeClass.Status.SecurityGroups = []v1beta1.SecurityGroup{
				{
					ID:   validSecurityGroup,
					Name: "test-securitygroup",
				},
			}
			nodeClass.Status.Subnets = []v1beta1.Subnet{
				{
					ID:   validSubnet1,
					Zone: "zone-1",
				},
				{
					ID:   validSubnet2,
					Zone: "zone-2",
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
				v1beta1.AnnotationEC2NodeClassHash: nodeClass.Hash(),
			})
			nodeClaim.Status.ProviderID = fake.ProviderID(lo.FromPtr(instance.InstanceId))
			nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{v1beta1.AnnotationEC2NodeClassHash: nodeClass.Hash()})
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
		It("should return drifted if the instance security groups doesn't match the status", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should return drifted if there are more instance security groups present than in the status", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}, {GroupId: aws.String(validSecurityGroup)}}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should return drifted if more security groups are present than instance security groups", func() {
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
			ExpectApplied(ctx, env.Client, nodeClass)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeClaim)
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.AMIDrift))
		})
		Context("Static Drift Detection", func() {
			DescribeTable("should return drifted if the spec is updated",
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
					Expect(isDrifted).To(Equal(cloudprovider.NodeClassDrift))
				},
				Entry("UserData Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
				Entry("Tags Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
				Entry("MetadataOptions Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPEndpoint: aws.String("disabled")}}}),
				Entry("BlockDeviceMappings Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: aws.String("map-device-test-3")}}}}),
				Entry("Context Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
				Entry("DetailedMonitoring Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
				Entry("AMIFamily Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMIFamily: aws.String(v1beta1.AMIFamilyBottlerocket)}}),
			)
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
				Entry("AMI Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMISelectorTerms: []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}}}),
				Entry("Subnet Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{{ID: "subnet-test1"}}}}),
				Entry("SecurityGroup Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{{Tags: map[string]string{"sg-key": "sg-value"}}}}}),
			)
			It("should not return drifted if karpenter.k8s.aws/nodeclass-hash annotation is not present on the NodeClaim", func() {
				nodeClaim.Annotations = map[string]string{}
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
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(10),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(100),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
		})
		It("should launch instances into subnet with the most available IP addresses in-between cache refreshes", func() {
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(10),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(11),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{MaxPods: aws.Int32(1)}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
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
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(10),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b"), AvailableIpAddressCount: aws.Int64(100),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{{Tags: map[string]string{"Name": "test-subnet-1"}}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
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
			nodeClaim.Spec.Requirements = []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"dl1.24xlarge"},
				},
			}
			nodeClaim.Spec.Resources.Requests = v1.ResourceList{v1beta1.ResourceEFA: resource.MustParse("1")}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
			cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
			Expect(err).To(BeNil())
			Expect(lo.Keys(cloudProviderNodeClaim.Status.Allocatable)).To(ContainElement(v1beta1.ResourceEFA))
		})
		It("shouldn't include vpc.amazonaws.com/efa on a nodeclaim if it doesn't request it", func() {
			nodeClaim.Spec.Requirements = []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"dl1.24xlarge"},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
			cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
			Expect(err).To(BeNil())
			Expect(lo.Keys(cloudProviderNodeClaim.Status.Allocatable)).ToNot(ContainElement(v1beta1.ResourceEFA))
		})
	})
})
