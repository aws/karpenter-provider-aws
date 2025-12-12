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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Allocation Strategy Parsing", func() {
	Describe("parseOnDemandAllocationStrategy", func() {
		It("should parse lowest-price correctly", func() {
			result := parseOnDemandAllocationStrategy("lowest-price")
			Expect(result).ToNot(BeNil())
			Expect(*result).To(Equal(ec2types.FleetOnDemandAllocationStrategyLowestPrice))
		})

		It("should parse prioritized correctly", func() {
			result := parseOnDemandAllocationStrategy("prioritized")
			Expect(result).ToNot(BeNil())
			Expect(*result).To(Equal(ec2types.FleetOnDemandAllocationStrategyPrioritized))
		})

		It("should return nil for invalid strategy", func() {
			result := parseOnDemandAllocationStrategy("invalid-strategy")
			Expect(result).To(BeNil())
		})

		It("should return nil for empty string", func() {
			result := parseOnDemandAllocationStrategy("")
			Expect(result).To(BeNil())
		})
	})

	Describe("parseSpotAllocationStrategy", func() {
		It("should parse lowest-price correctly", func() {
			result := parseSpotAllocationStrategy("lowest-price")
			Expect(result).ToNot(BeNil())
			Expect(*result).To(Equal(ec2types.SpotAllocationStrategyLowestPrice))
		})

		It("should parse price-capacity-optimized correctly", func() {
			result := parseSpotAllocationStrategy("price-capacity-optimized")
			Expect(result).ToNot(BeNil())
			Expect(*result).To(Equal(ec2types.SpotAllocationStrategyPriceCapacityOptimized))
		})

		It("should parse capacity-optimized correctly", func() {
			result := parseSpotAllocationStrategy("capacity-optimized")
			Expect(result).ToNot(BeNil())
			Expect(*result).To(Equal(ec2types.SpotAllocationStrategyCapacityOptimized))
		})

		It("should parse capacity-optimized-prioritized correctly", func() {
			result := parseSpotAllocationStrategy("capacity-optimized-prioritized")
			Expect(result).ToNot(BeNil())
			Expect(*result).To(Equal(ec2types.SpotAllocationStrategyCapacityOptimizedPrioritized))
		})

		It("should return nil for invalid strategy", func() {
			result := parseSpotAllocationStrategy("invalid-strategy")
			Expect(result).To(BeNil())
		})

		It("should return nil for empty string", func() {
			result := parseSpotAllocationStrategy("")
			Expect(result).To(BeNil())
		})
	})
})
