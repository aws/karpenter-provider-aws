/*
Copyright The Kubernetes Authors.

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

package lifecycle_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Launch", func() {
	var nodePool *v1.NodePool
	BeforeEach(func() {
		nodePool = test.NodePool()
	})
	DescribeTable(
		"Launch",
		func(isNodeClaimManaged bool) {
			nodeClaimOpts := []v1.NodeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
			}}
			if !isNodeClaimManaged {
				nodeClaimOpts = append(nodeClaimOpts, v1.NodeClaim{
					Spec: v1.NodeClaimSpec{
						NodeClassRef: &v1.NodeClassReference{
							Group: "karpenter.test.sh",
							Kind:  "UnmanagedNodeClass",
							Name:  "default",
						},
					},
				})
			}
			nodeClaim := test.NodeClaim(nodeClaimOpts...)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

			Expect(cloudProvider.CreateCalls).To(HaveLen(lo.Ternary(isNodeClaimManaged, 1, 0)))
			Expect(cloudProvider.CreatedNodeClaims).To(HaveLen(lo.Ternary(isNodeClaimManaged, 1, 0)))
			if isNodeClaimManaged {
				_, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID)
				Expect(err).ToNot(HaveOccurred())
			}
		},
		Entry("should launch an instance when a new NodeClaim is created", true),
		Entry("should ignore NodeClaims which aren't managed by this Karpenter instance", false),
	)
	It("should add the Launched status condition after creating the NodeClaim", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeLaunched).Status).To(Equal(metav1.ConditionTrue))
	})
	It("should delete the nodeclaim if InsufficientCapacity is returned from the cloudprovider", func() {
		cloudProvider.NextCreateErr = cloudprovider.NewInsufficientCapacityError(fmt.Errorf("all instance types were unavailable"))
		nodeClaim := test.NodeClaim()
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectFinalizersRemoved(ctx, env.Client, nodeClaim)
		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
	It("should delete the nodeclaim if NodeClassNotReady is returned from the cloudprovider", func() {
		cloudProvider.NextCreateErr = cloudprovider.NewNodeClassNotReadyError(fmt.Errorf("nodeClass isn't ready"))
		nodeClaim := test.NodeClaim()
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectFinalizersRemoved(ctx, env.Client, nodeClaim)
		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
	It("should set nodeClaim status condition from the condition message received if error returned is CreateError", func() {
		conditionReason := "CustomReason"
		conditionMessage := "instance creation failed"
		cloudProvider.NextCreateErr = cloudprovider.NewCreateError(fmt.Errorf("error launching instance"), conditionReason, conditionMessage)
		nodeClaim := test.NodeClaim()
		ExpectApplied(ctx, env.Client, nodeClaim)
		_ = ExpectObjectReconcileFailed(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		condition := ExpectStatusConditionExists(nodeClaim, v1.ConditionTypeLaunched)
		Expect(condition.Status).To(Equal(metav1.ConditionUnknown))
		Expect(condition.Reason).To(Equal(conditionReason))
		Expect(condition.Message).To(Equal(conditionMessage))
	})
})
