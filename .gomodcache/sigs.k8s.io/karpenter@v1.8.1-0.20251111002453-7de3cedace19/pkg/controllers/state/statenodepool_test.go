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

package state_test

import (
	"sync"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodePoolState", func() {
	var nodeClaim *v1.NodeClaim
	var nodePool *v1.NodePool

	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{ObjectMeta: metav1.ObjectMeta{Name: "nodepool-2"}})
		nodeClaim = test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
			},
		})
		ExpectApplied(ctx, env.Client, nodePool)
	})

	Context("ReserveNodeCount", func() {
		It("should reserve requested capacity when available", func() {
			granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 3)
			Expect(granted).To(Equal(int64(3)))

			// Should be able to reserve remaining capacity
			granted = cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 2)
			Expect(granted).To(Equal(int64(2)))
		})

		It("should grant partial capacity when requested exceeds available", func() {
			// Reserve most capacity first
			granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 4)
			Expect(granted).To(Equal(int64(4)))

			// Request more than available, should get only what's left
			granted = cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 5)
			Expect(granted).To(Equal(int64(1)))
		})

		It("should return zero when no capacity available", func() {
			// Reserve all capacity
			granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 5)
			Expect(granted).To(Equal(int64(5)))

			// No more capacity available
			granted = cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 1)
			Expect(granted).To(Equal(int64(0)))
		})

		It("should account for running NodeClaims", func() {
			// Add a running NodeClaim
			ExpectApplied(ctx, env.Client, nodeClaim)
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

			// Should have 4 slots available (5 limit - 1 running)
			granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 4)
			Expect(granted).To(Equal(int64(4)))

			// No more capacity
			granted = cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 1)
			Expect(granted).To(Equal(int64(0)))
		})

		It("should account for deleting NodeClaims", func() {
			// Add and mark NodeClaim for deletion
			ExpectApplied(ctx, env.Client, nodeClaim)
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			cluster.MarkForDeletion(nodeClaim.Status.ProviderID)

			// Should have 4 slots available (5 limit - 1 deleting)
			granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 4)
			Expect(granted).To(Equal(int64(4)))
		})

		It("should be thread-safe with concurrent reservations", func() {
			var wg sync.WaitGroup
			var totalGranted int64
			numGoroutines := 10
			requestPerGoroutine := int64(1)
			limit := int64(5)

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, limit, requestPerGoroutine)
					atomic.AddInt64(&totalGranted, granted)
				}()
			}

			wg.Wait()
			// Should not exceed the limit
			Expect(totalGranted).To(BeNumerically("<=", limit))
			Expect(totalGranted).To(Equal(limit)) // Should grant exactly the limit
		})
	})

	Context("ReleaseNodeCount", func() {
		It("should release reserved capacity", func() {
			// Reserve some capacity
			granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 3)
			Expect(granted).To(Equal(int64(3)))

			// Should have 2 slots available
			granted = cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 3)
			Expect(granted).To(Equal(int64(2)))

			// Release 1 slot
			cluster.NodePoolState.ReleaseNodeCount(nodePool.Name, 4)

			// Should now have 4 slots available
			granted = cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 4)
			Expect(granted).To(Equal(int64(4)))
		})

		It("should handle releasing more than reserved", func() {
			// Reserve some capacity
			granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 2)
			Expect(granted).To(Equal(int64(2)))

			// Release more than reserved - should not go negative
			cluster.NodePoolState.ReleaseNodeCount(nodePool.Name, 5)

			// Should have full capacity available
			granted = cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 5, 5)
			Expect(granted).To(Equal(int64(5)))
		})

		It("should be thread-safe with concurrent releases", func() {
			// Reserve capacity first
			granted := cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 10, 10)
			Expect(granted).To(Equal(int64(10)))

			var wg sync.WaitGroup
			numGoroutines := 50
			releasePerGoroutine := int64(2)

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					cluster.NodePoolState.ReleaseNodeCount(nodePool.Name, releasePerGoroutine)
				}()
			}

			wg.Wait()

			// Should have full capacity available after releases
			granted = cluster.NodePoolState.ReserveNodeCount(nodePool.Name, 10, 100)
			Expect(granted).To(Equal(int64(10)))
		})
	})
})
