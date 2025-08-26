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

package instanceprofile_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"

	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/fake"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

const nodeRole = "NodeRole"

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass test.TestNodeClass

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceProfileProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	nodeClass = test.TestNodeClass{
		EC2NodeClass: v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				AMISelectorTerms: []v1.AMISelectorTerm{{
					Alias: "al2@latest",
				}},
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						Tags: map[string]string{
							"*": "*",
						},
					},
				},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{
							"*": "*",
						},
					},
				},
			},
		},
	}
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("InstanceProfileProvider", func() {
	DescribeTable(
		"should support IAM roles",
		func(roleWithPath, role string) {
			const profileName = "profile-A"
			nodeClass.Spec.Role = roleWithPath
			Expect(awsEnv.InstanceProfileProvider.Create(ctx, profileName, role, nil, string(nodeClass.UID))).To(Succeed())
			Expect(profileName).ToNot(BeNil())
			Expect(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles).To(HaveLen(1))
			Expect(aws.ToString(awsEnv.IAMAPI.InstanceProfiles[profileName].Roles[0].RoleName)).To(Equal(role))
		},
		Entry("with custom paths", fmt.Sprintf("CustomPath/%s", nodeRole), nodeRole),
		Entry("without custom paths", nodeRole, nodeRole),
	)

	It("should list all instance profiles for a NodeClass", func() {
		// Create and apply first NodeClass
		nodeClass1 := test.TestNodeClass{
			EC2NodeClass: v1.EC2NodeClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nodeclass-1",
				},
				Spec: nodeClass.Spec,
			},
		}
		nodeClass1.Spec.Role = "role-1"
		ExpectApplied(ctx, env.Client, &nodeClass1.EC2NodeClass)

		// Create and apply second NodeClass
		nodeClass2 := test.TestNodeClass{
			EC2NodeClass: v1.EC2NodeClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nodeclass-2",
				},
				Spec: nodeClass.Spec,
			},
		}
		nodeClass2.Spec.Role = "role-2"
		ExpectApplied(ctx, env.Client, &nodeClass2.EC2NodeClass)

		// Create instance profiles using the UIDs from the applied NodeClasses
		profile1 := "profile-1"
		profile2 := "profile-2"

		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profile1: {
				InstanceProfileName: lo.ToPtr(profile1),
				Roles: []iamtypes.Role{
					{
						RoleId:   aws.String(fake.RoleID()),
						RoleName: aws.String("role-1"),
					},
				},
				Path: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, string(nodeClass1.UID))),
			},
			profile2: {
				InstanceProfileName: lo.ToPtr(profile2),
				Roles: []iamtypes.Role{
					{
						RoleId:   aws.String(fake.RoleID()),
						RoleName: aws.String("role-2"),
					},
				},
				Path: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, string(nodeClass2.UID))),
			},
		}

		// List profiles for first NodeClass
		profiles, err := awsEnv.InstanceProfileProvider.ListNodeClassProfiles(ctx, &nodeClass1.EC2NodeClass)
		Expect(err).To(BeNil())

		// Should only get profiles for first NodeClass
		Expect(profiles).To(HaveLen(1))
		Expect(aws.ToString(profiles[0].InstanceProfileName)).To(Equal(profile1))
	})

	It("should list all instance profiles for a Cluster", func() {
		// Create and apply first NodeClass
		nodeClass1 := test.TestNodeClass{
			EC2NodeClass: v1.EC2NodeClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nodeclass-1",
				},
				Spec: nodeClass.Spec, // Use the spec from BeforeEach
			},
		}
		nodeClass1.Spec.Role = "role-1"
		ExpectApplied(ctx, env.Client, &nodeClass1.EC2NodeClass)

		// Create and apply second NodeClass
		nodeClass2 := test.TestNodeClass{
			EC2NodeClass: v1.EC2NodeClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nodeclass-2",
				},
				Spec: nodeClass.Spec, // Use the spec from BeforeEach
			},
		}
		nodeClass2.Spec.Role = "role-2"
		ExpectApplied(ctx, env.Client, &nodeClass2.EC2NodeClass)

		profile1 := "profile-1"
		profile2 := "profile-2"
		profile3 := "profile-3"

		otherClusterCtx := options.ToContext(ctx, test.Options(test.OptionsFields{
			ClusterName: lo.ToPtr("other-cluster"),
		}))

		// Create instance profiles
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profile1: {
				InstanceProfileName: lo.ToPtr(profile1),
				Roles: []iamtypes.Role{
					{
						RoleId:   aws.String(fake.RoleID()),
						RoleName: aws.String("role-1"),
					},
				},
				Path: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, string(nodeClass1.UID))),
			},
			profile2: {
				InstanceProfileName: lo.ToPtr(profile2),
				Roles: []iamtypes.Role{
					{
						RoleId:   aws.String(fake.RoleID()),
						RoleName: aws.String("role-2"),
					},
				},
				Path: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, string(nodeClass2.UID))),
			},
			profile3: {
				InstanceProfileName: lo.ToPtr(profile3),
				Roles: []iamtypes.Role{
					{
						RoleId:   aws.String(fake.RoleID()),
						RoleName: aws.String("role-3"),
					},
				},
				Path: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(otherClusterCtx).ClusterName, "some-uid")),
			},
		}

		// List all cluster profiles
		profiles, err := awsEnv.InstanceProfileProvider.ListClusterProfiles(ctx)
		Expect(err).To(BeNil())

		// Should get both profiles in first cluster and not the other one
		Expect(profiles).To(HaveLen(2))
		profileNames := []string{
			aws.ToString(profiles[0].InstanceProfileName),
			aws.ToString(profiles[1].InstanceProfileName),
		}
		Expect(profileNames).To(ContainElements(profile1, profile2))
		Expect(profileNames).ToNot(ContainElement(profile3))
	})

	It("should create an instance profile with the correct path and name", func() {
		// Create instance profile
		profileName := "profile-A"
		nodeClassUID := "test-uid"
		expectedPath := fmt.Sprintf("/karpenter/%s/%s/%s/", fake.DefaultRegion, options.FromContext(ctx).ClusterName, nodeClassUID)

		Expect(awsEnv.InstanceProfileProvider.Create(ctx, profileName, nodeRole, nil, nodeClassUID)).To(Succeed())

		// Get the created profile
		profile, err := awsEnv.InstanceProfileProvider.Get(ctx, profileName)
		Expect(err).To(BeNil())

		// Verify name and path
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(profileName))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(aws.ToString(profile.InstanceProfileName)).To(Equal(profileName))
		Expect(aws.ToString(profile.Path)).To(Equal(expectedPath))
	})

	It("should delete an instance profile with the correct name", func() {
		// Create instance profile first
		profileName := "profile-A"
		nodeClassUID := "test-uid"

		Expect(awsEnv.InstanceProfileProvider.Create(ctx, profileName, nodeRole, nil, nodeClassUID)).To(Succeed())

		// Verify profile exists
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveKey(profileName))
		_, err := awsEnv.InstanceProfileProvider.Get(ctx, profileName)
		Expect(err).To(BeNil())

		// Delete the profile
		Expect(awsEnv.InstanceProfileProvider.Delete(ctx, profileName)).To(Succeed())

		// Verify profile no longer exists
		_, err = awsEnv.InstanceProfileProvider.Get(ctx, profileName)
		Expect(awserrors.IsNotFound(err)).To(BeTrue())

		// Verify it's removed from IAMAPI
		Expect(awsEnv.IAMAPI.InstanceProfiles).ToNot(HaveKey(profileName))
	})

	It("should reflect IsProtected updates", func() {
		// Create a profile
		profileName := "profile-A"
		Expect(awsEnv.InstanceProfileProvider.Create(ctx, profileName, nodeRole, nil, "test-uid")).To(Succeed())

		// Initially should not be protected (protection is set in instance profile reconciler)
		Expect(awsEnv.InstanceProfileProvider.IsProtected(profileName)).To(BeFalse())

		// Set to protected
		awsEnv.InstanceProfileProvider.SetProtectedState(profileName, true)
		Expect(awsEnv.InstanceProfileProvider.IsProtected(profileName)).To(BeTrue())

		// Set back to unprotected
		awsEnv.InstanceProfileProvider.SetProtectedState(profileName, false)
		Expect(awsEnv.InstanceProfileProvider.IsProtected(profileName)).To(BeFalse())
	})

	Context("Role Cache", func() {
		const roleName = "test-role"
		BeforeEach(func() {
			awsEnv.IAMAPI.EnableRoleValidation = true
			awsEnv.IAMAPI.Roles = map[string]*iamtypes.Role{
				roleName: &iamtypes.Role{RoleName: lo.ToPtr(roleName)},
			}
		})
		It("should not cache role not found errors when the role exists", func() {
			err := awsEnv.InstanceProfileProvider.Create(ctx, "test-profile", roleName, nil, "test-uid")
			Expect(err).ToNot(HaveOccurred())
			_, ok := awsEnv.InstanceProfileCache.Get(fmt.Sprintf("role:%s", roleName))
			Expect(ok).To(BeFalse())
		})
		It("should cache role not found errors when the role does not", func() {
			missingRoleName := "non-existent-role"
			err := awsEnv.InstanceProfileProvider.Create(ctx, "test-profile", missingRoleName, nil, "test-uid")
			Expect(err).To(HaveOccurred())
			_, ok := awsEnv.InstanceProfileCache.Get(fmt.Sprintf("role:%s", missingRoleName))
			Expect(ok).To(BeTrue())
		})
		It("should not attempt to create instance profile when role is cached as not found", func() {
			missingRoleName := "non-existent-role"
			awsEnv.InstanceProfileCache.SetDefault(fmt.Sprintf("role:%s", missingRoleName), errors.New("role not found"))

			err := awsEnv.InstanceProfileProvider.Create(ctx, "test-profile", missingRoleName, nil, "test-uid")
			Expect(err).To(HaveOccurred())

			Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
			Expect(awsEnv.IAMAPI.CreateInstanceProfileBehavior.Calls()).To(BeZero())
		})
	})
})
