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

package nodetemplate_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/controllers/nodetemplate"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var opts options.Options
var nodeTemplate *v1alpha1.AWSNodeTemplate
var controller corecontroller.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSNodeTemplateController")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	awsEnv = test.NewEnvironment(ctx, env)

	controller = nodetemplate.NewController(env.Client, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = injection.WithOptions(ctx, opts)

	nodeTemplate = &v1alpha1.AWSNodeTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: coretest.RandomName(),
		},
		Spec: v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SubnetSelector:        map[string]string{"*": "*"},
				SecurityGroupSelector: map[string]string{"*": "*"},
			},
			AMISelector: map[string]string{"*": "*"},
		},
	}

	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("AWSNodeTemplateController", func() {
	Context("Subnet Status", func() {
		It("Should update AWSNodeTemplate status for Subnets", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ := awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			subnetIDs := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) string {
				return *ec2subnet.SubnetId
			})
			sort.Strings(subnetIDs)
			subnetIDsInStatus := lo.Map(nodeTemplate.Status.Subnets, func(subnet v1alpha1.Subnet, _ int) string {
				return subnet.ID
			})
			sort.Strings(subnetIDsInStatus)
			Expect(subnetIDsInStatus).To(Equal(subnetIDs))
		})
		It("Should have the correct ordering for the Subnets", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ := awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			sort.Slice(subnet, func(i, j int) bool {
				return int(*subnet[i].AvailableIpAddressCount) > int(*subnet[j].AvailableIpAddressCount)
			})
			correctSubnetIDs := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) string {
				return *ec2subnet.SubnetId
			})
			subnetIDsInStatus := lo.Map(nodeTemplate.Status.Subnets, func(subnet v1alpha1.Subnet, _ int) string {
				return subnet.ID
			})
			Expect(subnetIDsInStatus).To(Equal(correctSubnetIDs))
		})
		It("Should resolve a valid selectors for Subnet by tags", func() {
			nodeTemplate.Spec.SubnetSelector = map[string]string{`Name`: `test-subnet-1,test-subnet-2`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ := awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			sort.Slice(subnet, func(i, j int) bool {
				return int(*subnet[i].AvailableIpAddressCount) > int(*subnet[j].AvailableIpAddressCount)
			})
			correctSubnets := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) v1alpha1.Subnet {
				return v1alpha1.Subnet{
					ID:   *ec2subnet.SubnetId,
					Zone: *ec2subnet.AvailabilityZone,
				}
			})
			Expect(nodeTemplate.Status.Subnets).To(Equal(correctSubnets))
		})
		It("Should resolve a valid selectors for Subnet by ids", func() {
			nodeTemplate.Spec.SubnetSelector = map[string]string{`aws-ids`: `subnet-test1`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ := awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			correctSubnetIDs := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) v1alpha1.Subnet {
				return v1alpha1.Subnet{
					ID:   *ec2subnet.SubnetId,
					Zone: *ec2subnet.AvailabilityZone,
				}
			})
			// Only one subnet will be resolved
			Expect(nodeTemplate.Status.Subnets).To(Equal(correctSubnetIDs))
		})
		It("Should update Subnet status when the Subnet selector gets updated by tags", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ := awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			subnetIDs := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) string {
				return *ec2subnet.SubnetId
			})
			sort.Strings(subnetIDs)
			subnetIDsInStatus := lo.Map(nodeTemplate.Status.Subnets, func(subnet v1alpha1.Subnet, _ int) string {
				return subnet.ID
			})
			sort.Strings(subnetIDsInStatus)
			Expect(subnetIDsInStatus).To(Equal(subnetIDs))

			nodeTemplate.Spec.SubnetSelector = map[string]string{`Name`: `test-subnet-1,test-subnet-2`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ = awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			sort.Slice(subnet, func(i, j int) bool {
				return int(*subnet[i].AvailableIpAddressCount) > int(*subnet[j].AvailableIpAddressCount)
			})
			correctSubnets := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) v1alpha1.Subnet {
				return v1alpha1.Subnet{
					ID:   *ec2subnet.SubnetId,
					Zone: *ec2subnet.AvailabilityZone,
				}
			})
			Expect(nodeTemplate.Status.Subnets).To(Equal(correctSubnets))
		})
		It("Should update Subnet status when the Subnet selector gets updated by ids", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ := awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			subnetIDs := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) string {
				return *ec2subnet.SubnetId
			})
			sort.Strings(subnetIDs)
			subnetIDsInStatus := lo.Map(nodeTemplate.Status.Subnets, func(subnet v1alpha1.Subnet, _ int) string {
				return subnet.ID
			})
			sort.Strings(subnetIDsInStatus)
			Expect(subnetIDsInStatus).To(Equal(subnetIDs))

			nodeTemplate.Spec.SubnetSelector = map[string]string{`aws-ids`: `subnet-test1`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ = awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			correctSubnetIDs := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) v1alpha1.Subnet {
				return v1alpha1.Subnet{
					ID:   *ec2subnet.SubnetId,
					Zone: *ec2subnet.AvailabilityZone,
				}
			})
			// Only one subnet will be resolved
			Expect(nodeTemplate.Status.Subnets).To(Equal(correctSubnetIDs))
		})
		It("Should not resolve a invalid selectors for Subnet", func() {
			nodeTemplate.Spec.SubnetSelector = map[string]string{`foo`: `invalid`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileFailed(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(BeNil())
		})
		It("Should not resolve a invalid selectors for an updated Subnet selectors", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			subnet, _ := awsEnv.SubnetProvider.List(ctx, nodeTemplate)
			subnetIDs := lo.Map(subnet, func(ec2subnet *ec2.Subnet, _ int) string {
				return *ec2subnet.SubnetId
			})
			sort.Strings(subnetIDs)
			subnetIDsInStatus := lo.Map(nodeTemplate.Status.Subnets, func(subnet v1alpha1.Subnet, _ int) string {
				return subnet.ID
			})
			sort.Strings(subnetIDsInStatus)
			Expect(subnetIDsInStatus).To(Equal(subnetIDs))

			nodeTemplate.Spec.SubnetSelector = map[string]string{`foo`: `invalid`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileFailed(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.Subnets).To(BeNil())
		})
	})
	Context("Security Groups Status", func() {
		It("Should expect no errors when security groups are not in the AWSNodeTemplate", func() {
			// TODO: Remove test for v1beta1, as security groups will be required
			nodeTemplate.Spec.SecurityGroupSelector = nil
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ := awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			securityGroupsIDInStatus := lo.Map(nodeTemplate.Status.SecurityGroups, func(securitygroup v1alpha1.SecurityGroup, _ int) string {
				return securitygroup.ID
			})
			Expect(securityGroupsIDInStatus).To(Equal(securityGroupsIDs))
		})
		It("Should update AWSNodeTemplate status for Security Groups", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ := awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			securityGroupsIDInStatus := lo.Map(nodeTemplate.Status.SecurityGroups, func(securitygroup v1alpha1.SecurityGroup, _ int) string {
				return securitygroup.ID
			})
			Expect(securityGroupsIDInStatus).To(Equal(securityGroupsIDs))
		})
		It("Should resolve a valid selectors for Security Groups by tags", func() {
			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`Name`: `test-security-group-1,test-security-group-2`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ := awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			correctSecurityGroupsIDs := lo.Map(securityGroupsIDs, func(securitygroup string, _ int) v1alpha1.SecurityGroup {
				return v1alpha1.SecurityGroup{
					ID: securitygroup,
				}
			})
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal(correctSecurityGroupsIDs))
		})
		It("Should resolve a valid selectors for Security Groups by ids", func() {
			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`aws-ids`: `sg-test1`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ := awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			correctSecurityGroupsIDs := lo.Map(securityGroupsIDs, func(securitygroup string, _ int) v1alpha1.SecurityGroup {
				return v1alpha1.SecurityGroup{
					ID: securitygroup,
				}
			})
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal(correctSecurityGroupsIDs))
		})
		It("Should update Security Groups status when the Security Groups selector gets updated by tags", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ := awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			securityGroupsIDInStatus := lo.Map(nodeTemplate.Status.SecurityGroups, func(securitygroup v1alpha1.SecurityGroup, _ int) string {
				return securitygroup.ID
			})
			Expect(securityGroupsIDInStatus).To(Equal(securityGroupsIDs))

			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`Name`: `test-security-group-1,test-security-group-2`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ = awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			correctSecurityGroupsIDs := lo.Map(securityGroupsIDs, func(securitygroup string, _ int) v1alpha1.SecurityGroup {
				return v1alpha1.SecurityGroup{
					ID: securitygroup,
				}
			})
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal(correctSecurityGroupsIDs))
		})
		It("Should update Security Groups status when the Security Groups selector gets updated by ids", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ := awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			securityGroupsIDInStatus := lo.Map(nodeTemplate.Status.SecurityGroups, func(securitygroup v1alpha1.SecurityGroup, _ int) string {
				return securitygroup.ID
			})
			Expect(securityGroupsIDInStatus).To(Equal(securityGroupsIDs))

			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`aws-ids`: `sg-test1`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ = awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			correctSecurityGroupsIDs := lo.Map(securityGroupsIDs, func(securitygroup string, _ int) v1alpha1.SecurityGroup {
				return v1alpha1.SecurityGroup{
					ID: securitygroup,
				}
			})
			Expect(nodeTemplate.Status.SecurityGroups).To(Equal(correctSecurityGroupsIDs))
		})
		It("Should not resolve a invalid selectors for Security Groups", func() {
			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`foo`: `invalid`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileFailed(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.SecurityGroups).To(BeNil())
		})
		It("Should not resolve a invalid selectors for an updated Security Groups selector", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			securityGroupsIDs, _ := awsEnv.SecurityGroupProvider.List(ctx, nodeTemplate)
			securityGroupsIDInStatus := lo.Map(nodeTemplate.Status.SecurityGroups, func(securitygroup v1alpha1.SecurityGroup, _ int) string {
				return securitygroup.ID
			})
			Expect(securityGroupsIDInStatus).To(Equal(securityGroupsIDs))

			nodeTemplate.Spec.SecurityGroupSelector = map[string]string{`foo`: `invalid`}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileFailed(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
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
			version := lo.Must(awsEnv.AMIProvider.KubeServerVersion(ctx))

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
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			sortRequirements(nodeTemplate.Status.AMIs)
			Expect(nodeTemplate.Status.AMIs).To(ContainElements([]v1alpha1.AMI{
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
			},
			))
		})
		It("Should resolve a valid AMI selector", func() {
			ExpectApplied(ctx, env.Client, nodeTemplate)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			Expect(nodeTemplate.Status.AMIs).To(ContainElements(
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
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
			nodeTemplate = ExpectExists(ctx, env.Client, nodeTemplate)
			sortRequirements(nodeTemplate.Status.AMIs)
			Expect(nodeTemplate.Status.AMIs).To(ContainElements([]v1alpha1.AMI{
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
			},
			))
		})
	})
})

func sortRequirements(amis []v1alpha1.AMI) {
	for i := range amis {
		sort.Slice(amis[i].Requirements, func(p, q int) bool {
			return amis[i].Requirements[p].Key > amis[i].Requirements[q].Key
		})
	}
}
