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
	"time"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"github.com/awslabs/operatorpkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/samber/lo"

	"sigs.k8s.io/controller-runtime/pkg/client"
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

		Eventually(func(g Gomega) {
			err := env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(nodeClass.Status.InstanceProfile).ToNot(BeEmpty())
		}).Should(Succeed())

		instance := env.GetInstance(node.Name)
		Expect(instance.IamInstanceProfile).ToNot(BeNil())
		Expect(lo.FromPtr(instance.IamInstanceProfile.Arn)).To(ContainSubstring(nodeClass.Status.InstanceProfile))

		instanceProfile := env.EventuallyExpectInstanceProfileExists(nodeClass.Status.InstanceProfile)
		Expect(instanceProfile.Roles).To(HaveLen(1))
		Expect(lo.FromPtr(instanceProfile.Roles[0].RoleName)).To(Equal(nodeClass.Spec.Role))
	})
	It("should remove the generated InstanceProfile when deleting the NodeClass", func() {
		pod := coretest.Pod()
		env.ExpectCreated(nodePool, nodeClass, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		Eventually(func(g Gomega) {
			err := env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(nodeClass.Status.InstanceProfile).ToNot(BeEmpty())
		}).Should(Succeed())

		instanceProfile := nodeClass.Status.InstanceProfile

		env.ExpectDeleted(nodePool, nodeClass)
		Eventually(func(g Gomega) {
			_, err := env.IAMAPI.GetInstanceProfile(env.Context, &iam.GetInstanceProfileInput{
				InstanceProfileName: lo.ToPtr(instanceProfile),
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
})
