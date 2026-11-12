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

package integration

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

	. "github.com/awslabs/operatorpkg/test/expectations" //nolint:staticcheck
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func registerInstanceProfile() bool {
	return Describe("InstanceProfile Generation", func() {
		BeforeEach(func() {
			if Env.PrivateCluster {
				Skip("skipping InstanceProfile Generation test for private cluster")
			}
		})
		It("should generate the InstanceProfile when setting the role", func() {
			pod := coretest.Pod()
			Env.ExpectCreated(NodePool, NodeClass, pod)
			Env.EventuallyExpectHealthy(pod)
			node := Env.ExpectCreatedNodeCount("==", 1)[0]

			Eventually(func(g Gomega) {
				err := Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(NodeClass.Status.InstanceProfile).ToNot(BeEmpty())
			}).Should(Succeed())

			instance := Env.GetInstance(node.Name)
			Expect(instance.IamInstanceProfile).ToNot(BeNil())
			Expect(lo.FromPtr(instance.IamInstanceProfile.Arn)).To(ContainSubstring(NodeClass.Status.InstanceProfile))

			instanceProfile := Env.EventuallyExpectInstanceProfileExists(NodeClass.Status.InstanceProfile)
			Expect(instanceProfile.Roles).To(HaveLen(1))
			Expect(lo.FromPtr(instanceProfile.Roles[0].RoleName)).To(Equal(NodeClass.Spec.Role))
		})
		It("should remove the generated InstanceProfile when deleting the NodeClass", func() {
			pod := coretest.Pod()
			Env.ExpectCreated(NodePool, NodeClass, pod)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Eventually(func(g Gomega) {
				err := Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(NodeClass.Status.InstanceProfile).ToNot(BeEmpty())
			}).Should(Succeed())

			instanceProfile := NodeClass.Status.InstanceProfile

			Env.ExpectDeleted(NodePool, NodeClass)
			Eventually(func(g Gomega) {
				_, err := Env.IAMAPI.GetInstanceProfile(Env.Context, &iam.GetInstanceProfileInput{
					InstanceProfileName: lo.ToPtr(instanceProfile),
				})
				g.Expect(awserrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})
		It("should use the unmanaged instance profile", func() {
			instanceProfileName := fmt.Sprintf("KarpenterNodeInstanceProfile-%s", Env.ClusterName)
			roleName := fmt.Sprintf("KarpenterNodeRole-%s", Env.ClusterName)
			Env.ExpectInstanceProfileCreated(instanceProfileName, roleName)
			DeferCleanup(func() {
				Env.ExpectInstanceProfileDeleted(instanceProfileName, roleName)
			})

			pod := coretest.Pod()
			NodeClass.Spec.Role = ""
			NodeClass.Spec.InstanceProfile = lo.ToPtr(fmt.Sprintf("KarpenterNodeInstanceProfile-%s", Env.ClusterName))
			Env.ExpectCreated(NodePool, NodeClass, pod)
			Env.EventuallyExpectHealthy(pod)
			node := Env.ExpectCreatedNodeCount("==", 1)[0]

			instance := Env.GetInstance(node.Name)
			Expect(instance.IamInstanceProfile).ToNot(BeNil())
			Expect(lo.FromPtr(instance.IamInstanceProfile.Arn)).To(ContainSubstring(NodeClass.Status.InstanceProfile))
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeInstanceProfileReady, Status: metav1.ConditionTrue})
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionTrue})
		})
		It("should have the EC2NodeClass status as not ready since Instance Profile was not resolved", func() {
			NodeClass.Spec.Role = fmt.Sprintf("KarpenterNodeRole-%s", "invalidRole")
			Env.ExpectCreated(NodeClass)
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeInstanceProfileReady, Status: metav1.ConditionUnknown})
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeValidationSucceeded, Status: metav1.ConditionUnknown})
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionUnknown})
		})
	})
}
