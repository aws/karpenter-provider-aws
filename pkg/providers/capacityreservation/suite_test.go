package capacityreservation_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
	"github.com/samber/lo"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2NodeClass")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(
		coretest.WithCRDs(test.DisableCapacityReservationIDValidation(test.RemoveNodeClassTagValidation(apis.CRDs))...),
		coretest.WithCRDs(v1alpha1.CRDs...),
	)
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
})

// NOTE: Tests for different selector terms can be found in the nodeclass reconciler tests
var _ = Describe("Capacity Reservation Provider", func() {
	var discoveryTags map[string]string
	var reservations map[string]int

	BeforeEach(func() {
		discoveryTags = map[string]string{
			"karpenter.sh/discovery": "test",
		}
		crs := []ec2types.CapacityReservation{
			{
				AvailabilityZone:       lo.ToPtr("test-zone-1a"),
				InstanceType:           lo.ToPtr("m5.large"),
				OwnerId:                lo.ToPtr("012345678901"),
				InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
				CapacityReservationId:  lo.ToPtr("cr-m5.large-1a-1"),
				AvailableInstanceCount: lo.ToPtr[int32](10),
				Tags: utils.MergeTags(discoveryTags),
				State:                  ec2types.CapacityReservationStateActive,
			},
			{
				AvailabilityZone:       lo.ToPtr("test-zone-1a"),
				InstanceType:           lo.ToPtr("m5.large"),
				OwnerId:                lo.ToPtr("012345678901"),
				InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
				CapacityReservationId:  lo.ToPtr("cr-m5.large-1a-2"),
				AvailableInstanceCount: lo.ToPtr[int32](15),
				Tags: utils.MergeTags(discoveryTags),
				State:                  ec2types.CapacityReservationStateActive,
			},
		}
		awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(&ec2.DescribeCapacityReservationsOutput{
			CapacityReservations: crs,
		})
		reservations = make(map[string]int)
		for _, cr := range crs {
			reservations[*cr.CapacityReservationId] = int(*cr.AvailableInstanceCount)
		}
	})
	Context("Availability Cache", func() {
		It("should sync availability cache when listing reservations", func() {
			crs, err := awsEnv.CapacityReservationProvider.List(ctx, v1.CapacityReservationSelectorTerm{
				Tags: discoveryTags,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(crs).To(HaveLen(2))
			for id, count := range reservations {
				Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount(id)).To(Equal(count))
			}
		})
		It("should decrement availability when reservation is marked as launched", func() {
			awsEnv.CapacityReservationProvider.SetAvailableInstanceCount("cr-test", 5)
			awsEnv.CapacityReservationProvider.MarkLaunched("cr-test-2")
			Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount("cr-test")).To(Equal(5))
			awsEnv.CapacityReservationProvider.MarkLaunched("cr-test")
			Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount("cr-test")).To(Equal(4))
		})
		It("should increment availability when reservation is marked as terminated", func() {
			awsEnv.CapacityReservationProvider.SetAvailableInstanceCount("cr-test", 5)
			awsEnv.CapacityReservationProvider.MarkTerminated("cr-test-2")
			Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount("cr-test")).To(Equal(5))
			awsEnv.CapacityReservationProvider.MarkTerminated("cr-test")
			Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount("cr-test")).To(Equal(6))
		})
	})
})
