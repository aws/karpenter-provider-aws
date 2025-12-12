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

package nodeclass_test

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass InstanceProfile Status Controller", func() {
	It("should create the instance profile when it doesn't exist", func() {

		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles[0].RoleName).To(Equal("role-A"))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Tags).To(ContainElements(
			iamtypes.Tag{Key: lo.ToPtr(fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName)), Value: lo.ToPtr("owned")},
			iamtypes.Tag{Key: lo.ToPtr(v1.LabelNodeClass), Value: lo.ToPtr(nodeClass.Name)},
			iamtypes.Tag{Key: lo.ToPtr(v1.EKSClusterNameTagKey), Value: lo.ToPtr(options.FromContext(ctx).ClusterName)},
		))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should delete all the instance profiles from cache when the nodeClass is deleted", func() {
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		nodeClass.Spec.Role = "role-B"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		nodeClass.Spec.Role = "role-C"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(3))
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())

		instanceProfileCount := 0
		for key := range awsEnv.InstanceProfileCache.Items() {
			if strings.HasPrefix(key, "instance-profile:") {
				instanceProfileCount++
			}
		}
		Expect(instanceProfileCount).To(Equal(3))

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))

		instanceProfileCount = 0
		for key := range awsEnv.InstanceProfileCache.Items() {
			if strings.HasPrefix(key, "instance-profile:") {
				instanceProfileCount++
			}
		}
		Expect(instanceProfileCount).To(Equal(0))
	})
	It("should add the role to the instance profile when it exists without a role", func() {
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
			},
		}

		nodeClass.Spec.Role = "role-A"
		nodeClass.Status.InstanceProfile = profileName

		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles[0].RoleName).To(Equal("role-A"))

		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})

	It("should update the role for the instance profile when the wrong role exists", func() {
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}

		nodeClass.Spec.Role = "other-role"
		nodeClass.Status.InstanceProfile = profileName
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles[0].RoleName).To(Equal("other-role"))

		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should not call CreateInstanceProfile or AddRoleToInstanceProfile when instance profile exists with correct role", func() {
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}

		nodeClass.Spec.Role = "role-A"
		nodeClass.Status.InstanceProfile = profileName
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("role-A"))

		Expect(awsEnv.IAMAPI.CreateInstanceProfileBehavior.Calls()).To(BeZero())
		Expect(awsEnv.IAMAPI.AddRoleToInstanceProfileBehavior.Calls()).To(BeZero())
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should resolve the specified instance profile into the status when using instanceProfile field", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(lo.FromPtr(nodeClass.Spec.InstanceProfile)))
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should not call the the IAM API when specifying an instance profile", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		Expect(awsEnv.IAMAPI.CreateInstanceProfileBehavior.Calls()).To(BeZero())
		Expect(awsEnv.IAMAPI.AddRoleToInstanceProfileBehavior.Calls()).To(BeZero())
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should create a new instance profile when spec.role changes", func() {
		// Initial setup with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		initialProfileName := nodeClass.Status.InstanceProfile

		// Verify initial profile
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[initialProfileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[initialProfileName].Roles[0].RoleName).To(Equal("role-A"))

		// Update to role-B
		nodeClass.Spec.Role = "role-B"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		newProfileName := nodeClass.Status.InstanceProfile

		// Verify both profiles exist but new one is used
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))
		Expect(newProfileName).NotTo(Equal(initialProfileName))
		Expect(awsEnv.IAMAPI.InstanceProfiles[newProfileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[newProfileName].Roles[0].RoleName).To(Equal("role-B"))
	})

	It("should mark new instance profile as protected", func() {
		// Create NodeClass with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		// Get profile name
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		profileName := nodeClass.Status.InstanceProfile

		// Verify profile exists and is protected
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.InstanceProfileProvider.IsProtected(profileName)).To(BeTrue())
	})

	It("should mark old instance profile as protected", func() {
		// Initial setup with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		// Get initial profile name
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		oldProfileName := nodeClass.Status.InstanceProfile

		// Remove the created profile from protectedProfiles cache to ensure
		// that it is again added once we switch to "role-B"
		awsEnv.InstanceProfileProvider.SetProtectedState(oldProfileName, false)
		Expect(awsEnv.InstanceProfileProvider.IsProtected(oldProfileName)).To(BeFalse())

		// Change to role-B
		nodeClass.Spec.Role = "role-B"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		// Verify old profile is still protected as well as new one
		Expect(awsEnv.InstanceProfileProvider.IsProtected(oldProfileName)).To(BeTrue())
	})

	It("should use the cached instance profile when spec.role is changed", func() {
		ExpectApplied(ctx, env.Client, nodeClass)

		// Set up cache with a profile for role-A
		cachedProfileName := "cached-profile"
		awsEnv.RecreationCache.SetDefault(fmt.Sprintf("%s/%s", "role-A", nodeClass.UID), cachedProfileName)

		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			cachedProfileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(cachedProfileName),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}

		// Apply NodeClass with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		// Verify it used the cached profile
		Expect(nodeClass.Status.InstanceProfile).To(Equal(cachedProfileName))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
	})

	It("should not use the cached instance profile if it was for a different role", func() {
		ExpectApplied(ctx, env.Client, nodeClass)

		// Set up cache with a profile for role-B
		cachedProfileName := "cached-profile"
		awsEnv.RecreationCache.SetDefault(fmt.Sprintf("%s/%s", "role-B", nodeClass.UID), cachedProfileName)

		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			cachedProfileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(cachedProfileName),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-B"),
					},
				},
			},
		}

		// Apply NodeClass with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		// Verify it created a new profile instead of using cached one
		Expect(nodeClass.Status.InstanceProfile).NotTo(Equal(cachedProfileName))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))
	})

	It("should not use the cached instance profile if it was for a different nodeclass", func() {
		// Set up cache with a profile but for a different nodeclass UID
		cachedProfileName := "cached-profile"
		awsEnv.RecreationCache.SetDefault(fmt.Sprintf("%s/%s", "role-A", "different-uid"), cachedProfileName)

		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			cachedProfileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(cachedProfileName),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}

		// Apply NodeClass with role-A but different UID
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		// Verify it created a new profile instead of using cached one
		Expect(nodeClass.Status.InstanceProfile).NotTo(Equal(cachedProfileName))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))
	})

	It("should update the cache when a new instance profile is created", func() {
		// Apply NodeClass with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		// Verify profile was created and cached
		newProfileName := nodeClass.Status.InstanceProfile
		cachedProfileName, ok := awsEnv.RecreationCache.Get(fmt.Sprintf("%s/%s", "role-A", nodeClass.UID))
		Expect(ok).To(BeTrue())
		Expect(cachedProfileName).To(Equal(newProfileName))
	})

	It("should mark old instance profile as protected when switching from spec.role to spec.instanceProfile", func() {
		// Initial setup with spec.role
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		// Get initial profile name
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		oldProfileName := nodeClass.Status.InstanceProfile

		// Remove protection to verify that the switch to spec.instanceProfile protects it again
		awsEnv.InstanceProfileProvider.SetProtectedState(oldProfileName, false)

		// Verify initial profile exists and is not protected
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.InstanceProfileProvider.IsProtected(oldProfileName)).To(BeFalse())

		// Switch to spec.instanceProfile
		nodeClass.Spec.Role = ""                                 // Clear role
		nodeClass.Spec.InstanceProfile = lo.ToPtr("new-profile") // Set instance profile
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		// Verify old profile remains protected
		Expect(awsEnv.InstanceProfileProvider.IsProtected(oldProfileName)).To(BeTrue())

		// Verify new profile is set in status
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal("new-profile"))
	})

	It("should allow different NodeClasses with same role to create instance profiles independently", func() {
		// Create first NodeClass with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass1 := ExpectExists(ctx, env.Client, nodeClass)
		profile1 := nodeClass1.Status.InstanceProfile

		// Create second NodeClass with same role
		nodeClass2 := test.EC2NodeClass() // This creates a new NodeClass with different UID
		nodeClass2.Spec.Role = "role-A"   // Same role as nodeClass1
		ExpectApplied(ctx, env.Client, nodeClass2)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass2)

		nodeClass2 = ExpectExists(ctx, env.Client, nodeClass2)
		profile2 := nodeClass2.Status.InstanceProfile

		// Verify both NodeClasses created their own profiles
		Expect(profile2).NotTo(Equal(profile1))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))

		// Verify both profiles have the correct role
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profile1].Roles[0].RoleName).To(Equal("role-A"))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profile2].Roles[0].RoleName).To(Equal("role-A"))
	})

	It("should handle multiple role transitions", func() {
		// Start with role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		profileA := nodeClass.Status.InstanceProfile

		// Transition to role-B
		nodeClass.Spec.Role = "role-B"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		profileB := nodeClass.Status.InstanceProfile
		Expect(profileB).NotTo(Equal(profileA))

		// Transition to role-C
		nodeClass.Spec.Role = "role-C"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		profileC := nodeClass.Status.InstanceProfile
		Expect(profileC).NotTo(Equal(profileB))

		// Assumes that the recreation cache for the instance profile previously created for Role A
		// has expired by the time user tries to reuse Role A
		awsEnv.RecreationCache.Flush()

		// Transition back to role-A
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		profileA2 := nodeClass.Status.InstanceProfile

		// Verify unique names even when reusing role-A
		Expect(profileA2).NotTo(Equal(profileA))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(4))
	})

	It("should return error on transient failures getting instance profile", func() {
		// Set up initial profile
		profileName := "profile-A"
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("role-A"),
					},
				},
			},
		}
		nodeClass.Status.InstanceProfile = profileName
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)

		nodeClass.Spec.Role = "role-B"

		// Simulate a transient error
		awsEnv.IAMAPI.GetInstanceProfileBehavior.Error.Set(fmt.Errorf("simulated transient error"))

		_, err := controller.Reconcile(ctx, nodeClass)
		Expect(err).To(MatchError(ContainSubstring("getting instance profile")))
		Expect(err).To(MatchError(ContainSubstring("simulated transient error")))

		// Verify no new profile was created
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
	})

	It("should properly tag new instance profiles", func() {
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		profileName := nodeClass.Status.InstanceProfile

		// Verify profile exists with correct tags
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		profile := awsEnv.IAMAPI.InstanceProfiles[profileName]
		Expect(profile.Tags).To(ContainElements(
			iamtypes.Tag{
				Key:   lo.ToPtr(fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName)),
				Value: lo.ToPtr("owned"),
			},
			iamtypes.Tag{
				Key:   lo.ToPtr(v1.LabelNodeClass),
				Value: lo.ToPtr(nodeClass.Name),
			},
			iamtypes.Tag{
				Key:   lo.ToPtr(v1.EKSClusterNameTagKey),
				Value: lo.ToPtr(options.FromContext(ctx).ClusterName),
			},
			iamtypes.Tag{
				Key:   lo.ToPtr("topology.kubernetes.io/region"),
				Value: lo.ToPtr(fake.DefaultRegion),
			},
		))

		// Verify role is attached
		Expect(profile.Roles).To(HaveLen(1))
		Expect(*profile.Roles[0].RoleName).To(Equal("role-A"))
	})

	It("should attempt to delete the instance profile if there was a creation failure", func() {
		awsEnv.IAMAPI.AddRoleToInstanceProfileBehavior.Error.Set(fmt.Errorf("failed to attach role"))
		nodeClass.Spec.Role = "role-A"
		ExpectApplied(ctx, env.Client, nodeClass)
		_ = ExpectObjectReconcileFailed(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
		Expect(awsEnv.IAMAPI.DeleteInstanceProfileBehavior.CalledWithInput.Len()).To(Equal(1))
	})
})
