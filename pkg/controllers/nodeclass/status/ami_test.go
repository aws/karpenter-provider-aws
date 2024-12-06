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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
	var k8sVersion string
	BeforeEach(func() {
		k8sVersion = awsEnv.VersionProvider.Get(ctx)
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
			Images: []ec2types.Image{
				{
					Name:         aws.String("amd64-standard"),
					ImageId:      aws.String("ami-amd64-standard"),
					CreationDate: aws.String(time.Now().Format(time.RFC3339)),
					Architecture: "x86_64",
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("amd64-standard")},
						{Key: aws.String("foo"), Value: aws.String("bar")},
					},
				},
				{
					Name:         aws.String("amd64-standard-new"),
					ImageId:      aws.String("ami-amd64-standard-new"),
					CreationDate: aws.String(time.Now().Add(time.Minute).Format(time.RFC3339)),
					Architecture: "x86_64",
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("amd64-standard")},
						{Key: aws.String("foo"), Value: aws.String("bar")},
					},
				},
				{
					Name:         aws.String("amd64-nvidia"),
					ImageId:      aws.String("ami-amd64-nvidia"),
					CreationDate: aws.String(time.Now().Format(time.RFC3339)),
					Architecture: "x86_64",
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("amd64-nvidia")},
						{Key: aws.String("foo"), Value: aws.String("bar")},
					},
				},
				{
					Name:         aws.String("amd64-neuron"),
					ImageId:      aws.String("ami-amd64-neuron"),
					CreationDate: aws.String(time.Now().Format(time.RFC3339)),
					Architecture: "x86_64",
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("amd64-neuron")},
						{Key: aws.String("foo"), Value: aws.String("bar")},
					},
				},
				{
					Name:         aws.String("arm64-standard"),
					ImageId:      aws.String("ami-arm64-standard"),
					CreationDate: aws.String(time.Now().Format(time.RFC3339)),
					Architecture: "arm64",
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("arm64-standard")},
						{Key: aws.String("foo"), Value: aws.String("bar")},
					},
				},
				{
					Name:         aws.String("arm64-nvidia"),
					ImageId:      aws.String("ami-arm64-nvidia"),
					CreationDate: aws.String(time.Now().Format(time.RFC3339)),
					Architecture: "arm64",
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("arm64-nvidia")},
						{Key: aws.String("foo"), Value: aws.String("bar")},
					},
				},
			},
		})
	})
	Context("Aliases", func() {
		It("Should resolve all AMIs with correct requirements for AL2023", func() {
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", k8sVersion): "ami-amd64-standard",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/nvidia/recommended/image_id", k8sVersion):   "ami-amd64-nvidia",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/neuron/recommended/image_id", k8sVersion):   "ami-amd64-neuron",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/arm64/standard/recommended/image_id", k8sVersion):  "ami-arm64-standard",
			}
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			Expect(len(nodeClass.Status.AMIs)).To(Equal(4))
			Expect(nodeClass.Status.AMIs).To(ContainElements([]v1.AMI{
				{
					Name: "amd64-standard",
					ID:   "ami-amd64-standard",
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
				{
					Name: "amd64-nvidia",
					ID:   "ami-amd64-nvidia",
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
					Name: "amd64-neuron",
					ID:   "ami-amd64-neuron",
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
					Name: "arm64-standard",
					ID:   "ami-arm64-standard",
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
			}))
			Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeTrue())
		})
		It("Should resolve all AMIs with correct requirements for AL2", func() {
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", k8sVersion):       "ami-amd64-standard",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", k8sVersion):   "ami-amd64-nvidia",
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", k8sVersion): "ami-arm64-standard",
			}
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			Expect(len(nodeClass.Status.AMIs)).To(Equal(4))
			Expect(nodeClass.Status.AMIs).To(ContainElements([]v1.AMI{
				{
					Name: "amd64-standard",
					ID:   "ami-amd64-standard",
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
				{
					Name: "amd64-nvidia",
					ID:   "ami-amd64-nvidia",
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
				// Note: AL2 uses the same AMI for nvidia and neuron, we use the nvidia AMI here
				{
					Name: "amd64-nvidia",
					ID:   "ami-amd64-nvidia",
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
					Name: "arm64-standard",
					ID:   "ami-arm64-standard",
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
			}))
			Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeTrue())
		})
		It("Should resolve all AMIs with correct requirements for Bottlerocket", func() {
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", k8sVersion):        "ami-amd64-standard",
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", k8sVersion):         "ami-arm64-standard",
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", k8sVersion): "ami-amd64-nvidia",
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/arm64/latest/image_id", k8sVersion):  "ami-arm64-nvidia",
			}
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			Expect(len(nodeClass.Status.AMIs)).To(Equal(4))
			Expect(nodeClass.Status.AMIs).To(ContainElements([]v1.AMI{
				{
					Name: "amd64-standard",
					ID:   "ami-amd64-standard",
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
				{
					Name: "arm64-standard",
					ID:   "ami-arm64-standard",
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
					Name: "amd64-nvidia",
					ID:   "ami-amd64-nvidia",
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
					Name: "arm64-nvidia",
					ID:   "ami-arm64-nvidia",
					Requirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.ArchitectureArm64},
						},
						{
							Key:      v1.LabelInstanceGPUCount,
							Operator: corev1.NodeSelectorOpExists,
						},
					},
				},
			}))
			Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeTrue())
		})
		It("Should resolve all AMIs with correct requirements for Windows2019", func() {
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2019-English-Core-EKS_Optimized-%s/image_id", k8sVersion): "ami-amd64-standard",
			}
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2019@latest"}}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			Expect(len(nodeClass.Status.AMIs)).To(Equal(1))
			Expect(nodeClass.Status.AMIs).To(ContainElements([]v1.AMI{
				{
					Name: "amd64-standard",
					ID:   "ami-amd64-standard",
					Requirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelOSStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{string(corev1.Windows)},
						},
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.ArchitectureAmd64},
						},
						{
							Key:      corev1.LabelWindowsBuild,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{v1.Windows2019Build},
						},
					},
				},
			}))
			Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeTrue())
		})
		It("Should resolve all AMIs with correct requirements for Windows2022", func() {
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2022-English-Core-EKS_Optimized-%s/image_id", k8sVersion): "ami-amd64-standard",
			}
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			Expect(len(nodeClass.Status.AMIs)).To(Equal(1))
			Expect(nodeClass.Status.AMIs).To(ContainElements([]v1.AMI{
				{
					Name: "amd64-standard",
					ID:   "ami-amd64-standard",
					Requirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelOSStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{string(corev1.Windows)},
						},
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.ArchitectureAmd64},
						},
						{
							Key:      corev1.LabelWindowsBuild,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{v1.Windows2022Build},
						},
					},
				},
			}))
			Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeAMIsReady)).To(BeTrue())
		})
	})
	It("should resolve amiSelector AMIs and requirements into status when all SSM parameters don't resolve", func() {
		// This parameter set doesn't include any of the Nvidia AMIs
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", k8sVersion): "ami-amd64-standard",
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", k8sVersion):  "ami-arm64-standard",
		}
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
			Alias: "bottlerocket@latest",
		}}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(len(nodeClass.Status.AMIs)).To(Equal(2))
		Expect(nodeClass.Status.AMIs).To(ContainElements([]v1.AMI{
			{
				Name: "arm64-standard",
				ID:   "ami-arm64-standard",
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
				Name: "amd64-standard",
				ID:   "ami-amd64-standard",
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
	It("should resolve a valid AMI selector", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
			Tags: map[string]string{"Name": "amd64-standard"},
		}}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.AMIs).To(Equal(
			[]v1.AMI{
				{
					Name: "amd64-standard-new",
					ID:   "ami-amd64-standard-new",
					Requirements: []corev1.NodeSelectorRequirement{{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.ArchitectureAmd64},
					}},
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
