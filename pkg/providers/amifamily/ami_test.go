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
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	. "knative.dev/pkg/logging/testing"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	"github.com/aws/karpenter-core/pkg/scheduling"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var nodeTemplate *v1alpha1.AWSNodeTemplate

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AMISelector")
}

const (
	amd64AMIName       = "amd64-ami"
	arm64AMIName       = "arm64-ami"
	amd64NvidiaAMIName = "amd64-nvidia-ami"
	arm64NvidiaAMIName = "arm64-nvidia-ami"
)

var (
	defaultOwners = []*string{aws.String("self"), aws.String("amazon")}
)

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = BeforeEach(func() {
	// Set up the DescribeImages API so that we can call it by ID with the mock parameters that we generate
	awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
		Images: []*ec2.Image{
			{
				Name:         aws.String(amd64AMIName),
				ImageId:      aws.String("amd64-ami-id"),
				CreationDate: aws.String(time.Now().Format(time.RFC3339)),
				Architecture: aws.String("x86_64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64AMIName)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(arm64AMIName),
				ImageId:      aws.String("arm64-ami-id"),
				CreationDate: aws.String(time.Now().Add(time.Minute).Format(time.RFC3339)),
				Architecture: aws.String("arm64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(arm64AMIName)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(amd64NvidiaAMIName),
				ImageId:      aws.String("amd64-nvidia-ami-id"),
				CreationDate: aws.String(time.Now().Add(2 * time.Minute).Format(time.RFC3339)),
				Architecture: aws.String("x86_64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(amd64NvidiaAMIName)},
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			},
			{
				Name:         aws.String(arm64NvidiaAMIName),
				ImageId:      aws.String("arm64-nvidia-ami-id"),
				CreationDate: aws.String(time.Now().Add(2 * time.Minute).Format(time.RFC3339)),
				Architecture: aws.String("arm64"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String(arm64NvidiaAMIName)},
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

var _ = Describe("AMI Provider", func() {
	var version string
	BeforeEach(func() {
		version = lo.Must(awsEnv.AMIProvider.KubeServerVersion(ctx))
	})
	It("should succeed to resolve AMIs (AL2)", func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily: &v1alpha1.AMIFamilyAL2,
			},
		})
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):       amd64AMIName,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version):   amd64NvidiaAMIName,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version): arm64AMIName,
		}
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeTemplate, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(4))
	})
	It("should succeed to resolve AMIs (Bottlerocket)", func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily: &v1alpha1.AMIFamilyBottlerocket,
			},
		})
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", version):        amd64AMIName,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", version): amd64NvidiaAMIName,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", version):         arm64AMIName,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/arm64/latest/image_id", version):  arm64NvidiaAMIName,
		}
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeTemplate, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(6))
	})
	It("should succeed to resolve AMIs (Ubuntu)", func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily: &v1alpha1.AMIFamilyUbuntu,
			},
		})
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/amd64/hvm/ebs-gp2/ami-id", version): amd64AMIName,
			fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/arm64/hvm/ebs-gp2/ami-id", version): arm64AMIName,
		}
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeTemplate, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(2))
	})
	It("should succeed to resolve AMIs (Custom)", func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily: &v1alpha1.AMIFamilyCustom,
			},
		})
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeTemplate, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(0))
	})
	It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Al2)", func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily: &v1alpha1.AMIFamilyAL2,
			},
		})
		// No GPU AMI exists here
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):       amd64AMIName,
			fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version): arm64AMIName,
		}
		// Only 2 of the requirements sets for the SSM aliases will resolve
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeTemplate, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(2))
	})
	It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Bottlerocket)", func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily: &v1alpha1.AMIFamilyBottlerocket,
			},
		})
		// No GPU AMI exists for AM64 here
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/latest/image_id", version):        amd64AMIName,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/latest/image_id", version): amd64NvidiaAMIName,
			fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/latest/image_id", version):         arm64AMIName,
		}
		// Only 4 of the requirements sets for the SSM aliases will resolve
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeTemplate, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(4))
	})
	It("should succeed to partially resolve AMIs if all SSM aliases don't exist (Ubuntu)", func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily: &v1alpha1.AMIFamilyUbuntu,
			},
		})
		// No AMD64 AMI exists here
		awsEnv.SSMAPI.Parameters = map[string]string{
			fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/arm64/hvm/ebs-gp2/ami-id", version): arm64AMIName,
		}
		// Only 1 of the requirements sets for the SSM aliases will resolve
		amis, err := awsEnv.AMIProvider.Get(ctx, nodeTemplate, &amifamily.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
	})
	Context("AMI Selectors", func() {
		It("should have default owners and use tags when prefixes aren't set", func() {
			amiSelector := map[string]string{
				"Name": "my-ami",
			}
			filters, owners := amifamily.getFiltersAndOwners(amiSelector)
			Expect(owners).Should(ConsistOf(defaultOwners))
			Expect(filters).Should(ConsistOf([]*ec2.Filter{
				{
					Name:   aws.String("tag:Name"),
					Values: aws.StringSlice([]string{"my-ami"}),
				},
			}))
		})
		It("should have default owners and use name when prefixed", func() {
			amiSelector := map[string]string{
				"aws::name": "my-ami",
			}
			filters, owners := amifamily.getFiltersAndOwners(amiSelector)
			Expect(owners).Should(ConsistOf(defaultOwners))
			Expect(filters).Should(ConsistOf([]*ec2.Filter{
				{
					Name:   aws.String("name"),
					Values: aws.StringSlice([]string{"my-ami"}),
				},
			}))
		})
		It("should not set owners when legacy ids are passed", func() {
			amiSelector := map[string]string{
				"aws-ids": "ami-abcd1234,ami-cafeaced",
			}
			filters, owners := amifamily.getFiltersAndOwners(amiSelector)
			Expect(owners).Should(BeNil())
			Expect(filters).Should(ConsistOf([]*ec2.Filter{
				{
					Name: aws.String("image-id"),
					Values: aws.StringSlice([]string{
						"ami-abcd1234",
						"ami-cafeaced",
					}),
				},
			}))
		})
		It("should not set owners when prefixed ids are passed", func() {
			amiSelector := map[string]string{
				"aws::ids": "ami-abcd1234,ami-cafeaced",
			}
			filters, owners := amifamily.getFiltersAndOwners(amiSelector)
			Expect(owners).Should(BeNil())
			Expect(filters).Should(ConsistOf([]*ec2.Filter{
				{
					Name: aws.String("image-id"),
					Values: aws.StringSlice([]string{
						"ami-abcd1234",
						"ami-cafeaced",
					}),
				},
			}))
		})
		It("should allow only specifying owners", func() {
			amiSelector := map[string]string{
				"aws::owners": "abcdef,123456789012",
			}
			_, owners := amifamily.getFiltersAndOwners(amiSelector)
			Expect(owners).Should(ConsistOf(
				[]*string{aws.String("abcdef"), aws.String("123456789012")},
			))
		})
		It("should allow prefixed id, prefixed name, and prefixed owners", func() {
			amiSelector := map[string]string{
				"aws::name":   "my-ami",
				"aws::ids":    "ami-abcd1234,ami-cafeaced",
				"aws::owners": "self,amazon",
			}
			filters, owners := amifamily.getFiltersAndOwners(amiSelector)
			Expect(owners).Should(ConsistOf(defaultOwners))
			Expect(filters).Should(ConsistOf([]*ec2.Filter{
				{
					Name:   aws.String("name"),
					Values: aws.StringSlice([]string{"my-ami"}),
				},
				{
					Name: aws.String("image-id"),
					Values: aws.StringSlice([]string{
						"ami-abcd1234",
						"ami-cafeaced",
					}),
				},
			}))
		})
		It("should allow prefixed name and prefixed owners", func() {
			amiSelector := map[string]string{
				"aws::name":   "my-ami",
				"aws::owners": "0123456789,self",
			}
			filters, owners := amifamily.getFiltersAndOwners(amiSelector)
			Expect(owners).Should(ConsistOf([]*string{
				aws.String("0123456789"),
				aws.String("self"),
			}))
			Expect(filters).Should(ConsistOf([]*ec2.Filter{
				{
					Name:   aws.String("name"),
					Values: aws.StringSlice([]string{"my-ami"}),
				},
			}))
		})
		It("should sort amis by creationDate", func() {
			amis := []amifamily.AMI{
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
			amifamily.SortAMIsByCreationDate(amis)
			Expect(amis).To(Equal(
				[]amifamily.AMI{
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
