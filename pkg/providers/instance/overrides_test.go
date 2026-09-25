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
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
)

// White-box coverage for getOverrides with the capacity/availability decoupling: a full-but-healthy reservation
// (Available=true, ReservationCapacity=0) must not be emitted as a CreateFleet override, since launching into a full
// reservation would ICE. getOverrides uses Offerings.Launchable() (not just Available offerings) to enforce this.
var _ = Describe("getOverrides (reserved capacity decoupled)", func() {
	It("does not emit a full reserved offering (Available, cap=0) as a CreateFleet override", func() {
		// z-1's only reserved offering is full (Available=true, ReservationCapacity=0); z-2 has a launchable on-demand
		// offering. Only the launchable z-2 offering should be emitted as an override.
		it := &cloudprovider.InstanceType{
			Name: "m5.large",
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeReserved, karpv1.CapacityTypeOnDemand),
				scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "z-1", "z-2"),
			),
			Offerings: cloudprovider.Offerings{
				&cloudprovider.Offering{Available: true, ReservationCapacity: 0, Requirements: scheduling.NewLabelRequirements(map[string]string{
					karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeReserved, corev1.LabelTopologyZone: "z-1",
				})},
				&cloudprovider.Offering{Available: true, Requirements: scheduling.NewLabelRequirements(map[string]string{
					karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "z-2",
				})},
			},
		}
		reqs := scheduling.NewRequirements(
			scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeReserved, karpv1.CapacityTypeOnDemand),
			scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpExists),
		)
		zonalSubnets := map[string]*subnet.Subnet{
			"z-1": {ID: "subnet-z1", Zone: "z-1"},
			"z-2": {ID: "subnet-z2", Zone: "z-2"},
		}
		// getOverrides does not depend on any DefaultProvider fields.
		overrides := (&DefaultProvider{}).getOverrides([]*cloudprovider.InstanceType{it}, zonalSubnets, reqs, "ami-test", "", false)
		zones := lo.Map(overrides, func(o ec2types.FleetLaunchTemplateOverridesRequest, _ int) string {
			return lo.FromPtr(o.AvailabilityZone)
		})
		// The full z-1 reservation is dropped; only the launchable z-2 offering is emitted.
		Expect(zones).To(ConsistOf("z-2"))
		Expect(zones).ToNot(ContainElement("z-1"))
	})
})
