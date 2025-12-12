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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/awslabs/operatorpkg/status"
	"github.com/google/uuid"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"

	. "github.com/awslabs/operatorpkg/test/expectations"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NodeClass IAM Permissions", func() {
	var (
		roleName   string
		policyName string
	)
	AfterEach(func(ctx context.Context) {
		By("deleting the deny policy")
		_, err := env.IAMAPI.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
			RoleName:   aws.String(roleName),
			PolicyName: aws.String(policyName),
		})
		Expect(awserrors.IgnoreNotFound(err)).To(BeNil())
		env.ExpectSettingsOverridden(corev1.EnvVar{Name: "DISABLE_DRY_RUN", Value: "false"}) // re-enable dry-run

		By("waiting for NodeClass to report ValidationSucceeded=true")
		// Ensure that we don't start the next test until the policy has been deleted and IAM returns that we have permissions back
		EventuallyExpectWithDryRunCacheInvalidation(func(g Gomega, currGeneration int64) {
			g.Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeValidationSucceeded) && nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).ObservedGeneration == currGeneration).To(BeTrue())
		})
	}, GracePeriod(time.Minute*15))
	DescribeTable("IAM Permission Failure Tests",
		func(action string, expectedMessage string) {
			policyDoc := fmt.Sprintf(`{
					"Version": "2012-10-17",
					"Statement": [
						{
							"Effect": "Deny",
							"Action": ["%s"],
							"Resource": "*"
						}
					]
				}`, action)

			pods := env.ExpectKarpenterPods()
			sa := &corev1.ServiceAccount{}
			Expect(env.Client.Get(env.Context, types.NamespacedName{
				Namespace: "kube-system",
				Name:      pods[0].Spec.ServiceAccountName,
			}, sa)).To(Succeed())

			roleName = strings.Split(sa.Annotations["eks.amazonaws.com/role-arn"], "/")[1]
			policyName = fmt.Sprintf("TestPolicy-%s", uuid.New().String())

			By(fmt.Sprintf("adding the deny policy for %s", action))
			_, err := env.IAMAPI.PutRolePolicy(env.Context, &iam.PutRolePolicyInput{
				RoleName:       aws.String(roleName),
				PolicyName:     aws.String(policyName),
				PolicyDocument: aws.String(policyDoc),
			})
			Expect(err).To(BeNil())

			env.ExpectCreated(nodeClass)

			By("waiting for NodeClass to report ValidationSucceeded=False")
			EventuallyExpectWithDryRunCacheInvalidation(func(g Gomega, currGeneration int64) {
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
				g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse() && nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).ObservedGeneration == currGeneration).To(BeTrue())
				g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(expectedMessage))
			})
			ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionFalse, Message: "ValidationSucceeded=False"})
		},
		Entry("should fail when CreateFleet is denied",
			"ec2:CreateFleet",
			"CreateFleetAuthCheckFailed"),
		Entry("should fail when CreateLaunchTemplate is denied",
			"ec2:CreateLaunchTemplate",
			"CreateLaunchTemplateAuthCheckFailed"),
		Entry("should fail when RunInstances is denied",
			"ec2:RunInstances",
			"RunInstancesAuthCheckFailed"),
	)

	It("should succeed with all required permissions", func() {
		env.ExpectCreated(nodeClass)
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		}).Should(Succeed())
	})
	It("should succeed EC2NodeClass validation when dry run validation is disabled", func() {
		// create a policy that blocks validation calls
		policyDoc := `{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Deny",
					"Action": ["ec2:CreateFleet", "ec2:CreateLaunchTemplate", "ec2:RunInstances"],
					"Resource": "*"
				}
			]
		}`
		pods := env.ExpectKarpenterPods()
		sa := &corev1.ServiceAccount{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{
			Namespace: "kube-system",
			Name:      pods[0].Spec.ServiceAccountName,
		}, sa)).To(Succeed())

		roleName = strings.Split(sa.Annotations["eks.amazonaws.com/role-arn"], "/")[1]
		policyName = fmt.Sprintf("TestPolicy-%s", uuid.New().String())

		By("adding the deny policy")
		_, err := env.IAMAPI.PutRolePolicy(env.Context, &iam.PutRolePolicyInput{
			RoleName:       aws.String(roleName),
			PolicyName:     aws.String(policyName),
			PolicyDocument: aws.String(policyDoc),
		})
		Expect(err).To(BeNil())

		// Ensure that we properly validate that we see the new policy
		env.ExpectCreated(nodeClass)

		By("waiting for NodeClass to report ValidationSucceeded=False")
		EventuallyExpectWithDryRunCacheInvalidation(func(g Gomega, currGeneration int64) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse() && nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).ObservedGeneration == currGeneration).To(BeTrue())
		})

		// Disable dry-run and now validation should succeed
		env.ExpectSettingsOverridden(corev1.EnvVar{Name: "DISABLE_DRY_RUN", Value: "true"})
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		}).Should(Succeed())
	})
})

func EventuallyExpectWithDryRunCacheInvalidation(f func(g Gomega, currentGeneration int64)) {
	lastUpdate := time.Time{}
	lastGeneration := int64(0)

	Eventually(func(g Gomega) {
		// Force the cache to be reset by updating MetadataOptions
		if time.Since(lastUpdate) > time.Second*15 {
			stored := nodeClass.DeepCopy()
			nodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit = lo.ToPtr(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit) + 1)
			g.Expect(env.Client.Patch(env.Context, nodeClass, client.MergeFrom(stored))).To(Succeed())
			lastUpdate = time.Now()
			lastGeneration = nodeClass.Generation
		}
		f(g, lastGeneration)
	}).Should(Succeed())
}
