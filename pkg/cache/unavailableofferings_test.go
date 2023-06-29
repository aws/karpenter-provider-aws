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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "knative.dev/pkg/system/testing"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/cache"
)

var unavailableOfferingsCache *cache.UnavailableOfferings
var recorder *test.EventRecorder

var _ = Describe("UnavailableOfferings", func() {
	BeforeEach(func() {
		ctx = context.Background()
		recorder = test.NewEventRecorder()
		unavailableOfferingsCache = cache.NewUnavailableOfferings(recorder)
	})

	AfterEach(func() {
		recorder.Reset()
	})

	It("should create an UnavailableOfferingEvent when receiving a CreateFleet error", func() {
		unavailableOfferingsCache.MarkUnavailableForFleetErr(ctx, &ec2.CreateFleetError{
			LaunchTemplateAndOverrides: &ec2.LaunchTemplateAndOverridesResponse{
				Overrides: &ec2.FleetLaunchTemplateOverrides{
					InstanceType:     aws.String("c5.large"),
					AvailabilityZone: aws.String("test-zone-1a"),
				},
			},
		}, v1alpha5.CapacityTypeSpot)
		Expect(recorder.Calls("UnavailableOffering")).To(BeNumerically("==", 1))
		Expect(recorder.DetectedEvent(`UnavailableOffering for {"instanceType": "c5.large", "availabilityZone": "test-zone-1a", "capacityType": "spot"}`))
	})
	It("should create an UnavailableOfferingEvent when marking an offering as unavailable", func() {
		unavailableOfferingsCache.MarkUnavailable(ctx, "offering is unavailable", "c5.large", "test-zone-1a", v1alpha5.CapacityTypeSpot)
		Expect(recorder.Calls("UnavailableOffering")).To(BeNumerically("==", 1))
		Expect(recorder.DetectedEvent(`UnavailableOffering for {"instanceType": "c5.large", "availabilityZone": "test-zone-1a", "capacityType": "spot"}`))
	})
	It("should create multiple UnavailableOfferingEvent when marking multiple offerings as unavailable", func() {
		type offering struct {
			instanceType     string
			availabilityZone string
			capacityType     string
		}

		offerings := []offering{
			{
				instanceType:     "c5.large",
				availabilityZone: "test-zone-1a",
				capacityType:     v1alpha5.CapacityTypeSpot,
			},
			{
				instanceType:     "g4dn.xlarge",
				availabilityZone: "test-zone-1b",
				capacityType:     v1alpha5.CapacityTypeOnDemand,
			},
			{
				instanceType:     "inf1.24xlarge",
				availabilityZone: "test-zone-1d",
				capacityType:     v1alpha5.CapacityTypeSpot,
			},
			{
				instanceType:     "t3.nano",
				availabilityZone: "test-zone-1b",
				capacityType:     v1alpha5.CapacityTypeOnDemand,
			},
		}

		for _, of := range offerings {
			unavailableOfferingsCache.MarkUnavailable(ctx, "offering is unavailable", of.instanceType, of.availabilityZone, of.capacityType)
		}
		Expect(recorder.Calls("UnavailableOffering")).To(BeNumerically("==", len(offerings)))
	})
})
