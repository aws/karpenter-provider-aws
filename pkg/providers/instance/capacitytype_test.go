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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
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
