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

package garbagecollection_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	nodeclaimgarbagecollection "sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/garbagecollection"
	nodeclaimlifcycle "sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/lifecycle"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/state/nodepoolhealth"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	ctx                         context.Context
	nodeClaimController         *nodeclaimlifcycle.Controller
	garbageCollectionController *nodeclaimgarbagecollection.Controller
	env                         *test.Environment
	fakeClock                   *clock.FakeClock
	cloudProvider               *fake.CloudProvider
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "GarbageCollection")
}

var _ = BeforeSuite(func() {
	fakeClock = clock.NewFakeClock(time.Now())
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...), test.WithFieldIndexers(test.NodeProviderIDFieldIndexer(ctx)))
	ctx = options.ToContext(ctx, test.Options())
	cloudProvider = fake.NewCloudProvider()
	garbageCollectionController = nodeclaimgarbagecollection.NewController(fakeClock, env.Client, cloudProvider)
	nodeClaimController = nodeclaimlifcycle.NewController(fakeClock, env.Client, cloudProvider, events.NewRecorder(&record.FakeRecorder{}), nodepoolhealth.NewState())
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	fakeClock.SetTime(time.Now())
	ExpectCleanedUp(ctx, env.Client)
	cloudProvider.Reset()
})

var _ = Describe("GarbageCollection", func() {
	var nodePool *v1.NodePool

	BeforeEach(func() {
		nodePool = test.NodePool()
	})
	It("should delete the NodeClaim when the Node is there in a NotReady state and the instance is gone", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

		nodeClaim, node, err := ExpectNodeClaimDeployed(ctx, env.Client, cloudProvider, nodeClaim)
		Expect(err).ToNot(HaveOccurred())

		// Mark the node as NotReady after the launch
		ExpectMakeNodesNotReady(ctx, env.Client, node)

		// Step forward to move past the cache eventual consistency timeout
		fakeClock.SetTime(time.Now().Add(time.Second * 20))

		// Delete the nodeClaim from the cloudprovider
		Expect(cloudProvider.Delete(ctx, nodeClaim)).To(Succeed())

		// Expect the NodeClaim to not be removed since there is a Node that exists that has a Ready "true" condition
		ExpectSingletonReconciled(ctx, garbageCollectionController)
		ExpectFinalizersRemoved(ctx, env.Client, nodeClaim)
		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
	It("shouldn't delete the NodeClaim when the Node is there in a Ready state and the instance is gone", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

		nodeClaim, _, err := ExpectNodeClaimDeployed(ctx, env.Client, cloudProvider, nodeClaim)
		Expect(err).ToNot(HaveOccurred())

		// Step forward to move past the cache eventual consistency timeout
		fakeClock.SetTime(time.Now().Add(time.Second * 20))

		// Delete the nodeClaim from the cloudprovider
		Expect(cloudProvider.Delete(ctx, nodeClaim)).To(Succeed())

		// Expect the NodeClaim to not be removed since there is a Node that exists that has a Ready "true" condition
		ExpectSingletonReconciled(ctx, garbageCollectionController)
		ExpectFinalizersRemoved(ctx, env.Client, nodeClaim)
		ExpectExists(ctx, env.Client, nodeClaim)
	})
	It("should delete many NodeClaims when the Nodes are there in a NotReady state and the instances are gone", func() {
		var nodeClaims []*v1.NodeClaim
		for i := 0; i < 100; i++ {
			nodeClaims = append(nodeClaims, test.NodeClaim(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
			}))
		}
		ExpectApplied(ctx, env.Client, nodePool)
		workqueue.ParallelizeUntil(ctx, len(nodeClaims), len(nodeClaims), func(i int) {
			defer GinkgoRecover()
			ExpectApplied(ctx, env.Client, nodeClaims[i])
			var node *corev1.Node
			var err error
			nodeClaims[i], node, err = ExpectNodeClaimDeployed(ctx, env.Client, cloudProvider, nodeClaims[i])
			Expect(err).ToNot(HaveOccurred())

			// Mark the node as NotReady after the launch
			ExpectMakeNodesNotReady(ctx, env.Client, node)
		})

		// Step forward to move past the cache eventual consistency timeout
		fakeClock.SetTime(time.Now().Add(time.Second * 20))

		workqueue.ParallelizeUntil(ctx, len(nodeClaims), len(nodeClaims), func(i int) {
			defer GinkgoRecover()
			// Delete the NodeClaim from the cloudprovider
			Expect(cloudProvider.Delete(ctx, nodeClaims[i])).To(Succeed())
		})

		// Expect the NodeClaims to be removed now that the Instance is gone
		ExpectSingletonReconciled(ctx, garbageCollectionController)

		workqueue.ParallelizeUntil(ctx, len(nodeClaims), len(nodeClaims), func(i int) {
			defer GinkgoRecover()
			ExpectFinalizersRemoved(ctx, env.Client, nodeClaims[i])
		})
		ExpectNotFound(ctx, env.Client, lo.Map(nodeClaims, func(n *v1.NodeClaim, _ int) client.Object { return n })...)
	})
	It("shouldn't delete the NodeClaim when the Node isn't there and the instance is gone", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		nodeClaim, err := ExpectNodeClaimDeployedNoNode(ctx, env.Client, cloudProvider, nodeClaim)
		Expect(err).ToNot(HaveOccurred())

		// Step forward to move past the cache eventual consistency timeout
		fakeClock.SetTime(time.Now().Add(time.Second * 20))

		// Delete the nodeClaim from the cloudprovider
		Expect(cloudProvider.Delete(ctx, nodeClaim)).To(Succeed())

		// Expect the NodeClaim to not be removed since the NodeClaim isn't registered
		ExpectSingletonReconciled(ctx, garbageCollectionController)
		ExpectFinalizersRemoved(ctx, env.Client, nodeClaim)
		ExpectExists(ctx, env.Client, nodeClaim)
	})
	It("shouldn't delete the NodeClaim when the Node isn't there but the instance is there", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		nodeClaim, node, err := ExpectNodeClaimDeployed(ctx, env.Client, cloudProvider, nodeClaim)
		Expect(err).ToNot(HaveOccurred())

		Expect(env.Client.Delete(ctx, node)).To(Succeed())

		// Step forward to move past the cache eventual consistency timeout
		fakeClock.SetTime(time.Now().Add(time.Second * 20))

		// Reconcile the NodeClaim. It should not be deleted by this flow since it has never been registered
		ExpectSingletonReconciled(ctx, garbageCollectionController)
		ExpectFinalizersRemoved(ctx, env.Client, nodeClaim)
		ExpectExists(ctx, env.Client, nodeClaim)
	})
})
