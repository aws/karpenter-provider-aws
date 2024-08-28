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

package termination_test

import (
	"fmt"
	"time"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("Emptiness", func() {
	var dep *appsv1.Deployment
	var selector labels.Selector
	var numPods int
	BeforeEach(func() {
		nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmpty
		nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")

		numPods = 1
		dep = test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
			},
		})
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})
	Context("Budgets", func() {
		It("should not allow emptiness if the budget is fully blocking", func() {
			// We're going to define a budget that doesn't allow any emptiness disruption to happen
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
				Nodes: "0",
			}}

			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			env.EventuallyExpectCreatedNodeCount("==", 1)
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			// Delete the deployment so there is nothing running on the node
			env.ExpectDeleted(dep)

			env.EventuallyExpectConsolidatable(nodeClaim)
			env.ConsistentlyExpectNoDisruptions(1, time.Minute)
		})
		It("should not allow emptiness if the budget is fully blocking during a scheduled time", func() {
			// We're going to define a budget that doesn't allow any emptiness disruption to happen
			// This is going to be on a schedule that only lasts 30 minutes, whose window starts 15 minutes before
			// the current time and extends 15 minutes past the current time
			// Times need to be in UTC since the karpenter containers were built in UTC time
			windowStart := time.Now().Add(-time.Minute * 15).UTC()
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
				Nodes:    "0",
				Schedule: lo.ToPtr(fmt.Sprintf("%d %d * * *", windowStart.Minute(), windowStart.Hour())),
				Duration: &metav1.Duration{Duration: time.Minute * 30},
			}}

			env.ExpectCreated(nodeClass, nodePool, dep)

			nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			env.EventuallyExpectCreatedNodeCount("==", 1)
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			// Delete the deployment so there is nothing running on the node
			env.ExpectDeleted(dep)

			env.EventuallyExpectConsolidatable(nodeClaim)
			env.ConsistentlyExpectNoDisruptions(1, time.Minute)
		})
	})
	It("should terminate an empty node", func() {
		nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("10s")

		const numPods = 1
		deployment := test.Deployment(test.DeploymentOptions{Replicas: numPods})

		By("kicking off provisioning for a deployment")
		env.ExpectCreated(nodeClass, nodePool, deployment)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), numPods)

		By("making the nodeclaim empty")
		persisted := deployment.DeepCopy()
		deployment.Spec.Replicas = lo.ToPtr(int32(0))
		Expect(env.Client.Patch(env, deployment, client.StrategicMergeFrom(persisted))).To(Succeed())

		env.EventuallyExpectConsolidatable(nodeClaim)

		By("waiting for the nodeclaim to deprovision when past its ConsolidateAfter timeout of 0")
		nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
		env.ExpectUpdated(nodePool)

		env.EventuallyExpectNotFound(nodeClaim, node)
	})
})
