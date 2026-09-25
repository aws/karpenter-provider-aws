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
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
)

// White-box coverage for updateUnavailableOfferingsCache on reserved launch failures, exercising the capacity/health
// decoupling: a full reservation (ReservationCapacityExceeded) is out of capacity but healthy, so it must stay
// Available; a genuine ICE marks the offering unavailable, scoped to its reservation ID so it can't poison siblings.
var _ = Describe("updateUnavailableOfferingsCache (reserved launch failures)", func() {
	const (
		instanceType = "m5.large"
		zone         = "test-zone-1a"
		reservation  = "cr-aaa"
		sibling      = "cr-bbb"
	)
	var (
		p             *DefaultProvider
		instanceTypes []*cloudprovider.InstanceType
	)
	BeforeEach(func() {
		p = &DefaultProvider{unavailableOfferings: awscache.NewUnavailableOfferings()}
		instanceTypes = []*cloudprovider.InstanceType{{
			Name: instanceType,
			Offerings: cloudprovider.Offerings{{
				Available: true,
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					karpv1.CapacityTypeLabelKey:      karpv1.CapacityTypeReserved,
					corev1.LabelTopologyZone:         zone,
					cloudprovider.ReservationIDLabel: reservation,
				}),
			}},
		}}
	})
	reservedError := func(code string) ec2types.CreateFleetError {
		return ec2types.CreateFleetError{
			ErrorCode: lo.ToPtr(code),
			LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
				Overrides: &ec2types.FleetLaunchTemplateOverrides{
					InstanceType:     instanceType,
					AvailabilityZone: lo.ToPtr(zone),
				},
			},
		}
	}
	updateCache := func(errs ...ec2types.CreateFleetError) {
		p.updateUnavailableOfferingsCache(context.Background(), errs, karpv1.CapacityTypeReserved, nil, instanceTypes, nil, "fleet-id", "", nil)
	}

	It("does not mark a full reservation (ReservationCapacityExceeded) unavailable — it stays Available", func() {
		updateCache(reservedError("ReservationCapacityExceeded"))
		Expect(p.unavailableOfferings.IsUnavailable(instanceType, zone, nil, karpv1.CapacityTypeReserved, awscache.WithReservationID(reservation))).To(BeFalse())
	})

	It("marks a genuinely ICE'd reservation unavailable, scoped to its reservation ID", func() {
		updateCache(reservedError("InsufficientInstanceCapacity"))
		// The failed reservation is unavailable...
		Expect(p.unavailableOfferings.IsUnavailable(instanceType, zone, nil, karpv1.CapacityTypeReserved, awscache.WithReservationID(reservation))).To(BeTrue())
		// ...but a sibling reservation of the same instance type + zone is not poisoned (per-reservation scoping)...
		Expect(p.unavailableOfferings.IsUnavailable(instanceType, zone, nil, karpv1.CapacityTypeReserved, awscache.WithReservationID(sibling))).To(BeFalse())
		// ...and the mark is keyed by reservation ID, not the bare capacity-type/zone key.
		Expect(p.unavailableOfferings.IsUnavailable(instanceType, zone, nil, karpv1.CapacityTypeReserved)).To(BeFalse())
	})
})
