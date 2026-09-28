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

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reboot", func() {
	var dep *appsv1.Deployment
	var selector labels.Selector

	BeforeEach(func() {
		dep = coretest.Deployment(coretest.DeploymentOptions{
			Replicas: 1,
			PodOptions: coretest.PodOptions{
				ObjectMeta:                    metav1.ObjectMeta{Labels: map[string]string{"app": "reboot"}},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})

	It("should reboot the EC2 instance in place when a reboot is handed off through the Rebooting condition", func() {
		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, 1)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		instance := env.GetInstance(node.Name)
		preBootID := node.Status.NodeInfo.BootID

		// Commit a reboot the way any consumer does: the drain bound, then Rebooting=RebootRequested.
		nodeClaim = env.ExpectExists(nodeClaim).(*karpv1.NodeClaim)
		nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{karpv1.RebootTerminationGracePeriodAnnotationKey: "5m"})
		env.ExpectUpdated(nodeClaim)
		nodeClaim = env.ExpectExists(nodeClaim).(*karpv1.NodeClaim)
		nodeClaim.StatusConditions().SetTrueWithReason(karpv1.ConditionTypeRebooting, karpv1.RebootReasonRequested, "rebooting for e2e")
		env.ExpectStatusUpdated(nodeClaim)

		// The reboot is issued to EC2, then succeeds once the node proves a new boot and rejoins.
		Eventually(func(g Gomega) {
			nc := &karpv1.NodeClaim{}
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nc)).To(Succeed())
			cond := nc.StatusConditions().Get(karpv1.ConditionTypeRebooting)
			g.Expect(cond).ToNot(BeNil())
			g.Expect(cond.IsFalse()).To(BeTrue())
			g.Expect(cond.Reason).To(Equal(karpv1.RebootReasonSucceeded))
		}).WithTimeout(20 * time.Minute).Should(Succeed())

		// In place: the same Node with a new boot, the fence removed, re-initialized; the same NodeClaim; and the same
		// EC2 instance, still running and with its original launch time.
		Eventually(func(g Gomega) {
			n := &corev1.Node{}
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(node), n)).To(Succeed())
			g.Expect(n.Status.NodeInfo.BootID).ToNot(Equal(preBootID))
			g.Expect(n.Spec.Taints).ToNot(ContainElement(HaveField("Key", karpv1.RebootingTaintKey)))
			g.Expect(n.Labels).To(HaveKeyWithValue(karpv1.NodeInitializedLabelKey, "true"))
		}).Should(Succeed())
		nodeClaims := env.EventuallyExpectCreatedNodeClaimCount("==", 1)
		Expect(nodeClaims[0].Name).To(Equal(nodeClaim.Name))
		rebooted := env.GetInstanceByID(aws.ToString(instance.InstanceId))
		Expect(rebooted.State.Name).To(Equal(ec2types.InstanceStateNameRunning))
		Expect(aws.ToTime(rebooted.LaunchTime)).To(Equal(aws.ToTime(instance.LaunchTime)))
		env.EventuallyExpectHealthyPodCount(selector, 1)
	})
})
