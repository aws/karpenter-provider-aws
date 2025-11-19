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
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var _ = Describe("CreateFleetInputBuilder", func() {
	var builder *CreateFleetInputBuilder
	var launchTemplateConfigs []ec2types.FleetLaunchTemplateConfigRequest

	BeforeEach(func() {
		launchTemplateConfigs = []ec2types.FleetLaunchTemplateConfigRequest{
			{
				LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateName: lo.ToPtr("test-template"),
				},
			},
		}
	})

	Describe("Build with allocation strategies", func() {
		Context("On-demand instances", func() {
			It("should use provided on-demand allocation strategy", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, map[string]string{}, launchTemplateConfigs)
				builder.WithOnDemandAllocationStrategy(ec2types.FleetOnDemandAllocationStrategyPrioritized)

				input := builder.Build()

				Expect(input.OnDemandOptions).ToNot(BeNil())
				Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(ec2types.FleetOnDemandAllocationStrategyPrioritized))
			})

			It("should use default lowest-price when no strategy is provided", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, map[string]string{}, launchTemplateConfigs)

				input := builder.Build()

				Expect(input.OnDemandOptions).ToNot(BeNil())
				Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(ec2types.FleetOnDemandAllocationStrategyLowestPrice))
			})

			It("should use prioritized when overlay is set and no strategy is provided", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, map[string]string{}, launchTemplateConfigs)
				builder.WithOverlay()

				input := builder.Build()

				Expect(input.OnDemandOptions).ToNot(BeNil())
				Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(ec2types.FleetOnDemandAllocationStrategyPrioritized))
			})

			It("should use provided strategy even when overlay is set", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, map[string]string{}, launchTemplateConfigs)
				builder.WithOverlay()
				builder.WithOnDemandAllocationStrategy(ec2types.FleetOnDemandAllocationStrategyLowestPrice)

				input := builder.Build()

				Expect(input.OnDemandOptions).ToNot(BeNil())
				Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(ec2types.FleetOnDemandAllocationStrategyLowestPrice))
			})
		})

		Context("Spot instances", func() {
			It("should use provided spot allocation strategy", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeSpot, map[string]string{}, launchTemplateConfigs)
				builder.WithSpotAllocationStrategy(ec2types.SpotAllocationStrategyCapacityOptimized)

				input := builder.Build()

				Expect(input.SpotOptions).ToNot(BeNil())
				Expect(input.SpotOptions.AllocationStrategy).To(Equal(ec2types.SpotAllocationStrategyCapacityOptimized))
			})

			It("should use default price-capacity-optimized when no strategy is provided", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeSpot, map[string]string{}, launchTemplateConfigs)

				input := builder.Build()

				Expect(input.SpotOptions).ToNot(BeNil())
				Expect(input.SpotOptions.AllocationStrategy).To(Equal(ec2types.SpotAllocationStrategyPriceCapacityOptimized))
			})

			It("should use capacity-optimized-prioritized when overlay is set and no strategy is provided", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeSpot, map[string]string{}, launchTemplateConfigs)
				builder.WithOverlay()

				input := builder.Build()

				Expect(input.SpotOptions).ToNot(BeNil())
				Expect(input.SpotOptions.AllocationStrategy).To(Equal(ec2types.SpotAllocationStrategyCapacityOptimizedPrioritized))
			})

			It("should use provided strategy even when overlay is set", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeSpot, map[string]string{}, launchTemplateConfigs)
				builder.WithOverlay()
				builder.WithSpotAllocationStrategy(ec2types.SpotAllocationStrategyLowestPrice)

				input := builder.Build()

				Expect(input.SpotOptions).ToNot(BeNil())
				Expect(input.SpotOptions.AllocationStrategy).To(Equal(ec2types.SpotAllocationStrategyLowestPrice))
			})

			It("should support all spot allocation strategies", func() {
				strategies := []ec2types.SpotAllocationStrategy{
					ec2types.SpotAllocationStrategyLowestPrice,
					ec2types.SpotAllocationStrategyPriceCapacityOptimized,
					ec2types.SpotAllocationStrategyCapacityOptimized,
					ec2types.SpotAllocationStrategyCapacityOptimizedPrioritized,
				}

				for _, strategy := range strategies {
					builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeSpot, map[string]string{}, launchTemplateConfigs)
					builder.WithSpotAllocationStrategy(strategy)

					input := builder.Build()

					Expect(input.SpotOptions).ToNot(BeNil())
					Expect(input.SpotOptions.AllocationStrategy).To(Equal(strategy))
				}
			})
		})

		Context("Reserved instances", func() {
			It("should not set allocation strategy for capacity block reservations", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeReserved, map[string]string{}, launchTemplateConfigs)
				builder.WithCapacityReservationType(v1.CapacityReservationTypeCapacityBlock)

				input := builder.Build()

				Expect(input.OnDemandOptions).To(BeNil())
			})

			It("should use provided on-demand allocation strategy for default reservations", func() {
				builder = NewCreateFleetInputBuilder(karpv1.CapacityTypeReserved, map[string]string{}, launchTemplateConfigs)
				builder.WithCapacityReservationType(v1.CapacityReservationTypeDefault)
				builder.WithOnDemandAllocationStrategy(ec2types.FleetOnDemandAllocationStrategyPrioritized)

				input := builder.Build()

				Expect(input.OnDemandOptions).ToNot(BeNil())
				Expect(input.OnDemandOptions.AllocationStrategy).To(Equal(ec2types.FleetOnDemandAllocationStrategyPrioritized))
			})
		})
	})
})
