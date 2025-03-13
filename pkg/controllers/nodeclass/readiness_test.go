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
	"fmt"

	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Status Condition Controller", func() {
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
			},
		})
	})
	DescribeTable(
		"should update status condition on nodeClass as Ready",
		func(reservedCapacity bool) {
			coreoptions.FromContext(ctx).FeatureGates.ReservedCapacity = reservedCapacity
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Conditions).To(HaveLen(lo.Ternary(reservedCapacity, 7, 6)))
			Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
		},
		Entry("when reserved capacity feature flag is enabled", true),
		Entry("when reserved capacity feature flag is disabled", false),
	)
	It("should update status condition as Not Ready", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"foo": "invalid"},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).Message).To(Equal("ValidationSucceeded=False, SecurityGroupsReady=False"))
	})
	It("should recover from temporary DescribeCluster failure", func() {
		// First reconcile with API error
		awsEnv.EKSAPI.DescribeClusterBehavior.Error.Set(fmt.Errorf("temporary AWS API error"))

		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)

		ExpectApplied(ctx, env.Client, nodeClass)

		ExpectObjectReconcileFailed(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).Message).To(ContainSubstring("Failed to detect the cluster CIDR"))

		// Clear the error and reconcile again
		awsEnv.EKSAPI.DescribeClusterBehavior.Error.Set(nil)
		awsEnv.EKSAPI.DescribeClusterBehavior.Output.Set(&eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{
				Version: lo.ToPtr("1.29"),
				KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
					ServiceIpv4Cidr: lo.ToPtr("10.100.0.0/16"),
				},
			},
		})

		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		// Verify conditions are now ready
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
	})
})
