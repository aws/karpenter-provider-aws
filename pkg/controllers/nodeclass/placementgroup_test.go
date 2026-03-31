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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
)

var _ = Describe("NodeClass Placement Group Reconciler", func() {
	BeforeEach(func() {
		awsEnv.EC2API.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{
			PlacementGroups: []ec2types.PlacementGroup{
				{
					GroupId:   lo.ToPtr("pg-cluster123"),
					GroupName: lo.ToPtr("my-cluster-pg"),
					State:     ec2types.PlacementGroupStateAvailable,
					Strategy:  ec2types.PlacementStrategyCluster,
				},
				{
					GroupId:        lo.ToPtr("pg-partition456"),
					GroupName:      lo.ToPtr("my-partition-pg"),
					State:          ec2types.PlacementGroupStateAvailable,
					Strategy:       ec2types.PlacementStrategyPartition,
					PartitionCount: lo.ToPtr[int32](7),
				},
				{
					GroupId:     lo.ToPtr("pg-spread789"),
					GroupName:   lo.ToPtr("my-spread-pg"),
					State:       ec2types.PlacementGroupStateAvailable,
					Strategy:    ec2types.PlacementStrategySpread,
					SpreadLevel: ec2types.SpreadLevelRack,
				},
				{
					GroupId:   lo.ToPtr("pg-pending000"),
					GroupName: lo.ToPtr("my-pending-pg"),
					State:     ec2types.PlacementGroupStatePending,
					Strategy:  ec2types.PlacementStrategyCluster,
				},
			},
		})
	})

	It("should not have PlacementGroupReady condition when no placement group selector is specified", func() {
		// nodeClass has no PlacementGroupSelector by default
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady)).To(BeNil())
		Expect(awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)).To(BeNil())
	})
	It("should resolve a cluster placement group by name", func() {
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsTrue()).To(BeTrue())
		pg := awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-cluster123"))
		Expect(pg.Name).To(Equal("my-cluster-pg"))
		Expect(pg.Strategy).To(Equal(placementgroup.StrategyCluster))
	})
	It("should resolve a placement group by ID", func() {
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{ID: "pg-cluster123"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsTrue()).To(BeTrue())
		pg := awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-cluster123"))
		Expect(pg.Name).To(Equal("my-cluster-pg"))
	})
	It("should resolve a partition placement group with partition count", func() {
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{Name: "my-partition-pg"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsTrue()).To(BeTrue())
		pg := awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-partition456"))
		Expect(pg.Name).To(Equal("my-partition-pg"))
		Expect(pg.Strategy).To(Equal(placementgroup.StrategyPartition))
		Expect(pg.PartitionCount).To(Equal(int32(7)))
	})
	It("should resolve a spread placement group with spread level", func() {
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{Name: "my-spread-pg"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsTrue()).To(BeTrue())
		pg := awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-spread789"))
		Expect(pg.Name).To(Equal("my-spread-pg"))
		Expect(pg.Strategy).To(Equal(placementgroup.StrategySpread))
		Expect(pg.SpreadLevel).To(Equal(placementgroup.SpreadLevelRack))
	})
	It("should set condition false when placement group is not found", func() {
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{Name: "nonexistent-pg"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		condition := nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady)
		Expect(condition.IsFalse()).To(BeTrue())
		Expect(condition.Reason).To(Equal("PlacementGroupNotFound"))
		Expect(condition.Message).To(ContainSubstring("nonexistent-pg"))
		Expect(awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)).To(BeNil())
	})
	It("should set condition false when placement group is not in available state", func() {
		// The DescribePlacementGroupsInput always filters by state=available, so a pending PG
		// is filtered out at the EC2 API level. The reconciler sees nil from the provider and
		// sets "PlacementGroupNotFound".
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{Name: "my-pending-pg"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		condition := nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady)
		Expect(condition.IsFalse()).To(BeTrue())
		Expect(condition.Reason).To(Equal("PlacementGroupNotFound"))
		Expect(condition.Message).To(ContainSubstring("my-pending-pg"))
		Expect(awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)).To(BeNil())
	})
	It("should clear in-memory state and condition when placement group selector is removed", func() {
		// First, set up with a placement group
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)).ToNot(BeNil())

		// Now remove the selector - PlacementGroupReady condition should be cleared
		nodeClass.Spec.PlacementGroupSelector = nil
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady)).To(BeNil())
		Expect(awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)).To(BeNil())
	})
	It("should update in-memory state when placement group selector changes", func() {
		// Start with cluster PG
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		pg := awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-cluster123"))

		// Switch to spread PG
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelectorTerm{Name: "my-spread-pg"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		pg = awsEnv.PlacementGroupProvider.GetForNodeClass(nodeClass)
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-spread789"))
		Expect(pg.Strategy).To(Equal(placementgroup.StrategySpread))
	})
})
