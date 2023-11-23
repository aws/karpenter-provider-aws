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

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/test"
)

var _ = Describe("Emptiness", func() {
	It("should terminate an empty node", func() {
		nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenEmpty
		nodePool.Spec.Disruption.ConsolidateAfter = &corev1beta1.NillableDuration{Duration: lo.ToPtr(time.Hour * 300)}

		const numPods = 1
		deployment := test.Deployment(test.DeploymentOptions{Replicas: numPods})

		By("kicking off provisioning for a deployment")
		env.ExpectCreated(nodeClass, nodePool, deployment)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), numPods)

		By("making the nodeclaim empty")
		persisted := deployment.DeepCopy()
		deployment.Spec.Replicas = ptr.Int32(0)
		Expect(env.Client.Patch(env, deployment, client.MergeFrom(persisted))).To(Succeed())

		By("waiting for the nodeclaim emptiness status condition to propagate")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Empty).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		By("waiting for the nodeclaim to deprovision when past its ConsolidateAfter timeout of 0")
		nodePool.Spec.Disruption.ConsolidateAfter = &corev1beta1.NillableDuration{Duration: lo.ToPtr(time.Duration(0))}
		env.ExpectUpdated(nodePool)

		env.EventuallyExpectNotFound(nodeClaim, node)
	})
})
