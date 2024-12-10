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
	"github.com/aws/smithy-go"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/status"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Tags Status Controller", func() {
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				Tags: map[string]string{
					"fakeKey": "fakeValue",
				},
			},
		})
	})
	BeforeEach(func() {
		awsEnv.Reset()
	})
	It("Should update EC2NodeClass status for Tags", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeTagsReady)).To(BeTrue())
	})
	It("Should not update EC2NodeClass status for Tags without appropriate permissions", func() {
		ec2api := fake.NewEC2API()
		ec2api.NextError.Set(&smithy.GenericAPIError{
			Code: "UnauthorizedOperation",
		})
		ExpectApplied(ctx, env.Client, nodeClass)
		statusController := status.NewController(env.Client, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider, awsEnv.LaunchTemplateProvider, ec2api)
		_, err := statusController.Reconcile(ctx, nodeClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("UnauthorizedOperation"))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(v1.ConditionTypeTagsReady)).To(BeFalse())
	})
})
