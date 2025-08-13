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
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Validation Status Controller", func() {
	Context("Preconditions", func() {
		var reconciler *nodeclass.Validation
		BeforeEach(func() {
			reconciler = nodeclass.NewValidationReconciler(env.Client, cloudProvider, awsEnv.EC2API, awsEnv.AMIResolver, awsEnv.InstanceTypesProvider, awsEnv.LaunchTemplateProvider, awsEnv.ValidationCache, options.FromContext(ctx).DisableDryRun)
			for _, cond := range []string{
				v1.ConditionTypeAMIsReady,
				v1.ConditionTypeInstanceProfileReady,
				v1.ConditionTypeSecurityGroupsReady,
				v1.ConditionTypeSubnetsReady,
			} {
				nodeClass.StatusConditions().SetTrue(cond)
			}
		})
		DescribeTable(
			"should set validated status condition to false when any required condition is false",
			func(cond string) {
				nodeClass.StatusConditions().SetFalse(cond, "test", "test")
				_, err := reconciler.Reconcile(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(nodeclass.ConditionReasonDependenciesNotReady))
			},
			Entry(v1.ConditionTypeAMIsReady, v1.ConditionTypeAMIsReady),
			Entry(v1.ConditionTypeInstanceProfileReady, v1.ConditionTypeInstanceProfileReady),
			Entry(v1.ConditionTypeSecurityGroupsReady, v1.ConditionTypeSecurityGroupsReady),
			Entry(v1.ConditionTypeSubnetsReady, v1.ConditionTypeSubnetsReady),
		)
		DescribeTable(
			"should set validated status condition to unknown when no required condition is false and any are unknown",
			func(cond string) {
				nodeClass.StatusConditions().SetUnknown(cond)
				_, err := reconciler.Reconcile(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsUnknown()).To(BeTrue())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(nodeclass.ConditionReasonDependenciesNotReady))
			},
			Entry(v1.ConditionTypeAMIsReady, v1.ConditionTypeAMIsReady),
			Entry(v1.ConditionTypeInstanceProfileReady, v1.ConditionTypeInstanceProfileReady),
			Entry(v1.ConditionTypeSecurityGroupsReady, v1.ConditionTypeSecurityGroupsReady),
			Entry(v1.ConditionTypeSubnetsReady, v1.ConditionTypeSubnetsReady),
		)
	})
	Context("Tag Validation", func() {
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
					Tags: map[string]string{
						"kubernetes.io/cluster/anothercluster": "owned",
					},
				},
			})
		})
		DescribeTable("should update status condition on nodeClass as NotReady when tag validation fails", func(illegalTag map[string]string) {
			nodeClass.Spec.Tags = illegalTag
			ExpectApplied(ctx, env.Client, nodeClass)
			err := ExpectObjectReconcileFailed(ctx, env.Client, controller, nodeClass)
			Expect(err).To(HaveOccurred())
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
			Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal("TagValidationFailed"))
			Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsFalse()).To(BeTrue())
			Expect(nodeClass.StatusConditions().Get(status.ConditionReady).Message).To(Equal("ValidationSucceeded=False"))
		},
			Entry("kubernetes.io/cluster*", map[string]string{"kubernetes.io/cluster/acluster": "owned"}),
			Entry(v1.NodePoolTagKey, map[string]string{v1.NodePoolTagKey: "testnodepool"}),
			Entry(v1.EKSClusterNameTagKey, map[string]string{v1.EKSClusterNameTagKey: "acluster"}),
			Entry(v1.NodeClassTagKey, map[string]string{v1.NodeClassTagKey: "testnodeclass"}),
			Entry(v1.NodeClaimTagKey, map[string]string{v1.NodeClaimTagKey: "testnodeclaim"}),
		)
		It("should update status condition as Ready when tags are valid", func() {
			nodeClass.Spec.Tags = map[string]string{}
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			nodeClass = ExpectExists(ctx, env.Client, nodeClass)

			Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
			Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
		})
	})
	Context("Authorization Validation", func() {
		DescribeTable(
			"NodeClass validation failure conditions",
			func(setupFn func(), reason string) {
				ExpectApplied(ctx, env.Client, nodeClass)
				setupFn()
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
				nodeClass = ExpectExists(ctx, env.Client, nodeClass)
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(reason))
				Expect(awsEnv.ValidationCache.Items()).To(HaveLen(1))

				// Even though we would succeed on the subsequent call, we should fail here because we hit the cache
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
				nodeClass = ExpectExists(ctx, env.Client, nodeClass)
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(reason))

				// After flushing the cache, we should succeed
				awsEnv.ValidationCache.Flush()
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
				nodeClass = ExpectExists(ctx, env.Client, nodeClass)
				Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
			},
			Entry("should update status condition as NotReady when CreateFleet unauthorized", func() {
				awsEnv.EC2API.CreateFleetBehavior.Error.Set(&smithy.GenericAPIError{
					Code: "UnauthorizedOperation",
				}, fake.MaxCalls(1))
			}, nodeclass.ConditionReasonCreateFleetAuthFailed),
			Entry("should update status condition as NotReady when RunInstances unauthorized", func() {
				awsEnv.EC2API.RunInstancesBehavior.Error.Set(&smithy.GenericAPIError{
					Code: "UnauthorizedOperation",
				}, fake.MaxCalls(1))
			}, nodeclass.ConditionReasonRunInstancesAuthFailed),
			Entry("should update status condition as NotReady when CreateLaunchTemplate unauthorized", func() {
				awsEnv.EC2API.CreateLaunchTemplateBehavior.Error.Set(&smithy.GenericAPIError{
					Code: "UnauthorizedOperation",
				}, fake.MaxCalls(1))
			}, nodeclass.ConditionReasonCreateLaunchTemplateAuthFailed),
		)
		Context("RunInstances Validation", func() {
			BeforeEach(func() {
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
					Images: []ec2types.Image{
						{
							Name:         lo.ToPtr("amd64-ami"),
							ImageId:      lo.ToPtr("amd64-ami-id"),
							CreationDate: lo.ToPtr(time.Time{}.Format(time.RFC3339)),
							Architecture: "x86_64",
							State:        ec2types.ImageStateAvailable,
						},
						{
							Name:         lo.ToPtr("arm64-ami"),
							ImageId:      lo.ToPtr("arm64-ami-id"),
							CreationDate: lo.ToPtr(time.Time{}.Add(time.Minute).Format(time.RFC3339)),
							Architecture: "arm64",
							State:        ec2types.ImageStateAvailable,
						},
						{
							Name:         lo.ToPtr("amd64-nvidia-ami"),
							ImageId:      lo.ToPtr("amd64-nvidia-ami-id"),
							CreationDate: lo.ToPtr(time.Time{}.Add(2 * time.Minute).Format(time.RFC3339)),
							Architecture: "x86_64",
							State:        ec2types.ImageStateAvailable,
						},
					},
				})
				version := awsEnv.VersionProvider.Get(ctx)
				awsEnv.SSMAPI.Parameters = map[string]string{
					fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version):       "amd64-ami-id",
					fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version):   "amd64-nvidia-ami-id",
					fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version): "arm64-ami-id",
				}
				Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
				Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
			})
			DescribeTable(
				"should fallback to static instance types when no linked NodePools exist",
				func(expectedInstanceType ec2types.InstanceType, expectedAMIID string) {
					// Filter out the non-target standard AMI to ensure the right instance is selected, but leave the nvidia AMI to
					// test AMI selection.
					awsEnv.SSMAPI.Parameters = lo.PickBy(awsEnv.SSMAPI.Parameters, func(_, amiID string) bool {
						return amiID == expectedAMIID || strings.Contains(amiID, "nvidia")
					})
					Expect(len(awsEnv.SSMAPI.Parameters)).To(BeNumerically(">", 1))

					ExpectApplied(ctx, env.Client, nodeClass)
					ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
					input := awsEnv.EC2API.RunInstancesBehavior.CalledWithInput.Pop()
					Expect(input.InstanceType).To(Equal(expectedInstanceType))
					Expect(input.ImageId).To(PointTo(Equal(expectedAMIID)))
				},
				Entry("m5.large", ec2types.InstanceTypeM5Large, "amd64-ami-id"),
				Entry("m6g.large", ec2types.InstanceTypeM6gLarge, "arm64-ami-id"),
			)
			It("should prioritize non-GPU instances", func() {
				nodePool := coretest.NodePool(karpv1.NodePool{Spec: karpv1.NodePoolSpec{Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      corev1.LabelInstanceTypeStable,
									Operator: corev1.NodeSelectorOpIn,
									Values: []string{
										string(ec2types.InstanceTypeC6gLarge),
										string(ec2types.InstanceTypeG4dn8xlarge),
									},
								},
							},
						},
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
					},
				}}})
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
				input := awsEnv.EC2API.RunInstancesBehavior.CalledWithInput.Pop()
				Expect(input.InstanceType).To(Equal(ec2types.InstanceTypeC6gLarge))
				Expect(input.ImageId).To(PointTo(Equal("arm64-ami-id")))
			})
			It("should fallback to GPU instances when no non-GPU instances exist", func() {
				nodePool := coretest.NodePool(karpv1.NodePool{Spec: karpv1.NodePoolSpec{Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      corev1.LabelInstanceTypeStable,
									Operator: corev1.NodeSelectorOpIn,
									Values: []string{
										string(ec2types.InstanceTypeG4dn8xlarge),
									},
								},
							},
						},
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
					},
				}}})
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
				input := awsEnv.EC2API.RunInstancesBehavior.CalledWithInput.Pop()
				Expect(input.InstanceType).To(Equal(ec2types.InstanceTypeG4dn8xlarge))
				Expect(input.ImageId).To(PointTo(Equal("amd64-nvidia-ami-id")))
			})
		})
	})
	It("should clear the validation cache when the nodeclass is deleted", func() {
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)
		nodeClass.Spec.Tags = map[string]string{}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
		Expect(awsEnv.ValidationCache.Items()).To(HaveLen(1))

		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		ExpectNotFound(ctx, env.Client, nodeClass)
		Expect(awsEnv.ValidationCache.Items()).To(HaveLen(0))
	})
	It("should pass validation when the validation controller is disabled", func() {
		controller = nodeclass.NewController(
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
			true,
		)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
		// The cache still has an entry so we don't revalidate tags
		Expect(awsEnv.ValidationCache.Items()).To(HaveLen(1))
		// We shouldn't make any new calls when validation is disabled
		Expect(awsEnv.EC2API.CreateFleetBehavior.Calls()).To(Equal(0))
		Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.Calls()).To(Equal(0))
		Expect(awsEnv.EC2API.RunInstancesBehavior.Calls()).To(Equal(0))
	})
})
