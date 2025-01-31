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

package nodeclass_test

import (
	status "github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"

	"github.com/aws/smithy-go"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Validation Status Controller", func() {
	Context("Tag Validation", func() {
		BeforeEach(func() {
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
					Tags: map[string]string{
						"kubernetes.io/cluster/anothercluster": "owned",
					},
				},
			})
		})
		DescribeTable("should update status condition on nodeClass as NotReady when tag validation fails", func(illegalTag map[string]string) {
			nodeClass.Spec.Tags = illegalTag
			ExpectApplied(ctx, env.Client, nodeClass)
			err := ExpectObjectReconcileFailed(ctx, env.Client, controller, nodeClass)
			Expect(err).To(HaveOccurred())
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
			Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsFalse()).To(BeTrue())
			Expect(nodeClass.StatusConditions().Get(status.ConditionReady).Message).To(Equal("ValidationSucceeded=False"))
		},
			Entry("kubernetes.io/cluster*", map[string]string{"kubernetes.io/cluster/acluster": "owned"}),
			Entry(v1.NodePoolTagKey, map[string]string{v1.NodePoolTagKey: "testnodepool"}),
			Entry(v1.EKSClusterNameTagKey, map[string]string{v1.EKSClusterNameTagKey: "acluster"}),
			Entry(v1.NodeClassTagKey, map[string]string{v1.NodeClassTagKey: "testnodeclass"}),
			Entry(v1.NodeClaimTagKey, map[string]string{v1.NodeClaimTagKey: "testnodeclaim"}),
		)
		It("should update status condition as Ready when tags are valid", func() {
			nodeClass.Spec.Tags = map[string]string{}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
			Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
		})
	})
	Context("Authorization Validation", func() {
		DescribeTable("NodeClass validation failure conditions",
			func(setupFn func()) {
				ExpectApplied(ctx, env.Client, nodeClass)
				setupFn()
				err := ExpectObjectReconcileFailed(ctx, env.Client, controller, nodeClass)
				Expect(err).To(HaveOccurred())
				nodeClass = ExpectExists(ctx, env.Client, nodeClass)
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
			},
			Entry("should update status condition as NotReady when CreateFleet unauthorized",
				func() {
					awsEnv.EC2API.CreateFleetBehavior.Error.Set(&smithy.GenericAPIError{
						Code: "UnauthorizedOperation",
					}, fake.MaxCalls(1))
				}),
			Entry("should update status condition as NotReady when RunInstances unauthorized",
				func() {
					awsEnv.EC2API.RunInstancesBehavior.Error.Set(&smithy.GenericAPIError{
						Code: "UnauthorizedOperation",
					}, fake.MaxCalls(1))
				}),
			Entry("should update status condition as NotReady when CreateLaunchTemplate unauthorized",
				func() {
					awsEnv.EC2API.CreateLaunchTemplateBehavior.Error.Set(&smithy.GenericAPIError{
						Code: "UnauthorizedOperation",
					}, fake.MaxCalls(1))
				}),
		)

		It("should update status condition as Ready when authorized", func() {
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		})
	})
})
