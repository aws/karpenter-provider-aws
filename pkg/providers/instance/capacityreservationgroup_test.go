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

package instance_test

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
)

var _ = Describe("Capacity reservation groups", func() {
	const groupARN = "arn:aws:resource-groups:us-west-2:123456789012:group/workload-a"
	var ec2Instance types.Instance
	BeforeEach(func() {
		ec2Instance = types.Instance{
			InstanceId: lo.ToPtr("i-12345678"), InstanceType: types.InstanceTypeM5Large,
			State:     &types.InstanceState{Name: types.InstanceStateNameRunning},
			Placement: &types.Placement{AvailabilityZone: lo.ToPtr("test-zone-1a")},
			CapacityReservationSpecification: &types.CapacityReservationSpecificationResponse{
				CapacityReservationTarget: &types.CapacityReservationTargetResponse{CapacityReservationResourceGroupArn: lo.ToPtr(groupARN)},
			},
		}
	})
	It("should observe coverage independently of the group target", func() {
		i := instance.NewInstance(ctx, ec2Instance)
		Expect(i.CapacityType).To(Equal(karpv1.CapacityTypeOnDemand))
		Expect(i.CapacityReservationGroupARN).To(Equal(groupARN))
		Expect(i.CapacityReservationDetails).To(BeNil())
		ec2Instance.CapacityReservationId = lo.ToPtr("cr-12345678")
		i = instance.NewInstance(ctx, ec2Instance)
		Expect(i.CapacityType).To(Equal(karpv1.CapacityTypeReserved))
		Expect(i.CapacityReservationGroupARN).To(Equal(groupARN))
		Expect(i.CapacityReservationLabels()).To(HaveKeyWithValue(corecloudprovider.ReservationIDLabel, "cr-12345678"))
		Expect(i.CapacityReservationLabels()).To(HaveKeyWithValue(v1.LabelCapacityReservationType, string(v1.CapacityReservationTypeDefault)))
		ec2Instance.CapacityReservationId = nil
		i = instance.NewInstance(ctx, ec2Instance)
		Expect(i.CapacityReservationGroupARN).To(Equal(groupARN))
		Expect(i.CapacityReservationLabels()).ToNot(HaveKey(corecloudprovider.ReservationIDLabel))
	})
	It("should preserve Spot classification", func() {
		ec2Instance.SpotInstanceRequestId = lo.ToPtr("sir-12345678")
		ec2Instance.CapacityReservationId = lo.ToPtr("cr-12345678")
		Expect(instance.NewInstance(ctx, ec2Instance).CapacityType).To(Equal(karpv1.CapacityTypeSpot))
	})
	It("should preserve non-group open matching behavior and tolerate missing specifications", func() {
		ec2Instance.CapacityReservationId = lo.ToPtr("cr-12345678")
		ec2Instance.CapacityReservationSpecification = nil
		Expect(instance.NewInstance(ctx, ec2Instance).CapacityType).To(Equal(karpv1.CapacityTypeOnDemand))
		ec2Instance.CapacityReservationSpecification = &types.CapacityReservationSpecificationResponse{CapacityReservationPreference: types.CapacityReservationPreferenceOpen}
		Expect(instance.NewInstance(ctx, ec2Instance).CapacityType).To(Equal(karpv1.CapacityTypeOnDemand))
	})
	It("should respect the existing ReservedCapacity feature gate", func() {
		disabled := coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(false)}}))
		ec2Instance.CapacityReservationId = lo.ToPtr("cr-12345678")
		i := instance.NewInstance(disabled, ec2Instance)
		Expect(i.CapacityType).To(Equal(karpv1.CapacityTypeOnDemand))
		Expect(i.CapacityReservationGroupARN).To(Equal(groupARN))
	})
	It("should request reservations first while retaining an On-Demand Fleet target", func() {
		input := instance.NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, nil, nil).WithCapacityReservationGroup(groupARN).Build()
		Expect(input.Type).To(Equal(types.FleetTypeInstant))
		Expect(input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(types.DefaultTargetCapacityTypeOnDemand))
		Expect(input.OnDemandOptions.CapacityReservationOptions.UsageStrategy).To(Equal(types.FleetCapacityReservationUsageStrategyUseCapacityReservationsFirst))
		Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(types.FleetOnDemandAllocationStrategyLowestPrice))
		input = instance.NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, nil, nil).WithOverlay().WithCapacityReservationGroup(groupARN).Build()
		Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(types.FleetOnDemandAllocationStrategyPrioritized))
	})
	It("should preserve non-group Fleet requests", func() {
		input := instance.NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, nil, nil).Build()
		Expect(input.OnDemandOptions.CapacityReservationOptions).To(BeNil())
		input = instance.NewCreateFleetInputBuilder(karpv1.CapacityTypeSpot, nil, nil).Build()
		Expect(input.OnDemandOptions).To(BeNil())
		Expect(input.SpotOptions).ToNot(BeNil())
	})
})
