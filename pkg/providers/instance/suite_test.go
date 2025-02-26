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
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider)
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
				},
			},
		})
		nodeClass.Status.CapacityReservations = append(nodeClass.Status.CapacityReservations, v1.CapacityReservation{
			ID:                    "cr-m5.large-1a-1",
			AvailabilityZone:      "test-zone-1a",
			InstanceMatchCriteria: string(ec2types.InstanceMatchCriteriaTargeted),
			InstanceType:          "m5.large",
			OwnerID:               "012345678901",
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
	It("should filter compatible reserved offerings such that only one offering per capacity pool is included in the CreateFleet request", func() {
		const targetReservationID = "cr-m5.large-1a-2"
		awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(&ec2.DescribeCapacityReservationsOutput{
			CapacityReservations: []ec2types.CapacityReservation{
				{
					AvailabilityZone:       lo.ToPtr("test-zone-1a"),
					InstanceType:           lo.ToPtr("m5.large"),
					OwnerId:                lo.ToPtr("012345678901"),
					InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
					CapacityReservationId:  lo.ToPtr("cr-m5.large-1a-1"),
					AvailableInstanceCount: lo.ToPtr[int32](1),
					State:                  ec2types.CapacityReservationStateActive,
				},
				{
					AvailabilityZone:       lo.ToPtr("test-zone-1a"),
					InstanceType:           lo.ToPtr("m5.large"),
					OwnerId:                lo.ToPtr("012345678901"),
					InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
					CapacityReservationId:  lo.ToPtr(targetReservationID),
					AvailableInstanceCount: lo.ToPtr[int32](2),
					State:                  ec2types.CapacityReservationStateActive,
				},
			},
		})
		awsEnv.CapacityReservationProvider.SetAvailableInstanceCount("cr-m5.large-1a-1", 1)
		awsEnv.CapacityReservationProvider.SetAvailableInstanceCount(targetReservationID, 2)
		nodeClass.Status.CapacityReservations = append(nodeClass.Status.CapacityReservations, []v1.CapacityReservation{
			{
				ID:                    "cr-m5.large-1a-1",
				AvailabilityZone:      "test-zone-1a",
				InstanceMatchCriteria: string(ec2types.InstanceMatchCriteriaTargeted),
				InstanceType:          "m5.large",
				OwnerID:               "012345678901",
			},
			{
				ID:                    "cr-m5.large-1a-2",
				AvailabilityZone:      "test-zone-1a",
				InstanceMatchCriteria: string(ec2types.InstanceMatchCriteriaTargeted),
				InstanceType:          "m5.large",
				OwnerID:               "012345678901",
			},
		}...)

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
		Expect(err).ToNot(HaveOccurred())
		Expect(instance.CapacityType).To(Equal(karpv1.CapacityTypeReserved))
		Expect(instance.CapacityReservationID).To(Equal(targetReservationID))

		// We should have only created a single launch template, for the single capacity reservation we're attempting to launch
		var launchTemplates []*ec2.CreateLaunchTemplateInput
		for awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len() > 0 {
			launchTemplates = append(launchTemplates, awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Pop())
		}
		Expect(launchTemplates).To(HaveLen(1))
		Expect(*launchTemplates[0].LaunchTemplateData.CapacityReservationSpecification.CapacityReservationTarget.CapacityReservationId).To(Equal(targetReservationID))

		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).ToNot(Equal(0))
		createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(createFleetInput.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(ec2types.DefaultTargetCapacityTypeOnDemand))
		Expect(createFleetInput.LaunchTemplateConfigs).To(HaveLen(1))
		Expect(createFleetInput.LaunchTemplateConfigs[0].Overrides).To(HaveLen(1))
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
})
