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
	"context"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/status/ami"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/status/instance_profile"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/status/security_group"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/status/subnet"
	"testing"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass *v1beta1.EC2NodeClass
var subnetController *subnet.Controller
var amiController *ami.Controller
var sgController *security_group.Controller
var ipController *instance_profile.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2NodeClass")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithFieldIndexers(test.EC2NodeClassFieldIndexer(ctx)))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
	subnetController = subnet.NewController(env.Client, awsEnv.SubnetProvider)
	amiController = ami.NewController(env.Client, awsEnv.AMIProvider)
	sgController = security_group.NewController(env.Client, awsEnv.SecurityGroupProvider)
	ipController = instance_profile.NewController(env.Client, awsEnv.InstanceProfileProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	nodeClass = test.EC2NodeClass()
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClass Status Condition Controller", func() {
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
	})
	It("should update status condition on nodeClass as Ready", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, amiController, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, sgController, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, subnetController, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, ipController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Conditions).To(HaveLen(5))
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
	})
	It("should update status condition as Not Ready", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"foo": "invalid"},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, amiController, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, sgController, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, subnetController, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, ipController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).Message).To(Equal("SecurityGroupsReady=False"))
	})
})
