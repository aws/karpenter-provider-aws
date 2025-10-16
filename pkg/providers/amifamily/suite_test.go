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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
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

	. "sigs.k8s.io/karpenter/pkg/test/expectations"
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
		Images: []*ec2.Image{
			{
				Name:         aws.String(amd64AMI),
				ImageId:      aws.String("amd64-ami-id"),
				CreationDate: aws.String(time.Time{}.Format(time.RFC3339)),
				Architecture: aws.String("x86_64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(arm64AMI),
				ImageId:      aws.String("arm64-ami-id"),
				CreationDate: aws.String(time.Time{}.Add(time.Minute).Format(time.RFC3339)),
				Architecture: aws.String("arm64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(arm64AMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(amd64NvidiaAMI),
				ImageId:      aws.String("amd64-nvidia-ami-id"),
				CreationDate: aws.String(time.Time{}.Add(2 * time.Minute).Format(time.RFC3339)),
				Architecture: aws.String("x86_64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64NvidiaAMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(arm64NvidiaAMI),
				ImageId:      aws.String("arm64-nvidia-ami-id"),
				CreationDate: aws.String(time.Time{}.Add(2 * time.Minute).Format(time.RFC3339)),
				Architecture: aws.String("arm64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(arm64NvidiaAMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
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
		version = lo.Must(awsEnv.VersionProvider.Get(ctx))
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
		Expect(amis).To(HaveLen(6))
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
			Expect(amis).To(HaveLen(4))
		})
	})
	Context("AMI Tag Requirements", func() {
		var img *ec2.Image
		BeforeEach(func() {
			img = &ec2.Image{
				Name:         aws.String(amd64AMI),
				ImageId:      aws.String("amd64-ami-id"),
				CreationDate: aws.String(time.Now().Format(time.RFC3339)),
				Architecture: aws.String("x86_64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
					{Key: aws.String(corev1.LabelInstanceTypeStable), Value: aws.String("m5.large")},
					{Key: aws.String(corev1.LabelTopologyZone), Value: aws.String("test-zone-1a")},
				},
			}
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
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
				Name:         aws.StringValue(img.Name),
				AmiID:        aws.StringValue(img.ImageId),
				CreationDate: aws.StringValue(img.CreationDate),
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, karpv1.ArchitectureAmd64),
				),
			}))
		})
	})
	Context("Provider Cache", func() {
		It("should resolve AMIs from cache that are filtered by id", func() {
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
				{
					Name:         aws.String(coretest.RandomName()),
					ImageId:      aws.String("ami-123"),
					Architecture: lo.ToPtr("x86_64"),
					Tags:         []*ec2.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
				{
					Name:         aws.String(coretest.RandomName()),
					ImageId:      aws.String("ami-456"),
					Architecture: lo.ToPtr("arm64"),
					Tags:         []*ec2.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
			}})
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					ID: "ami-123",
				},
				{
					ID: "ami-456",
				},
			}
			_, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())

			Expect(awsEnv.AMICache.Items()).To(HaveLen(1))
			cachedImages := lo.Values(awsEnv.AMICache.Items())[0].Object.(amifamily.AMIs)
			Expect(cachedImages).To(ContainElements(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"AmiID": Equal("ami-123"),
				}),
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"AmiID": Equal("ami-456"),
				}),
			))
		})
		It("should resolve AMIs from cache that are filtered by name", func() {
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
				{
					Name:         aws.String("ami-name-1"),
					ImageId:      aws.String("ami-123"),
					Architecture: lo.ToPtr("x86_64"),
					Tags:         []*ec2.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
				{
					Name:         aws.String("ami-name-2"),
					ImageId:      aws.String("ami-456"),
					Architecture: lo.ToPtr("arm64"),
					Tags:         []*ec2.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
			}})
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Name: "ami-name-1",
				},
				{
					Name: "ami-name-2",
				},
			}
			_, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())

			Expect(awsEnv.AMICache.Items()).To(HaveLen(1))
			cachedImages := lo.Values(awsEnv.AMICache.Items())[0].Object.(amifamily.AMIs)
			Expect(cachedImages).To(ContainElements(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Name": Equal("ami-name-1"),
				}),
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Name": Equal("ami-name-2"),
				}),
			))
		})
		It("should resolve AMIs from cache that are filtered by tags", func() {
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
				{
					Name:         aws.String("ami-name-1"),
					ImageId:      aws.String("ami-123"),
					Architecture: lo.ToPtr("x86_64"),
					Tags:         []*ec2.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
				{
					Name:         aws.String("ami-name-2"),
					ImageId:      aws.String("ami-456"),
					Architecture: lo.ToPtr("arm64"),
					Tags:         []*ec2.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
			}})
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{"test": "test"},
				},
			}
			_, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())

			Expect(awsEnv.AMICache.Items()).To(HaveLen(1))
			cachedImages := lo.Values(awsEnv.AMICache.Items())[0].Object.(amifamily.AMIs)
			Expect(cachedImages).To(ContainElements(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Name": Equal("ami-name-1"),
				}),
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Name": Equal("ami-name-2"),
				}),
			))
		})
		It("should correctly disambiguate AND vs OR semantics for tags", func() {
			// AND semantics
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
				{
					Name:         aws.String("ami-name-3"),
					ImageId:      aws.String("ami-789"),
					Architecture: lo.ToPtr("x86_64"),
					Tags:         []*ec2.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}, {Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
			}})
			nodeClass.Spec.AMIFamily = &v1.AMIFamilyAL2
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{"tag-key-1": "tag-value-1", "tag-key-2": "tag-value-2"},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())

			Expect(amis).To(ContainElements(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Name": Equal("ami-name-3"),
				}),
			))

			// OR semantics
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
				{
					Name:         aws.String("ami-name-1"),
					ImageId:      aws.String("ami-123"),
					Architecture: lo.ToPtr("x86_64"),
					Tags:         []*ec2.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
				{
					Name:         aws.String("ami-name-2"),
					ImageId:      aws.String("ami-456"),
					Architecture: lo.ToPtr("arm64"),
					Tags:         []*ec2.Tag{{Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        lo.ToPtr(ec2.ImageStateAvailable),
				},
			}})
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{"tag-key-1": "tag-value-1"},
				},
				{
					Tags: map[string]string{"tag-key-2": "tag-value-2"},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			amis, err = awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())

			Expect(amis).To(ContainElements(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Name": Equal("ami-name-1"),
				}),
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Name": Equal("ami-name-2"),
				}),
			))

			cacheItems := awsEnv.AMICache.Items()
			Expect(cacheItems).To(HaveLen(2))
			cachedImages := make([]amifamily.AMIs, 0, len(cacheItems))
			for _, item := range cacheItems {
				cachedImages = append(cachedImages, item.Object.(amifamily.AMIs))
			}

			Expect(cachedImages).To(ConsistOf(
				ConsistOf(
					gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("ami-name-3"),
					}),
				),
				ConsistOf(
					gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("ami-name-1"),
					}),
					gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("ami-name-2"),
					}),
				),
			))
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
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("tag:Name"),
							Values: aws.StringSlice([]string{"my-ami"}),
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
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("name"),
							Values: aws.StringSlice([]string{"my-ami"}),
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
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("image-id"),
							Values: aws.StringSlice([]string{"ami-abcd1234", "ami-cafeaced"}),
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
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("name"),
							Values: aws.StringSlice([]string{"my-name"}),
						},
					},
				},
				{
					Owners: []string{"self"},
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("name"),
							Values: aws.StringSlice([]string{"my-name"}),
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
	})
})

func ExpectConsistsOfAMIQueries(expected, actual []amifamily.DescribeImageQuery) {
	GinkgoHelper()
	Expect(actual).To(HaveLen(len(expected)))

	for _, list := range [][]amifamily.DescribeImageQuery{expected, actual} {
		for _, elem := range list {
			for _, f := range elem.Filters {
				sort.Slice(f.Values, func(i, j int) bool {
					return lo.FromPtr(f.Values[i]) < lo.FromPtr(f.Values[j])
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
