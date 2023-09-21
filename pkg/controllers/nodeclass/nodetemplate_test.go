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

package nodeclass_test

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/imdario/mergo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/test"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

var _ = Describe("NodeTemplateController", func() {
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	BeforeEach(func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SubnetSelector:        map[string]string{"*": "*"},
				SecurityGroupSelector: map[string]string{"*": "*"},
			},
			AMISelector: map[string]string{"*": "*"},
		})
	})
	Context("Subnet Status", func() {
		It("Should update AWSNodeTemplate status for Subnets", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
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
			}))
		})
		It("Should have the correct ordering for the Subnets", func() {
			awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("subnet-test1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(20)},
				{SubnetId: aws.String("subnet-test2"), AvailabilityZone: aws.String("test-zone-1b"), AvailableIpAddressCount: aws.Int64(100)},
				{SubnetId: aws.String("subnet-test3"), AvailabilityZone: aws.String("test-zone-1c"), AvailableIpAddressCount: aws.Int64(50)},
			}})
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
				{
					ID:   "subnet-test2",
					Zone: "test-zone-1b",
				},
				{
					ID:   "subnet-test3",
					Zone: "test-zone-1c",
				},
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
			}))
		})
		It("Should resolve a valid selectors for Subnet by tags", func() {
			nodeTemplate.Spec.SubnetSelector = map[string]string{`Name`: `test-subnet-1,test-subnet-2`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
				{
					ID:   "subnet-test2",
					Zone: "test-zone-1b",
				},
			}))
		})
		It("Should resolve a valid selectors for Subnet by ids", func() {
			nodeTemplate.Spec.SubnetSelector = map[string]string{`aws-ids`: `subnet-test1`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
			}))
		})
		It("Should update Subnet status when the Subnet selector gets updated by tags", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
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
			}))

			nodeTemplate.Spec.SubnetSelector = map[string]string{`Name`: `test-subnet-1,test-subnet-2`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
				{
					ID:   "subnet-test2",
					Zone: "test-zone-1b",
				},
			}))
		})
		It("Should update Subnet status when the Subnet selector gets updated by ids", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
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
			}))

			nodeTemplate.Spec.SubnetSelector = map[string]string{`aws-ids`: `subnet-test1`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
			}))
		})
		It("Should not resolve a invalid selectors for Subnet", func() {
			nodeTemplate.Spec.SubnetSelector = map[string]string{`foo`: `invalid`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileFailed(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(BeNil())
		})
		It("Should not resolve a invalid selectors for an updated Subnet selectors", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(Equal([]v1alpha1.Subnet{
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
			}))

			nodeTemplate.Spec.SubnetSelector = map[string]string{`foo`: `invalid`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileFailed(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(BeNil())
		})
	})
	Context("Security Groups Status", func() {
		It("Should expect no errors when security groups are not in the AWSNodeTemplate", func() {
			// TODO: Remove test for v1beta1, as security groups will be required
			nodeTemplate.Spec.SecurityGroupSelector = nil
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			Expect(nodeTemplate.Status.SecurityGroups).To(BeNil())
		})
		It("Should update AWSNodeTemplate status for Security Groups", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal([]v1alpha1.SecurityGroup{
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
			}))
		})
		It("Should resolve a valid selectors for Security Groups by tags", func() {
			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`Name`: `test-security-group-1,test-security-group-2`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal([]v1alpha1.SecurityGroup{
				{
					ID:   "sg-test1",
					Name: "securityGroup-test1",
				},
				{
					ID:   "sg-test2",
					Name: "securityGroup-test2",
				},
			}))
		})
		It("Should resolve a valid selectors for Security Groups by ids", func() {
			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`aws-ids`: `sg-test1`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal([]v1alpha1.SecurityGroup{
				{
					ID:   "sg-test1",
					Name: "securityGroup-test1",
				},
			}))
		})
		It("Should update Security Groups status when the Security Groups selector gets updated by tags", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal([]v1alpha1.SecurityGroup{
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
			}))

			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`Name`: `test-security-group-1,test-security-group-2`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal([]v1alpha1.SecurityGroup{
				{
					ID:   "sg-test1",
					Name: "securityGroup-test1",
				},
				{
					ID:   "sg-test2",
					Name: "securityGroup-test2",
				},
			}))
		})
		It("Should update Security Groups status when the Security Groups selector gets updated by ids", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal([]v1alpha1.SecurityGroup{
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
			}))

			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`aws-ids`: `sg-test1`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal([]v1alpha1.SecurityGroup{
				{
					ID:   "sg-test1",
					Name: "securityGroup-test1",
				},
			}))
		})
		It("Should not resolve a invalid selectors for Security Groups", func() {
			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`foo`: `invalid`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileFailed(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(BeNil())
		})
		It("Should not resolve a invalid selectors for an updated Security Groups selector", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal([]v1alpha1.SecurityGroup{
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
			}))

			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`foo`: `invalid`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileFailed(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(BeNil())
		})
	})
	Context("AMI Status", func() {
		BeforeEach(func() {
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					{
						Name:         aws.String("test-ami-1"),
						ImageId:      aws.String("ami-test1"),
						CreationDate: aws.String(time.Now().Format(time.RFC3339)),
						Architecture: aws.String("x86_64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-1")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
					},
					{
						Name:         aws.String("test-ami-2"),
						ImageId:      aws.String("ami-test2"),
						CreationDate: aws.String(time.Now().Add(time.Minute).Format(time.RFC3339)),
						Architecture: aws.String("x86_64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-2")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
					},
					{
						Name:         aws.String("test-ami-3"),
						ImageId:      aws.String("ami-test3"),
						CreationDate: aws.String(time.Now().Add(2 * time.Minute).Format(time.RFC3339)),
						Architecture: aws.String("x86_64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-3")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
					},
				},
			})
		})
		It("should resolve amiSelector AMIs and requirements into status", func() {
			version := lo.Must(awsEnv.VersionProvider.Get(ctx))

			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):                                                   "ami-id-123",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version):                                               "ami-id-456",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2%s/recommended/image_id", version, fmt.Sprintf("-%s", v1alpha5.ArchitectureArm64)): "ami-id-789",
			}

			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					{
						Name:         aws.String("test-ami-1"),
						ImageId:      aws.String("ami-id-123"),
						CreationDate: aws.String(time.Now().Format(time.RFC3339)),
						Architecture: aws.String("x86_64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-1")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
					},
					{
						Name:         aws.String("test-ami-2"),
						ImageId:      aws.String("ami-id-456"),
						CreationDate: aws.String(time.Now().Add(time.Minute).Format(time.RFC3339)),
						Architecture: aws.String("x86_64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-2")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
					},
					{
						Name:         aws.String("test-ami-3"),
						ImageId:      aws.String("ami-id-789"),
						CreationDate: aws.String(time.Now().Add(2 * time.Minute).Format(time.RFC3339)),
						Architecture: aws.String("x86_64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-3")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
					},
				},
			})
			nodeTemplate.Spec.AMISelector = nil
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.AMIs).To(Equal([]v1alpha1.AMI{
				{
					Name: "test-ami-3",
					ID:   "ami-id-789",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.ArchitectureArm64},
						},
						{
							Key:      v1alpha1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
						{
							Key:      v1alpha1.LabelInstanceAcceleratorCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
					},
				},
				{
					Name: "test-ami-2",
					ID:   "ami-id-456",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.ArchitectureAmd64},
						},
						{
							Key:      v1alpha1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpExists,
						},
					},
				},
				{
					Name: "test-ami-2",
					ID:   "ami-id-456",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.ArchitectureAmd64},
						},
						{
							Key:      v1alpha1.LabelInstanceAcceleratorCount,
							Operator: v1.NodeSelectorOpExists,
						},
					},
				},
				{
					Name: "test-ami-1",
					ID:   "ami-id-123",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.ArchitectureAmd64},
						},
						{
							Key:      v1alpha1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
						{
							Key:      v1alpha1.LabelInstanceAcceleratorCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
					},
				},
			}))
		})
		It("should resolve amiSelector AMis and requirements into status when all SSM aliases don't resolve", func() {
			version := lo.Must(awsEnv.VersionProvider.Get(ctx))
			// This parameter set doesn't include any of the Nvidia AMIs
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", version): "ami-id-123",
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", version):  "ami-id-456",
			}
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
			nodeTemplate.Spec.AMISelector = nil
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					{
						Name:         aws.String("test-ami-1"),
						ImageId:      aws.String("ami-id-123"),
						CreationDate: aws.String(time.Now().Format(time.RFC3339)),
						Architecture: aws.String("x86_64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-1")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
					},
					{
						Name:         aws.String("test-ami-2"),
						ImageId:      aws.String("ami-id-456"),
						CreationDate: aws.String(time.Now().Add(time.Minute).Format(time.RFC3339)),
						Architecture: aws.String("arm64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-2")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)

			Expect(nodeTemplate.Status.AMIs).To(Equal([]v1alpha1.AMI{
				{
					Name: "test-ami-2",
					ID:   "ami-id-456",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.ArchitectureArm64},
						},
						{
							Key:      v1alpha1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
						{
							Key:      v1alpha1.LabelInstanceAcceleratorCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
					},
				},
				{
					Name: "test-ami-1",
					ID:   "ami-id-123",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.ArchitectureAmd64},
						},
						{
							Key:      v1alpha1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
						{
							Key:      v1alpha1.LabelInstanceAcceleratorCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
					},
				},
			}))
		})
		It("Should resolve a valid AMI selector", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.AMIs).To(Equal(
				[]v1alpha1.AMI{
					{
						Name: "test-ami-3",
						ID:   "ami-test3",
						Requirements: []v1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: "In",
								Values: []string{
									"amd64",
								},
							},
						},
					},
				},
			))
		})
		It("should resolve amiSelector AMIs that have well-known tags as AMI requirements into status", func() {
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					{
						Name:         aws.String("test-ami-4"),
						ImageId:      aws.String("ami-test4"),
						CreationDate: aws.String(time.Now().Add(2 * time.Minute).Format(time.RFC3339)),
						Architecture: aws.String("x86_64"),
						Tags: []*ec2.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-3")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
							{Key: aws.String("kubernetes.io/os"), Value: aws.String("test-requirement-1")},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)

			Expect(nodeTemplate.Status.AMIs).To(Equal([]v1alpha1.AMI{
				{
					Name: "test-ami-4",
					ID:   "ami-test4",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      "kubernetes.io/os",
							Operator: "In",
							Values: []string{
								"test-requirement-1",
							},
						},
						{
							Key:      "kubernetes.io/arch",
							Operator: "In",
							Values: []string{
								"amd64",
							},
						},
					},
				},
			}))
		})
	})
	Context("Static Drift Hash", func() {
		DescribeTable("should update the static drift hash when static field is updated", func(changes v1alpha1.AWSNodeTemplateSpec) {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)

			expectedHash := nodeTemplate.Hash()
			Expect(nodeTemplate.Annotations[v1alpha1.AnnotationNodeTemplateHash]).To(Equal(expectedHash))

			Expect(mergo.Merge(&nodeTemplate.Spec, changes, mergo.WithOverride)).To(Succeed())

			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)

			expectedHashTwo := nodeTemplate.Hash()
			Expect(nodeTemplate.Annotations[v1alpha1.AnnotationNodeTemplateHash]).To(Equal(expectedHashTwo))
			Expect(expectedHash).ToNot(Equal(expectedHashTwo))
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
		It("should not update the static drift hash when dynamic field is updated", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)

			expectedHash := nodeTemplate.Hash()
			Expect(nodeTemplate.ObjectMeta.Annotations[v1alpha1.AnnotationNodeTemplateHash]).To(Equal(expectedHash))

			nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1"}
			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{"aws-ids": "sg-test1"}
			nodeTemplate.Spec.AMISelector = map[string]string{"ami-test-key": "ami-test-value"}

			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, nodeTemplateController, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)

			Expect(nodeTemplate.ObjectMeta.Annotations[v1alpha1.AnnotationNodeTemplateHash]).To(Equal(expectedHash))
		})
		It("should maintain the same hash, before and after the EC2NodeClass conversion", func() {
			hash := nodeTemplate.Hash()
			nodeClass := nodeclassutil.New(nodeTemplate)
			convertedHash := nodeclassutil.HashAnnotation(nodeClass)
			Expect(convertedHash).To(HaveKeyWithValue(v1alpha1.AnnotationNodeTemplateHash, hash))
		})
	})
})
