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

package cloudprovider_test

import (
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var _ = Describe("Capacity reservation group requirements", func() {
	var nc *v1.EC2NodeClass
	var np *karpv1.NodePool
	BeforeEach(func() {
		nc = test.EC2NodeClass()
		nc.Spec.CapacityReservationGroupARN = lo.ToPtr("arn:aws:resource-groups:us-west-2:123456789012:group/workload-a")
		nc.StatusConditions().SetTrue(status.ConditionReady)
		np = coretest.NodePool()
		np.Spec.Template.Spec.NodeClassRef = &karpv1.NodeClassReference{Group: "karpenter.k8s.aws", Kind: "EC2NodeClass", Name: nc.Name}
		_, err := awsEnv.SubnetProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
	})
	DescribeTable("should validate the effective NodePool capacity types", func(values []string, valid bool) {
		if values != nil {
			np.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: values}}
		}
		ExpectApplied(ctx, env.Client, nc, np)
		_, err := cloudProvider.GetInstanceTypes(ctx, np)
		if valid {
			Expect(err).ToNot(HaveOccurred())
		} else {
			Expect(err).To(MatchError(ContainSubstring("requires both on-demand and reserved")))
		}
	}, Entry("unconstrained", []string(nil), true), Entry("both", []string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeReserved}, true), Entry("On-Demand only", []string{karpv1.CapacityTypeOnDemand}, false), Entry("reserved only", []string{karpv1.CapacityTypeReserved}, false))
	It("should reject reservation-ID constraints in group mode", func() {
		np.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{{Key: corecloudprovider.ReservationIDLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{"cr-12345678"}}}
		ExpectApplied(ctx, env.Client, nc, np)
		_, err := cloudProvider.GetInstanceTypes(ctx, np)
		Expect(err).To(MatchError(ContainSubstring("incompatible with requirements")))
	})
	It("should reject fixed capacity-type template labels in group mode", func() {
		np.Spec.Template.Labels = map[string]string{karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand}
		ExpectApplied(ctx, env.Client, nc, np)
		_, err := cloudProvider.GetInstanceTypes(ctx, np)
		Expect(err).To(MatchError(ContainSubstring("requires both on-demand and reserved")))
	})
	DescribeTable("should reject incompatible standalone NodeClaims before launch", func(requirements []karpv1.NodeSelectorRequirementWithMinValues, labels map[string]string, message string) {
		ExpectApplied(ctx, env.Client, nc)
		claim := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Spec:       karpv1.NodeClaimSpec{NodeClassRef: np.Spec.Template.Spec.NodeClassRef, Requirements: requirements},
		})
		_, err := cloudProvider.Create(ctx, claim)
		Expect(err).To(MatchError(ContainSubstring(message)))
		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeZero())
	},
		Entry("On-Demand only", []karpv1.NodeSelectorRequirementWithMinValues{{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeOnDemand}}}, map[string]string{}, "requires both on-demand and reserved"),
		Entry("fixed reservation label", []karpv1.NodeSelectorRequirementWithMinValues{}, map[string]string{corecloudprovider.ReservationIDLabel: "cr-12345678"}, "incompatible with requirements"),
	)
	It("should preserve group intent after creating a scheduler-narrowed On-Demand NodeClaim", func() {
		np.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeReserved}}}
		ExpectApplied(ctx, env.Client, nc, np)
		claim := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{karpv1.NodePoolLabelKey: np.Name}},
			Spec:       karpv1.NodeClaimSpec{NodeClassRef: np.Spec.Template.Spec.NodeClassRef, Requirements: []karpv1.NodeSelectorRequirementWithMinValues{{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeOnDemand}}}},
		})
		launched, err := cloudProvider.Create(ctx, claim)
		Expect(err).ToNot(HaveOccurred())
		Expect(launched.Status.ProviderID).ToNot(BeEmpty())
		Expect(launched.Annotations).To(HaveKeyWithValue(v1.AnnotationCapacityReservationGroupARN, *nc.Spec.CapacityReservationGroupARN))
		Expect(launched.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
		Expect(launched.Labels).ToNot(HaveKey(corecloudprovider.ReservationIDLabel))
	})
	It("should not advertise stale individual reservations for a group NodeClass", func() {
		nc.Status.CapacityReservations = []v1.CapacityReservation{{ID: "cr-12345678", InstanceType: "m5.large", AvailabilityZone: "test-zone-1a", State: v1.CapacityReservationStateActive, OwnerID: "123456789012", InstanceMatchCriteria: "targeted", ReservationType: v1.CapacityReservationTypeDefault}}
		Expect(nc.CapacityReservations()).To(BeEmpty())
		ExpectApplied(ctx, env.Client, nc, np)
		its, err := cloudProvider.GetInstanceTypes(ctx, np)
		Expect(err).ToNot(HaveOccurred())
		for _, it := range its {
			for _, offering := range it.Offerings {
				Expect(offering.CapacityType()).ToNot(Equal(karpv1.CapacityTypeReserved))
			}
		}
	})
})
