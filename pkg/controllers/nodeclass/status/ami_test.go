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
	v1 "k8s.io/api/core/v1"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass AMI Status Controller", func() {
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
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
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
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
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
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.AMIs).To(Equal(
			[]v1beta1.AMI{
				{
					Name: "test-ami-3",
					ID:   "ami-test3",
					Requirements: []v1.NodeSelectorRequirement{{
						Key:      v1.LabelArchStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{corev1beta1.ArchitectureAmd64},
					},
					},
				},
			},
		))
	})
})
