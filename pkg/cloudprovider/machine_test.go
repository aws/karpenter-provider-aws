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
	"fmt"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	nodeclaimutil "github.com/aws/karpenter-core/pkg/utils/nodeclaim"
	nodepoolutil "github.com/aws/karpenter-core/pkg/utils/nodepool"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter/pkg/fake"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecloudproivder "github.com/aws/karpenter-core/pkg/cloudprovider"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
)

var _ = Describe("Machine/CloudProvider", func() {
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	var provisioner *v1alpha5.Provisioner
	var machine *v1alpha5.Machine
	var _ = BeforeEach(func() {
		nodeTemplate = &v1alpha1.AWSNodeTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name: coretest.RandomName(),
			},
			Spec: v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"*": "*"},
					SecurityGroupSelector: map[string]string{"*": "*"},
				},
			},
		}
		provisioner = test.Provisioner(coretest.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{{
				Key:      v1alpha1.LabelInstanceCategory,
				Operator: v1.NodeSelectorOpExists,
			}},
			ProviderRef: &v1alpha5.MachineTemplateRef{
				APIVersion: nodeTemplate.APIVersion,
				Kind:       nodeTemplate.Kind,
				Name:       nodeTemplate.Name,
			},
		})
		machine = coretest.Machine(v1alpha5.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
				},
			},
			Spec: v1alpha5.MachineSpec{
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})
	})
	It("should return an ICE error when there are no instance types to launch", func() {
		// Specify no instance types and expect to receive a capacity error
		machine.Spec.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelInstanceTypeStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate, machine)
		cloudProviderMachine, err := cloudProvider.Create(ctx, nodeclaimutil.New(machine))
		Expect(corecloudproivder.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(cloudProviderMachine).To(BeNil())
	})
	It("should return AWSNodetemplate Hash on the machine", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate, machine)
		cloudProviderMachine, err := cloudProvider.Create(ctx, nodeclaimutil.New(machine))
		Expect(err).To(BeNil())
		Expect(cloudProviderMachine).ToNot(BeNil())
		_, ok := cloudProviderMachine.ObjectMeta.Annotations[v1alpha1.AnnotationNodeTemplateHash]
		Expect(ok).To(BeTrue())
	})
	Context("Defaulting", func() {
		// Intent here is that if updates occur on the provisioningController, the Provisioner doesn't need to be recreated
		It("should not set the InstanceProfile with the default if none provided in Provisioner", func() {
			provisioner.SetDefaults(ctx)
			constraints, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.InstanceProfile).To(BeNil())
		})
		It("should default requirements", func() {
			provisioner.SetDefaults(ctx)
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.CapacityTypeOnDemand},
			}))
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			}))
		})
	})
	Context("EC2 Context", func() {
		It("should set context on the CreateFleet request if specified on the Provisioner", func() {
			provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
			Expect(err).ToNot(HaveOccurred())
			provider.Context = aws.String("context-1234")
			provider.SubnetSelector = map[string]string{"*": "*"}
			provider.SecurityGroupSelector = map[string]string{"*": "*"}
			provisioner = coretest.Provisioner(coretest.ProvisionerOptions{Provider: provider})
			provisioner.SetDefaults(ctx)
			ExpectApplied(ctx, env.Client, provisioner)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(aws.StringValue(createFleetInput.Context)).To(Equal("context-1234"))
		})
		It("should default to no EC2 Context", func() {
			provisioner.SetDefaults(ctx)
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.Context).To(BeNil())
		})
	})
	Context("Machine Drift", func() {
		var validAMI string
		var validSecurityGroup string
		var selectedInstanceType *corecloudproivder.InstanceType
		var instance *ec2.Instance
		var machine *v1alpha5.Machine
		var validSubnet1 string
		var validSubnet2 string
		BeforeEach(func() {
			validAMI = fake.ImageID()
			validSecurityGroup = fake.SecurityGroupID()
			validSubnet1 = fake.SubnetID()
			validSubnet2 = fake.SubnetID()
			awsEnv.SSMAPI.GetParameterOutput = &ssm.GetParameterOutput{
				Parameter: &ssm.Parameter{Value: aws.String(validAMI)},
			}
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String(validAMI),
						Architecture: aws.String("arm64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
				},
			})
			nodeTemplate.Status.SecurityGroups = []v1alpha1.SecurityGroup{
				{
					ID:   validSecurityGroup,
					Name: "test-securitygroup",
				},
			}
			nodeTemplate.Status.Subnets = []v1alpha1.Subnet{
				{
					ID:   validSubnet1,
					Zone: "zone-1",
				},
				{
					ID:   validSubnet2,
					Zone: "zone-2",
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodepoolutil.New(provisioner))
			Expect(err).ToNot(HaveOccurred())
			selectedInstanceType = instanceTypes[0]

			// Create the instance we want returned from the EC2 API
			instance = &ec2.Instance{
				ImageId:               aws.String(validAMI),
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
			nodeTemplateHash := nodeTemplate.Hash()
			nodeTemplate.Annotations = lo.Assign(nodeTemplate.Annotations, map[string]string{
				v1alpha1.AnnotationNodeTemplateHash: nodeTemplateHash,
			})
			machine = coretest.Machine(v1alpha5.Machine{
				Status: v1alpha5.MachineStatus{
					ProviderID: fake.ProviderID(lo.FromPtr(instance.InstanceId)),
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1.LabelInstanceTypeStable:       selectedInstanceType.Name,
					},
					Annotations: map[string]string{
						v1alpha1.AnnotationNodeTemplateHash: nodeTemplateHash,
					},
				},
			})
		})
		It("should not fail if node template does not exist", func() {
			ExpectDeleted(ctx, env.Client, nodeTemplate)
			drifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(drifted).To(BeEmpty())
		})
		It("should return false if providerRef is not defined", func() {
			provisioner.Spec.ProviderRef = nil
			ExpectApplied(ctx, env.Client, provisioner)
			drifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(drifted).To(BeEmpty())
		})
		It("should not fail if provisioner does not exist", func() {
			ExpectDeleted(ctx, env.Client, provisioner)
			drifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(drifted).To(BeEmpty())
		})
		It("should return drifted if the AMI is not valid", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.ImageId = aws.String(fake.ImageID())
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.AMIDrift))
		})
		It("should return node template drifted if there are multiple drift reasons", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.ImageId = aws.String(fake.ImageID())
			instance.SubnetId = aws.String(fake.SubnetID())
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			// Assign a fake hash
			nodeTemplate.Annotations = lo.Assign(nodeTemplate.Annotations, map[string]string{
				v1alpha1.AnnotationNodeTemplateHash: "abcdefghijkl",
			})
			ExpectApplied(ctx, env.Client, nodeTemplate)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.NodeTemplateDrift))
		})
		It("should return drifted if the subnet is not valid", func() {
			instance.SubnetId = aws.String(fake.SubnetID())
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SubnetDrift))
		})
		It("should return an error if AWSNodeTemplate subnets are empty", func() {
			nodeTemplate.Status.Subnets = []v1alpha1.Subnet{}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			_, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).To(HaveOccurred())
		})
		It("should not return drifted if the machine is valid", func() {
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should return an error if the AWSNodeTemplate securitygroup are empty", func() {
			nodeTemplate.Status.SecurityGroups = []v1alpha1.SecurityGroup{}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			_, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).To(HaveOccurred())
		})
		It("should return drifted if the instance securitygroup do not match the AWSNodeTemplateStatus", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should return drifted if there are more instance securitygroups are present than AWSNodeTemplate Status", func() {
			// Instance is a reference to what we return in the GetInstances call
			instance.SecurityGroups = []*ec2.GroupIdentifier{{GroupId: aws.String(fake.SecurityGroupID())}, {GroupId: aws.String(validSecurityGroup)}}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should return drifted if more AWSNodeTemplate securitygroups are present than instance securitygroups", func() {
			nodeTemplate.Status.SecurityGroups = []v1alpha1.SecurityGroup{
				{
					ID:   validSecurityGroup,
					Name: "test-securitygroup",
				},
				{
					ID:   fake.SecurityGroupID(),
					Name: "test-securitygroup",
				},
			}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(Equal(cloudprovider.SecurityGroupDrift))
		})
		It("should not return drifted if launchTemplateName is defined", func() {
			nodeTemplate.Spec.LaunchTemplateName = aws.String("validLaunchTemplateName")
			nodeTemplate.Spec.SecurityGroupSelector = nil
			nodeTemplate.Status.SecurityGroups = nil
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should not return drifted if the securitygroups match", func() {
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).ToNot(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should error if the machine doesn't have the instance-type label", func() {
			machine.Labels = map[string]string{
				v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
			}
			_, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).To(HaveOccurred())
		})
		It("should error drift if machine doesn't have provider id", func() {
			machine.Status = v1alpha5.MachineStatus{}
			isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).To(HaveOccurred())
			Expect(isDrifted).To(BeEmpty())
		})
		It("should error drift if the underlying machine does not exist", func() {
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
				Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{}}},
			})
			_, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
			Expect(err).To(HaveOccurred())
		})
		Context("Static Drift Detection", func() {
			BeforeEach(func() {
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Kind: nodeTemplate.Kind, Name: nodeTemplate.Name},
				})
				machine.ObjectMeta.Labels = lo.Assign(machine.ObjectMeta.Labels, map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
				})
			})
			DescribeTable("should return drifted if the AWSNodeTemplate.Spec is updated",
				func(awsnodetemplatespec v1alpha1.AWSNodeTemplateSpec) {
					ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
					isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())

					updatedAWSNodeTemplate := test.AWSNodeTemplate(*nodeTemplate.Spec.DeepCopy(), awsnodetemplatespec)
					updatedAWSNodeTemplate.ObjectMeta = nodeTemplate.ObjectMeta
					updatedAWSNodeTemplate.Status = nodeTemplate.Status
					updatedAWSNodeTemplate.Annotations = map[string]string{v1alpha1.AnnotationNodeTemplateHash: updatedAWSNodeTemplate.Hash()}

					ExpectApplied(ctx, env.Client, updatedAWSNodeTemplate)
					isDrifted, err = cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(Equal(cloudprovider.NodeTemplateDrift))
				},
				Entry("InstanceProfile Drift", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{InstanceProfile: aws.String("profile-2")}}),
				Entry("UserData Drift", v1alpha1.AWSNodeTemplateSpec{UserData: aws.String("userdata-test-2")}),
				Entry("Tags Drift", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
				Entry("MetadataOptions Drift", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{LaunchTemplate: v1alpha1.LaunchTemplate{MetadataOptions: &v1alpha1.MetadataOptions{HTTPEndpoint: aws.String("test-metadata-2")}}}}),
				Entry("BlockDeviceMappings Drift", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{LaunchTemplate: v1alpha1.LaunchTemplate{BlockDeviceMappings: []*v1alpha1.BlockDeviceMapping{{DeviceName: aws.String("map-device-test-3")}}}}}),
				Entry("Context Drift", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{Context: aws.String("context-2")}}),
				Entry("DetailedMonitoring Drift", v1alpha1.AWSNodeTemplateSpec{DetailedMonitoring: aws.Bool(true)}),
				Entry("AMIFamily Drift", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{AMIFamily: aws.String(v1alpha1.AMIFamilyBottlerocket)}}),
			)
			DescribeTable("should not return drifted if dynamic fields are updated",
				func(awsnodetemplatespec v1alpha1.AWSNodeTemplateSpec) {
					ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
					isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())

					updatedAWSNodeTemplate := test.AWSNodeTemplate(*nodeTemplate.Spec.DeepCopy(), awsnodetemplatespec)
					updatedAWSNodeTemplate.ObjectMeta = nodeTemplate.ObjectMeta
					updatedAWSNodeTemplate.Status = nodeTemplate.Status
					updatedAWSNodeTemplate.Annotations = map[string]string{v1alpha1.AnnotationNodeTemplateHash: updatedAWSNodeTemplate.Hash()}

					ExpectApplied(ctx, env.Client, updatedAWSNodeTemplate)
					isDrifted, err = cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
					Expect(err).NotTo(HaveOccurred())
					Expect(isDrifted).To(BeEmpty())
				},
				Entry("AMISelector Drift", v1alpha1.AWSNodeTemplateSpec{AMISelector: map[string]string{"aws::ids": validAMI}}),
				Entry("SubnetSelector Drift", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{SubnetSelector: map[string]string{"aws-ids": "subnet-test1"}}}),
				Entry("SecurityGroupSelector Drift", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{SecurityGroupSelector: map[string]string{"sg-key": "sg-value"}}}),
			)
			It("should not return drifted if karpenter.k8s.aws/nodetemplate-hash annotation is not present on the machine", func() {
				machine.Annotations = map[string]string{}
				nodeTemplate.Spec.Tags = map[string]string{
					"Test Key": "Test Value",
				}
				ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
				isDrifted, err := cloudProvider.IsDrifted(ctx, nodeclaimutil.New(machine))
				Expect(err).NotTo(HaveOccurred())
				Expect(isDrifted).To(BeEmpty())
			})
		})
	})
	Context("Provider Backwards Compatibility", func() {
		It("should launch a machine using provider defaults", func() {
			provisioner = test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"*": "*"},
					SecurityGroupSelector: map[string]string{"*": "*"},
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)

			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			launchSpecNames := lo.Map(createFleetInput.LaunchTemplateConfigs, func(req *ec2.FleetLaunchTemplateConfigRequest, _ int) string {
				return *req.LaunchTemplateSpecification.LaunchTemplateName
			})
			Expect(len(createFleetInput.LaunchTemplateConfigs)).To(BeNumerically("==", awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()))
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(launchSpecNames).To(ContainElement(*ltInput.LaunchTemplateName))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(Equal(aws.Bool(true)))
			})
			for _, ltSpec := range createFleetInput.LaunchTemplateConfigs {
				Expect(*ltSpec.LaunchTemplateSpecification.Version).To(Equal("$Latest"))
			}
		})
		It("should discover security groups by ID", func() {
			provisioner = test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"*": "*"},
					SecurityGroupSelector: map[string]string{"aws-ids": "sg-test1"},
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(aws.StringValueSlice(ltInput.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf("sg-test1"))
			})
		})
		It("should discover security groups by ID in the LT when no network interfaces are defined", func() {
			provisioner = test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"aws-ids": "subnet-test2"},
					SecurityGroupSelector: map[string]string{"aws-ids": "sg-test1"},
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(aws.StringValueSlice(ltInput.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf("sg-test1"))
			})
		})
		It("should discover subnets by ID", func() {
			provisioner = test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"aws-ids": "subnet-test1"},
					SecurityGroupSelector: map[string]string{"*": "*"},
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("subnet-test1"))
		})
		It("should use the instance profile on the Provisioner when specified", func() {
			provisioner = test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"*": "*"},
					SecurityGroupSelector: map[string]string{"*": "*"},
					InstanceProfile:       aws.String("overridden-profile"),
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(*ltInput.LaunchTemplateData.IamInstanceProfile.Name).To(Equal("overridden-profile"))
			})
		})
	})
	Context("Subnet Compatibility", func() {
		// Note when debugging these tests -
		// hard coded fixture data (ex. what the aws api will return) is maintained in fake/ec2api.go
		It("should default to the cluster's subnets", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := coretest.UnschedulablePod(
				coretest.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: v1alpha5.ArchitectureAmd64}})
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
							&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test1"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1a")},
							&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test2"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1b")},
							&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test3"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1c")},
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
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
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
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{MaxPods: aws.Int32(1)}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
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
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate, pod1)
			awsEnv.EC2API.CreateFleetBehavior.Error.Set(fmt.Errorf("CreateFleet synthetic error"))
			bindings := ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1)
			Expect(len(bindings)).To(Equal(0))
		})
		It("should launch instances into subnets that are excluded by another provisioner", func() {
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(10),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b"), AvailableIpAddressCount: aws.Int64(100),
					Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
			}})
			nodeTemplate.Spec.SubnetSelector = map[string]string{"Name": "test-subnet-1"}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			podSubnet1 := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, podSubnet1)
			ExpectScheduled(ctx, env.Client, podSubnet1)
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-1"))

			provisioner = test.Provisioner(coretest.ProvisionerOptions{Provider: &v1alpha1.AWS{
				SubnetSelector:        map[string]string{"Name": "test-subnet-2"},
				SecurityGroupSelector: map[string]string{"*": "*"},
			}})
			ExpectApplied(ctx, env.Client, provisioner)
			podSubnet2 := coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, podSubnet2)
			ExpectScheduled(ctx, env.Client, podSubnet2)
			createFleetInput = awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
		})
		It("should launch instances with an alternate provisioner when an awsnodetemplate selects 0 subnets, security groups, or amis", func() {
			misconfiguredNodeTemplate := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					// select nothing!
					SubnetSelector: map[string]string{"Name": "nothing"},
					// select nothing!
					SecurityGroupSelector: map[string]string{"Name": "nothing"},
				},
				// select nothing!
				AMISelector: map[string]string{"Name": "nothing"},
			})
			misconfiguredNodeTemplate.Name = "misconfigured"
			prov2 := test.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.MachineTemplateRef{
					APIVersion: misconfiguredNodeTemplate.APIVersion,
					Kind:       misconfiguredNodeTemplate.Kind,
					Name:       "misconfigured",
				},
			})
			ExpectApplied(ctx, env.Client, provisioner, prov2, nodeTemplate, misconfiguredNodeTemplate)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
	})
})
