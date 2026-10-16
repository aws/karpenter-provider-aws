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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
)

var _ = Describe("Running group-node observations", func() {
	const groupARN = "arn:aws:resource-groups:us-west-2:123456789012:group/workload-a"
	var nc *karpv1.NodeClaim
	var output *ec2.DescribeInstancesOutput
	BeforeEach(func() {
		nc = &karpv1.NodeClaim{ObjectMeta: metav1.ObjectMeta{Name: "group-node", Annotations: map[string]string{v1.AnnotationCapacityReservationGroupARN: groupARN}}, Status: karpv1.NodeClaimStatus{ProviderID: "aws:///test-zone-1a/i-12345678"}}
		output = &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{
			InstanceId: lo.ToPtr("i-12345678"), InstanceType: ec2types.InstanceTypeM5Large,
			ImageId: lo.ToPtr("ami-test1"), State: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			Placement:                        &ec2types.Placement{AvailabilityZone: lo.ToPtr("test-zone-1a")},
			CapacityReservationSpecification: &ec2types.CapacityReservationSpecificationResponse{CapacityReservationTarget: &ec2types.CapacityReservationTargetResponse{CapacityReservationResourceGroupArn: lo.ToPtr(groupARN)}},
		}}}}}
		awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(output)
	})
	It("should refresh promotion, reassociation, and demotion without a NodeClass or Kubernetes label update", func() {
		odPrice, found := awsEnv.PricingProvider.OnDemandPrice(ec2types.InstanceTypeM5Large)
		Expect(found).To(BeTrue())
		for _, reservation := range []string{"", "cr-12345678", "cr-87654321", ""} {
			output.Reservations[0].Instances[0].CapacityReservationId = lo.ToPtr(reservation)
			awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(output)
			before := time.Now()
			states, err := cloudProvider.RefreshNodeClaimStates(ctx, []*karpv1.NodeClaim{nc})
			Expect(err).ToNot(HaveOccurred())
			observed := states[nc.Status.ProviderID]
			Expect(observed.ValidAt(time.Now())).To(BeTrue())
			Expect(observed.ObservedAt.Before(before)).To(BeFalse())
			Expect(observed.Scope).To(Equal(groupARN))
			if reservation == "" {
				Expect(observed.Price).To(Equal(odPrice))
				Expect(observed.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
				Expect(observed.Labels).ToNot(HaveKey(corecloudprovider.ReservationIDLabel))
			} else {
				Expect(observed.Price).To(Equal(odPrice / 10_000_000.0))
				Expect(observed.Labels).To(HaveKeyWithValue(corecloudprovider.ReservationIDLabel, reservation))
			}
		}
		Expect(awsEnv.EC2API.DescribeInstancesBehavior.CalledWithInput.Len()).To(Equal(4))
		Expect(nc.Labels).To(BeEmpty())
	})
	It("should retain scope but provide no fresh price on throttling or incomplete target data", func() {
		output.Reservations[0].Instances[0].CapacityReservationSpecification = nil
		awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(output)
		states, err := cloudProvider.RefreshNodeClaimStates(ctx, []*karpv1.NodeClaim{nc})
		Expect(err).To(HaveOccurred())
		Expect(states[nc.Status.ProviderID].Scope).To(Equal(groupARN))
		Expect(states[nc.Status.ProviderID].ValidAt(time.Now())).To(BeFalse())
		awsEnv.EC2API.DescribeInstancesBehavior.Error.Set(fmt.Errorf("throttled"), fake.MaxCalls(10))
		states, err = cloudProvider.RefreshNodeClaimStates(ctx, []*karpv1.NodeClaim{nc})
		Expect(err).To(HaveOccurred())
		Expect(states[nc.Status.ProviderID].ValidAt(time.Now())).To(BeFalse())
	})
	It("should skip stable nodes without issuing EC2 requests", func() {
		nc.Annotations = nil
		states, err := cloudProvider.RefreshNodeClaimStates(ctx, []*karpv1.NodeClaim{nc})
		Expect(err).ToNot(HaveOccurred())
		Expect(states).To(BeEmpty())
		Expect(awsEnv.EC2API.DescribeInstancesBehavior.CalledWithInput.Len()).To(BeZero())
	})
	It("should defer observations when reserved capacity is disabled", func() {
		opts := *coreoptions.FromContext(ctx)
		opts.FeatureGates.ReservedCapacity = false
		states, err := cloudProvider.RefreshNodeClaimStates(coreoptions.ToContext(ctx, &opts), []*karpv1.NodeClaim{nc})
		Expect(err).To(HaveOccurred())
		Expect(states[nc.Status.ProviderID].ValidAt(time.Now())).To(BeFalse())
		Expect(awsEnv.EC2API.DescribeInstancesBehavior.CalledWithInput.Len()).To(BeZero())
	})
})
