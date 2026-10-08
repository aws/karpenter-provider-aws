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

package repair_test

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Core's reboot specs run on EC2 through upstream-e2etests; this adds what only EC2 can show: the instance is kept.
var _ = Describe("Reboot", func() {
	setGPUCondition := func(node *corev1.Node, status corev1.ConditionStatus, reason string) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			n := &corev1.Node{}
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(node), n)).To(Succeed())
			stored := n.DeepCopy()
			common.ReplaceNodeConditions(n, corev1.NodeCondition{
				Type:               "AcceleratedHardwareReady",
				Status:             status,
				Reason:             reason,
				Message:            "injected by e2e",
				LastTransitionTime: metav1.Time{Time: lo.Ternary(status == corev1.ConditionFalse, time.Now().Add(-11*time.Minute), time.Now())},
			})
			g.Expect(env.Client.Status().Patch(env, n, client.StrategicMergeFrom(stored, client.MergeFromWithOptimisticLock{}))).To(Succeed())
		}).Should(Succeed())
	}
	getNodeClaim := func(g Gomega, nodeClaim *karpv1.NodeClaim) *karpv1.NodeClaim {
		nc := &karpv1.NodeClaim{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nc)).To(Succeed())
		return nc
	}

	It("should reboot the same EC2 instance in place for a reboot-clearable GPU fault", func() {
		dep := coretest.Deployment(coretest.DeploymentOptions{
			Replicas: 1,
			PodOptions: coretest.PodOptions{
				ObjectMeta:                    metav1.ObjectMeta{Labels: map[string]string{"app": "reboot"}},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, 1)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		instance := env.GetInstance(node.Name)

		setGPUCondition(node, corev1.ConditionFalse, "NvidiaXID46Error")
		Eventually(func(g Gomega) {
			g.Expect(getNodeClaim(g, nodeClaim).StatusConditions().Get(karpv1.ConditionTypeRebooting).IsTrue()).To(BeTrue())
		}).Should(Succeed())
		setGPUCondition(node, corev1.ConditionTrue, "NvidiaGPUIsReady")
		Eventually(func(g Gomega) {
			cond := getNodeClaim(g, nodeClaim).StatusConditions().Get(karpv1.ConditionTypeRebooting)
			g.Expect(cond).ToNot(BeNil())
			g.Expect(cond.IsFalse()).To(BeTrue())
			g.Expect(cond.Reason).To(Equal(karpv1.RebootReasonSucceeded))
		}).WithTimeout(20 * time.Minute).Should(Succeed())

		Expect(env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0].Name).To(Equal(nodeClaim.Name))
		rebooted := env.GetInstanceByID(aws.ToString(instance.InstanceId))
		Expect(rebooted.State.Name).To(Equal(ec2types.InstanceStateNameRunning))
		Expect(aws.ToTime(rebooted.LaunchTime)).To(Equal(aws.ToTime(instance.LaunchTime)))
		env.EventuallyExpectHealthyPodCount(selector, 1)
	})
})
