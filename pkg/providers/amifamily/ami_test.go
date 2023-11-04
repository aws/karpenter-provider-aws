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
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	coreoptions "github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	"github.com/aws/karpenter-core/pkg/scheduling"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass *v1beta1.EC2NodeClass

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
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx = settings.ToContext(ctx, test.Settings())
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = BeforeEach(func() {
	// Set up the DescribeImages API so that we can call it by ID with the mock parameters that we generate
	awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
		Images: []*ec2.Image{
			{
				Name:         aws.String(amd64AMI),
				ImageId:      aws.String("amd64-ami-id"),
				CreationDate: aws.String(time.Now().Format(time.RFC3339)),
				Architecture: aws.String("x86_64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64AMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(arm64AMI),
				ImageId:      aws.String("arm64-ami-id"),
				CreationDate: aws.String(time.Now().Add(time.Minute).Format(time.RFC3339)),
				Architecture: aws.String("arm64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(arm64AMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(amd64NvidiaAMI),
				ImageId:      aws.String("amd64-nvidia-ami-id"),
				CreationDate: aws.String(time.Now().Add(2 * time.Minute).Format(time.RFC3339)),
				Architecture: aws.String("x86_64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64NvidiaAMI)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(arm64NvidiaAMI),
				ImageId:      aws.String("arm64-nvidia-ami-id"),
				CreationDate: aws.String(time.Now().Add(2 * time.Minute).Format(time.RFC3339)),
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
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):       amd64AMI,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version):   amd64NvidiaAMI,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version): arm64AMI,
		}
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(4))
	})
	It("should succeed to resolve AMIs (Bottlerocket)", func() {
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", version):        amd64AMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", version): amd64NvidiaAMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", version):         arm64AMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/arm64/latest/image_id", version):  arm64NvidiaAMI,
		}
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(6))
	})
	It("should succeed to resolve AMIs (Ubuntu)", func() {
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyUbuntu
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/amd64/hvm/ebs-gp2/ami-id", version): amd64AMI,
			fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/arm64/hvm/ebs-gp2/ami-id", version): arm64AMI,
		}
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(2))
	})
	It("should succeed to resolve AMIs (Windows2019)", func() {
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyWindows2019
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2019-English-Core-EKS_Optimized-%s/image_id", version): amd64AMI,
		}
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
	})
	It("should succeed to resolve AMIs (Windows2022)", func() {
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyWindows2022
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2022-English-Core-EKS_Optimized-%s/image_id", version): amd64AMI,
		}
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
	})
	It("should succeed to resolve AMIs (Custom)", func() {
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(0))
	})
	Context("SSM Alias Missing", func() {
		It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Al2)", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
			// No GPU AMI exists here
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):       amd64AMI,
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version): arm64AMI,
			}
			// Only 2 of the requirements sets for the SSM aliases will resolve
			amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(2))
		})
		It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Bottlerocket)", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
			// No GPU AMI exists for AM64 here
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", version):        amd64AMI,
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", version): amd64NvidiaAMI,
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", version):         arm64AMI,
			}
			// Only 4 of the requirements sets for the SSM aliases will resolve
			amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(4))
		})
		It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Ubuntu)", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyUbuntu
			// No AMD64 AMI exists here
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/arm64/hvm/ebs-gp2/ami-id", version): arm64AMI,
			}
			// Only 1 of the requirements sets for the SSM aliases will resolve
			amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
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
					{Key: aws.String(v1.LabelInstanceTypeStable), Value: aws.String("m5.large")},
					{Key: aws.String(v1.LabelTopologyZone), Value: aws.String("test-zone-1a")},
				},
			}
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					img,
				},
			})
		})
		It("should succeed to resolve tags as requirements for NodeTemplates", func() {
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{"*": "*"},
				},
			}
			nodeClass.IsNodeTemplate = true
			amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis).To(ConsistOf(amifamily.AMI{
				Name:         aws.StringValue(img.Name),
				AmiID:        aws.StringValue(img.ImageId),
				CreationDate: aws.StringValue(img.CreationDate),
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, v1alpha5.ArchitectureAmd64),
					scheduling.NewRequirement(v1.LabelInstanceTypeStable, v1.NodeSelectorOpIn, "m5.large"),
					scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, "test-zone-1a"),
				),
			}))
		})
		It("should succeed to not resolve tags as requirements for NodeClasses", func() {
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{"*": "*"},
				},
			}
			amis, err := awsEnv.AMIProvider.Get(ctx, nodeClass, &amifamily.Options{})
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis).To(ConsistOf(amifamily.AMI{
				Name:         aws.StringValue(img.Name),
				AmiID:        aws.StringValue(img.ImageId),
				CreationDate: aws.StringValue(img.CreationDate),
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, v1alpha5.ArchitectureAmd64),
				),
			}))
		})
	})
	Context("AMI Selectors", func() {
		// When you tag public or shared resources, the tags you assign are available only to your AWS account; no other AWS account will have access to those tags
		// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html#tag-restrictions
		It("should have empty owners and use tags when prefixes aren't set", func() {
			amiSelectorTerms := []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"Name": "my-ami",
					},
				},
			}
			filterAndOwnersSets := amifamily.GetFilterAndOwnerSets(amiSelectorTerms)
			ExpectConsistsOfFiltersAndOwners([]amifamily.FiltersAndOwners{
				{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("tag:Name"),
							Values: aws.StringSlice([]string{"my-ami"}),
						},
					},
					Owners: []string{},
				},
			}, filterAndOwnersSets)
		})
		It("should have default owners and use name when prefixed", func() {
			amiSelectorTerms := []v1beta1.AMISelectorTerm{
				{
					Name: "my-ami",
				},
			}
			filterAndOwnersSets := amifamily.GetFilterAndOwnerSets(amiSelectorTerms)
			ExpectConsistsOfFiltersAndOwners([]amifamily.FiltersAndOwners{
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
			}, filterAndOwnersSets)
		})
		It("should not set owners when legacy ids are passed", func() {
			amiSelectorTerms := []v1beta1.AMISelectorTerm{
				{
					ID: "ami-abcd1234",
				},
				{
					ID: "ami-cafeaced",
				},
			}
			filterAndOwnersSets := amifamily.GetFilterAndOwnerSets(amiSelectorTerms)
			ExpectConsistsOfFiltersAndOwners([]amifamily.FiltersAndOwners{
				{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("image-id"),
							Values: aws.StringSlice([]string{"ami-abcd1234", "ami-cafeaced"}),
						},
					},
				},
			}, filterAndOwnersSets)
		})
		It("should allow only specifying owners", func() {
			amiSelectorTerms := []v1beta1.AMISelectorTerm{
				{
					Owner: "abcdef",
				},
				{
					Owner: "123456789012",
				},
			}
			filterAndOwnersSets := amifamily.GetFilterAndOwnerSets(amiSelectorTerms)
			ExpectConsistsOfFiltersAndOwners([]amifamily.FiltersAndOwners{
				{
					Owners: []string{"abcdef"},
				},
				{
					Owners: []string{"123456789012"},
				},
			}, filterAndOwnersSets)
		})
		It("should allow prefixed name and prefixed owners", func() {
			amiSelectorTerms := []v1beta1.AMISelectorTerm{
				{
					Name:  "my-name",
					Owner: "0123456789",
				},
				{
					Name:  "my-name",
					Owner: "self",
				},
			}
			filterAndOwnersSets := amifamily.GetFilterAndOwnerSets(amiSelectorTerms)
			ExpectConsistsOfFiltersAndOwners([]amifamily.FiltersAndOwners{
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
			}, filterAndOwnersSets)
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
	})
})

func ExpectConsistsOfFiltersAndOwners(expected, actual []amifamily.FiltersAndOwners) {
	GinkgoHelper()
	Expect(actual).To(HaveLen(len(expected)))

	for _, list := range [][]amifamily.FiltersAndOwners{expected, actual} {
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
	Expect(actual).To(ConsistOf(lo.Map(expected, func(f amifamily.FiltersAndOwners, _ int) interface{} { return f })...))
}
