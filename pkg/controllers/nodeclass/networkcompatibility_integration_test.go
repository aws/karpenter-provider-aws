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
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/awslabs/operatorpkg/status"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/karpenter/pkg/events"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Network Compatibility", func() {
	DescribeTable("should refresh diagnostics independently of dry-run validation", func(disableDryRun bool) {
		controller = nodeclass.NewController(
			awsEnv.Clock, env.Client, cloudProvider, events.NewRecorder(record.NewFakeRecorder(100)), fake.DefaultRegion,
			awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider,
			awsEnv.InstanceTypesProvider, awsEnv.LaunchTemplateProvider, awsEnv.CapacityReservationProvider,
			awsEnv.PlacementGroupProvider, awsEnv.EC2API, awsEnv.ValidationCache, awsEnv.RecreationCache,
			awsEnv.AMIResolver, awsEnv.CELEnvironment, disableDryRun,
		)
		awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
			networkSubnet("subnet-test1", "vpc-test1"), networkSubnet("subnet-test2", "vpc-other"),
		}})
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("PartiallyIncompatible"))
		Expect(nodeClass.Status.Subnets).To(HaveLen(2))
		Expect(nodeClass.Status.SecurityGroups).To(HaveLen(3))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
		Expect(awsEnv.ValidationCache.Items()).To(HaveLen(1))
		validationCalls := awsEnv.EC2API.RunInstancesBehavior.Calls()
		if disableDryRun {
			Expect(validationCalls).To(BeZero())
		} else {
			Expect(validationCalls).To(Equal(1))
		}

		// Neither the NodeClass spec nor the dry-run cache changes when an association is added in AWS.
		awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Output.Set(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{
			networkAssociation("sg-test1", "vpc-other", ec2types.SecurityGroupVpcAssociationStateAssociated),
			networkAssociation("sg-test2", "vpc-other", ec2types.SecurityGroupVpcAssociationStateAssociated),
			networkAssociation("sg-test3", "vpc-other", ec2types.SecurityGroupVpcAssociationStateAssociated),
		}})
		awsEnv.Clock.Step(time.Minute)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).IsTrue()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("Compatible"))
		Expect(awsEnv.EC2API.RunInstancesBehavior.Calls()).To(Equal(validationCalls))
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())

		// Association permission failures are Unknown diagnostics, and do not invalidate the NodeClass.
		awsEnv.Clock.Step(time.Minute)
		awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Error.Set(&smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "access denied"})
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).IsUnknown()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("CompatibilityCheckFailed"))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded).IsTrue()).To(BeTrue())
		Expect(nodeClass.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
		Expect(awsEnv.EC2API.RunInstancesBehavior.Calls()).To(Equal(validationCalls))
	}, Entry("with dry-run disabled", true), Entry("with cached successful dry-run", false))

	It("should wait when security group selection matches nothing", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{ID: "sg-missing"}}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.SecurityGroups).To(BeEmpty())
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("DependenciesNotReady"))
		Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(BeZero())
	})
})
