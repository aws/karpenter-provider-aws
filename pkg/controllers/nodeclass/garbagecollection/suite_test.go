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
	"fmt"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"

	"github.com/aws/aws-sdk-go-v2/aws"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/garbagecollection"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	ctx           context.Context
	env           *coretest.Environment
	awsEnv        *test.Environment
	gcController  *garbagecollection.Controller
	nodeClass     *v1.EC2NodeClass
	cloudProvider *cloudprovider.CloudProvider
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Instance Profile GarbageCollection")
}

var _ = BeforeSuite(func() {

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
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {

	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	nodeClass = test.EC2NodeClass()
	awsEnv.Reset()
	Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
	Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	ExpectDeleted(ctx, env.Client, nodeClass)
})

var _ = Describe("Instance Profile GarbageCollection", func() {
	It("should not delete active profiles", func() {
		ExpectApplied(ctx, env.Client, nodeClass)

		// Manually add profile to IAMAPI InstanceProfiles map
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Path:                lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, nodeClass.UID)),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}

		// Create NodeClaim to ensure created profile is Active
		nodeClaim := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.AnnotationInstanceProfile: profileName,
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

		// Ensure profile is not protected
		awsEnv.InstanceProfileProvider.SetProtectedState(profileName, false)

		// Run GC
		ExpectSingletonReconciled(ctx, gcController)

		// Verify profile not deleted
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(profileName))
	})

	It("should not delete current profiles", func() {
		ExpectApplied(ctx, env.Client, nodeClass)

		// Manually add profile to IAMAPI InstanceProfiles map
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Path:                lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, nodeClass.UID)),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}

		// Ensure profile is current
		nodeClass.Status.InstanceProfile = profileName
		ExpectApplied(ctx, env.Client, nodeClass)

		// Ensure profile is not protected
		awsEnv.InstanceProfileProvider.SetProtectedState(profileName, false)

		// Run GC
		ExpectSingletonReconciled(ctx, gcController)

		// Verify profile not deleted
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(profileName))
	})

	It("should not delete protected profiles", func() {
		ExpectApplied(ctx, env.Client, nodeClass)

		// Manually add profile to IAMAPI InstanceProfiles map
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Path:                lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, nodeClass.UID)),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}

		// Ensure profile is protected
		awsEnv.InstanceProfileProvider.SetProtectedState(profileName, true)

		// Run GC
		ExpectSingletonReconciled(ctx, gcController)

		// Verify profile not deleted
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(profileName))
	})

	It("should delete inactive profiles which are not current or protected", func() {
		ExpectApplied(ctx, env.Client, nodeClass)

		// Manually add profile to IAMAPI InstanceProfiles map
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Path:                lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, nodeClass.UID)),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}

		// Ensure profile is not protected
		awsEnv.InstanceProfileProvider.SetProtectedState(profileName, false)

		// Run GC
		ExpectSingletonReconciled(ctx, gcController)

		// Verify profile not deleted
		Expect(awsEnv.IAMAPI.InstanceProfiles).ToNot(HaveKey(profileName))
	})

	It("should requeue after 30 minutes for a successful run", func() {
		// Run GC with no profiles to clean up
		result, err := gcController.Reconcile(ctx)
		Expect(err).To(BeNil())

		// Verify requeue time is 30 minutes
		Expect(result.RequeueAfter).To(Equal(30 * time.Minute))
	})

	It("should requeue immediately on deletion failure", func() {
		ExpectApplied(ctx, env.Client, nodeClass)

		// Add a profile that should be deleted
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Path:                lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, nodeClass.UID)),
			},
		}

		// Make DeleteInstanceProfile fail
		awsEnv.IAMAPI.DeleteInstanceProfileBehavior.Error.Set(fmt.Errorf("get failed"))

		// Run GC
		result, err := gcController.Reconcile(ctx)

		// Should return error from deletion
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("get failed"))

		// Should requeue immediately (no delay)
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

		// Profile should still exist since deletion failed
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(profileName))
	})
})
