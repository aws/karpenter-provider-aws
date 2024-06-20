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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("Termination", func() {
	It("should terminate the node and the instance on deletion", func() {
		pod := test.Pod()
		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		nodes := env.Monitor.CreatedNodes()
		instanceID := env.ExpectParsedProviderID(nodes[0].Spec.ProviderID)
		env.GetInstance(nodes[0].Name)

		// Pod is deleted so that we don't re-provision after node deletion
		// NOTE: We have to do this right now to deal with a race condition in nodepool ownership
		// This can be removed once this race is resolved with the NodePool
		env.ExpectDeleted(pod)

		// Node is deleted and now should be not found
		env.ExpectDeleted(nodes[0])
		env.EventuallyExpectNotFound(nodes[0])
		Eventually(func(g Gomega) {
			g.Expect(lo.FromPtr(env.GetInstanceByID(instanceID).State.Name)).To(BeElementOf("terminated", "shutting-down"))
		}, time.Second*10).Should(Succeed())
	})
})
