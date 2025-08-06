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

package instance_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var cloudProvider *cloudprovider.CloudProvider

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(test.DisableCapacityReservationIDValidation(apis.CRDs)...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider, awsEnv.InstanceTypeStore)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv.Reset()
})

var _ = Describe("InstanceProvider", func() {
	var nodeClass *v1.EC2NodeClass
	var nodePool *karpv1.NodePool
	var nodeClaim *karpv1.NodeClaim
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass()
		nodePool = coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
					},
				},
			},
		})
		nodeClaim = coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					karpv1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass) // Hydrate the subnet cache
		Expect(err).To(BeNil())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
	})
	It("should return an ICE error when all attempted instance types return an ICE error", func() {
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
			{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
			{CapacityType: karpv1.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: karpv1.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
		})
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).ToNot(HaveOccurred())

		// Filter down to a single instance type
		instanceTypes = lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool { return i.Name == "m5.xlarge" })

		// Since all the capacity pools are ICEd. This should return back an ICE error
		instance, err := awsEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, nil, instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())

		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1b", karpv1.CapacityTypeOnDemand)).To(BeFalse())

		// Try creating again for on-demand
		instanceTypes, err = cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).ToNot(HaveOccurred())

		// Filter down to a single instance type
		instanceTypes = lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool { return i.Name == "m5.xlarge" })

		instance, err = awsEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, nil, instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())

		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1b", karpv1.CapacityTypeOnDemand)).To(BeTrue())
	})
	It("should return an ICE error when spot instances are used and SpotSLR can't be created", func() {
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		awsEnv.EC2API.CreateFleetBehavior.Output.Set(&ec2.CreateFleetOutput{
			Errors: []ec2types.CreateFleetError{
				{
					ErrorCode:    lo.ToPtr("AuthFailure.ServiceLinkedRoleCreationNotPermitted"),
					ErrorMessage: lo.ToPtr("The provided credentials do not have permission to create the service-linked role for EC2 Spot Instances."),
					LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
						Overrides: &ec2types.FleetLaunchTemplateOverrides{
							InstanceType:     "m5.xlarge",
							AvailabilityZone: lo.ToPtr("test-zone-1a"),
						},
					},
				},
				{
					ErrorCode:    lo.ToPtr("AuthFailure.ServiceLinkedRoleCreationNotPermitted"),
					ErrorMessage: lo.ToPtr("The provided credentials do not have permission to create the service-linked role for EC2 Spot Instances."),
					LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
						Overrides: &ec2types.FleetLaunchTemplateOverrides{
							InstanceType:     "m5.xlarge",
							AvailabilityZone: lo.ToPtr("test-zone-1b"),
						},
					},
				},
			},
		})
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).ToNot(HaveOccurred())

		// Filter down to a single instance type
		instanceTypes = lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool {
			return i.Name == "m5.xlarge"
		})

		// Since all the capacity pools are ICEd. This should return back an ICE error
		instance, err := awsEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, nil, instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())

		// Capacity should get ICEd when this error is received
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.large", "test-zone-1a", karpv1.CapacityTypeSpot)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.large", "test-zone-1b", karpv1.CapacityTypeSpot)).To(BeTrue())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.xlarge", "test-zone-1b", karpv1.CapacityTypeOnDemand)).To(BeFalse())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.large", "test-zone-1a", karpv1.CapacityTypeOnDemand)).To(BeFalse())
		Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable("m5.large", "test-zone-1b", karpv1.CapacityTypeOnDemand)).To(BeFalse())

		// Expect that an event is fired for Spot SLR not being created
		awsEnv.EventRecorder.DetectedEvent(`Attempted to launch a spot instance but failed due to "AuthFailure.ServiceLinkedRoleCreationNotPermitted"`)
	})
	It("should return an ICE error when all attempted instance types return a ReservedCapacityReservation error", func() {
		const targetReservationID = "cr-m5.large-1a-1"
		// Ensure that Karpenter believes a reservation is available, but the API returns no capacity when attempting to launch
		awsEnv.CapacityReservationProvider.SetAvailableInstanceCount(targetReservationID, 1)
		awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(&ec2.DescribeCapacityReservationsOutput{
			CapacityReservations: []ec2types.CapacityReservation{
				{
					AvailabilityZone:       lo.ToPtr("test-zone-1a"),
					InstanceType:           lo.ToPtr("m5.large"),
					OwnerId:                lo.ToPtr("012345678901"),
					InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
					CapacityReservationId:  lo.ToPtr(targetReservationID),
					AvailableInstanceCount: lo.ToPtr[int32](0),
					State:                  ec2types.CapacityReservationStateActive,
					ReservationType:        ec2types.CapacityReservationTypeDefault,
				},
			},
		})
		nodeClass.Status.CapacityReservations = append(nodeClass.Status.CapacityReservations, v1.CapacityReservation{
			ID:                    "cr-m5.large-1a-1",
			AvailabilityZone:      "test-zone-1a",
			InstanceMatchCriteria: string(ec2types.InstanceMatchCriteriaTargeted),
			InstanceType:          "m5.large",
			OwnerID:               "012345678901",
			State:                 v1.CapacityReservationStateActive,
			ReservationType:       v1.CapacityReservationTypeDefault,
		})
		nodeClaim.Spec.Requirements = append(
			nodeClaim.Spec.Requirements,
			karpv1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      karpv1.CapacityTypeLabelKey,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{karpv1.CapacityTypeReserved},
			}},
		)
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool, nodeClass)

		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).ToNot(HaveOccurred())
		instance, err := awsEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, nil, instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())

		// Ensure we marked the reservation as unavailable after encountering the error
		Expect(awsEnv.CapacityReservationProvider.GetAvailableInstanceCount(targetReservationID)).To(Equal(0))
	})
	It("should treat instances which launched into open ODCRs as on-demand when the ReservedCapacity gate is disabled", func() {
		id := fake.InstanceID()
		awsEnv.EC2API.DescribeInstancesBehavior.Output.Set(&ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{{
				Instances: []ec2types.Instance{{
					State:                 &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
					PrivateDnsName:        lo.ToPtr(fake.PrivateDNSName()),
					Placement:             &ec2types.Placement{AvailabilityZone: lo.ToPtr(fake.DefaultRegion)},
					LaunchTime:            lo.ToPtr(time.Now().Add(-time.Minute)),
					InstanceId:            &id,
					InstanceType:          "m5.large",
					CapacityReservationId: lo.ToPtr("cr-foo"),
				}},
			}},
		})

		coreoptions.FromContext(ctx).FeatureGates.ReservedCapacity = false
		nodeClaims, err := cloudProvider.List(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeClaims).To(HaveLen(1))
		Expect(nodeClaims[0].Status.ProviderID).To(ContainSubstring(id))
		Expect(nodeClaims[0].Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
		Expect(nodeClaims[0].Labels).ToNot(HaveKey(v1.LabelCapacityReservationID))
	})
	It("should return all NodePool-owned instances from List", func() {
		ids := sets.New[string]()
		// Provision instances that have the karpenter.sh/nodepool key
		for i := 0; i < 20; i++ {
			instanceID := fake.InstanceID()
			awsEnv.EC2API.Instances.Store(
				instanceID,
				ec2types.Instance{
					State: &ec2types.InstanceState{
						Name: ec2types.InstanceStateNameRunning,
					},
					Tags: []ec2types.Tag{
						{
							Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName)),
							Value: aws.String("owned"),
						},
						{
							Key:   aws.String(karpv1.NodePoolLabelKey),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(v1.LabelNodeClass),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(v1.EKSClusterNameTagKey),
							Value: aws.String(options.FromContext(ctx).ClusterName),
						},
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2types.Placement{
						AvailabilityZone: aws.String(fake.DefaultRegion),
					},
					// Launch time was 1m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   lo.ToPtr(instanceID),
					InstanceType: "m5.large",
				},
			)
			ids.Insert(instanceID)
		}
		// Provision instances that do not have this tag key
		for i := 0; i < 20; i++ {
			instanceID := fake.InstanceID()
			awsEnv.EC2API.Instances.Store(
				instanceID,
				ec2types.Instance{
					State: &ec2types.InstanceState{
						Name: ec2types.InstanceStateNameRunning,
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2types.Placement{
						AvailabilityZone: aws.String(fake.DefaultRegion),
					},
					// Launch time was 1m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   lo.ToPtr(instanceID),
					InstanceType: "m5.large",
				},
			)
		}
		instances, err := awsEnv.InstanceProvider.List(ctx)
		Expect(err).To(BeNil())
		Expect(instances).To(HaveLen(20))

		retrievedIDs := sets.New[string](lo.Map(instances, func(i *instance.Instance, _ int) string { return i.ID })...)
		Expect(ids.Equal(retrievedIDs)).To(BeTrue())
	})
	It("should mark subnets as unavailable when they run out of IPs", func() {
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		awsEnv.EC2API.CreateFleetBehavior.Output.Set(&ec2.CreateFleetOutput{
			Errors: []ec2types.CreateFleetError{
				{
					ErrorCode:    lo.ToPtr("InsufficientFreeAddressesInSubnet"),
					ErrorMessage: lo.ToPtr("There are insufficient free addresses in that subnet to run instance"),
					LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
						Overrides: &ec2types.FleetLaunchTemplateOverrides{
							InstanceType:     "m5.xlarge",
							AvailabilityZone: lo.ToPtr("test-zone-1a"),
						},
					},
				},
			},
		})
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).ToNot(HaveOccurred())

		// We expect to treat that error as an ICE
		instance, err := awsEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, nil, instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())

		// We should have set the zone used in the request as unavailable for all instance types
		for _, instance := range instanceTypes {
			Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable(ec2types.InstanceType(instance.Name), "test-zone-1a", "on-demand")).To(BeTrue())
		}
		// But we should not have set the other zones as unavailable
		zones := []string{"test-zone-1b", "test-zone-1c"}
		for _, zone := range zones {
			for _, instance := range instanceTypes {
				Expect(awsEnv.UnavailableOfferingsCache.IsUnavailable(ec2types.InstanceType(instance.Name), zone, "on-demand")).To(BeFalse())
			}
		}
	})
})
