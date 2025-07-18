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

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass InstanceProfile Status Controller", func() {
	var profileName string
	BeforeEach(func() {
		profileName = nodeClass.InstanceProfileName(options.FromContext(ctx).ClusterName, fake.DefaultRegion)
	})
	It("should create the instance profile when it doesn't exist", func() {
		nodeClass.Spec.Role = "test-role"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles[0].RoleName).To(Equal("test-role"))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Tags).To(ContainElements(
			iamtypes.Tag{Key: lo.ToPtr(fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName)), Value: lo.ToPtr("owned")},
			iamtypes.Tag{Key: lo.ToPtr(v1.LabelNodeClass), Value: lo.ToPtr(nodeClass.Name)},
			iamtypes.Tag{Key: lo.ToPtr(v1.EKSClusterNameTagKey), Value: lo.ToPtr(options.FromContext(ctx).ClusterName)},
		))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should delete the instance profile from cache when the nodeClass is deleted", func() {
		nodeClass.Spec.Role = "test-role"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles[0].RoleName).To(Equal("test-role"))

		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
		Expect(awsEnv.InstanceProfileCache.Items()).To(HaveLen(1))

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.InstanceProfileCache.Items()).To(HaveLen(0))
	})
	It("should add the role to the instance profile when it exists without a role", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
			},
		}

		nodeClass.Spec.Role = "test-role"
		nodeClass.Status.InstanceProfile = profileName

		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles[0].RoleName).To(Equal("test-role"))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})

	It("should update the role for the instance profile when the wrong role exists", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("other-role"),
					},
				},
			},
		}

		nodeClass.Spec.Role = "test-role"
		nodeClass.Status.InstanceProfile = profileName
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(2))
		Expect(awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[nodeClass.Status.InstanceProfile].Roles[0].RoleName).To(Equal("test-role"))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should not call CreateInstanceProfile or AddRoleToInstanceProfile when instance profile exists with correct role", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("test-role"),
					},
				},
			},
		}

		nodeClass.Spec.Role = "test-role"
		nodeClass.Status.InstanceProfile = profileName
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

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

		awsEnv.InstanceProfileProvider.Reset()
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

	It("should properly tag new instance profiles", func() {
		nodeClass.Spec.Role = "test-role"
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
		Expect(*profile.Roles[0].RoleName).To(Equal("test-role"))
	})
})
