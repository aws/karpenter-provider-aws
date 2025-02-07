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

package amifamily_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass *v1.EC2NodeClass

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AMISelector")
}

const (
	amd64AMI       = "amd64-ami-id"
	arm64AMI       = "arm64-ami-id"
	amd64NvidiaAMI = "amd64-nvidia-ami-id"
	arm64NvidiaAMI = "arm64-nvidia-ami-id"
)

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = BeforeEach(func() {
	// Set up the DescribeImages API so that we can call it by ID with the mock parameters that we generate
	awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
		Images: []ec2types.Image{
			{
				Name:         aws.String(amd64AMI),
				ImageId:      aws.String("amd64-ami-id"),
				CreationDate: aws.String(time.Time{}.Format(time.RFC3339)),
				Architecture: "x86_64",
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
				State: ec2types.ImageStateAvailable,
			},
			{
				Name:         aws.String(arm64AMI),
				ImageId:      aws.String("arm64-ami-id"),
				CreationDate: aws.String(time.Time{}.Add(time.Minute).Format(time.RFC3339)),
				Architecture: "arm64",
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(arm64AMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
				State: ec2types.ImageStateAvailable,
			},
			{
				Name:         aws.String(amd64NvidiaAMI),
				ImageId:      aws.String("amd64-nvidia-ami-id"),
				CreationDate: aws.String(time.Time{}.Add(2 * time.Minute).Format(time.RFC3339)),
				Architecture: "x86_64",
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64NvidiaAMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
				State: ec2types.ImageStateAvailable,
			},
			{
				Name:         aws.String(arm64NvidiaAMI),
				ImageId:      aws.String("arm64-nvidia-ami-id"),
				CreationDate: aws.String(time.Time{}.Add(2 * time.Minute).Format(time.RFC3339)),
				Architecture: "arm64",
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(arm64NvidiaAMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
				State: ec2types.ImageStateAvailable,
			},
		},
	})
})

var _ = AfterEach(func() {
	awsEnv.Reset()
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("AMIProvider", func() {
	var version string
	BeforeEach(func() {
		version = awsEnv.VersionProvider.Get(ctx)
		nodeClass = test.EC2NodeClass()
	})
	It("should succeed to resolve AMIs (AL2)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):       amd64AMI,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version):   amd64NvidiaAMI,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version): arm64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(4))
	})
	It("should succeed to resolve AMIs (AL2023)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", version): amd64AMI,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/arm64/standard/recommended/image_id", version):  arm64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(2))
	})
	It("should succeed to resolve AMIs (Bottlerocket)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", version):        amd64AMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", version): amd64NvidiaAMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", version):         arm64AMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/arm64/latest/image_id", version):  arm64NvidiaAMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(4))
	})
	It("should succeed to resolve AMIs (Windows2019)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2019@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2019-English-Core-EKS_Optimized-%s/image_id", version): amd64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
	})
	It("should succeed to resolve AMIs (Windows2022)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2022-English-Core-EKS_Optimized-%s/image_id", version): amd64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
	})
	It("should not cause data races when calling Get() simultaneously", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
			{
				ID: "amd64-ami-id",
			},
			{
				ID: "arm64-ami-id",
			},
		}
		wg := sync.WaitGroup{}
		for i := 0; i < 10000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				images, err := awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())

				Expect(images).To(HaveLen(2))
				// Sort everything in parallel and ensure that we don't get data races
				images.Sort()
				Expect(images).To(BeEquivalentTo([]amifamily.AMI{
					{
						Name:         arm64AMI,
						AmiID:        "arm64-ami-id",
						CreationDate: time.Time{}.Add(time.Minute).Format(time.RFC3339),
						Requirements: scheduling.NewLabelRequirements(map[string]string{
							corev1.LabelArchStable: karpv1.ArchitectureArm64,
						}),
					},
					{
						Name:         amd64AMI,
						AmiID:        "amd64-ami-id",
						CreationDate: time.Time{}.Format(time.RFC3339),
						Requirements: scheduling.NewLabelRequirements(map[string]string{
							corev1.LabelArchStable: karpv1.ArchitectureAmd64,
						}),
					},
				}))
			}()
		}
		wg.Wait()
	})
	DescribeTable(
		"should ignore images when image.state != available",
		func(state ec2types.ImageState) {
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
				{
					Name:         aws.String(coretest.RandomName()),
					ImageId:      aws.String("ami-123"),
					Architecture: "x86_64",
					Tags:         []ec2types.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        ec2types.ImageStateAvailable,
				},
				{
					Name:         aws.String(coretest.RandomName()),
					ImageId:      aws.String("ami-456"),
					Architecture: "arm64",
					Tags:         []ec2types.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        state,
				},
			}})
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
				Tags: map[string]string{
					"test": "test",
				},
			}}
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis[0].AmiID).To(Equal("ami-123"))
		},
		lo.FilterMap(ec2types.ImageState("").Values(), func(state ec2types.ImageState, _ int) (TableEntry, bool) {
			if state == ec2types.ImageStateAvailable {
				return TableEntry{}, false
			}
			return Entry(string(state), state), true
		}),
	)

	Context("SSM Alias Missing", func() {
		It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Al2)", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			// No GPU AMI exists here
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):       amd64AMI,
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version): arm64AMI,
			}
			// Only 2 of the requirements sets for the SSM aliases will resolve
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(2))
		})
		It("should succeed to partially resolve AMIs if all SSM aliases don't exist (AL2023)", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", version): amd64AMI,
			}
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
		})
		It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Bottlerocket)", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			// No GPU AMI exists for AM64 here
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", version):        amd64AMI,
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", version): amd64NvidiaAMI,
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", version):         arm64AMI,
			}
			// Only 4 of the requirements sets for the SSM aliases will resolve
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(3))
		})
	})
	Context("AMI Tag Requirements", func() {
		var img ec2types.Image
		BeforeEach(func() {
			img = ec2types.Image{
				Name:         aws.String(amd64AMI),
				ImageId:      aws.String("amd64-ami-id"),
				CreationDate: aws.String(time.Now().Format(time.RFC3339)),
				Architecture: "x86_64",
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
					{Key: aws.String(corev1.LabelInstanceTypeStable), Value: aws.String("m5.large")},
					{Key: aws.String(corev1.LabelTopologyZone), Value: aws.String("test-zone-1a")},
				},
				State: ec2types.ImageStateAvailable,
			}
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					img,
				},
			})
		})
		It("should succeed to not resolve tags as requirements for NodeClasses", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{"*": "*"},
				},
			}
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis).To(ConsistOf(amifamily.AMI{
				Name:         aws.ToString(img.Name),
				AmiID:        aws.ToString(img.ImageId),
				CreationDate: aws.ToString(img.CreationDate),
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, karpv1.ArchitectureAmd64),
				),
			}))
		})
	})
	Context("AMI List requirements", func() {
		BeforeEach(func() {
			// Set time using the injectable/fake clock to now
			awsEnv.Clock.SetTime(time.Now())
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{"*": "*"},
				},
			}
		})
		It("should priortize the older non-deprecated ami without deprecation time", func() {
			// Here we have two AMIs one which is deprecated and newer and one which is older and non-deprecated without a deprecation time
			// List operation will priortize the non-deprecated AMI without the deprecation time
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						Name:            aws.String(amd64AMI),
						ImageId:         aws.String("ami-5678"),
						CreationDate:    aws.String("2021-08-31T00:12:42.000Z"),
						DeprecationTime: aws.String(awsEnv.Clock.Now().Add(-1 * time.Hour).Format(time.RFC3339)),
						Architecture:    "x86_64",
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
						State: ec2types.ImageStateAvailable,
					},
					{
						Name:         aws.String(amd64AMI),
						ImageId:      aws.String("ami-1234"),
						CreationDate: aws.String("2020-08-31T00:08:42.000Z"),
						Architecture: "x86_64",
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
						State: ec2types.ImageStateAvailable,
					},
				},
			})
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis).To(ConsistOf(amifamily.AMI{
				Name:         amd64AMI,
				AmiID:        "ami-1234",
				CreationDate: "2020-08-31T00:08:42.000Z",
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, karpv1.ArchitectureAmd64),
				),
			}))
		})
		It("should priortize the non-deprecated ami with deprecation time when both have same creation time", func() {
			// Here we have two AMIs one which is deprecated and one which is non-deprecated both with the same creation time
			// List operation will priortize the non-deprecated AMI
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						Name:            aws.String(amd64AMI),
						ImageId:         aws.String("ami-5678"),
						CreationDate:    aws.String("2021-08-31T00:12:42.000Z"),
						DeprecationTime: aws.String(awsEnv.Clock.Now().Add(-10 * time.Minute).Format(time.RFC3339)),
						Architecture:    "x86_64",
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
						State: ec2types.ImageStateAvailable,
					},
					{
						Name:            aws.String(amd64AMI),
						ImageId:         aws.String("ami-1234"),
						CreationDate:    aws.String("2021-08-31T00:12:42.000Z"),
						DeprecationTime: aws.String(awsEnv.Clock.Now().Add(10 * time.Minute).Format(time.RFC3339)),
						Architecture:    "x86_64",
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
						State: ec2types.ImageStateAvailable,
					},
				},
			})
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis).To(ConsistOf(amifamily.AMI{
				Name:         amd64AMI,
				AmiID:        "ami-1234",
				CreationDate: "2021-08-31T00:12:42.000Z",
				Deprecated:   false,
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, karpv1.ArchitectureAmd64),
				),
			}))
		})
		It("should priortize the non-deprecated ami with deprecation time when both have same creation time and different name", func() {
			// Here we have two AMIs one which is deprecated and one which is non-deprecated both with the same creation time but with different names
			// List operation will priortize the non-deprecated AMI
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						Name:            aws.String("test-ami-2"),
						ImageId:         aws.String("ami-5678"),
						CreationDate:    aws.String("2021-08-31T00:12:42.000Z"),
						DeprecationTime: aws.String(awsEnv.Clock.Now().Add(-10 * time.Minute).Format(time.RFC3339)),
						Architecture:    "x86_64",
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-2")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
						State: ec2types.ImageStateAvailable,
					},
					{
						Name:            aws.String("test-ami-1"),
						ImageId:         aws.String("ami-1234"),
						CreationDate:    aws.String("2021-08-31T00:12:42.000Z"),
						DeprecationTime: aws.String(awsEnv.Clock.Now().Add(10 * time.Minute).Format(time.RFC3339)),
						Architecture:    "x86_64",
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-ami-1")},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
						State: ec2types.ImageStateAvailable,
					},
				},
			})
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis).To(ConsistOf(amifamily.AMI{
				Name:         "test-ami-1",
				AmiID:        "ami-1234",
				CreationDate: "2021-08-31T00:12:42.000Z",
				Deprecated:   false,
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, karpv1.ArchitectureAmd64),
				),
			}))
		})
		It("should priortize the newer ami if both are deprecated", func() {
			//Both amis are deprecated and have the same deprecation time
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						Name:            aws.String(amd64AMI),
						ImageId:         aws.String("ami-5678"),
						CreationDate:    aws.String("2021-08-31T00:12:42.000Z"),
						DeprecationTime: aws.String(awsEnv.Clock.Now().Add(-1 * time.Hour).Format(time.RFC3339)),
						Architecture:    "x86_64",
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
						State: ec2types.ImageStateAvailable,
					},
					{
						Name:            aws.String(amd64AMI),
						ImageId:         aws.String("ami-1234"),
						CreationDate:    aws.String("2020-08-31T00:08:42.000Z"),
						DeprecationTime: aws.String(awsEnv.Clock.Now().Add(-1 * time.Hour).Format(time.RFC3339)),
						Architecture:    "x86_64",
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
							{Key: aws.String("foo"), Value: aws.String("bar")},
						},
						State: ec2types.ImageStateAvailable,
					},
				},
			})
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis).To(ConsistOf(amifamily.AMI{
				Name:         amd64AMI,
				AmiID:        "ami-5678",
				CreationDate: "2021-08-31T00:12:42.000Z",
				Deprecated:   true,
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, karpv1.ArchitectureAmd64),
				),
			}))
		})
	})
	Context("AMI Selectors", func() {
		// When you tag public or shared resources, the tags you assign are available only to your AWS account; no other AWS account will have access to those tags
		// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html#tag-restrictions
		It("should have empty owners and use tags when prefixes aren't set", func() {
			queries, err := awsEnv.AMIProvider.DescribeImageQueries(ctx, &v1.EC2NodeClass{
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms: []v1.AMISelectorTerm{{
						Tags: map[string]string{
							"Name": "my-ami",
						},
					}},
				},
			})
			Expect(err).To(BeNil())
			ExpectConsistsOfAMIQueries([]amifamily.DescribeImageQuery{
				{
					Filters: []ec2types.Filter{
						{
							Name:   lo.ToPtr("tag:Name"),
							Values: []string{"my-ami"},
						},
					},
					Owners: []string{},
				},
			}, queries)
		})
		It("should have default owners and use name when prefixed", func() {
			queries, err := awsEnv.AMIProvider.DescribeImageQueries(ctx, &v1.EC2NodeClass{
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms: []v1.AMISelectorTerm{{
						Name: "my-ami",
					}},
				},
			})
			Expect(err).To(BeNil())
			ExpectConsistsOfAMIQueries([]amifamily.DescribeImageQuery{
				{
					Filters: []ec2types.Filter{
						{
							Name:   lo.ToPtr("name"),
							Values: []string{"my-ami"},
						},
					},
					Owners: []string{
						"amazon",
						"self",
					},
				},
			}, queries)
		})
		It("should not set owners when legacy ids are passed", func() {
			queries, err := awsEnv.AMIProvider.DescribeImageQueries(ctx, &v1.EC2NodeClass{
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{
							ID: "ami-abcd1234",
						},
						{
							ID: "ami-cafeaced",
						},
					},
				},
			})
			Expect(err).To(BeNil())
			ExpectConsistsOfAMIQueries([]amifamily.DescribeImageQuery{
				{
					Filters: []ec2types.Filter{
						{
							Name:   lo.ToPtr("image-id"),
							Values: []string{"ami-abcd1234", "ami-cafeaced"},
						},
					},
				},
			}, queries)
		})
		It("should allow only specifying owners", func() {
			queries, err := awsEnv.AMIProvider.DescribeImageQueries(ctx, &v1.EC2NodeClass{
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{
							Owner: "abcdef",
						},
						{
							Owner: "123456789012",
						},
					},
				},
			})
			Expect(err).To(BeNil())
			ExpectConsistsOfAMIQueries([]amifamily.DescribeImageQuery{
				{
					Owners: []string{"abcdef"},
				},
				{
					Owners: []string{"123456789012"},
				},
			}, queries)
		})
		It("should allow prefixed name and prefixed owners", func() {
			queries, err := awsEnv.AMIProvider.DescribeImageQueries(ctx, &v1.EC2NodeClass{
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{
							Name:  "my-name",
							Owner: "0123456789",
						},
						{
							Name:  "my-name",
							Owner: "self",
						},
					},
				},
			})
			Expect(err).To(BeNil())
			ExpectConsistsOfAMIQueries([]amifamily.DescribeImageQuery{
				{
					Owners: []string{"0123456789"},
					Filters: []ec2types.Filter{
						{
							Name:   lo.ToPtr("name"),
							Values: []string{"my-name"},
						},
					},
				},
				{
					Owners: []string{"self"},
					Filters: []ec2types.Filter{
						{
							Name:   lo.ToPtr("name"),
							Values: []string{"my-name"},
						},
					},
				},
			}, queries)
		})
		It("should sort amis by creationDate", func() {
			amis := amifamily.AMIs{
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-1-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-2",
					AmiID:        "test-ami-2-id",
					CreationDate: "2021-08-31T00:12:42.000Z",
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-3",
					AmiID:        "test-ami-3-id",
					CreationDate: "2021-08-31T00:08:42.000Z",
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-4",
					AmiID:        "test-ami-4-id",
					CreationDate: "",
					Requirements: scheduling.NewRequirements(),
				},
			}
			amis.Sort()
			Expect(amis).To(Equal(
				amifamily.AMIs{
					{
						Name:         "test-ami-2",
						AmiID:        "test-ami-2-id",
						CreationDate: "2021-08-31T00:12:42.000Z",
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-1-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-3",
						AmiID:        "test-ami-3-id",
						CreationDate: "2021-08-31T00:08:42.000Z",
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-4",
						AmiID:        "test-ami-4-id",
						CreationDate: "",
						Requirements: scheduling.NewRequirements(),
					},
				},
			))
		})
		It("should sort amis with the same name and creation date consistently", func() {
			amis := amifamily.AMIs{
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-4-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-3-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-2-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-1-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Requirements: scheduling.NewRequirements(),
				},
			}

			amis.Sort()
			Expect(amis).To(Equal(
				amifamily.AMIs{
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-1-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-2-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-3-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-4-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Requirements: scheduling.NewRequirements(),
					},
				},
			))
		})
		It("should sort deprecated amis with the same name and deprecation time consistently", func() {
			amis := amifamily.AMIs{
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-4-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Deprecated:   true,
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-3-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Deprecated:   true,
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-2-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Deprecated:   true,
					Requirements: scheduling.NewRequirements(),
				},
				{
					Name:         "test-ami-1",
					AmiID:        "test-ami-1-id",
					CreationDate: "2021-08-31T00:10:42.000Z",
					Deprecated:   true,
					Requirements: scheduling.NewRequirements(),
				},
			}

			amis.Sort()
			Expect(amis).To(Equal(
				amifamily.AMIs{
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-1-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Deprecated:   true,
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-2-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Deprecated:   true,
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-3-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Deprecated:   true,
						Requirements: scheduling.NewRequirements(),
					},
					{
						Name:         "test-ami-1",
						AmiID:        "test-ami-4-id",
						CreationDate: "2021-08-31T00:10:42.000Z",
						Deprecated:   true,
						Requirements: scheduling.NewRequirements(),
					},
				},
			))
		})
	})
})

func ExpectConsistsOfAMIQueries(expected, actual []amifamily.DescribeImageQuery) {
	GinkgoHelper()
	Expect(actual).To(HaveLen(len(expected)))

	for _, list := range [][]amifamily.DescribeImageQuery{expected, actual} {
		for _, elem := range list {
			for _, f := range elem.Filters {
				sort.Slice(f.Values, func(i, j int) bool {
					return f.Values[i] < f.Values[j]
				})
			}
			sort.Slice(elem.Owners, func(i, j int) bool { return elem.Owners[i] < elem.Owners[j] })
			sort.Slice(elem.Filters, func(i, j int) bool {
				return lo.FromPtr(elem.Filters[i].Name) < lo.FromPtr(elem.Filters[j].Name)
			})
		}
	}
	Expect(actual).To(ConsistOf(lo.Map(expected, func(q amifamily.DescribeImageQuery, _ int) interface{} { return q })...))
}
