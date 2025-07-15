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

package garbagecollection_test

import (
	"context"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	clock "k8s.io/utils/clock/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/garbagecollection"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	ctx                 context.Context
	env                 *coretest.Environment
	awsEnv              *test.Environment
	fakeClock           *clock.FakeClock
	gcController        *garbagecollection.Controller
	nodeClassController *nodeclass.Controller
	nodeClass           *v1.EC2NodeClass
	cloudProvider       *cloudprovider.CloudProvider
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Instance Profile GarbageCollection")
}

var _ = BeforeSuite(func() {
	fakeClock = clock.NewFakeClock(time.Now())

	// Setup core environment with necessary CRDs
	env = coretest.NewEnvironment(
		coretest.WithCRDs(test.DisableCapacityReservationIDValidation(test.RemoveNodeClassTagValidation(apis.CRDs))...),
		coretest.WithCRDs(v1alpha1.CRDs...),
		coretest.WithFieldIndexers(coretest.NodeClaimNodeClassRefFieldIndexer(ctx)),
		coretest.WithFieldIndexers(coretest.NodePoolNodeClassRefFieldIndexer(ctx)),
	)

	// Setup context with options
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{
		FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)},
	}))
	ctx = options.ToContext(ctx, test.Options())

	// Setup AWS environment
	awsEnv = test.NewEnvironment(ctx, env)

	// Ensure we use a fake clock when adding to instance profile time in-memory map
	awsEnv.InstanceProfileProvider.SetClock(fakeClock)

	// Create cloudporivder
	cloudProvider = cloudprovider.New(
		awsEnv.InstanceTypesProvider,
		awsEnv.InstanceProvider,
		events.NewRecorder(&record.FakeRecorder{}),
		env.Client,
		awsEnv.AMIProvider,
		awsEnv.SecurityGroupProvider,
		awsEnv.CapacityReservationProvider,
	)

	gcController = garbagecollection.NewController(
		env.Client,
		cloudProvider,
		awsEnv.InstanceProfileProvider,
		fake.DefaultRegion,
	)
	nodeClassController = nodeclass.NewController(
		awsEnv.Clock,
		env.Client,
		cloudProvider,
		events.NewRecorder(&record.FakeRecorder{}),
		fake.DefaultRegion,
		awsEnv.SubnetProvider,
		awsEnv.SecurityGroupProvider,
		awsEnv.AMIProvider,
		awsEnv.InstanceProfileProvider,
		awsEnv.InstanceTypesProvider,
		awsEnv.LaunchTemplateProvider,
		awsEnv.CapacityReservationProvider,
		awsEnv.EC2API,
		awsEnv.ValidationCache,
		awsEnv.AMIResolver,
	)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	fakeClock.SetTime(time.Now())

	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	nodeClass = test.EC2NodeClass()
	awsEnv.Reset()
	Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
	Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Instance Profile GarbageCollection", func() {
	It("should not delete active profiles", func() {
		// Create NodeClass with role-A
		nodeClass.Spec.Role = "role-A"

		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, nodeClassController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		oldProfile := nodeClass.Status.InstanceProfile

		// Create NodeClaim using profile
		nodeClaim := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.AnnotationInstanceProfile: nodeClass.Status.InstanceProfile,
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
		ExpectApplied(ctx, env.Client, nodeClaim)

		fakeClock.SetTime(fakeClock.Now().Add(-2 * time.Minute))

		nodeClass.Spec.Role = "role-B"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, nodeClassController, nodeClass)

		// Run GC
		ExpectSingletonReconciled(ctx, gcController)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		// Verify profile not deleted
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(oldProfile))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(nodeClass.Status.InstanceProfile))
	})

	It("should delete inactive profiles after grace period", func() {
		// Create NodeClass with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, nodeClassController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		oldProfile := nodeClass.Status.InstanceProfile

		// Update to role-B
		fakeClock.SetTime(fakeClock.Now().Add(-2 * time.Minute))
		nodeClass.Spec.Role = "role-B"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, nodeClassController, nodeClass)

		// fakeClock.Step(2 * time.Minute) // > 1 minute grace period

		// Run GC
		ExpectSingletonReconciled(ctx, gcController)

		// Verify old profile deleted
		Expect(awsEnv.IAMAPI.InstanceProfiles).ToNot(HaveKey(oldProfile))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(nodeClass.Status.InstanceProfile))

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
	})

	// Continue with more tests....

	// Context("Race Conditions", func() {
	// 	It("should handle concurrent role changes", func() {
	// 		// count := 0
	// 		// instanceprofile.CreationTimeMap.Range(func(key, value interface{}) bool {
	// 		// 	count++
	// 		// 	return true // Continue iteration
	// 		// })
	// 		// log.Printf("NOT GOOD:%d", count)

	// 		// Create NodeClass with role-A
	// 		nodeClass.Spec.Role = "role-A"
	// 		ExpectApplied(ctx, env.Client, nodeClass)
	// 		ExpectObjectReconciled(ctx, env.Client, nodeClassController, nodeClass)
	// 		// nodeClass = ExpectExists(ctx, env.Client, nodeClass)
	// 		// log.Printf("HA: %s", nodeClass.Status.InstanceProfile)

	// 		// Create NodeClaim during transition
	// 		// nodeClaim := coretest.NodeClaim(karpv1.NodeClaim{
	// 		// 	ObjectMeta: metav1.ObjectMeta{
	// 		// 		Annotations: map[string]string{
	// 		// 			v1.AnnotationInstanceProfile: nodeClass.Status.InstanceProfile,
	// 		// 		},
	// 		// 	},
	// 		// 	Spec: karpv1.NodeClaimSpec{
	// 		// 		NodeClassRef: &karpv1.NodeClassReference{
	// 		// 			Group: object.GVK(nodeClass).Group,
	// 		// 			Kind:  object.GVK(nodeClass).Kind,
	// 		// 			Name:  nodeClass.Name,
	// 		// 		},
	// 		// 	},
	// 		// })
	// 		// ExpectApplied(ctx, env.Client, nodeClaim)

	// 		// Update to role-B
	// 		nodeClass.Spec.Role = "role-B"
	// 		ExpectApplied(ctx, env.Client, nodeClass)
	// 		ExpectObjectReconciled(ctx, env.Client, nodeClassController, nodeClass)

	// 		// log.Printf("HA2: %s", nodeClass.Status.InstanceProfile)

	// 		// // Run GC before grace period
	// 		// t, ok := instanceprofile.GetCreationTime(nodeClass.Status.InstanceProfile)
	// 		// if !ok {
	// 		// 	log.Printf("RIP")
	// 		// }

	// 		// log.Printf("TIME MAP: %s", t)
	// 		// log.Printf("TIME NOW: %s", time.Now())
	// 		ExpectSingletonReconciled(ctx, gcController)

	// 		// Verify both profiles exist
	// 		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))
	// 	})

	// 	// 	It("should protect profiles during grace period", func() {
	// 	// 		// Create NodeClass with role-A
	// 	// 		nodeClass.Spec.Role = "role-A"
	// 	// 		ExpectApplied(ctx, env.Client, nodeClass)
	// 	// 		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
	// 	// 		oldProfile := nodeClass.Status.InstanceProfile

	// 	// 		// Update to role-B
	// 	// 		nodeClass.Spec.Role = "role-B"
	// 	// 		ExpectApplied(ctx, env.Client, nodeClass)
	// 	// 		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

	// 	// 		// Create NodeClaim during grace period
	// 	// 		nodeClaim := test.NodeClaim()
	// 	// 		nodeClaim.Annotations = map[string]string{
	// 	// 			v1.AnnotationInstanceProfile: oldProfile,
	// 	// 		}
	// 	// 		ExpectApplied(ctx, env.Client, nodeClaim)

	// 	// 		// Wait for grace period
	// 	// 		fakeClock.Step(2 * time.Minute)

	// 	// 		// Run GC
	// 	// 		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

	// 	// 		// Verify old profile protected
	// 	// 		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(oldProfile))

	// 	// 		// Delete NodeClaim
	// 	// 		ExpectDeleted(ctx, env.Client, nodeClaim)

	// 	// 		// Run GC again
	// 	// 		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

	// 	// 		// Verify old profile now deleted
	// 	// 		Expect(awsEnv.IAMAPI.InstanceProfiles).ToNot(HaveKey(oldProfile))
	// 	// 	})
	// 	// })

	// 	// Context("Edge Cases", func() {
	// 	// 	It("should handle missing tags", func() {
	// 	// 		// Create untagged profile
	// 	// 		profileName := "untagged-profile"
	// 	// 		awsEnv.IAMAPI.InstanceProfiles[profileName] = &iamtypes.InstanceProfile{
	// 	// 			InstanceProfileName: aws.String(profileName),
	// 	// 			Tags:                []iamtypes.Tag{},
	// 	// 		}

	// 	// 		// Run GC
	// 	// 		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

	// 	// 		// Verify untagged profile not deleted
	// 	// 		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(profileName))
	// 	// 	})

	// 	// 	It("should handle pre-upgrade NodeClaims", func() {
	// 	// 		// Create NodeClass
	// 	// 		nodeClass.Spec.Role = "role-A"
	// 	// 		ExpectApplied(ctx, env.Client, nodeClass)
	// 	// 		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

	// 	// 		// Create NodeClaim without instance profile annotation
	// 	// 		nodeClaim := test.NodeClaim()
	// 	// 		ExpectApplied(ctx, env.Client, nodeClaim)

	// 	// 		// Run GC
	// 	// 		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

	// 	// 		// Verify profile protected via old name format
	// 	// 		clusterName := options.FromContext(ctx).ClusterName
	// 	// 		hash := lo.Must(hashstructure.Hash(fmt.Sprintf("%s%s", fake.DefaultRegion, nodeClass.Name), hashstructure.FormatV2, nil))
	// 	// 		oldProfileName := fmt.Sprintf("%s_%d", clusterName, hash)
	// 	// 		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(oldProfileName))
	// 	// 	})
	// })
})
