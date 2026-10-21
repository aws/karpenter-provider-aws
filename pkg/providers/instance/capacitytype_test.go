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

package instance

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	coretest "sigs.k8s.io/karpenter/pkg/test"
)

// White-box coverage for getCapacityType with the capacity/availability decoupling: a full-but-healthy reservation
// (Available=true, ReservationCapacity=0) must not make us choose the reserved capacity type — we'd target a full
// reservation and ICE — so we fall back to on-demand.
var _ = Describe("getCapacityType (reserved capacity decoupled)", func() {
	// reservedAndOD returns an instance type permitting reserved+on-demand, with a reserved offering of the given
	// remaining capacity plus an available on-demand offering, all in zone z-1.
	reservedAndOD := func(reservedCap int) *cloudprovider.InstanceType {
		return &cloudprovider.InstanceType{
			Name: "it",
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeReserved, karpv1.CapacityTypeOnDemand),
				scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "z-1"),
			),
			Offerings: cloudprovider.Offerings{
				&cloudprovider.Offering{Available: true, ReservationCapacity: reservedCap, Requirements: scheduling.NewLabelRequirements(map[string]string{
					karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeReserved, corev1.LabelTopologyZone: "z-1",
				})},
				&cloudprovider.Offering{Available: true, Requirements: scheduling.NewLabelRequirements(map[string]string{
					karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "z-1",
				})},
			},
		}
	}
	nc := &karpv1.NodeClaim{Spec: karpv1.NodeClaimSpec{Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
		{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeReserved, karpv1.CapacityTypeOnDemand}},
	}}}

	It("falls back to on-demand when the reservation is full", func() {
		Expect(getCapacityType(nc, []*cloudprovider.InstanceType{reservedAndOD(0)})).To(Equal(karpv1.CapacityTypeOnDemand))
	})
	It("chooses reserved when the reservation has capacity", func() {
		Expect(getCapacityType(nc, []*cloudprovider.InstanceType{reservedAndOD(1)})).To(Equal(karpv1.CapacityTypeReserved))
	})
})

const onDemandAlt = "on-demand-alt"

func registerOnDemandAlt() {
	Expect(cloudprovider.SetCapacityTypeClassResolver(cloudprovider.DefaultCapacityTypeClasses.With(cloudprovider.CapacityTypeClasses{
		onDemandAlt: cloudprovider.CapacityTypeClassOnDemand,
	}))).To(Succeed())
	DeferCleanup(coretest.ResetCapacityTypeClasses)
}

var _ = Describe("getCapacityType (additional on-demand class capacity types)", func() {
	// instanceType returns an instance type with an offering of each given capacity type in zone z-1.
	instanceType := func(launchable map[string]bool) *cloudprovider.InstanceType {
		it := &cloudprovider.InstanceType{
			Name: "it",
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeSpot, karpv1.CapacityTypeOnDemand, onDemandAlt),
				scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "z-1"),
			),
		}
		for ct, available := range launchable {
			it.Offerings = append(it.Offerings, &cloudprovider.Offering{Available: available, Requirements: scheduling.NewLabelRequirements(map[string]string{
				karpv1.CapacityTypeLabelKey: ct, corev1.LabelTopologyZone: "z-1",
			})})
		}
		return it
	}
	nodeClaim := func(capacityTypes ...string) *karpv1.NodeClaim {
		return &karpv1.NodeClaim{Spec: karpv1.NodeClaimSpec{Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
			{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: capacityTypes},
		}}}
	}
	BeforeEach(registerOnDemandAlt)

	It("chooses the additional capacity type when it's the only one allowed", func() {
		Expect(getCapacityType(nodeClaim(onDemandAlt), []*cloudprovider.InstanceType{instanceType(map[string]bool{onDemandAlt: true})})).To(Equal(onDemandAlt))
	})
	It("chooses the additional capacity type when it's the only one allowed, even without a launchable offering", func() {
		Expect(getCapacityType(nodeClaim(onDemandAlt), []*cloudprovider.InstanceType{instanceType(map[string]bool{onDemandAlt: false})})).To(Equal(onDemandAlt))
	})
	It("chooses the additional capacity type before on-demand", func() {
		Expect(getCapacityType(nodeClaim(karpv1.CapacityTypeOnDemand, onDemandAlt), []*cloudprovider.InstanceType{instanceType(map[string]bool{karpv1.CapacityTypeOnDemand: true, onDemandAlt: true})})).To(Equal(onDemandAlt))
	})
	It("falls back to on-demand when the additional capacity type isn't launchable", func() {
		Expect(getCapacityType(nodeClaim(karpv1.CapacityTypeOnDemand, onDemandAlt), []*cloudprovider.InstanceType{instanceType(map[string]bool{karpv1.CapacityTypeOnDemand: true, onDemandAlt: false})})).To(Equal(karpv1.CapacityTypeOnDemand))
	})
	It("chooses spot before the additional capacity type", func() {
		Expect(getCapacityType(nodeClaim(karpv1.CapacityTypeSpot, onDemandAlt), []*cloudprovider.InstanceType{instanceType(map[string]bool{karpv1.CapacityTypeSpot: true, onDemandAlt: true})})).To(Equal(karpv1.CapacityTypeSpot))
	})
})

type fixedAllocationStrategies struct{}

func (fixedAllocationStrategies) OnDemandAllocationStrategy(_ context.Context, nodeClaim *karpv1.NodeClaim, capacityType string, _ bool) ec2types.FleetOnDemandAllocationStrategy {
	if nodeClaim == nil {
		return "no-nodeclaim"
	}
	return ec2types.FleetOnDemandAllocationStrategy(capacityType + "-strategy")
}

func (fixedAllocationStrategies) SpotAllocationStrategy(_ context.Context, _ *karpv1.NodeClaim, _ bool) ec2types.SpotAllocationStrategy {
	return "spot-strategy"
}

var _ = Describe("CreateFleetInputBuilder", func() {
	BeforeEach(registerOnDemandAlt)

	It("should launch an additional on-demand class capacity type as on-demand", func() {
		input := NewCreateFleetInputBuilder(onDemandAlt, nil, nil).Build(context.Background(), &karpv1.NodeClaim{})
		Expect(input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(ec2types.DefaultTargetCapacityTypeOnDemand))
		Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(ec2types.FleetOnDemandAllocationStrategyLowestPrice))
		Expect(input.SpotOptions).To(BeNil())
	})
	It("should panic on an unregistered capacity type", func() {
		Expect(func() { NewCreateFleetInputBuilder("unknown", nil, nil).Build(context.Background(), nil) }).To(Panic())
	})
	DescribeTable("should use the default allocation strategies",
		func(capacityType string, overlay bool, onDemand ec2types.FleetOnDemandAllocationStrategy, spot ec2types.SpotAllocationStrategy) {
			builder := NewCreateFleetInputBuilder(capacityType, nil, nil)
			if overlay {
				builder.WithOverlay()
			}
			input := builder.Build(context.Background(), nil)
			Expect(input).To(Equal(builder.Build(context.Background(), &karpv1.NodeClaim{})))
			if onDemand != "" {
				Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(onDemand))
			}
			if spot != "" {
				Expect(input.SpotOptions.AllocationStrategy).To(Equal(spot))
			}
		},
		Entry("on-demand", karpv1.CapacityTypeOnDemand, false, ec2types.FleetOnDemandAllocationStrategyLowestPrice, ec2types.SpotAllocationStrategy("")),
		Entry("on-demand with overlay", karpv1.CapacityTypeOnDemand, true, ec2types.FleetOnDemandAllocationStrategyPrioritized, ec2types.SpotAllocationStrategy("")),
		Entry("spot", karpv1.CapacityTypeSpot, false, ec2types.FleetOnDemandAllocationStrategy(""), ec2types.SpotAllocationStrategyPriceCapacityOptimized),
		Entry("spot with overlay", karpv1.CapacityTypeSpot, true, ec2types.FleetOnDemandAllocationStrategy(""), ec2types.SpotAllocationStrategyCapacityOptimizedPrioritized),
	)
	It("should use the AllocationStrategyProvider's strategies", func() {
		nodeClaim := &karpv1.NodeClaim{}
		input := NewCreateFleetInputBuilder(onDemandAlt, nil, nil).WithAllocationStrategyProvider(fixedAllocationStrategies{}).Build(context.Background(), nodeClaim)
		Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(ec2types.FleetOnDemandAllocationStrategy(onDemandAlt + "-strategy")))
		input = NewCreateFleetInputBuilder(karpv1.CapacityTypeSpot, nil, nil).WithAllocationStrategyProvider(fixedAllocationStrategies{}).Build(context.Background(), nodeClaim)
		Expect(input.SpotOptions.AllocationStrategy).To(Equal(ec2types.SpotAllocationStrategy("spot-strategy")))
	})
	It("should pass a nil NodeClaim to the AllocationStrategyProvider", func() {
		input := NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, nil, nil).WithAllocationStrategyProvider(fixedAllocationStrategies{}).Build(context.Background(), nil)
		Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(ec2types.FleetOnDemandAllocationStrategy("no-nodeclaim")))
	})
})
