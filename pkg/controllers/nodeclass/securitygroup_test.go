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

package nodeclass_test

import (
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Security Group Status Controller", func() {
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				AMIFamily: lo.ToPtr(v1.AMIFamilyCustom),
				AMISelectorTerms: []v1.AMISelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
			},
		})
	})
	It("Should update EC2NodeClass status for Security Groups", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1.SecurityGroup{
			{
				ID:   "sg-test1",
				Name: "securityGroup-test1",
			},
			{
				ID:   "sg-test2",
				Name: "securityGroup-test2",
			},
			{
				ID:   "sg-test3",
				Name: "securityGroup-test3",
			},
		}))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsTrue()).To(BeTrue())
	})
	It("Should resolve a valid selectors for Security Groups by tags", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"Name": "test-security-group-1"},
			},
			{
				Tags: map[string]string{"Name": "test-security-group-2"},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1.SecurityGroup{
			{
				ID:   "sg-test1",
				Name: "securityGroup-test1",
			},
			{
				ID:   "sg-test2",
				Name: "securityGroup-test2",
			},
		}))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsTrue()).To(BeTrue())
	})
	It("Should resolve a valid selectors for Security Groups by ids", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				ID: "sg-test1",
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1.SecurityGroup{
			{
				ID:   "sg-test1",
				Name: "securityGroup-test1",
			},
		}))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsTrue()).To(BeTrue())
	})
	It("Should update Security Groups status when the Security Groups selector gets updated by tags", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1.SecurityGroup{
			{
				ID:   "sg-test1",
				Name: "securityGroup-test1",
			},
			{
				ID:   "sg-test2",
				Name: "securityGroup-test2",
			},
			{
				ID:   "sg-test3",
				Name: "securityGroup-test3",
			},
		}))

		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"Name": "test-security-group-1"},
			},
			{
				Tags: map[string]string{"Name": "test-security-group-2"},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1.SecurityGroup{
			{
				ID:   "sg-test1",
				Name: "securityGroup-test1",
			},
			{
				ID:   "sg-test2",
				Name: "securityGroup-test2",
			},
		}))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsTrue()).To(BeTrue())
	})
	It("Should update Security Groups status when the Security Groups selector gets updated by ids", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1.SecurityGroup{
			{
				ID:   "sg-test1",
				Name: "securityGroup-test1",
			},
			{
				ID:   "sg-test2",
				Name: "securityGroup-test2",
			},
			{
				ID:   "sg-test3",
				Name: "securityGroup-test3",
			},
		}))

		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				ID: "sg-test1",
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1.SecurityGroup{
			{
				ID:   "sg-test1",
				Name: "securityGroup-test1",
			},
		}))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsTrue()).To(BeTrue())
	})
	It("Should not resolve a invalid selectors for Security Groups", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{`foo`: `invalid`},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(BeNil())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsFalse()).To(BeTrue())
	})
	It("Should not resolve a invalid selectors for an updated Security Groups selector", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(Equal([]v1.SecurityGroup{
			{
				ID:   "sg-test1",
				Name: "securityGroup-test1",
			},
			{
				ID:   "sg-test2",
				Name: "securityGroup-test2",
			},
			{
				ID:   "sg-test3",
				Name: "securityGroup-test3",
			},
		}))

		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{`foo`: `invalid`},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(BeNil())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsFalse()).To(BeTrue())
	})
})
