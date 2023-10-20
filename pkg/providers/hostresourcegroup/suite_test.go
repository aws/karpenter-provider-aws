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

package hostresourcegroup_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/test"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
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
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
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

var _ = Describe("HostResourceGroupProvider", func() {
	Context("Get", func() {
		It("should discover host resource groups by name", func() {
			expected := &v1beta1.HostResourceGroup{ARN: "expected-group-arn", Name: "expected-group-name"}
			awsEnv.ResourceGroupsAPI.ListGroupsBehavior.Output.Set(&resourcegroups.ListGroupsOutput{
				GroupIdentifiers: []*resourcegroups.GroupIdentifier{
					{GroupArn: aws.String(expected.ARN), GroupName: aws.String(expected.Name)},
				},
			})
			nodeClass.Spec.HostResourceGroupSelectorTerms = []v1beta1.HostResourceGroupSelectorTerm{
				{
					Name: expected.Name,
				},
			}
			hrgs, err := awsEnv.HostResourceGroupProvider.Get(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(hrgs).To(Equal(expected))
		})
		It("should filter only matching groups", func() {
			expected := &v1beta1.HostResourceGroup{ARN: "expected-group-arn", Name: "expected-group-name"}
			awsEnv.ResourceGroupsAPI.ListGroupsBehavior.Output.Set(&resourcegroups.ListGroupsOutput{
				GroupIdentifiers: []*resourcegroups.GroupIdentifier{
					{GroupArn: aws.String(expected.ARN), GroupName: aws.String(expected.Name)},
				},
			})
			nodeClass.Spec.HostResourceGroupSelectorTerms = []v1beta1.HostResourceGroupSelectorTerm{
				{
					Name: "does-not-match",
				},
			}
			hrgs, err := awsEnv.HostResourceGroupProvider.Get(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(hrgs).To(BeNil())
		})
	})
})
