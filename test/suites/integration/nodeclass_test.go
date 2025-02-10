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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/awslabs/operatorpkg/status"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/awslabs/operatorpkg/test/expectations"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NodeClass IAM Permissions", func() {
	var (
		roleName   string
		policyName string
	)
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

			deployment := &appsv1.Deployment{}
			err := env.Client.Get(env.Context, types.NamespacedName{
				Namespace: "kube-system",
				Name:      "karpenter",
			}, deployment)
			Expect(err).To(BeNil())

			sa := &corev1.ServiceAccount{}
			err = env.Client.Get(env.Context, types.NamespacedName{
				Namespace: "kube-system",
				Name:      deployment.Spec.Template.Spec.ServiceAccountName,
			}, sa)
			Expect(err).To(BeNil())

			roleName = strings.Split(sa.Annotations["eks.amazonaws.com/role-arn"], "/")[1]
			policyName = fmt.Sprintf("TestPolicy-%s", uuid.New().String())

			_, err = env.IAMAPI.PutRolePolicy(env.Context, &iam.PutRolePolicyInput{
				RoleName:       aws.String(roleName),
				PolicyName:     aws.String(policyName),
				PolicyDocument: aws.String(policyDoc),
			})
			Expect(err).To(BeNil())

			DeferCleanup(func() {
				_, err := env.IAMAPI.DeleteRolePolicy(env.Context, &iam.DeleteRolePolicyInput{
					RoleName:   aws.String(roleName),
					PolicyName: aws.String(policyName),
				})
				Expect(err).To(BeNil())
			})

			env.ExpectCreated(nodeClass)
			Eventually(func(g Gomega) {
				env.ExpectUpdated(nodeClass)
				g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsFalse()).To(BeTrue())
				g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Reason).To(Equal(expectedMessage))
			}, "240s", "5s").Should(Succeed())
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
			env.ExpectUpdated(nodeClass)
			g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		}, "60s", "5s").Should(Succeed())
	})
})
