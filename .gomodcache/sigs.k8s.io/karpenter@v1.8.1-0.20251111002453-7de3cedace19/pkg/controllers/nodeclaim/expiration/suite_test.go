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

package expiration_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/expiration"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var expirationController *expiration.Controller
var env *test.Environment
var cp *fake.CloudProvider
var fakeClock *clock.FakeClock

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Disruption")
}

var _ = BeforeSuite(func() {
	fakeClock = clock.NewFakeClock(time.Now())
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...), test.WithFieldIndexers(test.NodeProviderIDFieldIndexer(ctx)))
	ctx = options.ToContext(ctx, test.Options())
	cp = fake.NewCloudProvider()
	expirationController = expiration.NewController(fakeClock, env.Client, cp)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = options.ToContext(ctx, test.Options())
	fakeClock.SetTime(time.Now())
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Expiration", func() {
	var nodePool *v1.NodePool
	var nodeClaim *v1.NodeClaim
	var node *corev1.Node
	BeforeEach(func() {
		nodePool = test.NodePool()
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodePoolLabelKey: nodePool.Name},
			},
			Spec: v1.NodeClaimSpec{
				ExpireAfter: v1.MustParseNillableDuration("30s"),
			},
		})
		metrics.NodeClaimsDisruptedTotal.Reset()
	})
	Context("Metrics", func() {
		It("should fire a karpenter_nodeclaims_disrupted_total metric when expired", func() {
			ExpectApplied(ctx, env.Client, nodeClaim)

			// step forward to make the node expired
			fakeClock.Step(60 * time.Second)
			ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)

			ExpectNotFound(ctx, env.Client, nodeClaim)

			ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 1, map[string]string{
				metrics.ReasonLabel: metrics.ExpiredReason,
				"nodepool":          nodePool.Name,
			})
		})
		It("should fire a karpenter_nodeclaims_disrupted_total metric when expired", func() {
			nodeClaim.Labels[v1.CapacityTypeLabelKey] = v1.CapacityTypeSpot
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

			// step forward to make the node expired
			fakeClock.Step(60 * time.Second)
			ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)

			ExpectNotFound(ctx, env.Client, nodeClaim)
			ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 1, map[string]string{
				metrics.ReasonLabel: metrics.ExpiredReason,
				"nodepool":          nodePool.Name,
			})
		})
	})
	DescribeTable(
		"Expiration",
		func(isNodeClaimManaged bool) {
			nodeClaim.Spec.ExpireAfter = v1.MustParseNillableDuration("30s")
			if !isNodeClaimManaged {
				nodeClaim.Spec.NodeClassRef = &v1.NodeClassReference{
					Group: "karpenter.test.sh",
					Kind:  "UnmanagedNodeClass",
					Name:  "default",
				}
			}
			ExpectApplied(ctx, env.Client, nodeClaim)

			// step forward to make the node expired
			fakeClock.Step(60 * time.Second)
			ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)
			if isNodeClaimManaged {
				// with forceful termination, when we see a nodeclaim meets the conditions for expiration
				// we should remove it
				ExpectNotFound(ctx, env.Client, nodeClaim)
			} else {
				ExpectExists(ctx, env.Client, nodeClaim)
			}
		},
		Entry("should remove nodeclaims that are expired", true),
		Entry("should ignore expired NodeClaims that are not managed by this Karpenter instance", false),
	)

	It("should not remove the NodeClaims when expiration is disabled", func() {
		nodeClaim.Spec.ExpireAfter = v1.MustParseNillableDuration("Never")
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
	})
	It("should not remove non-expired NodeClaims", func() {
		nodeClaim.Spec.ExpireAfter = v1.MustParseNillableDuration("200s")
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
	})
	It("should delete NodeClaims if the nodeClaim is expired but the node isn't", func() {
		nodeClaim.Spec.ExpireAfter = v1.MustParseNillableDuration("30s")
		ExpectApplied(ctx, env.Client, nodeClaim)

		// step forward to make the node expired
		fakeClock.Step(60 * time.Second)
		ExpectApplied(ctx, env.Client, node) // node shouldn't be expired, but nodeClaim will be
		ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)

		ExpectNotFound(ctx, env.Client, nodeClaim)
	})
	It("should return the requeue interval for the time between now and when the nodeClaim expires", func() {
		nodeClaim.Spec.ExpireAfter = v1.MustParseNillableDuration("200s")
		ExpectApplied(ctx, env.Client, nodeClaim, node)

		fakeClock.SetTime(nodeClaim.CreationTimestamp.Add(time.Second * 100))

		result := ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)
		Expect(result.RequeueAfter).To(BeNumerically("~", time.Second*100, time.Second))
	})
	It("shouldn't expire the same NodeClaim multiple times", func() {
		nodeClaim.Finalizers = append(nodeClaim.Finalizers, "test-finalizer")
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

		// step forward to make the node expired
		fakeClock.Step(60 * time.Second)
		ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)
		ExpectExists(ctx, env.Client, nodeClaim)
		ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 1, map[string]string{
			metrics.ReasonLabel: metrics.ExpiredReason,
			"nodepool":          nodePool.Name,
		})
		ExpectObjectReconciled(ctx, env.Client, expirationController, nodeClaim)
		ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 1, map[string]string{
			metrics.ReasonLabel: metrics.ExpiredReason,
			"nodepool":          nodePool.Name,
		})
	})
})
