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

package integration_test

import (
	"fmt"
	"strings"
	"time"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"github.com/awslabs/operatorpkg/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/samber/lo"

	coretest "sigs.k8s.io/karpenter/pkg/test"

	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"

	. "github.com/awslabs/operatorpkg/test/expectations"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstanceProfile Generation", func() {
	BeforeEach(func() {
		if env.PrivateCluster {
			Skip("skipping InstanceProfile Generation test for private cluster")
		}
	})
	It("should generate the InstanceProfile when setting the role", func() {
		pod := coretest.Pod()
		env.ExpectCreated(nodePool, nodeClass, pod)
		env.EventuallyExpectHealthy(pod)
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		instance := env.GetInstance(node.Name)
		Expect(instance.IamInstanceProfile).ToNot(BeNil())
		Expect(lo.FromPtr(instance.IamInstanceProfile.Arn)).To(ContainSubstring(nodeClass.Status.InstanceProfile))

		instanceProfile := env.EventuallyExpectInstanceProfileExists(env.GetInstanceProfileName(nodeClass))
		Expect(instanceProfile.Roles).To(HaveLen(1))
		Expect(lo.FromPtr(instanceProfile.Roles[0].RoleName)).To(Equal(nodeClass.Spec.Role))
	})
	It("should remove the generated InstanceProfile when deleting the NodeClass", func() {
		pod := coretest.Pod()
		env.ExpectCreated(nodePool, nodeClass, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectDeleted(nodePool, nodeClass)
		Eventually(func(g Gomega) {
			_, err := env.IAMAPI.GetInstanceProfile(env.Context, &iam.GetInstanceProfileInput{
				InstanceProfileName: lo.ToPtr(env.GetInstanceProfileName(nodeClass)),
			})
			g.Expect(awserrors.IsNotFound(err)).To(BeTrue())
		}).Should(Succeed())
	})
	It("should use the unmanaged instance profile", func() {
		instanceProfileName := fmt.Sprintf("KarpenterNodeInstanceProfile-%s", env.ClusterName)
		roleName := fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName)
		env.ExpectInstanceProfileCreated(instanceProfileName, roleName)
		DeferCleanup(func() {
			env.ExpectInstanceProfileDeleted(instanceProfileName, roleName)
		})

		pod := coretest.Pod()
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr(fmt.Sprintf("KarpenterNodeInstanceProfile-%s", env.ClusterName))
		env.ExpectCreated(nodePool, nodeClass, pod)
		env.EventuallyExpectHealthy(pod)
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		instance := env.GetInstance(node.Name)
		Expect(instance.IamInstanceProfile).ToNot(BeNil())
		Expect(lo.FromPtr(instance.IamInstanceProfile.Arn)).To(ContainSubstring(nodeClass.Status.InstanceProfile))
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeInstanceProfileReady, Status: metav1.ConditionTrue})
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionTrue})
	})
	It("should have the EC2NodeClass status as not ready since Instance Profile was not resolved", func() {
		nodeClass.Spec.Role = fmt.Sprintf("KarpenterNodeRole-%s", "invalidRole")
		env.ExpectCreated(nodeClass)
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeInstanceProfileReady, Status: metav1.ConditionUnknown})
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeValidationSucceeded, Status: metav1.ConditionUnknown})
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionUnknown})
	})

	Context("NodeClass IAM Tests", func() {
		var (
			roleName     string
			testRoleName string
			accountID    string
		)

		BeforeEach(func() {
			identity, err := env.STSAPI.GetCallerIdentity(env.Context, &sts.GetCallerIdentityInput{})
			Expect(err).NotTo(HaveOccurred())
			accountID = *identity.Account

			deployment := &appsv1.Deployment{}
			Expect(env.Client.Get(env.Context, types.NamespacedName{
				Namespace: "kube-system",
				Name:      "karpenter",
			}, deployment)).To(Succeed())

			sa := &corev1.ServiceAccount{}
			Expect(env.Client.Get(env.Context, types.NamespacedName{
				Namespace: "kube-system",
				Name:      deployment.Spec.Template.Spec.ServiceAccountName,
			}, sa)).To(Succeed())

			roleName = strings.Split(sa.Annotations["eks.amazonaws.com/role-arn"], "/")[1]
			testRoleName = "KarpenterNodeRole-MockNodeRole"

		})

		It("should set the InstanceProfileReady status condition to false when the instance profile is not found", func() {
			nodeClass.Spec.InstanceProfile = lo.ToPtr(fmt.Sprintf("KarpenterNodeInstanceProfile-%s", env.ClusterName))
			nodeClass.Spec.Role = ""
			env.ExpectCreated(nodeClass)
			ExpectStatusConditions(env, env.Client, 2*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionFalse, Message: "ValidationSucceeded=False, InstanceProfileReady=False"})
			ExpectStatusConditions(env, env.Client, 2*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeInstanceProfileReady, Status: metav1.ConditionFalse, Reason: "InstanceProfileNotFound"})
		})
		It("should set the InstanceProfileReady status condition to false when the role is not found", func() {
			_, err := env.IAMAPI.PutRolePolicy(env.Context, &iam.PutRolePolicyInput{
				RoleName:   lo.ToPtr(roleName),
				PolicyName: lo.ToPtr("allowpassrole"),
				PolicyDocument: aws.String(fmt.Sprintf(`{
					"Version": "2012-10-17",
					"Statement": [
						{
							"Effect": "Allow",
							"Action": "iam:PassRole",
							"Resource": "arn:aws:iam::%s:role/KarpenterNodeRole-MockNodeRole"
						}
					]
				}`, accountID)),
			})
			Expect(err).To(BeNil())
			Eventually(func() error {
				_, err := env.IAMAPI.GetRolePolicy(env.Context, &iam.GetRolePolicyInput{
					RoleName:   lo.ToPtr(roleName),
					PolicyName: lo.ToPtr("allowpassrole"),
				})
				return err
			}, 300*time.Second, 60*time.Second).Should(BeNil())
			DeferCleanup(func() {
				_, err := env.IAMAPI.DeleteRolePolicy(env.Context, &iam.DeleteRolePolicyInput{
					RoleName:   lo.ToPtr(roleName),
					PolicyName: lo.ToPtr("allowpassrole"),
				})
				Expect(err).To(BeNil())
			})
			nodeClass.Spec.Role = testRoleName
			env.ExpectCreated(nodeClass)
			ExpectStatusConditions(env, env.Client, 2*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionFalse, Message: "ValidationSucceeded=False, InstanceProfileReady=False"})
			ExpectStatusConditions(env, env.Client, 2*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeInstanceProfileReady, Status: metav1.ConditionFalse, Reason: "NodeRoleNotFound"})
		})
		It("should set the InstanceProfileReady status condition to false when not authorized", func() {
			nodeClass.Spec.Role = testRoleName
			env.ExpectCreated(nodeClass)
			ExpectStatusConditions(env, env.Client, 2*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionFalse, Message: "ValidationSucceeded=False, InstanceProfileReady=False"})
			ExpectStatusConditions(env, env.Client, 2*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeInstanceProfileReady, Status: metav1.ConditionFalse, Reason: "NodeRoleAuthFailure"})
		})
		It("should set the InstanceProfileReady status condition to true when the user-specified instance profile exists", func() {

			roleName = fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName)
			instanceProfileName := fmt.Sprintf("KarpenterNodeInstanceProfile-%s", env.ClusterName)

			// Create instance profile
			_, err := env.IAMAPI.CreateInstanceProfile(env.Context, &iam.CreateInstanceProfileInput{
				InstanceProfileName: aws.String(instanceProfileName),
			})
			Expect(err).To(BeNil())

			_, err = env.IAMAPI.AddRoleToInstanceProfile(env.Context, &iam.AddRoleToInstanceProfileInput{
				InstanceProfileName: aws.String(instanceProfileName),
				RoleName:            aws.String(roleName),
			})
			Expect(err).To(BeNil())

			DeferCleanup(func() {
				// Remove role from instance profile
				_, err := env.IAMAPI.RemoveRoleFromInstanceProfile(env.Context, &iam.RemoveRoleFromInstanceProfileInput{
					InstanceProfileName: aws.String(instanceProfileName),
					RoleName:            aws.String(roleName),
				})
				Expect(err).To(BeNil())

				// Delete instance profile
				_, err = env.IAMAPI.DeleteInstanceProfile(env.Context, &iam.DeleteInstanceProfileInput{
					InstanceProfileName: aws.String(instanceProfileName),
				})
				Expect(err).To(BeNil())
			})

			// Verify instance profile creation
			Eventually(func(g Gomega) {
				_, err := env.IAMAPI.GetInstanceProfile(env.Context, &iam.GetInstanceProfileInput{
					InstanceProfileName: aws.String(instanceProfileName),
				})
				g.Expect(err).To(BeNil())
			}, "30s", "5s").Should(Succeed())

			nodeClass.Spec.InstanceProfile = lo.ToPtr(instanceProfileName)
			nodeClass.Spec.Role = ""
			env.ExpectCreated(nodeClass)
			ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionTrue})
			ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeInstanceProfileReady, Status: metav1.ConditionTrue})
		})
	})
})
