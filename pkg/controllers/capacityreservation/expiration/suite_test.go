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

package expiration_test

import (
	"context"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/option"
	"github.com/imdario/mergo"
	"github.com/samber/lo"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/capacityreservation/expiration"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var controller *expiration.Controller

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CapacityReservationExpiration")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...), coretest.WithFieldIndexers(coretest.NodeProviderIDFieldIndexer(ctx)))
	ctx = options.ToContext(ctx, test.Options())
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)

	cloudProvider := cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider)
	controller = expiration.NewController(awsEnv.Clock, env.Client, cloudProvider, awsEnv.CapacityReservationProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Capacity Reservation Expiration Controller", func() {
	BeforeEach(func() {
		awsEnv.Clock.SetTime(time.Now())
	})
	It("should delete nodeclaims associated with expiring capacity-block reservations", func() {
		crs := []ec2types.CapacityReservation{
			makeCapacityReservation(
				"cr-expiring",
				withEndTime(awsEnv.Clock.Now().Add(time.Minute*39)),
				withReservationType(ec2types.CapacityReservationTypeCapacityBlock),
			),
			makeCapacityReservation(
				"cr-active",
				withEndTime(awsEnv.Clock.Now().Add(time.Minute*60)),
				withReservationType(ec2types.CapacityReservationTypeCapacityBlock),
			),
		}
		awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(&ec2.DescribeCapacityReservationsOutput{
			CapacityReservations: crs,
		})
		ncs := lo.Map(crs, func(cr ec2types.CapacityReservation, _ int) *karpv1.NodeClaim {
			return coretest.NodeClaim(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: *cr.CapacityReservationId,
					Labels: map[string]string{
						v1.LabelCapacityReservationID: *cr.CapacityReservationId,
					},
				},
			})
		})
		for _, nc := range ncs {
			ExpectApplied(ctx, env.Client, nc)
		}
		ExpectSingletonReconciled(ctx, controller)
		ncs = ExpectNodeClaims(ctx, env.Client)
		Expect(ncs).To(HaveLen(1))
		Expect(ncs[0].Name).To(Equal("cr-active"))
	})
	It("should not delete nodeclaims associated with standard capacity reservations within the capacity block expiration window", func() {
		awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(&ec2.DescribeCapacityReservationsOutput{
			CapacityReservations: []ec2types.CapacityReservation{
				makeCapacityReservation("cr-default", withEndTime(awsEnv.Clock.Now().Add(time.Minute*39))),
			},
		})
		nc := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.LabelCapacityReservationID: "cr-default",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nc)
		ExpectSingletonReconciled(ctx, controller)
		ncs := ExpectNodeClaims(ctx, env.Client)
		Expect(ncs).To(HaveLen(1))
	})
})

type mockCapacityReservationOpts = option.Function[ec2types.CapacityReservation]

func withReservationType(crt ec2types.CapacityReservationType) mockCapacityReservationOpts {
	return func(cr *ec2types.CapacityReservation) {
		cr.ReservationType = crt
	}
}

func withEndTime(t time.Time) mockCapacityReservationOpts {
	return func(cr *ec2types.CapacityReservation) {
		cr.EndDate = lo.ToPtr(t)
	}
}

func makeCapacityReservation(id string, opts ...mockCapacityReservationOpts) ec2types.CapacityReservation {
	cr := ec2types.CapacityReservation{
		AvailabilityZone:       lo.ToPtr("test-zone-1a"),
		InstanceType:           lo.ToPtr("m5.large"),
		OwnerId:                lo.ToPtr("012345678901"),
		InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
		CapacityReservationId:  &id,
		AvailableInstanceCount: lo.ToPtr[int32](1),
		State:                  ec2types.CapacityReservationStateActive,
		ReservationType:        ec2types.CapacityReservationTypeDefault,
	}
	lo.Must0(mergo.Merge(&cr, option.Resolve(opts...), mergo.WithOverride))
	return cr
}
