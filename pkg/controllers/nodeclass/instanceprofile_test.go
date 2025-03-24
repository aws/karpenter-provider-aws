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

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Tags).To(ContainElements(
			iamtypes.Tag{Key: lo.ToPtr(fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName)), Value: lo.ToPtr("owned")},
			iamtypes.Tag{Key: lo.ToPtr(v1.LabelNodeClass), Value: lo.ToPtr(nodeClass.Name)},
			iamtypes.Tag{Key: lo.ToPtr(v1.EKSClusterNameTagKey), Value: lo.ToPtr(options.FromContext(ctx).ClusterName)},
		))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should add the role to the instance profile when it exists without a role", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
			},
		}

		nodeClass.Spec.Role = "test-role"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
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
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
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
		awsEnv.IAMAPI.InstanceProfiles[lo.FromPtr(nodeClass.Spec.InstanceProfile)] = &iamtypes.InstanceProfile{}
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.InstanceProfileCache.Items()).To(HaveLen(1))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(lo.FromPtr(nodeClass.Spec.InstanceProfile)))
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
})
