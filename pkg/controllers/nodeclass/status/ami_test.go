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

package status_test

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass AMI Status Controller", func() {
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				AMIFamily: lo.ToPtr(v1.AMIFamilyCustom),
				AMISelectorTerms: []v1.AMISelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
			},
		})
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
	It("should fail to resolve AMIs if the nodeclass has ubuntu incompatible annotation", func() {
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2)
		nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{v1.AnnotationUbuntuCompatibilityKey: v1.AnnotationUbuntuCompatibilityIncompatible})
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		cond := nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady)
		Expect(cond.IsTrue()).To(BeFalse())
		Expect(cond.Message).To(Equal("Ubuntu AMI discovery is not supported at v1, refer to the upgrade guide (https://karpenter.sh/docs/upgrading/upgrade-guide/#upgrading-to-100)"))
		Expect(cond.Reason).To(Equal("AMINotFound"))

	})
	It("should resolve amiSelector AMIs and requirements into status", func() {
		version := lo.Must(awsEnv.VersionProvider.Get(ctx))

		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):                                                 "ami-id-123",
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version):                                             "ami-id-456",
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2%s/recommended/image_id", version, fmt.Sprintf("-%s", karpv1.ArchitectureArm64)): "ami-id-789",
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
					Architecture: aws.String("arm64"),
					Tags: []*ec2.Tag{
						{Key: aws.String("Name"), Value: aws.String("test-ami-3")},
						{Key: aws.String("foo"), Value: aws.String("bar")},
					},
				},
			},
		})
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(len(nodeClass.Status.AMIs)).To(Equal(4))
		Expect(nodeClass.Status.AMIs).To(ContainElements([]v1.AMI{
			{
				Name: "test-ami-3",
				ID:   "ami-id-789",
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.ArchitectureArm64},
					},
					{
						Key:      v1.LabelInstanceGPUCount,
						Operator: corev1.NodeSelectorOpDoesNotExist,
					},
					{
						Key:      v1.LabelInstanceAcceleratorCount,
						Operator: corev1.NodeSelectorOpDoesNotExist,
					},
				},
			},
			{
				Name: "test-ami-2",
				ID:   "ami-id-456",
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.ArchitectureAmd64},
					},
					{
						Key:      v1.LabelInstanceGPUCount,
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				Name: "test-ami-2",
				ID:   "ami-id-456",
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.ArchitectureAmd64},
					},
					{
						Key:      v1.LabelInstanceAcceleratorCount,
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				Name: "test-ami-1",
				ID:   "ami-id-123",
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.ArchitectureAmd64},
					},
					{
						Key:      v1.LabelInstanceGPUCount,
						Operator: corev1.NodeSelectorOpDoesNotExist,
					},
					{
						Key:      v1.LabelInstanceAcceleratorCount,
						Operator: corev1.NodeSelectorOpDoesNotExist,
					},
				},
			},
		}))
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeTrue())
	})
	It("should resolve amiSelector AMis and requirements into status when all SSM aliases don't resolve", func() {
		version := lo.Must(awsEnv.VersionProvider.Get(ctx))
		// This parameter set doesn't include any of the Nvidia AMIs
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", version): "ami-id-123",
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", version):  "ami-id-456",
		}
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
			Alias: "bottlerocket@latest",
		}}
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
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(len(nodeClass.Status.AMIs)).To(Equal(2))
		Expect(nodeClass.Status.AMIs).To(ContainElements([]v1.AMI{
			{
				Name: "test-ami-2",
				ID:   "ami-id-456",
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.ArchitectureArm64},
					},
					{
						Key:      v1.LabelInstanceGPUCount,
						Operator: corev1.NodeSelectorOpDoesNotExist,
					},
					{
						Key:      v1.LabelInstanceAcceleratorCount,
						Operator: corev1.NodeSelectorOpDoesNotExist,
					},
				},
			},
			{
				Name: "test-ami-1",
				ID:   "ami-id-123",
				Requirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.ArchitectureAmd64},
					},
					{
						Key:      v1.LabelInstanceGPUCount,
						Operator: corev1.NodeSelectorOpDoesNotExist,
					},
					{
						Key:      v1.LabelInstanceAcceleratorCount,
						Operator: corev1.NodeSelectorOpDoesNotExist,
					},
				},
			},
		}))
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeTrue())
	})
	It("Should resolve a valid AMI selector", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.AMIs).To(Equal(
			[]v1.AMI{
				{
					Name: "test-ami-3",
					ID:   "ami-test3",
					Requirements: []corev1.NodeSelectorRequirement{{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.ArchitectureAmd64},
					},
					},
				},
			},
		))
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeTrue())
	})
	It("should get error when resolving AMIs and have status condition set to false", func() {
		awsEnv.EC2API.NextError.Set(fmt.Errorf("unable to resolve AMI"))
		ExpectApplied(ctx, env.Client, nodeClass)
		_ = ExpectObjectReconcileFailed(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeFalse())
	})
})
