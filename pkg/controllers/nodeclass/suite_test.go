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
	"context"
	"fmt"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass *v1.EC2NodeClass
var controller *nodeclass.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2NodeClass")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(test.RemoveNodeClassTagValidation(apis.CRDs)...), coretest.WithCRDs(v1alpha1.CRDs...), coretest.WithFieldIndexers(coretest.NodeClaimNodeClassRefFieldIndexer(ctx)))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)

	controller = nodeclass.NewController(
		env.Client, events.NewRecorder(&record.FakeRecorder{}),
		awsEnv.SubnetProvider,
		awsEnv.SecurityGroupProvider,
		awsEnv.AMIProvider,
		awsEnv.InstanceProfileProvider,
		awsEnv.LaunchTemplateProvider,
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

var _ = Describe("NodeClass Termination", func() {
	var profileName string
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				AMIFamily: lo.ToPtr(v1.AMIFamilyCustom),
				AMISelectorTerms: []v1.AMISelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
			},
		})
		profileName = nodeClass.InstanceProfileName(options.FromContext(ctx).ClusterName, fake.DefaultRegion)

	})
	It("should not delete the NodeClass if launch template deletion fails", func() {
		launchTemplateName := aws.String(fake.LaunchTemplateName())
		awsEnv.EC2API.LaunchTemplates.Store(launchTemplateName, ec2types.LaunchTemplate{LaunchTemplateName: launchTemplateName, LaunchTemplateId: aws.String(fake.LaunchTemplateID()), Tags: []ec2types.Tag{{Key: aws.String("karpenter.k8s.aws/cluster"), Value: aws.String("test-cluster")}}})
		_, ok := awsEnv.EC2API.LaunchTemplates.Load(launchTemplateName)
		Expect(ok).To(BeTrue())
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		awsEnv.EC2API.NextError.Set(fmt.Errorf("delete Launch Template Error"))
		_ = ExpectObjectReconcileFailed(ctx, env.Client, controller, nodeClass)
		ExpectExists(ctx, env.Client, nodeClass)
	})
	It("should not delete the launch template not associated with the nodeClass", func() {
		launchTemplateName := aws.String(fake.LaunchTemplateName())
		awsEnv.EC2API.LaunchTemplates.Store(launchTemplateName, ec2types.LaunchTemplate{LaunchTemplateName: launchTemplateName, LaunchTemplateId: aws.String(fake.LaunchTemplateID()), Tags: []ec2types.Tag{{Key: aws.String("karpenter.k8s.aws/cluster"), Value: aws.String("test-cluster")}}})
		_, ok := awsEnv.EC2API.LaunchTemplates.Load(launchTemplateName)
		Expect(ok).To(BeTrue())
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		_, ok = awsEnv.EC2API.LaunchTemplates.Load(launchTemplateName)
		Expect(ok).To(BeTrue())
		ExpectNotFound(ctx, env.Client, nodeClass)
	})
	It("should succeed to delete the launch template", func() {
		ltName1 := aws.String(fake.LaunchTemplateName())
		awsEnv.EC2API.LaunchTemplates.Store(ltName1, ec2types.LaunchTemplate{LaunchTemplateName: ltName1, LaunchTemplateId: aws.String(fake.LaunchTemplateID()), Tags: []ec2types.Tag{{Key: aws.String("eks:eks-cluster-name"), Value: aws.String("test-cluster")}, {Key: aws.String("karpenter.k8s.aws/ec2nodeclass"), Value: aws.String(nodeClass.Name)}}})
		ltName2 := aws.String(fake.LaunchTemplateName())
		awsEnv.EC2API.LaunchTemplates.Store(ltName2, ec2types.LaunchTemplate{LaunchTemplateName: ltName2, LaunchTemplateId: aws.String(fake.LaunchTemplateID()), Tags: []ec2types.Tag{{Key: aws.String("eks:eks-cluster-name"), Value: aws.String("test-cluster")}, {Key: aws.String("karpenter.k8s.aws/ec2nodeclass"), Value: aws.String(nodeClass.Name)}}})
		_, ok := awsEnv.EC2API.LaunchTemplates.Load(ltName1)
		Expect(ok).To(BeTrue())
		_, ok = awsEnv.EC2API.LaunchTemplates.Load(ltName2)
		Expect(ok).To(BeTrue())
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		_, ok = awsEnv.EC2API.LaunchTemplates.Load(ltName1)
		Expect(ok).To(BeFalse())
		_, ok = awsEnv.EC2API.LaunchTemplates.Load(ltName2)
		Expect(ok).To(BeFalse())
		ExpectNotFound(ctx, env.Client, nodeClass)
	})
	It("should succeed to delete the instance profile with no NodeClaims", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileName: aws.String(profileName),
				Roles: []iamtypes.Role{
					{
						RoleId:   aws.String(fake.RoleID()),
						RoleName: aws.String(nodeClass.Spec.Role),
					},
				},
			},
		}
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClass)
	})
	It("should succeed to delete the instance profile when no roles exist with no NodeClaims", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileName: aws.String(profileName),
			},
		}
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClass)
	})
	It("should succeed to delete the NodeClass when the instance profile doesn't exist", func() {
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		ExpectApplied(ctx, env.Client, nodeClass)

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClass)
	})
	It("should not delete the EC2NodeClass until all associated NodeClaims are terminated", func() {
		var nodeClaims []*karpv1.NodeClaim
		for i := 0; i < 2; i++ {
			nc := coretest.NodeClaim(karpv1.NodeClaim{
				Spec: karpv1.NodeClaimSpec{
					NodeClassRef: &karpv1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.Name,
					},
				},
			})
			ExpectApplied(ctx, env.Client, nc)
			nodeClaims = append(nodeClaims, nc)
		}
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileName: aws.String(profileName),
				Roles: []iamtypes.Role{
					{
						RoleId:   aws.String(fake.RoleID()),
						RoleName: aws.String(nodeClass.Spec.Role),
					},
				},
			},
		}
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		res := ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(res.RequeueAfter).To(Equal(time.Minute * 10))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		ExpectExists(ctx, env.Client, nodeClass)

		// Delete one of the NodeClaims
		// The NodeClass should still not delete
		ExpectDeleted(ctx, env.Client, nodeClaims[0])
		res = ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(res.RequeueAfter).To(Equal(time.Minute * 10))
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		ExpectExists(ctx, env.Client, nodeClass)

		// Delete the last NodeClaim
		// The NodeClass should now delete
		ExpectDeleted(ctx, env.Client, nodeClaims[1])
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, nodeClass)
	})
	It("should not call the IAM API when deleting a NodeClass with an instanceProfile specified", func() {
		awsEnv.IAMAPI.InstanceProfiles = map[string]*iamtypes.InstanceProfile{
			profileName: {
				InstanceProfileName: aws.String("test-instance-profile"),
				Roles: []iamtypes.Role{
					{
						RoleId:   aws.String(fake.RoleID()),
						RoleName: aws.String("fake-role"),
					},
				},
			},
		}
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(awsEnv.IAMAPI.InstanceProfiles).To(HaveLen(1))
		ExpectNotFound(ctx, env.Client, nodeClass)

		Expect(awsEnv.IAMAPI.DeleteInstanceProfileBehavior.Calls()).To(BeZero())
		Expect(awsEnv.IAMAPI.RemoveRoleFromInstanceProfileBehavior.Calls()).To(BeZero())
	})
})
