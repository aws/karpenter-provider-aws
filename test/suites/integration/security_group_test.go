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
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/awslabs/operatorpkg/test/expectations"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SecurityGroups", func() {
	It("should use the security-group-id selector", func() {
		securityGroups := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(securityGroups)).To(BeNumerically(">", 1))
		nodeClass.Spec.SecurityGroupSelectorTerms = lo.Map(securityGroups, func(sg aws.SecurityGroup, _ int) v1.SecurityGroupSelectorTerm {
			return v1.SecurityGroupSelectorTerm{
				ID: lo.FromPtr(sg.GroupId),
			}
		})
		pod := test.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SecurityGroups", ConsistOf(securityGroups[0].GroupIdentifier, securityGroups[1].GroupIdentifier)))
	})

	It("should use the security group selector with multiple tag values", func() {
		securityGroups := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(securityGroups)).To(BeNumerically(">", 1))
		first := securityGroups[0]
		last := securityGroups[len(securityGroups)-1]

		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"Name": lo.FromPtr(lo.FindOrElse(first.Tags, ec2types.Tag{}, func(tag ec2types.Tag) bool { return lo.FromPtr(tag.Key) == "Name" }).Value)},
			},
			{
				Tags: map[string]string{"Name": lo.FromPtr(lo.FindOrElse(last.Tags, ec2types.Tag{}, func(tag ec2types.Tag) bool { return lo.FromPtr(tag.Key) == "Name" }).Value)},
			},
		}
		pod := test.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SecurityGroups", ConsistOf(first.GroupIdentifier, last.GroupIdentifier)))
	})

	It("should update the EC2NodeClass status security groups", func() {
		env.ExpectCreated(nodeClass)
		EventuallyExpectSecurityGroups(env, nodeClass)
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeSecurityGroupsReady, Status: metav1.ConditionTrue})
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionTrue})
	})

	It("should have the NodeClass status as not ready since security groups were not resolved", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"karpenter.sh/discovery": "invalidName"},
			},
		}
		env.ExpectCreated(nodeClass)
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: v1.ConditionTypeSecurityGroupsReady, Status: metav1.ConditionFalse, Message: "SecurityGroupSelector did not match any SecurityGroups"})
		ExpectStatusConditions(env, env.Client, 1*time.Minute, nodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionFalse, Message: "ValidationSucceeded=False, SecurityGroupsReady=False"})
	})
})

func EventuallyExpectSecurityGroups(env *aws.Environment, nodeClass *v1.EC2NodeClass) {
	securityGroups := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": env.ClusterName})
	Expect(securityGroups).ToNot(HaveLen(0))

	ids := sets.New(lo.Map(securityGroups, func(s aws.SecurityGroup, _ int) string {
		return lo.FromPtr(s.GroupId)
	})...)
	Eventually(func(g Gomega) {
		temp := &v1.EC2NodeClass{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClass), temp)).To(Succeed())
		g.Expect(sets.New(lo.Map(temp.Status.SecurityGroups, func(s v1.SecurityGroup, _ int) string {
			return s.ID
		})...).Equal(ids))
	}).WithTimeout(10 * time.Second).Should(Succeed())
}
