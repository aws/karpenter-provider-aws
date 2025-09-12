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
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

const selfOwnerID = "012345678901"
const altOwnerID = "123456789012"

var discoveryTags = map[string]string{
	"karpenter.sh/discovery": "test",
}

var _ = Describe("NodeClass Capacity Reservation Reconciler", func() {
	BeforeEach(func() {
		awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(&ec2.DescribeCapacityReservationsOutput{
			CapacityReservations: []ec2types.CapacityReservation{
				{
					AvailabilityZone:       lo.ToPtr("test-zone-1a"),
					InstanceType:           lo.ToPtr("m5.large"),
					OwnerId:                lo.ToPtr(selfOwnerID),
					InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
					CapacityReservationId:  lo.ToPtr("cr-m5.large-1a-1"),
					AvailableInstanceCount: lo.ToPtr[int32](10),
					State:                  ec2types.CapacityReservationStateActive,
				},
				{
					AvailabilityZone:       lo.ToPtr("test-zone-1a"),
					InstanceType:           lo.ToPtr("m5.large"),
					OwnerId:                lo.ToPtr(selfOwnerID),
					InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
					CapacityReservationId:  lo.ToPtr("cr-m5.large-1a-2"),
					AvailableInstanceCount: lo.ToPtr[int32](10),
					Tags:                   utils.MergeTags(discoveryTags),
					State:                  ec2types.CapacityReservationStateActive,
				},
				{
					AvailabilityZone:       lo.ToPtr("test-zone-1b"),
					InstanceType:           lo.ToPtr("m5.large"),
					OwnerId:                lo.ToPtr(selfOwnerID),
					InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
					CapacityReservationId:  lo.ToPtr("cr-m5.large-1b-1"),
					AvailableInstanceCount: lo.ToPtr[int32](15),
					State:                  ec2types.CapacityReservationStateActive,
				},
				{
					AvailabilityZone:       lo.ToPtr("test-zone-1b"),
					InstanceType:           lo.ToPtr("m5.large"),
					OwnerId:                lo.ToPtr(altOwnerID),
					InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
					CapacityReservationId:  lo.ToPtr("cr-m5.large-1b-2"),
					AvailableInstanceCount: lo.ToPtr[int32](15),
					Tags:                   utils.MergeTags(discoveryTags),
					State:                  ec2types.CapacityReservationStateActive,
				},
			},
		})
	})
	It("should resolve capacity reservations by ID", func() {
		const targetID = "cr-m5.large-1a-1"
		nodeClass.Spec.CapacityReservationSelectorTerms = append(nodeClass.Spec.CapacityReservationSelectorTerms, v1.CapacityReservationSelectorTerm{
			ID: targetID,
		})
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeCapacityReservationsReady).IsTrue()).To(BeTrue())
		Expect(nodeClass.Status.CapacityReservations).To(HaveLen(1))
		Expect(nodeClass.Status.CapacityReservations[0]).To(Equal(v1.CapacityReservation{
			ID:                    targetID,
			InstanceMatchCriteria: string(ec2types.InstanceMatchCriteriaTargeted),
			OwnerID:               selfOwnerID,
			InstanceType:          "m5.large",
			AvailabilityZone:      "test-zone-1a",
			EndTime:               nil,
		}))
	})
	It("should resolve capacity reservations by tags", func() {
		nodeClass.Spec.CapacityReservationSelectorTerms = append(nodeClass.Spec.CapacityReservationSelectorTerms, v1.CapacityReservationSelectorTerm{
			Tags: discoveryTags,
		})
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeCapacityReservationsReady).IsTrue()).To(BeTrue())
		Expect(nodeClass.Status.CapacityReservations).To(HaveLen(2))
		Expect(lo.Map(nodeClass.Status.CapacityReservations, func(cr v1.CapacityReservation, _ int) string {
			return cr.ID
		})).To(ContainElements("cr-m5.large-1a-2", "cr-m5.large-1b-2"))
	})
	It("should resolve capacity reservations by tags + owner", func() {
		nodeClass.Spec.CapacityReservationSelectorTerms = append(nodeClass.Spec.CapacityReservationSelectorTerms, v1.CapacityReservationSelectorTerm{
			Tags:    discoveryTags,
			OwnerID: selfOwnerID,
		})
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeCapacityReservationsReady).IsTrue()).To(BeTrue())
		Expect(nodeClass.Status.CapacityReservations).To(HaveLen(1))
		Expect(lo.Map(nodeClass.Status.CapacityReservations, func(cr v1.CapacityReservation, _ int) string {
			return cr.ID
		})).To(ContainElements("cr-m5.large-1a-2"))
	})
	It("should exclude expired capacity reservations", func() {
		out := awsEnv.EC2API.DescribeCapacityReservationsOutput.Clone()
		targetReservationID := *out.CapacityReservations[0].CapacityReservationId
		out.CapacityReservations[0].EndDate = lo.ToPtr(awsEnv.Clock.Now().Add(time.Hour))
		awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(out)

		nodeClass.Spec.CapacityReservationSelectorTerms = append(nodeClass.Spec.CapacityReservationSelectorTerms, v1.CapacityReservationSelectorTerm{
			ID: targetReservationID,
		})
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeCapacityReservationsReady).IsTrue()).To(BeTrue())
		Expect(nodeClass.Status.CapacityReservations).To(HaveLen(1))
		Expect(lo.Map(nodeClass.Status.CapacityReservations, func(cr v1.CapacityReservation, _ int) string {
			return cr.ID
		})).To(ContainElements(targetReservationID))

		awsEnv.Clock.Step(2 * time.Hour)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeCapacityReservationsReady).IsTrue()).To(BeTrue())
		Expect(nodeClass.Status.CapacityReservations).To(HaveLen(0))
	})
	DescribeTable(
		"should exclude non-active capacity reservations",
		func(state ec2types.CapacityReservationState) {
			out := awsEnv.EC2API.DescribeCapacityReservationsOutput.Clone()
			targetReservationID := *out.CapacityReservations[0].CapacityReservationId
			out.CapacityReservations[0].State = state
			awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(out)

			nodeClass.Spec.CapacityReservationSelectorTerms = append(nodeClass.Spec.CapacityReservationSelectorTerms, v1.CapacityReservationSelectorTerm{
				ID: targetReservationID,
			})
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeCapacityReservationsReady).IsTrue()).To(BeTrue())
			Expect(nodeClass.Status.CapacityReservations).To(HaveLen(0))
		},
		lo.FilterMap(ec2types.CapacityReservationStateActive.Values(), func(state ec2types.CapacityReservationState, _ int) (TableEntry, bool) {
			return Entry(string(state), state), state != ec2types.CapacityReservationStateActive
		}),
	)
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
})
