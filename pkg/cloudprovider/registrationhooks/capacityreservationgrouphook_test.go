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

package registrationhooks_test

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider/registrationhooks"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
)

var _ = Describe("CapacityReservationGroupRegistrationHook", func() {
	const groupARN = "arn:aws:resource-groups:us-west-2:123456789012:group/workload-a"
	var hook *registrationhooks.CapacityReservationGroupRegistrationHook
	var nc *karpv1.NodeClaim
	var output *ec2.DescribeInstancesOutput
	BeforeEach(func() {
		hook = registrationhooks.NewCapacityReservationGroupRegistrationHook(awsEnv.InstanceProvider)
		nc = coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1.AnnotationCapacityReservationGroupARN: groupARN}},
			Status:     karpv1.NodeClaimStatus{ProviderID: "aws:///test-zone-1a/i-12345678"},
		})
		output = &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{
			InstanceId: lo.ToPtr("i-12345678"), InstanceType: ec2types.InstanceTypeM5Large,
			ImageId: lo.ToPtr("ami-test1"), State: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			Placement: &ec2types.Placement{AvailabilityZone: lo.ToPtr("test-zone-1a")},
			CapacityReservationSpecification: &ec2types.CapacityReservationSpecificationResponse{
				CapacityReservationTarget: &ec2types.CapacityReservationTargetResponse{CapacityReservationResourceGroupArn: lo.ToPtr(groupARN)},
			},
		}}}}}
		awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(output)
	})
	DescribeTable("should observe initial coverage before registration", func(reservationID string, capacityType string) {
		if reservationID != "" {
			output.Reservations[0].Instances[0].CapacityReservationId = lo.ToPtr(reservationID)
		}
		awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(output)
		nc.Labels[corecloudprovider.ReservationIDLabel] = "cr-stale"
		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
		Expect(nc.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, capacityType))
		Expect(nc.Annotations).To(HaveKeyWithValue(v1.AnnotationCapacityReservationGroupARN, groupARN))
		if reservationID == "" {
			Expect(nc.Labels).ToNot(HaveKey(corecloudprovider.ReservationIDLabel))
		} else {
			Expect(nc.Labels).To(HaveKeyWithValue(corecloudprovider.ReservationIDLabel, reservationID))
		}
		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeZero())
	}, Entry("covered", "cr-12345678", karpv1.CapacityTypeReserved), Entry("On-Demand fallback", "", karpv1.CapacityTypeOnDemand))
	It("should resolve placement and reservation metadata through one serialized AWS hook", func() {
		nc.Labels[v1.LabelPlacementGroupID] = "pg-12345678"
		nc.Labels[v1.LabelPlacementGroupPartition] = ""
		output.Reservations[0].Instances[0].Placement.PartitionNumber = lo.ToPtr[int32](2)
		output.Reservations[0].Instances[0].CapacityReservationId = lo.ToPtr("cr-12345678")
		awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(output)
		result, err := registrationhooks.NewAWSRegistrationHook(awsEnv.InstanceProvider).Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
		Expect(nc.Labels).To(HaveKeyWithValue(v1.LabelPlacementGroupPartition, "2"))
		Expect(nc.Labels).To(HaveKeyWithValue(corecloudprovider.ReservationIDLabel, "cr-12345678"))
	})
	It("should defer registration until group target data is visible", func() {
		output.Reservations[0].Instances[0].CapacityReservationSpecification = nil
		awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(output)
		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
	})
	It("should retry failed observations without losing intent or launching again", func() {
		awsEnv.EC2API.DescribeInstancesBehavior.Error.Set(fmt.Errorf("throttled"), fake.MaxCalls(10))
		_, err := hook.Registered(ctx, nc)
		Expect(err).To(HaveOccurred())
		Expect(nc.Annotations).To(HaveKeyWithValue(v1.AnnotationCapacityReservationGroupARN, groupARN))
		awsEnv.EC2API.DescribeInstancesBehavior.Error.Reset()
		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeZero())
	})
	It("should skip nodes without a group target", func() {
		delete(nc.Annotations, v1.AnnotationCapacityReservationGroupARN)
		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
		Expect(awsEnv.EC2API.DescribeInstancesBehavior.CalledWithInput.Len()).To(BeZero())
	})
	It("should defer registration until the provider ID is available", func() {
		nc.Status.ProviderID = ""
		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
		Expect(awsEnv.EC2API.DescribeInstancesBehavior.CalledWithInput.Len()).To(BeZero())
	})
})
