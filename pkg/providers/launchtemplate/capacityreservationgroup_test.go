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

package launchtemplate_test

import (
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
)

var _ = Describe("Capacity reservation group launch templates", func() {
	const groupARN = "arn:aws:resource-groups:us-west-2:123456789012:group/workload-a"
	var options *amifamily.LaunchTemplate
	BeforeEach(func() {
		options = &amifamily.LaunchTemplate{
			Options: &amifamily.Options{}, MetadataOptions: &v1.MetadataOptions{},
			CapacityType: karpv1.CapacityTypeOnDemand, CapacityReservationGroupARN: groupARN,
		}
	})
	It("should target a group without opting out or requiring reserved-only capacity", func() {
		input := launchtemplate.NewCreateLaunchTemplateInputBuilder(options, corev1.IPv4Protocol, "").Build(ctx)
		spec := input.LaunchTemplateData.CapacityReservationSpecification
		Expect(spec.CapacityReservationPreference).To(BeEmpty())
		Expect(spec.CapacityReservationTarget.CapacityReservationResourceGroupArn).To(Equal(lo.ToPtr(groupARN)))
		Expect(spec.CapacityReservationTarget.CapacityReservationId).To(BeNil())
		Expect(input.LaunchTemplateData.InstanceMarketOptions).To(BeNil())
	})
	It("should produce separate cached templates for distinct group targets", func() {
		original := launchtemplate.LaunchTemplateName(options)
		options.CapacityReservationGroupARN = "arn:aws:resource-groups:us-west-2:123456789012:group/workload-b"
		Expect(launchtemplate.LaunchTemplateName(options)).ToNot(Equal(original))
	})
	It("should keep Spot out of group matching", func() {
		options.CapacityType = karpv1.CapacityTypeSpot
		input := launchtemplate.NewCreateLaunchTemplateInputBuilder(options, corev1.IPv4Protocol, "").Build(ctx)
		Expect(input.LaunchTemplateData.CapacityReservationSpecification.CapacityReservationPreference).To(Equal(ec2types.CapacityReservationPreferenceNone))
		Expect(input.LaunchTemplateData.CapacityReservationSpecification.CapacityReservationTarget).To(BeNil())
	})
	It("should preserve reservation-ID targeting", func() {
		options.CapacityType = karpv1.CapacityTypeReserved
		options.CapacityReservationID = "cr-12345678"
		input := launchtemplate.NewCreateLaunchTemplateInputBuilder(options, corev1.IPv4Protocol, "").Build(ctx)
		spec := input.LaunchTemplateData.CapacityReservationSpecification
		Expect(spec.CapacityReservationPreference).To(Equal(ec2types.CapacityReservationPreferenceCapacityReservationsOnly))
		Expect(spec.CapacityReservationTarget.CapacityReservationId).To(Equal(lo.ToPtr("cr-12345678")))
		Expect(spec.CapacityReservationTarget.CapacityReservationResourceGroupArn).To(BeNil())
	})
})
