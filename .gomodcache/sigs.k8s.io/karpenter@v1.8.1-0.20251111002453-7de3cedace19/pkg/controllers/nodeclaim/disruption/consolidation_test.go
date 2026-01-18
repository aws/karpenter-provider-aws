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

package disruption_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Underutilized", func() {
	var nodePool *v1.NodePool
	var nodeClaim *v1.NodeClaim
	BeforeEach(func() {
		nodePool = test.NodePool()
		nodePool.Spec.Disruption.ConsolidationPolicy = v1.ConsolidationPolicyWhenEmptyOrUnderutilized
		nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("1m")
		nodeClaim, _ = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: it.Name,
				},
			},
		})
		// set the lastPodEvent to 5 minutes in the past
		nodeClaim.Status.LastPodEventTime.Time = fakeClock.Now().Add(-5 * time.Minute)
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeInitialized)
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool)
	})
	It("should ignore NodeClaims not managed by this instance of Karpenter", func() {
		unmanagedNodeClaim, _ := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: it.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				NodeClassRef: &v1.NodeClassReference{
					Group: "karpenter.test.sh",
					Kind:  "UnmanagedNodeClass",
					Name:  "default",
				},
			},
		})
		unmanagedNodeClaim.Status.LastPodEventTime.Time = fakeClock.Now().Add(-5 * time.Minute)
		unmanagedNodeClaim.StatusConditions().SetTrue(v1.ConditionTypeInitialized)
		ExpectApplied(ctx, env.Client, unmanagedNodeClaim, nodePool)

		// set the lastPodEvent as now, so it's first marked as not consolidatable
		unmanagedNodeClaim.Status.LastPodEventTime.Time = fakeClock.Now()
		ExpectApplied(ctx, env.Client, unmanagedNodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, unmanagedNodeClaim)
		unmanagedNodeClaim = ExpectExists(ctx, env.Client, unmanagedNodeClaim)
		Expect(unmanagedNodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable).IsUnknown()).To(BeTrue())

		fakeClock.Step(1 * time.Minute)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, unmanagedNodeClaim)
		unmanagedNodeClaim = ExpectExists(ctx, env.Client, unmanagedNodeClaim)
		Expect(unmanagedNodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable).IsUnknown()).To(BeTrue())
	})
	It("should mark NodeClaims as consolidatable", func() {
		// set the lastPodEvent as now, so it's first marked as not consolidatable
		nodeClaim.Status.LastPodEventTime.Time = fakeClock.Now()
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable).IsTrue()).To(BeFalse())

		fakeClock.Step(1 * time.Minute)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable).IsTrue()).To(BeTrue())
	})
	It("should mark NodeClaims as consolidatable based on the nodeclaim initialized time", func() {
		// set the lastPodEvent as zero, so it's like no pods have scheduled
		nodeClaim.Status.LastPodEventTime.Time = time.Time{}

		ExpectApplied(ctx, env.Client, nodeClaim)
		fakeClock.SetTime(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).LastTransitionTime.Time)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable).IsTrue()).To(BeFalse())

		fakeClock.Step(1 * time.Minute)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable).IsTrue()).To(BeTrue())
	})
	It("should remove the status condition from the nodeClaim when lastPodEvent is too recent", func() {
		nodeClaim.Status.LastPodEventTime.Time = fakeClock.Now()
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		ExpectApplied(ctx, env.Client, nodeClaim)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable)).To(BeNil())
	})
	It("should remove the status condition from the nodeClaim when consolidateAfter is never", func() {
		nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("Never")
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable)).To(BeNil())
	})
	It("should remove the status condition from the nodeClaim when the nodeClaim initialization condition is unknown", func() {
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		nodeClaim.StatusConditions().SetUnknown(v1.ConditionTypeInitialized)
		ExpectApplied(ctx, env.Client, nodeClaim)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable)).To(BeNil())
	})
	It("should remove the status condition from the nodeClaim when the nodeClaim is not initialized", func() {
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		nodeClaim.StatusConditions().SetFalse(v1.ConditionTypeInitialized, "NotInitialized", "NotInitialized")
		ExpectApplied(ctx, env.Client, nodeClaim)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimDisruptionController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable)).To(BeNil())
	})
})
