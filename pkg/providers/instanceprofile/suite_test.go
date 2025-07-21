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

package instanceprofile_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

const nodeRole = "NodeRole"

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass test.TestNodeClass

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceProfileProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	nodeClass = test.TestNodeClass{
		EC2NodeClass: v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				AMISelectorTerms: []v1.AMISelectorTerm{{
					Alias: "al2@latest",
				}},
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						Tags: map[string]string{
							"*": "*",
						},
					},
				},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{
							"*": "*",
						},
					},
				},
			},
		},
	}
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("InstanceProfileProvider", func() {
	DescribeTable(
		"should support IAM roles",
		func(roleWithPath, role string) {
			const profileName = "test-profile"
			nodeClass.Spec.Role = roleWithPath
			Expect(awsEnv.InstanceProfileProvider.Create(ctx, "", profileName, role, nil, string(nodeClass.UID))).To(Succeed())
			Expect(profileName).ToNot(BeNil())
			Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
			Expect(aws.ToString(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName)).To(Equal(role))
		},
		Entry("with custom paths", fmt.Sprintf("CustomPath/%s", nodeRole), nodeRole),
		Entry("without custom paths", nodeRole, nodeRole),
	)
})
