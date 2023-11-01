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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/imdario/mergo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/events"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	coreoptions "github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/controllers/nodeclass"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/providers/instanceprofile"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClassController corecontroller.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2NodeClass")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...), coretest.WithFieldIndexers(test.EC2NodeClassFieldIndexer(ctx)))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx = settings.ToContext(ctx, test.Settings())
	awsEnv = test.NewEnvironment(ctx, env)

	nodeClassController = nodeclass.NewNodeClassController(env.Client, events.NewRecorder(&record.FakeRecorder{}), awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClassController", func() {
	var nodeClass *v1beta1.EC2NodeClass
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1beta1.EC2NodeClass{
			Spec: v1beta1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				AMISelectorTerms: []v1beta1.AMISelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
			},
		})
	})
	Context("Subnet Status", func() {
		It("Should update EC2NodeClass status for Subnets", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
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
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
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
			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{`Name`: `test-subnet-1`},
				},
				{
					Tags: map[string]string{`Name`: `test-subnet-2`},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
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
			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					ID: "subnet-test1",
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
			}))
		})
		It("Should update Subnet status when the Subnet selector gets updated by tags", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
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

			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"Name": "test-subnet-1",
					},
				},
				{
					Tags: map[string]string{
						"Name": "test-subnet-2",
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
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
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
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

			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					ID: "subnet-test1",
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
			}))
		})
		It("Should not resolve a invalid selectors for Subnet", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{`foo`: `invalid`},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileFailed(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(BeNil())
		})
		It("Should not resolve a invalid selectors for an updated subnet selector", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
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

			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{`foo`: `invalid`},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileFailed(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(BeNil())
		})
	})
	Context("Security Groups Status", func() {
		It("Should update EC2NodeClass status for Security Groups", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1beta1.SecurityGroup{
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
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"Name": "test-security-group-1"},
				},
				{
					Tags: map[string]string{"Name": "test-security-group-2"},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1beta1.SecurityGroup{
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
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					ID: "sg-test1",
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1beta1.SecurityGroup{
				{
					ID:   "sg-test1",
					Name: "securityGroup-test1",
				},
			}))
		})
		It("Should update Security Groups status when the Security Groups selector gets updated by tags", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1beta1.SecurityGroup{
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

			nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"Name": "test-security-group-1"},
				},
				{
					Tags: map[string]string{"Name": "test-security-group-2"},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1beta1.SecurityGroup{
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
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1beta1.SecurityGroup{
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

			nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					ID: "sg-test1",
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1beta1.SecurityGroup{
				{
					ID:   "sg-test1",
					Name: "securityGroup-test1",
				},
			}))
		})
		It("Should not resolve a invalid selectors for Security Groups", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{`foo`: `invalid`},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileFailed(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(BeNil())
		})
		It("Should not resolve a invalid selectors for an updated Security Groups selector", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1beta1.SecurityGroup{
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

			nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{`foo`: `invalid`},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileFailed(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.SecurityGroups).To(BeNil())
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
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):                                                      "ami-id-123",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version):                                                  "ami-id-456",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2%s/recommended/image_id", version, fmt.Sprintf("-%s", corev1beta1.ArchitectureArm64)): "ami-id-789",
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
			nodeClass.Spec.AMISelectorTerms = nil
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.AMIs).To(Equal([]v1beta1.AMI{
				{
					Name: "test-ami-3",
					ID:   "ami-id-789",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1beta1.ArchitectureArm64},
						},
						{
							Key:      v1beta1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
						{
							Key:      v1beta1.LabelInstanceAcceleratorCount,
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
							Values:   []string{corev1beta1.ArchitectureAmd64},
						},
						{
							Key:      v1beta1.LabelInstanceGPUCount,
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
							Values:   []string{corev1beta1.ArchitectureAmd64},
						},
						{
							Key:      v1beta1.LabelInstanceAcceleratorCount,
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
							Values:   []string{corev1beta1.ArchitectureAmd64},
						},
						{
							Key:      v1beta1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
						{
							Key:      v1beta1.LabelInstanceAcceleratorCount,
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
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
			nodeClass.Spec.AMISelectorTerms = nil
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
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			Expect(nodeClass.Status.AMIs).To(Equal([]v1beta1.AMI{
				{
					Name: "test-ami-2",
					ID:   "ami-id-456",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1beta1.ArchitectureArm64},
						},
						{
							Key:      v1beta1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
						{
							Key:      v1beta1.LabelInstanceAcceleratorCount,
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
							Values:   []string{corev1beta1.ArchitectureAmd64},
						},
						{
							Key:      v1beta1.LabelInstanceGPUCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
						{
							Key:      v1beta1.LabelInstanceAcceleratorCount,
							Operator: v1.NodeSelectorOpDoesNotExist,
						},
					},
				},
			}))
		})
		It("Should resolve a valid AMI selector", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.AMIs).To(Equal(
				[]v1beta1.AMI{
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
	})
	Context("Static Drift Hash", func() {
		DescribeTable("should update the static drift hash when static field is updated", func(changes *v1beta1.EC2NodeClass) {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			expectedHash := nodeClass.Hash()
			Expect(nodeClass.ObjectMeta.Annotations[v1beta1.AnnotationNodeClassHash]).To(Equal(expectedHash))

			Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride)).To(Succeed())

			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			expectedHashTwo := nodeClass.Hash()
			Expect(nodeClass.Annotations[v1beta1.AnnotationNodeClassHash]).To(Equal(expectedHashTwo))
			Expect(expectedHash).ToNot(Equal(expectedHashTwo))

		},
			Entry("AMIFamily Drift", &v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMIFamily: aws.String(v1beta1.AMIFamilyBottlerocket)}}),
			Entry("UserData Drift", &v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
			Entry("Tags Drift", &v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
			Entry("BlockDeviceMappings Drift", &v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: aws.String("map-device-test-3")}}}}),
			Entry("DetailedMonitoring Drift", &v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
			Entry("MetadataOptions Drift", &v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPEndpoint: aws.String("disabled")}}}),
			Entry("Context Drift", &v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
		)
		It("should not update the static drift hash when dynamic field is updated", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			expectedHash := nodeClass.Hash()
			Expect(nodeClass.Annotations[v1beta1.AnnotationNodeClassHash]).To(Equal(expectedHash))

			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					ID: "subnet-test1",
				},
			}
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					ID: "sg-test1",
				},
			}
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{"ami-test-key": "ami-test-value"},
				},
			}

			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Annotations[v1beta1.AnnotationNodeClassHash]).To(Equal(expectedHash))
		})
	})
	Context("NodeClass Termination", func() {
		var profileName string
		BeforeEach(func() {
			nodeClass.Spec.Role = "test-role"
			ExpectApplied(ctx, env.Client, nodeClass)
			profileName = instanceprofile.GetProfileName(ctx, fake.DefaultRegion, nodeClass)
		})
		It("should succeed to delete the instance profile with no NodeClaims", func() {
			awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
				profileName: {
					InstanceProfileName: aws.String(profileName),
					Roles: []*iam.Role{
						{
							RoleId:   aws.String(fake.RoleID()),
							RoleName: aws.String(nodeClass.Spec.Role),
						},
					},
				},
			}
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))

			Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
			ExpectNotFound(ctx, env.Client, nodeClass)
		})
		It("should succeed to delete the instance profile when no roles exist with no NodeClaims", func() {
			awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
				profileName: {
					InstanceProfileName: aws.String(profileName),
				},
			}
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))

			Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
			ExpectNotFound(ctx, env.Client, nodeClass)
		})
		It("should succeed to delete the NodeClass when the instance profile doesn't exist", func() {
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))

			Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
			ExpectNotFound(ctx, env.Client, nodeClass)
		})
		It("should not delete the EC2NodeClass until all associated NodeClaims are terminated", func() {
			var nodeClaims []*corev1beta1.NodeClaim
			for i := 0; i < 2; i++ {
				nc := coretest.NodeClaim(corev1beta1.NodeClaim{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
					},
				})
				ExpectApplied(ctx, env.Client, nc)
				nodeClaims = append(nodeClaims, nc)
			}
			awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
				profileName: {
					InstanceProfileName: aws.String(profileName),
					Roles: []*iam.Role{
						{
							RoleId:   aws.String(fake.RoleID()),
							RoleName: aws.String(nodeClass.Spec.Role),
						},
					},
				},
			}
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))

			Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
			res := ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(res.RequeueAfter).To(Equal(time.Minute * 10))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClass)

			// Delete one of the NodeClaims
			// The NodeClass should still not delete
			ExpectDeleted(ctx, env.Client, nodeClaims[0])
			res = ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(res.RequeueAfter).To(Equal(time.Minute * 10))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
			ExpectExists(ctx, env.Client, nodeClass)

			// Delete the last NodeClaim
			// The NodeClass should now delete
			ExpectDeleted(ctx, env.Client, nodeClaims[1])
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
			ExpectNotFound(ctx, env.Client, nodeClass)
		})
	})
	Context("Instance Profile Status", func() {
		var profileName string
		BeforeEach(func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			profileName = instanceprofile.GetProfileName(ctx, fake.DefaultRegion, nodeClass)
		})
		It("should create the instance profile when it doesn't exist", func() {
			nodeClass.Spec.Role = "test-role"
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))

			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
			Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
			Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
		})
		It("should add the role to the instance profile when it exists without a role", func() {
			awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
				profileName: {
					InstanceProfileId:   aws.String(fake.InstanceProfileID()),
					InstanceProfileName: aws.String(profileName),
				},
			}

			nodeClass.Spec.Role = "test-role"
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))

			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
			Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
			Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
		})
		It("should update the role for the instance profile when the wrong role exists", func() {
			awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
				profileName: {
					InstanceProfileId:   aws.String(fake.InstanceProfileID()),
					InstanceProfileName: aws.String(profileName),
					Roles: []*iam.Role{
						{
							RoleName: aws.String("other-role"),
						},
					},
				},
			}

			nodeClass.Spec.Role = "test-role"
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))

			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
			Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
			Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
		})
		It("should not call CreateInstanceProfile or AddRoleToInstanceProfile when instance profile exists with correct role", func() {
			awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
				profileName: {
					InstanceProfileId:   aws.String(fake.InstanceProfileID()),
					InstanceProfileName: aws.String(profileName),
					Roles: []*iam.Role{
						{
							RoleName: aws.String("test-role"),
						},
					},
				},
			}

			nodeClass.Spec.Role = "test-role"
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))

			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
			Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
			Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

			Expect(awsEnv.IAMAPI.CreateInstanceProfileBehavior.Calls()).To(BeZero())
			Expect(awsEnv.IAMAPI.AddRoleToInstanceProfileBehavior.Calls()).To(BeZero())
		})
	})
})
