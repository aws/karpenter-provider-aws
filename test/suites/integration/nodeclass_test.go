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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/google/uuid"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NodeClass IAM Permissions", func() {
	var (
		roleName           string
		createPolicyOutput *iam.CreatePolicyOutput
		err                error
	)

	AfterEach(func() {
		_, err := env.IAMAPI.DetachRolePolicy(env.Context, &iam.DetachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: createPolicyOutput.Policy.Arn,
		})
		Expect(err).To(BeNil())

		_, err = env.IAMAPI.DeletePolicy(env.Context, &iam.DeletePolicyInput{
			PolicyArn: createPolicyOutput.Policy.Arn,
		})
		Expect(err).To(BeNil())
	})

	DescribeTable("IAM Permission Tests",
		func(effect string, actions []string, expectSuccess bool) {
			policyDoc := fmt.Sprintf(`{
                "Version": "2012-10-17",
                "Statement": [
                    {
                        "Effect": "%s",
                        "Action": %s,
                        "Resource": "*"
                    }
                ]
            }`, effect, generateJSONArray(actions))

			createPolicyInput := &iam.CreatePolicyInput{
				PolicyName:     aws.String(fmt.Sprintf("TestPolicy-%s", uuid.New().String())),
				PolicyDocument: aws.String(policyDoc),
			}

			roleName = fmt.Sprintf("%s-karpenter", env.ClusterName)

			createPolicyOutput, err = env.IAMAPI.CreatePolicy(env.Context, createPolicyInput)
			Expect(err).To(BeNil())

			_, err = env.IAMAPI.AttachRolePolicy(env.Context, &iam.AttachRolePolicyInput{
				RoleName:  aws.String(roleName),
				PolicyArn: createPolicyOutput.Policy.Arn,
			})
			Expect(err).To(BeNil())

			env.ExpectCreated(nodeClass)

			if expectSuccess {
				Eventually(func(g Gomega) {
					env.ExpectUpdated(nodeClass)
					g.Expect(string(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Status)).To(Equal("True"))
				}, "60s", "5s").Should(Succeed())
			} else {
				Eventually(func(g Gomega) {
					env.ExpectUpdated(nodeClass)
					g.Expect(string(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).Status)).To(Equal("False"))

					condition := nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded)
					g.Expect(condition).ToNot(BeNil())
					g.Expect(condition.Message).To(ContainSubstring("unauthorized operation"))
					for _, action := range actions {
						g.Expect(condition.Message).To(ContainSubstring(strings.TrimPrefix(action, "ec2:")))
					}
				}, "90s", "5s").Should(Succeed())
			}
		},
		Entry("should fail when CreateFleet is denied",
			"Deny",
			[]string{"ec2:CreateFleet"},
			false),
		Entry("should fail when CreateLaunchTemplate is denied",
			"Deny",
			[]string{"ec2:CreateLaunchTemplate"},
			false),
		Entry("should fail when RunInstances is denied",
			"Deny",
			[]string{"ec2:RunInstances"},
			false),
		Entry("should pass with all required permissions",
			"Allow",
			[]string{"ec2:CreateFleet", "ec2:CreateLaunchTemplate", "ec2:RunInstances"},
			true),
	)
})

func generateJSONArray(actions []string) string {
	jsonActions, _ := json.Marshal(actions)
	return string(jsonActions)
}
