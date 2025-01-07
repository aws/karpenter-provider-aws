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
	"github.com/aws/karpenter-provider-aws/pkg/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Authorized Status Controller", func() {
	It("should update status condition on nodeClass as NotReady when nodeclass has authorization failure due to createfleet", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		awsEnv.EC2API.CreateFleetBehavior.Error.Set(&smithy.GenericAPIError{
			Code: "UnauthorizedOperation",
		}, fake.MaxCalls(1))
		err := ExpectObjectReconcileFailed(ctx, env.Client, statusController, nodeClass)
		Expect(err).To(HaveOccurred())
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Conditions).To(HaveLen(7))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeAuthorization).IsFalse()).To(BeTrue())
	})
	It("should update status condition on nodeClass as NotReady when nodeclass has authorization failure due to terminateinstances", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		awsEnv.EC2API.TerminateInstancesBehavior.Error.Set(&smithy.GenericAPIError{
			Code: "UnauthorizedOperation",
		}, fake.MaxCalls(2))
		err := ExpectObjectReconcileFailed(ctx, env.Client, statusController, nodeClass)
		Expect(err).To(HaveOccurred())
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Conditions).To(HaveLen(7))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeAuthorization).IsFalse()).To(BeTrue())
	})
	It("should update status condition on nodeClass as NotReady when nodeclass has authorization failure due to CreateTags", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		awsEnv.EC2API.CreateTagsBehavior.Error.Set(&smithy.GenericAPIError{
			Code: "UnauthorizedOperation",
		}, fake.MaxCalls(1))
		err := ExpectObjectReconcileFailed(ctx, env.Client, statusController, nodeClass)
		Expect(err).To(HaveOccurred())
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Status.Conditions).To(HaveLen(7))
		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeAuthorization).IsFalse()).To(BeTrue())
	})
	It("should update status condition as Ready", func() {
		nodeClass.Spec.Tags = map[string]string{}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeAuthorization).IsTrue()).To(BeTrue())
	})
})
