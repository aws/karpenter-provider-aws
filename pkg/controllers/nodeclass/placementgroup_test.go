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
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Placement Group Status Controller", func() {
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				PlacementGroup: &v1.PlacementGroup{Name: "analytics-cluster"},
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{Tags: map[string]string{"*": "*"}},
				},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{Tags: map[string]string{"*": "*"}},
				},
				AMIFamily: lo.ToPtr(v1.AMIFamilyCustom),
				AMISelectorTerms: []v1.AMISelectorTerm{
					{Tags: map[string]string{"*": "*"}},
				},
			},
		})
		awsEnv.EC2API.PlacementGroups.Store("analytics-cluster", ec2types.PlacementGroup{
			GroupId:   aws.String("pg-0123456789abcdef0"),
			GroupName: aws.String("analytics-cluster"),
			State:     ec2types.PlacementGroupStateAvailable,
			Strategy:  ec2types.PlacementStrategyCluster,
		})
		awsEnv.EC2API.PlacementGroups.Store("partitioned-workload", ec2types.PlacementGroup{
			GroupId:        aws.String("pg-0fedcba9876543210"),
			GroupName:      aws.String("partitioned-workload"),
			State:          ec2types.PlacementGroupStateAvailable,
			Strategy:       ec2types.PlacementStrategyPartition,
			PartitionCount: lo.ToPtr(int32(3)),
		})
	})

	It("should resolve the configured placement group into status", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.PlacementGroup).To(Equal(&v1.PlacementGroupStatus{
			ID:       "pg-0123456789abcdef0",
			Name:     "analytics-cluster",
			Strategy: "cluster",
			State:    "available",
		}))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsTrue()).To(BeTrue())
	})

	It("should set the placement group condition true when placement groups are not configured", func() {
		nodeClass.Spec.PlacementGroup = nil
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.PlacementGroup).To(BeNil())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsTrue()).To(BeTrue())
	})

	It("should set the placement group condition false when the placement group is missing", func() {
		nodeClass.Spec.PlacementGroup = &v1.PlacementGroup{Name: "missing-placement-group"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.PlacementGroup).To(BeNil())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsFalse()).To(BeTrue())
	})

	It("should set the placement group condition false when the placement group is not available", func() {
		awsEnv.EC2API.PlacementGroups.Store("pending-group", ec2types.PlacementGroup{
			GroupId:   aws.String("pg-pending123"),
			GroupName: aws.String("pending-group"),
			State:     ec2types.PlacementGroupStatePending,
			Strategy:  ec2types.PlacementStrategyCluster,
		})
		nodeClass.Spec.PlacementGroup = &v1.PlacementGroup{Name: "pending-group"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).Reason).To(Equal("PlacementGroupNotAvailable"))
	})

	It("should set the placement group condition false when the placement group is deleting", func() {
		awsEnv.EC2API.PlacementGroups.Store("deleting-group", ec2types.PlacementGroup{
			GroupId:   aws.String("pg-deleting123"),
			GroupName: aws.String("deleting-group"),
			State:     ec2types.PlacementGroupStateDeleting,
			Strategy:  ec2types.PlacementStrategyCluster,
		})
		nodeClass.Spec.PlacementGroup = &v1.PlacementGroup{Name: "deleting-group"}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).Reason).To(Equal("PlacementGroupNotAvailable"))
	})

	It("should reject partition overrides for non-partition placement groups", func() {
		nodeClass.Spec.PlacementGroup = &v1.PlacementGroup{Name: "analytics-cluster", Partition: lo.ToPtr(int32(1))}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).Reason).To(Equal("PlacementGroupInvalid"))
	})

	It("should reject partition number exceeding partition count", func() {
		nodeClass.Spec.PlacementGroup = &v1.PlacementGroup{Name: "partitioned-workload", Partition: lo.ToPtr(int32(5))}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).Reason).To(Equal("PlacementGroupInvalid"))
	})

	It("should allow partition overrides for partition placement groups", func() {
		nodeClass.Spec.PlacementGroup = &v1.PlacementGroup{Name: "partitioned-workload", Partition: lo.ToPtr(int32(1))}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.PlacementGroup).To(Equal(&v1.PlacementGroupStatus{
			ID:             "pg-0fedcba9876543210",
			Name:           "partitioned-workload",
			Strategy:       "partition",
			PartitionCount: lo.ToPtr(int32(3)),
			State:          "available",
		}))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypePlacementGroupReady).IsTrue()).To(BeTrue())
	})
})
