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

package status_test

import (
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Subnet Status Controller", func() {
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1beta1.EC2NodeClass{
			Spec: v1beta1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				AMISelectorTerms: []v1beta1.AMISelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
			},
		})
	})
	It("Should update EC2NodeClass status for Subnets", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
			{
				ID:   "subnet-test2",
				Zone: "test-zone-1b",
			},
			{
				ID:   "subnet-test3",
				Zone: "test-zone-1c",
			},
			{
				ID:   "subnet-test4",
				Zone: "test-zone-1a-local",
			},
		}))
	})
	It("Should have the correct ordering for the Subnets", func() {
		awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
			{SubnetId: aws.String("subnet-test1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(20)},
			{SubnetId: aws.String("subnet-test2"), AvailabilityZone: aws.String("test-zone-1b"), AvailableIpAddressCount: aws.Int64(100)},
			{SubnetId: aws.String("subnet-test3"), AvailabilityZone: aws.String("test-zone-1c"), AvailableIpAddressCount: aws.Int64(50)},
		}})
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test2",
				Zone: "test-zone-1b",
			},
			{
				ID:   "subnet-test3",
				Zone: "test-zone-1c",
			},
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
		}))
	})
	It("Should resolve a valid selectors for Subnet by tags", func() {
		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{`Name`: `test-subnet-1`},
			},
			{
				Tags: map[string]string{`Name`: `test-subnet-2`},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
			{
				ID:   "subnet-test2",
				Zone: "test-zone-1b",
			},
		}))
	})
	It("Should resolve a valid selectors for Subnet by ids", func() {
		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				ID: "subnet-test1",
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
		}))
	})
	It("Should update Subnet status when the Subnet selector gets updated by tags", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
			{
				ID:   "subnet-test2",
				Zone: "test-zone-1b",
			},
			{
				ID:   "subnet-test3",
				Zone: "test-zone-1c",
			},
			{
				ID:   "subnet-test4",
				Zone: "test-zone-1a-local",
			},
		}))

		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{
					"Name": "test-subnet-1",
				},
			},
			{
				Tags: map[string]string{
					"Name": "test-subnet-2",
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
			{
				ID:   "subnet-test2",
				Zone: "test-zone-1b",
			},
		}))
	})
	It("Should update Subnet status when the Subnet selector gets updated by ids", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
			{
				ID:   "subnet-test2",
				Zone: "test-zone-1b",
			},
			{
				ID:   "subnet-test3",
				Zone: "test-zone-1c",
			},
			{
				ID:   "subnet-test4",
				Zone: "test-zone-1a-local",
			},
		}))

		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				ID: "subnet-test1",
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
		}))
	})
	It("Should not resolve a invalid selectors for Subnet", func() {
		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{`foo`: `invalid`},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(BeNil())
		Expect(nodeClass.StatusConditions().Get(v1beta1.ConditionTypeNodeClassReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(v1beta1.ConditionTypeNodeClassReady).Message).To(Equal("unable to resolve subnets"))
	})
	It("Should not resolve a invalid selectors for an updated subnet selector", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(Equal([]v1beta1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
			{
				ID:   "subnet-test2",
				Zone: "test-zone-1b",
			},
			{
				ID:   "subnet-test3",
				Zone: "test-zone-1c",
			},
			{
				ID:   "subnet-test4",
				Zone: "test-zone-1a-local",
			},
		}))

		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{`foo`: `invalid`},
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectReconcileSucceeded(ctx, statusController, client.ObjectKeyFromObject(nodeClass))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Subnets).To(BeNil())
		Expect(nodeClass.StatusConditions().Get(v1beta1.ConditionTypeNodeClassReady).IsFalse()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(v1beta1.ConditionTypeNodeClassReady).Message).To(Equal("unable to resolve subnets"))
	})
})
