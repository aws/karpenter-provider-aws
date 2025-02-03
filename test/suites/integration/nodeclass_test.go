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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/google/uuid"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NodeClass IAM Permissions", func() {
	var (
		roleName   string
		policyName string
	)

	DescribeTable("IAM Permission Failure Tests",
		func(action string) {
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

			roleName = fmt.Sprintf("%s-karpenter", env.ClusterName)
			policyName = fmt.Sprintf("TestPolicy-%s", uuid.New().String())

			_, err := env.IAMAPI.PutRolePolicy(env.Context, &iam.PutRolePolicyInput{
				RoleName:       aws.String(roleName),
				PolicyName:     aws.String(policyName),
				PolicyDocument: aws.String(policyDoc),
			})
			if err != nil {
				//e2e test role naming
				roleName = fmt.Sprintf("karpenter-irsa-%s", env.ClusterName)
				_, err = env.IAMAPI.PutRolePolicy(env.Context, &iam.PutRolePolicyInput{
					RoleName:       aws.String(roleName),
					PolicyName:     aws.String(policyName),
					PolicyDocument: aws.String(policyDoc),
				})
				Expect(err).To(BeNil())
			}

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
				g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeFalse())

				condition := nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded)
				g.Expect(condition).ToNot(BeNil())
				g.Expect(condition.Message).To(ContainSubstring("unauthorized operation"))
			}, "90s", "5s").Should(Succeed())
		},
		Entry("should fail when CreateFleet is denied", "ec2:CreateFleet"),
		Entry("should fail when CreateLaunchTemplate is denied", "ec2:CreateLaunchTemplate"),
		Entry("should fail when RunInstances is denied", "ec2:RunInstances"),
	)

	It("should succeed with all required permissions", func() {
		env.ExpectCreated(nodeClass)
		Eventually(func(g Gomega) {
			env.ExpectUpdated(nodeClass)
			g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		}, "60s", "5s").Should(Succeed())
	})
})
