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

package instance_profile

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/samber/lo"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"testing"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass *v1beta1.EC2NodeClass
var instanceProfileController *Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2NodeClass")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithFieldIndexers(test.EC2NodeClassFieldIndexer(ctx)))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)

	instanceProfileController = NewController(
		env.Client,
		awsEnv.InstanceProfileProvider,
	)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	nodeClass = test.EC2NodeClass()
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClass InstanceProfile Status Controller", func() {
	var profileName string
	BeforeEach(func() {
		profileName = nodeClass.InstanceProfileName(options.FromContext(ctx).ClusterName, fake.DefaultRegion)
	})
	It("should create the instance profile when it doesn't exist", func() {
		nodeClass.Spec.Role = "test-role"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, instanceProfileController, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
		Expect(nodeClass.StatusConditions().IsTrue(v1beta1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should add the role to the instance profile when it exists without a role", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
			},
		}

		nodeClass.Spec.Role = "test-role"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, instanceProfileController, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
		Expect(nodeClass.StatusConditions().IsTrue(v1beta1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should update the role for the instance profile when the wrong role exists", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Roles: []*iam.Role{
					{
						RoleName: aws.String("other-role"),
					},
				},
			},
		}

		nodeClass.Spec.Role = "test-role"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, instanceProfileController, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(profileName))
		Expect(nodeClass.StatusConditions().IsTrue(v1beta1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should not call CreateInstanceProfile or AddRoleToInstanceProfile when instance profile exists with correct role", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iam.InstanceProfile{
			profileName: {
				InstanceProfileId:   aws.String(fake.InstanceProfileID()),
				InstanceProfileName: aws.String(profileName),
				Roles: []*iam.Role{
					{
						RoleName: aws.String("test-role"),
					},
				},
			},
		}

		nodeClass.Spec.Role = "test-role"
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, instanceProfileController, nodeClass)

		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
		Expect(*awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName).To(Equal("test-role"))

		Expect(awsEnv.IAMAPI.CreateInstanceProfileBehavior.Calls()).To(BeZero())
		Expect(awsEnv.IAMAPI.AddRoleToInstanceProfileBehavior.Calls()).To(BeZero())
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1beta1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should resolve the specified instance profile into the status when using instanceProfile field", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, instanceProfileController, nodeClass)

		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.InstanceProfile).To(Equal(lo.FromPtr(nodeClass.Spec.InstanceProfile)))
		Expect(nodeClass.StatusConditions().IsTrue(v1beta1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
	It("should not call the the IAM API when specifying an instance profile", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, instanceProfileController, nodeClass)

		Expect(awsEnv.IAMAPI.CreateInstanceProfileBehavior.Calls()).To(BeZero())
		Expect(awsEnv.IAMAPI.AddRoleToInstanceProfileBehavior.Calls()).To(BeZero())
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1beta1.ConditionTypeInstanceProfileReady)).To(BeTrue())
	})
})
