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
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Validation Status Controller", func() {
	Context("Preconditions", func() {
		var reconciler *nodeclass.Validation
		BeforeEach(func() {
			reconciler = nodeclass.NewValidationReconciler(awsEnv.EC2API, awsEnv.AMIProvider, awsEnv.ValidationCache)
			for _, cond := range []string{
				v1.ConditionTypeAMIsReady,
				v1.ConditionTypeInstanceProfileReady,
				v1.ConditionTypeSecurityGroupsReady,
				v1.ConditionTypeSubnetsReady,
			} {
				nodeClass.StatusConditions().SetTrue(cond)
			}
		})
		DescribeTable(
			"should set validated status condition to false when any required condition is false",
			func(cond string) {
				nodeClass.StatusConditions().SetFalse(cond, "test", "test")
				_, err := reconciler.Reconcile(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(nodeclass.ConditionReasonDependenciesNotReady))
			},
			Entry(v1.ConditionTypeAMIsReady, v1.ConditionTypeAMIsReady),
			Entry(v1.ConditionTypeInstanceProfileReady, v1.ConditionTypeInstanceProfileReady),
			Entry(v1.ConditionTypeSecurityGroupsReady, v1.ConditionTypeSecurityGroupsReady),
			Entry(v1.ConditionTypeSubnetsReady, v1.ConditionTypeSubnetsReady),
		)
		DescribeTable(
			"should set validated status condition to unknown when no required condition is false and any are unknown",
			func(cond string) {
				nodeClass.StatusConditions().SetUnknown(cond)
				_, err := reconciler.Reconcile(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsUnknown()).To(BeTrue())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(nodeclass.ConditionReasonDependenciesNotReady))
			},
			Entry(v1.ConditionTypeAMIsReady, v1.ConditionTypeAMIsReady),
			Entry(v1.ConditionTypeInstanceProfileReady, v1.ConditionTypeInstanceProfileReady),
			Entry(v1.ConditionTypeSecurityGroupsReady, v1.ConditionTypeSecurityGroupsReady),
			Entry(v1.ConditionTypeSubnetsReady, v1.ConditionTypeSubnetsReady),
		)
	})
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
			Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal("TagValidationFailed"))
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
		DescribeTable(
			"NodeClass validation failure conditions",
			func(setupFn func(), reason string) {
				ExpectApplied(ctx, env.Client, nodeClass)
				setupFn()
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
				nodeClass = ExpectExists(ctx, env.Client, nodeClass)
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(reason))
				Expect(awsEnv.ValidationCache.Items()).To(HaveLen(1))

				// Even though we would succeed on the subsequent call, we should fail here because we hit the cache
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
				nodeClass = ExpectExists(ctx, env.Client, nodeClass)
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(reason))

				// After flushing the cache, we should succeed
				awsEnv.ValidationCache.Flush()
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
				nodeClass = ExpectExists(ctx, env.Client, nodeClass)
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
			},
			Entry("should update status condition as NotReady when CreateFleet unauthorized", func() {
				awsEnv.EC2API.CreateFleetBehavior.Error.Set(&smithy.GenericAPIError{
					Code: "UnauthorizedOperation",
				}, fake.MaxCalls(1))
			}, nodeclass.ConditionReasonCreateFleetAuthFailed),
			Entry("should update status condition as NotReady when RunInstances unauthorized", func() {
				awsEnv.EC2API.RunInstancesBehavior.Error.Set(&smithy.GenericAPIError{
					Code: "UnauthorizedOperation",
				}, fake.MaxCalls(1))
			}, nodeclass.ConditionReasonRunInstancesAuthFailed),
			Entry("should update status condition as NotReady when CreateLaunchTemplate unauthorized", func() {
				awsEnv.EC2API.CreateLaunchTemplateBehavior.Error.Set(&smithy.GenericAPIError{
					Code: "UnauthorizedOperation",
				}, fake.MaxCalls(1))
			}, nodeclass.ConditionReasonCreateLaunchTemplateAuthFailed),
		)
	})
})
