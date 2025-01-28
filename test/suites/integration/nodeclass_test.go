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

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"sigs.k8s.io/controller-runtime/pkg/client"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Nodeclass Validation", func() {
	var createPolicyOutput *iam.CreatePolicyOutput
	var roleName string
	var err error

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
	It("should fail reconciliation when a required permission is explicitly denied", func() {
		createPolicyInput := &iam.CreatePolicyInput{
			PolicyName: aws.String("DenyPolicy"),
			PolicyDocument: aws.String(`{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Deny",
						"Action": "ec2:CreateFleet",
						"Resource": "*"
					}
				]
			}`),
		}

		roleName = fmt.Sprintf("%s-karpenter", env.ClusterName)

		createPolicyOutput, err = env.IAMAPI.CreatePolicy(env.Context, createPolicyInput)
		Expect(err).To(BeNil())

		_, err = env.IAMAPI.AttachRolePolicy(env.Context, &iam.AttachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: createPolicyOutput.Policy.Arn,
		})
		Expect(err).To(BeNil())

		pod := coretest.Pod()
		env.ExpectCreated(nodePool, nodeClass, pod)

		createdPod := &corev1.Pod{}
		Eventually(func(g Gomega) {
			err := env.Client.Get(env.Context, client.ObjectKey{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}, createdPod)
			g.Expect(err).To(BeNil())
		}, "30s", "5s").Should(Succeed())

		Consistently(func(g Gomega) {
			err := env.Client.Get(env.Context, client.ObjectKeyFromObject(pod), pod)
			g.Expect(err).To(BeNil())
			g.Expect(pod.Spec.NodeName).To(Equal(""))
		}, "30s", "5s").Should(Succeed())

		Eventually(func(g Gomega) {
			events := &corev1.EventList{}
			err := env.Client.List(env.Context, events)
			g.Expect(err).To(BeNil())

			found := false
			for _, event := range events.Items {
				if strings.Contains(event.Message, "Validation") &&
					strings.Contains(event.Message, "False") &&
					strings.Contains(event.Message, "unauthorized operation") &&
					strings.Contains(event.Message, "create fleet") {
					found = true
					break
				}
			}
			g.Expect(found).To(BeTrue())
		}, "90s", "5s").Should(Succeed())
	})
	It("should pass reconciliation when policy has all required permissions", func() {
		createPolicyInput := &iam.CreatePolicyInput{
			PolicyName: aws.String("DenyPolicy"),
			PolicyDocument: aws.String(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"ec2:RunInstances",
				"ec2:CreateFleet",
				"ec2:CreateLaunchTemplate",
				"ec2:DescribeLaunchTemplates"
			],
			"Resource": "*"
		}
	]
}`),
		}

		roleName = fmt.Sprintf("%s-karpenter", env.ClusterName)

		createPolicyOutput, err = env.IAMAPI.CreatePolicy(env.Context, createPolicyInput)
		Expect(err).To(BeNil())

		_, err = env.IAMAPI.AttachRolePolicy(env.Context, &iam.AttachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: createPolicyOutput.Policy.Arn,
		})
		Expect(err).To(BeNil())

		pod := coretest.Pod()
		env.ExpectCreated(nodePool, nodeClass, pod)

		createdPod := &corev1.Pod{}
		Eventually(func(g Gomega) {
			err := env.Client.Get(env.Context, client.ObjectKey{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}, createdPod)
			g.Expect(err).To(BeNil())
		}, "30s", "5s").Should(Succeed())

		Consistently(func(g Gomega) {
			err := env.Client.Get(env.Context, client.ObjectKeyFromObject(pod), pod)
			g.Expect(err).To(BeNil())
			g.Expect(pod.Spec.NodeName).To(Equal(""))
		}, "30s", "5s").Should(Succeed())

		Eventually(func(g Gomega) {
			events := &corev1.EventList{}
			err := env.Client.List(env.Context, events)
			g.Expect(err).To(BeNil())

			found := false
			for _, event := range events.Items {
				if strings.Contains(event.Message, "Validation") &&
					strings.Contains(event.Message, "True") {
					found = true
					break
				}
			}
			g.Expect(found).To(BeTrue())
		}, "60s", "5s").Should(Succeed())
	})
})
