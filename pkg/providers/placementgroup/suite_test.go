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

package placementgroup_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"

	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass *v1beta1.EC2NodeClass

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	nodeClass = test.EC2NodeClass(v1beta1.EC2NodeClass{
		Spec: v1beta1.EC2NodeClassSpec{
			AMIFamily: aws.String(v1beta1.AMIFamilyAL2),
			SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"*": "*",
					},
				},
			},
			SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"*": "*",
					},
				},
			},
		},
	})
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("PlacementGroupProvider", func() {
	Context("Get", func() {
		It("should discover placement groups by name", func() {
			expected := &ec2.PlacementGroup{GroupArn: aws.String("test-group-arn"), GroupName: aws.String("test-group-name")}
			awsEnv.EC2API.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{PlacementGroups: []*ec2.PlacementGroup{expected}})
			nodeClass.Spec.PlacementGroupSelectorTerms = []v1beta1.PlacementGroupSelectorTerm{
				{
					Name: *expected.GroupName,
				},
			}
			pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(pg).To(Equal(expected))
		})
		It("should filter only matches", func() {
			expected := &ec2.PlacementGroup{GroupArn: aws.String("test-group-arn"), GroupName: aws.String("test-group-name")}
			awsEnv.EC2API.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{PlacementGroups: []*ec2.PlacementGroup{expected}})
			nodeClass.Spec.PlacementGroupSelectorTerms = []v1beta1.PlacementGroupSelectorTerm{
				{
					Name: "does-not-match",
				},
			}
			pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(pg).To(BeNil())
		})
	})
})
