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

package cache_test

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/aws/karpenter-provider-aws/pkg/cache"
)

var ctx context.Context

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cache")
}

var _ = Describe("Cache", func() {
	var unavailableOfferingCache *cache.UnavailableOfferings

	BeforeEach(func() {
		unavailableOfferingCache = cache.NewUnavailableOfferings()
	})
	Context("Unavailable Offering Cache", func() {
		It("should mark offerings as unavailable when calling MarkUnavailable", func() {
			// offerings should initially not be marked as unavailable
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.large on-demand should return that it's unavailable when we mark it
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand, map[string]interface{}{"reason": "test"})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.xlarge shouldn't return that it's unavailable when marking an unrelated instance type
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Large, "test-zone-1b", karpv1.CapacityTypeOnDemand, map[string]interface{}{"reason": "test"})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.xlarge spot should return that it's unavailable when we mark it
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot, map[string]interface{}{"reason": "test"})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeTrue())
		})
		It("should mark offerings as unavailable when calling MarkUnavailableForFleetErr", func() {
			// offerings should initially not be marked as unavailable
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.large on-demand should return that it's unavailable when we mark it
			unavailableOfferingCache.MarkUnavailableForFleetErr(ctx, ec2types.CreateFleetError{
				LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
					Overrides: &ec2types.FleetLaunchTemplateOverrides{
						InstanceType:     ec2types.InstanceTypeM5Large,
						AvailabilityZone: lo.ToPtr("test-zone-1a"),
					},
				},
			}, karpv1.CapacityTypeOnDemand)
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.xlarge shouldn't return that it's unavailable when marking an unrelated instance type
			unavailableOfferingCache.MarkUnavailableForFleetErr(ctx, ec2types.CreateFleetError{
				LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
					Overrides: &ec2types.FleetLaunchTemplateOverrides{
						InstanceType:     ec2types.InstanceTypeM5Large,
						AvailabilityZone: lo.ToPtr("test-zone-1b"),
					},
				},
			}, karpv1.CapacityTypeOnDemand)
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.xlarge spot should return that it's unavailable when we mark it
			unavailableOfferingCache.MarkUnavailableForFleetErr(ctx, ec2types.CreateFleetError{
				LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
					Overrides: &ec2types.FleetLaunchTemplateOverrides{
						InstanceType:     ec2types.InstanceTypeM5Xlarge,
						AvailabilityZone: lo.ToPtr("test-zone-1b"),
					},
				},
			}, karpv1.CapacityTypeSpot)
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeTrue())
		})
		It("should mark offerings as unavailable when calling MarkCapacityTypeUnavailable", func() {
			// offerings should initially not be marked as unavailable
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())

			// mark all spot offerings as unavailable
			unavailableOfferingCache.MarkCapacityTypeUnavailable(karpv1.CapacityTypeSpot)
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeTrue())

			// mark all on-demand offerings as unavailable
			unavailableOfferingCache.MarkCapacityTypeUnavailable(karpv1.CapacityTypeOnDemand)
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeTrue())
		})
		It("should mark offerings as unavailable when calling MarkAZUnavailable", func() {
			// offerings should initially not be marked as unavailable
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())

			// mark all test-zone-1a offerings as unavailable
			unavailableOfferingCache.MarkAZUnavailable("test-zone-1a")
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeFalse())
		})
		It("should increase sequence number when unavailability changes", func() {
			// sequence numbers should initially be 0
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 0))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 0))

			// marking m5.large as unavailable should increase the sequence number for that instance type but not others
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand, map[string]interface{}{"reason": "test"})
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 1))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 0))

			// marking m5.xlarge as unavailable should increase the sequence number for that instance type but not others
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand, map[string]interface{}{"reason": "test"})
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 1))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 1))

			// marking test-zone-1a as unavailable should increase the sequence number for all instance types
			unavailableOfferingCache.MarkAZUnavailable("test-zone-1a")
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 2))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 2))

			// marking test-zone-1b as unavailable should increase the sequence number for all instance types
			unavailableOfferingCache.MarkAZUnavailable("test-zone-1b")
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 3))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 3))

			// marking on-demand capacity type as unavailable should increase the sequence number for all instance types
			unavailableOfferingCache.MarkCapacityTypeUnavailable(karpv1.CapacityTypeOnDemand)
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 4))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 4))

			// deleting m5.xlarge from the cache should increase the sequence number for that instance type but not others
			unavailableOfferingCache.Delete(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand)
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 4))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 5))
		})
	})
})
