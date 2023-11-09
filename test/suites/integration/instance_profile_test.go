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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	coretest "github.com/aws/karpenter-core/pkg/test"
	awserrors "github.com/aws/karpenter/pkg/errors"
)

var _ = Describe("InstanceProfile Generation", func() {
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
			_, err := env.IAMAPI.GetInstanceProfileWithContext(env.Context, &iam.GetInstanceProfileInput{
				InstanceProfileName: aws.String(env.GetInstanceProfileName(nodeClass)),
			})
			g.Expect(awserrors.IsNotFound(err)).To(BeTrue())
		}).Should(Succeed())
	})
})
