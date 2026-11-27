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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/version"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	"github.com/awslabs/operatorpkg/object"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	corecloudfake "sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool
var nodeClaim *karpv1.NodeClaim
var instanceTypes []*corecloudprovider.InstanceType

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AMISelector")
}

const (
	amd64AMI       = "ami-id-amd64"
	arm64AMI       = "ami-id-arm64"
	amd64NvidiaAMI = "ami-id-amd64-nvidia"
	arm64NvidiaAMI = "ami-id-arm64-nvidia"
)

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = BeforeEach(func() {
	// Set up the DescribeImages API so that we can call it by ID with the mock parameters that we generate
	awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
		Images: []ec2types.Image{
			{
				Name:         aws.String(amd64AMI),
				ImageId:      aws.String("ami-id-amd64"),
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
				ImageId:      aws.String("ami-id-arm64"),
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
				ImageId:      aws.String("ami-id-amd64-nvidia"),
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
				ImageId:      aws.String("ami-id-arm64-nvidia"),
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

type MockVersionProvider struct {
	version string
}

func (m *MockVersionProvider) Get(ctx context.Context) string {
	return m.version
}

func amiProviderWithEKSVersionOverride(version string) *amifamily.DefaultProvider {
	mockVersionProvider := &MockVersionProvider{version: version}
	return amifamily.NewDefaultProvider(awsEnv.Clock, mockVersionProvider, awsEnv.SSMProvider, awsEnv.EC2API, awsEnv.EC2Cache)
}

var _ = Describe("AMIProvider", func() {
	var k8sVersion string
	BeforeEach(func() {
		k8sVersion = awsEnv.VersionProvider.Get(ctx)
		nodeClass = test.EC2NodeClass()
	})
	DescribeTable(
		"should fail when AL2 is used with Kubernetes version 1.33 or greater",
		func(k8sVersion string, amiAlias string, expectError bool) {
			amiProvider := amiProviderWithEKSVersionOverride(k8sVersion)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: amiAlias}}
			_, err := amiProvider.DescribeImageQueries(ctx, nodeClass)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		},
		Entry("should fail for AL2 on 1.33.0", "1.33.0", "al2@latest", true),
		Entry("should fail for AL2 on 1.34.2", "1.34.2", "al2@latest", true),
		Entry("should succeed for AL2 on 1.32.0", "1.32.0", "al2@latest", false),
		Entry("should succeed for AL2023 on 1.33.0", "1.33.0", "al2023@latest", false),
	)

	It("should succeed to resolve AMIs (AL2)", func() {
		if version.MustParseGeneric(k8sVersion).Minor() > 32 {
			Skip("AL2 is not supported on versions > 1.32")
		}
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", k8sVersion):       amd64AMI,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", k8sVersion):   amd64NvidiaAMI,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", k8sVersion): arm64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(4))
	})
	It("should succeed to resolve AMIs (AL2023)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", k8sVersion): amd64AMI,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/arm64/standard/recommended/image_id", k8sVersion):  arm64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(2))
	})
	It("should succeed to resolve AMIs (Bottlerocket)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", k8sVersion):        amd64AMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", k8sVersion): amd64NvidiaAMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", k8sVersion):         arm64AMI,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/arm64/latest/image_id", k8sVersion):  arm64NvidiaAMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(5))
	})
	It("should succeed to resolve AMIs (Windows2019)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2019@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2019-English-Core-EKS_Optimized-%s/image_id", k8sVersion): amd64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
	})
	It("should succeed to resolve AMIs (Windows2022)", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2022-English-Core-EKS_Optimized-%s/image_id", k8sVersion): amd64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
	})
	It("should succeed to resolve AMIs (Windows2025)", func() {
		// Check if EKS version supports Windows 2025 (requires 1.35+)
		minorVersion, err := strconv.Atoi(strings.Split(k8sVersion, ".")[1])
		if err != nil || minorVersion < 35 {
			Skip("Windows 2025 requires EKS 1.35+, current version: " + k8sVersion)
		}

		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2025@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2025-English-Core-EKS_Optimized-%s/image_id", k8sVersion): amd64AMI,
		}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
	})
	It("should fail to resolve AMIs for Windows2025 when EKS version is below 1.35", func() {
		amiProvider := amiProviderWithEKSVersionOverride("1.34.0")
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2025@latest"}}
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2025-English-Core-EKS_Optimized-%s/image_id", "1.34.0"): amd64AMI,
		}
		_, err := amiProvider.DescribeImageQueries(ctx, nodeClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Windows Server 2025 requires EKS version 1.35 or higher"))
	})

	It("should not cause data races when calling Get() simultaneously", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
			{
				ID: "ami-id-amd64",
			},
			{
				ID: "ami-id-arm64",
			},
		}
		wg := sync.WaitGroup{}
		for range 10000 {
			wg.Go(func() {
				defer GinkgoRecover()
				images, err := awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())

				Expect(images).To(HaveLen(2))
				// Sort everything in parallel and ensure that we don't get data races
				images.Sort()
				Expect(images).To(BeEquivalentTo([]amifamily.AMI{
					{
						Name:         arm64AMI,
						AmiID:        "ami-id-arm64",
						CreationDate: time.Time{}.Add(time.Minute).Format(time.RFC3339),
						Requirements: scheduling.NewLabelRequirements(map[string]string{
							corev1.LabelArchStable: karpv1.ArchitectureArm64,
						}),
					},
					{
						Name:         amd64AMI,
						AmiID:        "ami-id-amd64",
						CreationDate: time.Time{}.Format(time.RFC3339),
						Requirements: scheduling.NewLabelRequirements(map[string]string{
							corev1.LabelArchStable: karpv1.ArchitectureAmd64,
						}),
					},
				}))
			})
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
			if version.MustParseGeneric(k8sVersion).Minor() > 32 {
				Skip("AL2 is not supported on versions > 1.32")
			}
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			// No GPU AMI exists here
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", k8sVersion):       amd64AMI,
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", k8sVersion): arm64AMI,
			}
			// Only 2 of the requirements sets for the SSM aliases will resolve
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(2))
		})
		It("should succeed to partially resolve AMIs if all SSM aliases don't exist (AL2023)", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", k8sVersion): amd64AMI,
			}
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
		})
		It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Bottlerocket)", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			// No GPU AMI exists for AM64 here
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", k8sVersion):        amd64AMI,
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", k8sVersion): amd64NvidiaAMI,
				fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", k8sVersion):         arm64AMI,
			}
			// Only 4 of the requirements sets for the SSM aliases will resolve
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(4))
		})
	})
	Context("AMI Tag Requirements", func() {
		var img ec2types.Image
		BeforeEach(func() {
			img = ec2types.Image{
				Name:         aws.String(amd64AMI),
				ImageId:      aws.String("ami-id-amd64"),
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
		It("should prioritize the older non-deprecated ami without deprecation time", func() {
			// Here we have two AMIs one which is deprecated and newer and one which is older and non-deprecated without a deprecation time
			// List operation will prioritize the non-deprecated AMI without the deprecation time
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
		It("should prioritize the non-deprecated ami with deprecation time when both have same creation time", func() {
			// Here we have two AMIs one which is deprecated and one which is non-deprecated both with the same creation time
			// List operation will prioritize the non-deprecated AMI
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
		It("should prioritize the non-deprecated ami with deprecation time when both have same creation time and different name", func() {
			// Here we have two AMIs one which is deprecated and one which is non-deprecated both with the same creation time but with different names
			// List operation will prioritize the non-deprecated AMI
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
		It("should prioritize the newer ami if both are deprecated", func() {
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
	Context("Provider Cache", func() {
		DescribeTable("should resolve AMIs from cache that are filtered by",
			func(terms []v1.AMISelectorTerm, wantEntries int) {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
					{
						Name:         aws.String("ami-name-1"),
						ImageId:      aws.String("ami-123"),
						Architecture: "x86_64",
						Tags:         []ec2types.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
						State:        ec2types.ImageStateAvailable,
					},
					{
						Name:         aws.String("ami-name-2"),
						ImageId:      aws.String("ami-456"),
						Architecture: "arm64",
						Tags:         []ec2types.Tag{{Key: lo.ToPtr("test"), Value: lo.ToPtr("test")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
						State:        ec2types.ImageStateAvailable,
					},
				}})
				nodeClass.Spec.AMISelectorTerms = terms
				_, err := awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())

				// One entry per DescribeImages request
				Expect(awsEnv.AMICache.Items()).To(HaveLen(wantEntries))
				cachedImages := []ec2types.Image{}
				for _, item := range awsEnv.AMICache.Items() {
					cachedImages = append(cachedImages, item.Object.([]ec2types.Image)...)
				}
				Expect(cachedImages).To(ContainElements(
					gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name":    gstruct.PointTo(Equal("ami-name-1")),
						"ImageId": gstruct.PointTo(Equal("ami-123")),
					}),
					gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name":    gstruct.PointTo(Equal("ami-name-2")),
						"ImageId": gstruct.PointTo(Equal("ami-456")),
					}),
				))
			},
			// ID terms are batched into one image-id filter
			Entry("id", []v1.AMISelectorTerm{{ID: "ami-123"}, {ID: "ami-456"}}, 1),
			// Each name term is a distinct query, cached independently
			Entry("name", []v1.AMISelectorTerm{{Name: "ami-name-1"}, {Name: "ami-name-2"}}, 2),
			Entry("tags", []v1.AMISelectorTerm{{Tags: map[string]string{"test": "test"}}}, 1),
		)
		It("should correctly disambiguate AND vs OR semantics for tags", func() {
			// AND semantics
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
				{
					Name:         aws.String("ami-name-3"),
					ImageId:      aws.String("ami-789"),
					Architecture: "x86_64",
					Tags:         []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}, {Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        ec2types.ImageStateAvailable,
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
			awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
				{
					Name:         aws.String("ami-name-1"),
					ImageId:      aws.String("ami-123"),
					Architecture: "x86_64",
					Tags:         []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        ec2types.ImageStateAvailable,
				},
				{
					Name:         aws.String("ami-name-2"),
					ImageId:      aws.String("ami-456"),
					Architecture: "arm64",
					Tags:         []ec2types.Tag{{Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
					State:        ec2types.ImageStateAvailable,
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
			// One entry per DescribeImages request: one for the AND term, one for each OR term
			Expect(cacheItems).To(HaveLen(3))
			cachedImages := make([][]ec2types.Image, 0, len(cacheItems))
			for _, item := range cacheItems {
				cachedImages = append(cachedImages, item.Object.([]ec2types.Image))
			}

			Expect(cachedImages).To(ConsistOf(
				ConsistOf(
					gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": gstruct.PointTo(Equal("ami-name-3")),
					}),
				),
				ConsistOf(
					gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": gstruct.PointTo(Equal("ami-name-1")),
					}),
				),
				ConsistOf(
					gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": gstruct.PointTo(Equal("ami-name-2")),
					}),
				),
			))
		})
		Context("Cross-NodeClass sharing", func() {
			amiIDs := func(amis amifamily.AMIs) []string {
				return lo.Map(amis, func(a amifamily.AMI, _ int) string { return a.AmiID })
			}
			image := func(id, arch, deprecationTime string) ec2types.Image {
				return ec2types.Image{
					Name: aws.String(id), ImageId: aws.String(id), Architecture: ec2types.ArchitectureValues(arch),
					CreationDate: aws.String("2022-01-01T00:00:00Z"), DeprecationTime: lo.EmptyableToPtr(deprecationTime), State: ec2types.ImageStateAvailable,
					Tags: []ec2types.Tag{{Key: aws.String("foo"), Value: aws.String("bar")}},
				}
			}
			It("should share a cache entry across NodeClasses with the same selector term", func() {
				// Distinct UIDs and Generations, so a per-NodeClass key would not share
				nc1 := test.EC2NodeClass()
				nc1.UID, nc1.Generation = "uid-1", 1
				nc1.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
				nc2 := test.EC2NodeClass()
				nc2.UID, nc2.Generation = "uid-2", 3
				nc2.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
				_, err := awsEnv.AMIProvider.List(ctx, nc1)
				Expect(err).ToNot(HaveOccurred())
				_, err = awsEnv.AMIProvider.List(ctx, nc2)
				Expect(err).ToNot(HaveOccurred())
				Expect(awsEnv.AMICache.Items()).To(HaveLen(1))
				Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
			})
			It("should not share a cache entry across NodeClasses with different owners, and pass each owner to DescribeImages", func() {
				// Sharing here would surface an AMI from an account the second NodeClass never authorized
				nc1 := test.EC2NodeClass()
				nc1.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI, Owner: "111111111111"}}
				nc2 := test.EC2NodeClass()
				nc2.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI, Owner: "222222222222"}}
				_, err := awsEnv.AMIProvider.List(ctx, nc1)
				Expect(err).ToNot(HaveOccurred())
				_, err = awsEnv.AMIProvider.List(ctx, nc2)
				Expect(err).ToNot(HaveOccurred())
				Expect(awsEnv.AMICache.Items()).To(HaveLen(2))
				owners := []string{}
				for awsEnv.EC2API.CalledWithDescribeImagesInput.Len() > 0 {
					owners = append(owners, awsEnv.EC2API.CalledWithDescribeImagesInput.Pop().Owners...)
				}
				Expect(owners).To(ConsistOf("111111111111", "222222222222"))
			})
			It("should share a cache entry between an ID term and an SSM parameter term resolving to the same AMI", func() {
				awsEnv.SSMAPI.Parameters = map[string]string{"/custom/ami": "ami-id-amd64"}
				idNC := test.EC2NodeClass()
				idNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-id-amd64"}}
				ssmNC := test.EC2NodeClass()
				ssmNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{SSMParameter: "/custom/ami"}}
				_, err := awsEnv.AMIProvider.List(ctx, idNC)
				Expect(err).ToNot(HaveOccurred())
				amis, err := awsEnv.AMIProvider.List(ctx, ssmNC)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64"))
				Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
			})
			// ID terms are batched into one image-id filter, so partially overlapping ID selectors issue
			// distinct requests and do not share entries (unlike name/tag terms, which are one query each)
			It("should not share a cache entry between ID selectors that only partially overlap", func() {
				nc1 := test.EC2NodeClass()
				nc1.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-id-amd64"}}
				nc2 := test.EC2NodeClass()
				nc2.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-id-amd64"}, {ID: "ami-id-arm64"}}
				_, err := awsEnv.AMIProvider.List(ctx, nc1)
				Expect(err).ToNot(HaveOccurred())
				amis, err := awsEnv.AMIProvider.List(ctx, nc2)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64", "ami-id-arm64"))
				Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(2))
				Expect(awsEnv.AMICache.Items()).To(HaveLen(2))
			})
			Context("Known requirements", func() {
				var k8sVersion string
				windowsParam := func(version string) string {
					return fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-%s-English-Core-EKS_Optimized-%s/image_id", version, k8sVersion)
				}
				BeforeEach(func() {
					k8sVersion = awsEnv.VersionProvider.Get(ctx)
				})
				It("should share a cache entry between an alias and an ID term for the same AMI while each derives its own requirements", func() {
					awsEnv.SSMAPI.Parameters = map[string]string{windowsParam("2022"): amd64AMI}
					aliasNC := test.EC2NodeClass()
					aliasNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
					idNC := test.EC2NodeClass()
					idNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amd64AMI}}
					// ID, alias, ID: neither order lets one NodeClass's requirements leak into the other's
					for _, nc := range []*v1.EC2NodeClass{idNC, aliasNC, idNC} {
						amis, err := awsEnv.AMIProvider.List(ctx, nc)
						Expect(err).ToNot(HaveOccurred())
						Expect(amis).To(HaveLen(1))
						if nc == aliasNC {
							Expect(amis[0].Requirements.Get(corev1.LabelOSStable).Has(string(corev1.Windows))).To(BeTrue())
							Expect(amis[0].Requirements.Get(corev1.LabelWindowsBuild).Has(v1.Windows2022Build)).To(BeTrue())
						} else {
							Expect(amis[0].Requirements.Has(corev1.LabelOSStable)).To(BeFalse())
						}
					}
					Expect(awsEnv.AMICache.Items()).To(HaveLen(1))
					Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
				})
				It("should resolve each alias's own requirements when two aliases resolve the same AMI", func() {
					// Both aliases send the same request and differ only in the values of their known requirements
					awsEnv.SSMAPI.Parameters = map[string]string{windowsParam("2022"): amd64AMI, windowsParam("2019"): amd64AMI}
					for alias, build := range map[string]string{"windows2022@latest": v1.Windows2022Build, "windows2019@latest": v1.Windows2019Build} {
						nc := test.EC2NodeClass()
						nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
						amis, err := awsEnv.AMIProvider.List(ctx, nc)
						Expect(err).ToNot(HaveOccurred())
						Expect(amis).To(HaveLen(1))
						Expect(amis[0].Requirements.Get(corev1.LabelWindowsBuild).Values()).To(ConsistOf(build), alias)
					}
					Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
				})
			})
			It("should evaluate deprecation against the current time on a cache hit, inclusive of the deprecation time", func() {
				now := time.Now().Truncate(time.Second)
				awsEnv.Clock.SetTime(now)
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
					image(amd64AMI, "x86_64", now.Add(time.Hour).Format(time.RFC3339)),
				}})
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amd64AMI}}
				amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(amis).To(HaveLen(1))
				Expect(amis[0].Deprecated).To(BeFalse())

				// The deprecation time arrives while the result is still cached
				awsEnv.Clock.Step(time.Hour)
				amis, err = awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(amis).To(HaveLen(1))
				Expect(amis[0].Deprecated).To(BeTrue())
				Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
			})
			It("should bypass the cache with SkipCache and write the fresh result through", func() {
				awsEnv.Clock.SetTime(time.Now())
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{image(amd64AMI, "x86_64", "")}})
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amd64AMI}}
				amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(amis[0].Deprecated).To(BeFalse())

				// The AMI is now deprecated at the API, but a cached List still returns the stale result
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
					image(amd64AMI, "x86_64", awsEnv.Clock.Now().Add(-time.Hour).Format(time.RFC3339)),
				}})
				amis, err = awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(amis[0].Deprecated).To(BeFalse())

				amis, err = awsEnv.AMIProvider.List(ctx, nodeClass, amifamily.SkipCache)
				Expect(err).ToNot(HaveOccurred())
				Expect(amis[0].Deprecated).To(BeTrue())

				// The fresh result replaced the cached one
				calls := awsEnv.EC2API.CalledWithDescribeImagesInput.Len()
				amis, err = awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(amis[0].Deprecated).To(BeTrue())
				Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(calls))
			})
			It("should choose the best AMI across queries regardless of which query was cached by another NodeClass", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
					{Name: aws.String("img-old"), ImageId: aws.String("ami-old"), Architecture: "x86_64", CreationDate: aws.String("2020-01-01T00:00:00Z"), State: ec2types.ImageStateAvailable},
					{Name: aws.String("img-new"), ImageId: aws.String("ami-new"), Architecture: "x86_64", CreationDate: aws.String("2022-01-01T00:00:00Z"), State: ec2types.ImageStateAvailable},
				}})
				oldNC := test.EC2NodeClass()
				oldNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: "img-old"}}
				amis, err := awsEnv.AMIProvider.List(ctx, oldNC)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-old"))

				for _, terms := range [][]v1.AMISelectorTerm{
					{{Name: "img-old"}, {Name: "img-new"}},
					{{Name: "img-new"}, {Name: "img-old"}},
				} {
					nc := test.EC2NodeClass()
					nc.Spec.AMISelectorTerms = terms
					amis, err = awsEnv.AMIProvider.List(ctx, nc)
					Expect(err).ToNot(HaveOccurred())
					Expect(amiIDs(amis)).To(ConsistOf("ami-new"))
				}
				// oldNC is not polluted
				amis, err = awsEnv.AMIProvider.List(ctx, oldNC)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-old"))
				Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(2))
			})
			It("should cache queries resolved before a failure in the same List, and not the failed one", func() {
				failing := &failingNameEC2API{EC2API: awsEnv.EC2API, failName: "fail-me"}
				amiCache := cache.New(time.Hour, time.Hour)
				provider := amifamily.NewDefaultProvider(awsEnv.Clock, awsEnv.VersionProvider, awsEnv.SSMProvider, failing, amiCache)
				nc := test.EC2NodeClass()
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI}, {Name: "fail-me"}}
				_, err := provider.List(ctx, nc)
				Expect(err).To(HaveOccurred())
				Expect(amiCache.Items()).To(HaveLen(1))
				Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))

				// The query resolved before the failure is served from the cache
				other := test.EC2NodeClass()
				other.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI}}
				amis, err := provider.List(ctx, other)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64"))
				Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
			})
			It("should cache every page of a paginated DescribeImages response", func() {
				paged := &pagedImagesEC2API{EC2API: awsEnv.EC2API, pages: [][]ec2types.Image{
					{image("ami-page1", "x86_64", "")},
					{image("ami-page2", "arm64", "")},
				}}
				provider := amifamily.NewDefaultProvider(awsEnv.Clock, awsEnv.VersionProvider, awsEnv.SSMProvider, paged, cache.New(time.Hour, time.Hour))
				nc := test.EC2NodeClass()
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
				amis, err := provider.List(ctx, nc)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-page1", "ami-page2"))
				Expect(paged.calls).To(Equal(2))

				// Served from the cache: both pages, no further calls
				amis, err = provider.List(ctx, nc)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-page1", "ami-page2"))
				Expect(paged.calls).To(Equal(2))

				// SkipCache re-reads every page
				amis, err = provider.List(ctx, nc, amifamily.SkipCache)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-page1", "ami-page2"))
				Expect(paged.calls).To(Equal(4))
			})
			It("should drop images whose architecture Karpenter does not support", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
					image("ami-x86", "x86_64", ""),
					{Name: aws.String("img-i386"), ImageId: aws.String("ami-i386"), Architecture: "i386", CreationDate: aws.String("2023-01-01T00:00:00Z"), State: ec2types.ImageStateAvailable, Tags: []ec2types.Tag{{Key: aws.String("foo"), Value: aws.String("bar")}}},
				}})
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
				// Both the uncached and cached paths
				for range 2 {
					amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
					Expect(err).ToNot(HaveOccurred())
					Expect(amiIDs(amis)).To(ConsistOf("ami-x86"))
				}
			})
			It("should return the new selector's AMIs after a NodeClass changes its selector", func() {
				nc := test.EC2NodeClass()
				nc.UID, nc.Generation = "uid-1", 1
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI}}
				amis, err := awsEnv.AMIProvider.List(ctx, nc)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64"))
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: arm64AMI}}
				amis, err = awsEnv.AMIProvider.List(ctx, nc)
				Expect(err).ToNot(HaveOccurred())
				Expect(amiIDs(amis)).To(ConsistOf("ami-id-arm64"))
			})
		})
		Context("DescribeImagesCacheKey", func() {
			f := func(name string, values ...string) ec2types.Filter {
				return ec2types.Filter{Name: aws.String(name), Values: values}
			}
			input := func(owners []string, filters ...ec2types.Filter) *ec2.DescribeImagesInput {
				return &ec2.DescribeImagesInput{Filters: filters, Owners: owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(1000)}
			}
			a, b, state := f("tag:a", "1"), f("tag:b", "2"), f("state", "available")
			base := input([]string{"self"}, a, state)

			DescribeTable("keys requests that select the same images alike",
				func(x, y *ec2.DescribeImagesInput) {
					Expect(amifamily.DescribeImagesCacheKey(x)).To(Equal(amifamily.DescribeImagesCacheKey(y)))
				},
				// Filters built from a tag map are emitted in map iteration order
				Entry("filter order", input(nil, a, b, state), input(nil, state, b, a)),
				Entry("owner order", input([]string{"self", "amazon"}, a), input([]string{"amazon", "self"}, a)),
				Entry("duplicate owners", input([]string{"self", "amazon", "self"}, a), input([]string{"amazon", "self"}, a)),
				Entry("nil and empty owners", input(nil, a), input([]string{}, a)),
			)
			DescribeTable("keys requests that select different images differently",
				func(other *ec2.DescribeImagesInput) {
					Expect(amifamily.DescribeImagesCacheKey(base)).ToNot(Equal(amifamily.DescribeImagesCacheKey(other)))
				},
				Entry("owner", input([]string{"amazon"}, a, state)),
				Entry("additional owner", input([]string{"self", "self", "amazon", "amazon"}, a, state)),
				Entry("no owner", input(nil, a, state)),
				Entry("filter value", input([]string{"self"}, f("tag:a", "2"), state)),
				Entry("extra filter", input([]string{"self"}, a, b, state)),
				Entry("different filter", input([]string{"self"}, b, state)),
				Entry("state filter value", input([]string{"self"}, a, f("state", "pending"))),
				// Every other request field is part of the key, including ones added later
				Entry("MaxResults", &ec2.DescribeImagesInput{Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(5)}),
				Entry("IncludeDeprecated", &ec2.DescribeImagesInput{Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(false), MaxResults: aws.Int32(1000)}),
				Entry("ImageIds", &ec2.DescribeImagesInput{Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(1000), ImageIds: []string{"ami-1"}}),
			)
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
		It("should succeed to resolve AMIs that use an SSM parameter", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
				SSMParameter: fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", k8sVersion),
			}}
			awsEnv.SSMAPI.Parameters = map[string]string{
				fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", k8sVersion): amd64AMI,
			}
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis[0].AmiID).To(Equal("ami-id-amd64"))
			Expect(amis[0].Name).To(Equal(amd64AMI))
		})
		It("should succeed to resolve AMIs that use a custom SSM parameter", func() {
			customParameter := "/my/custom/ami/parameter"
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
				SSMParameter: customParameter,
			}}
			awsEnv.SSMAPI.Parameters = map[string]string{
				customParameter: amd64AMI,
			}
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(1))
			Expect(amis[0].AmiID).To(Equal("ami-id-amd64"))
			Expect(amis[0].Name).To(Equal(amd64AMI))
		})
		It("should not throw an error if SSM parameter is not found", func() {
			customParameter := "/my/custom/ami/parameter"
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
				SSMParameter: customParameter,
			}}
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amis).To(HaveLen(0))
		})
		It("should throw an error if SSM parameter returns a different error", func() {
			customParameter := "/my/custom/ami/parameter"
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
				SSMParameter: customParameter,
			}}
			awsEnv.SSMAPI.WantErr = fmt.Errorf("some error")
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).To(HaveOccurred())
			Expect(amis).To(HaveLen(0))
		})
	})
})

var _ = Describe("AMIResolver", func() {
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass()
		nodePool = coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
					},
				},
			},
		})
		nodeClaim = coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					karpv1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		instanceTypes = []*corecloudprovider.InstanceType{
			corecloudfake.NewInstanceType("t3.medium"),
			corecloudfake.NewInstanceType("m5.large"),
		}
	})
	DescribeTable(
		"should set launch template metadata options correctly per region",
		func(region string, expect *string) {
			amiResolver := amifamily.NewDefaultResolver(region, nil, awsEnv.CELEnvironment)
			launchTemplates, err := amiResolver.Resolve(ctx, nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, string(ec2types.TenancyDefault), &amifamily.Options{ClusterName: "test"}, "", 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(launchTemplates).To(HaveLen(1))
			lo.ForEach(launchTemplates, func(launchTemplate *amifamily.LaunchTemplate, _ int) {
				Expect(launchTemplate.MetadataOptions.HTTPProtocolIPv6).To(Equal(expect))
			})
		},
		Entry("should be disabled for supported regions", fake.DefaultRegion, lo.ToPtr("disabled")),
		Entry("should be nil for iso", "us-iso-east-1", nil),
		Entry("should be nil for isob", "us-isob-east-1", nil),
		Entry("should be nil for isof", "us-isof-south-1", nil),
	)
	It("should fail launch template resolution rather than default a kubelet config that won't decode", func() {
		// Unreachable in practice: the validation controller rejects a config ParseKubeletConfig can't
		// read before the NodeClass goes Ready, so Create never gets here but here just in case
		nodeClass.Spec.Kubelet = v1.KubeletConfiguration{"clusterDNS": v1.JSONValue("10.0.0.10")}
		amiResolver := amifamily.NewDefaultResolver(fake.DefaultRegion, nil, awsEnv.CELEnvironment)
		launchTemplates, err := amiResolver.Resolve(ctx, nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, string(ec2types.TenancyDefault), &amifamily.Options{ClusterName: "test"}, "", 0)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("parsing kubelet configuration"))
		Expect(launchTemplates).To(BeEmpty())
	})
	Context("EnclaveEnabled", func() {
		It("should set EnclaveEnabled to false by default when no resources are requested", func() {
			amiResolver := amifamily.NewDefaultResolver(fake.DefaultRegion, nil, awsEnv.CELEnvironment)
			launchTemplates, err := amiResolver.Resolve(ctx, nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, string(ec2types.TenancyDefault), &amifamily.Options{ClusterName: "test"}, "", 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(launchTemplates).ToNot(BeEmpty())
			lo.ForEach(launchTemplates, func(lt *amifamily.LaunchTemplate, _ int) {
				Expect(lt.EnclaveEnabled).To(BeFalse())
			})
		})
		It("should set EnclaveEnabled to true when ResourceNitroSandbox is in the NodeClaim's resource requests", func() {
			nodeClaim.Spec.Resources.Requests = corev1.ResourceList{
				v1.ResourceNitroSandbox: resource.MustParse("1"),
			}
			amiResolver := amifamily.NewDefaultResolver(fake.DefaultRegion, nil, awsEnv.CELEnvironment)
			launchTemplates, err := amiResolver.Resolve(ctx, nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, string(ec2types.TenancyDefault), &amifamily.Options{ClusterName: "test"}, "", 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(launchTemplates).ToNot(BeEmpty())
			lo.ForEach(launchTemplates, func(lt *amifamily.LaunchTemplate, _ int) {
				Expect(lt.EnclaveEnabled).To(BeTrue())
			})
		})
		It("should set EnclaveEnabled to false when only other resources are requested (not ResourceNitroSandbox)", func() {
			nodeClaim.Spec.Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			}
			amiResolver := amifamily.NewDefaultResolver(fake.DefaultRegion, nil, awsEnv.CELEnvironment)
			launchTemplates, err := amiResolver.Resolve(ctx, nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, string(ec2types.TenancyDefault), &amifamily.Options{ClusterName: "test"}, "", 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(launchTemplates).ToNot(BeEmpty())
			lo.ForEach(launchTemplates, func(lt *amifamily.LaunchTemplate, _ int) {
				Expect(lt.EnclaveEnabled).To(BeFalse())
			})
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
	Expect(actual).To(ConsistOf(lo.Map(expected, func(q amifamily.DescribeImageQuery, _ int) any { return q })...))
}

// pagedImagesEC2API serves DescribeImages as fixed pages; the shared fake never paginates images.
type pagedImagesEC2API struct {
	*fake.EC2API
	pages [][]ec2types.Image
	calls int
}

func (p *pagedImagesEC2API) DescribeImages(_ context.Context, in *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	p.calls++
	idx := 0
	if in.NextToken != nil {
		idx = lo.Must(strconv.Atoi(*in.NextToken))
	}
	out := &ec2.DescribeImagesOutput{Images: p.pages[idx]}
	if idx+1 < len(p.pages) {
		out.NextToken = aws.String(strconv.Itoa(idx + 1))
	}
	return out, nil
}

// failingNameEC2API fails DescribeImages for one name filter value and delegates every other call.
type failingNameEC2API struct {
	*fake.EC2API
	failName string
}

func (f *failingNameEC2API) DescribeImages(ctx context.Context, in *ec2.DescribeImagesInput, opts ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	for _, flt := range in.Filters {
		if aws.ToString(flt.Name) == "name" && lo.Contains(flt.Values, f.failName) {
			return nil, fmt.Errorf("throttled")
		}
	}
	return f.EC2API.DescribeImages(ctx, in, opts...)
}
