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

package health_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/node/health"
	"sigs.k8s.io/karpenter/pkg/controllers/node/termination/terminator"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var healthController *health.Controller
var env *test.Environment
var fakeClock *clock.FakeClock
var cloudProvider *fake.CloudProvider
var recorder *test.EventRecorder
var queue *terminator.Queue

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Termination")
}

var _ = BeforeSuite(func() {
	fakeClock = clock.NewFakeClock(time.Now())
	env = test.NewEnvironment(
		test.WithCRDs(apis.CRDs...),
		test.WithCRDs(v1alpha1.CRDs...),
		test.WithFieldIndexers(test.NodeClaimProviderIDFieldIndexer(ctx), test.VolumeAttachmentFieldIndexer(ctx), test.NodeProviderIDFieldIndexer(ctx)),
	)
	cloudProvider = fake.NewCloudProvider()
	cloudProvider = fake.NewCloudProvider()
	recorder = test.NewEventRecorder()
	queue = terminator.NewQueue(env.Client, recorder)
	healthController = health.NewController(env.Client, cloudProvider, fakeClock, recorder)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Node Health", func() {
	var node *corev1.Node
	var nodeClaim *v1.NodeClaim
	var nodePool *v1.NodePool

	BeforeEach(func() {
		fakeClock.SetTime(time.Now())
		cloudProvider.Reset()

		nodePool = test.NodePool()
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{v1.TerminationFinalizer}}})
		node.Labels[v1.NodePoolLabelKey] = nodePool.Name
		nodeClaim.Labels[v1.NodePoolLabelKey] = nodePool.Name
		cloudProvider.CreatedNodeClaims[node.Spec.ProviderID] = nodeClaim
	})

	AfterEach(func() {
		ExpectCleanedUp(ctx, env.Client)

		// Reset the metrics collectors
		metrics.NodeClaimsDisruptedTotal.Reset()
	})

	Context("Reconciliation", func() {
		It("should delete nodes that are unhealthy by the cloud provider", func() {
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:               "BadNode",
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.DeletionTimestamp).ToNot(BeNil())
		})
		It("should not delete node when unhealthy type does not match cloud provider passed in value", func() {
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:               "FakeHealthyNode",
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.DeletionTimestamp).To(BeNil())
		})
		It("should not delete node when health status does not match cloud provider passed in value", func() {
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:               "BadNode",
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.DeletionTimestamp).To(BeNil())
		})
		It("should not delete node when health duration is not reached", func() {
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:   "BadNode",
				Status: corev1.ConditionFalse,
				// We expect the last transition for HealthyNode condition to wait 30 minutes
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			})
			fakeClock.Step(20 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.DeletionTimestamp).To(BeNil())
		})
		It("should set annotation termination grace period when force termination is started", func() {
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:   "BadNode",
				Status: corev1.ConditionFalse,
				// We expect the last transition for HealthyNode condition to wait 30 minutes
				LastTransitionTime: metav1.Time{Time: time.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodeClaimTerminationTimestampAnnotationKey, fakeClock.Now().Format(time.RFC3339)))
		})
		It("should not respect termination grace period if set on the nodepool", func() {
			nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{v1.NodeClaimTerminationTimestampAnnotationKey: fakeClock.Now().Add(120 * time.Minute).Format(time.RFC3339)})
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:   "BadNode",
				Status: corev1.ConditionFalse,
				// We expect the last transition for HealthyNode condition to wait 30 minutes
				LastTransitionTime: metav1.Time{Time: time.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodeClaimTerminationTimestampAnnotationKey, fakeClock.Now().Format(time.RFC3339)))
		})
		It("should not update termination grace period if set before the current time", func() {
			terminationTime := fakeClock.Now().Add(-3 * time.Minute).Format(time.RFC3339)
			nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{v1.NodeClaimTerminationTimestampAnnotationKey: terminationTime})
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:   "BadNode",
				Status: corev1.ConditionFalse,
				// We expect the last transition for HealthyNode condition to wait 30 minutes
				LastTransitionTime: metav1.Time{Time: time.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodeClaimTerminationTimestampAnnotationKey, terminationTime))
		})
		It("should return the requeue interval for the condition closest to its terminationDuration", func() {
			cloudProvider.RepairPolicy = []cloudprovider.RepairPolicy{
				{
					ConditionType:      "BadNode",
					ConditionStatus:    corev1.ConditionFalse,
					TolerationDuration: 60 * time.Minute,
				},
				{
					ConditionType:      "ValidUnhealthyCondition",
					ConditionStatus:    corev1.ConditionFalse,
					TolerationDuration: 30 * time.Minute,
				},
			}
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:   "ValidUnhealthyCondition",
				Status: corev1.ConditionFalse,
				// We expect the last transition for HealthyNode condition to wait 30 minutes
				LastTransitionTime: metav1.Time{Time: time.Now()},
			}, corev1.NodeCondition{
				Type:   "BadNode",
				Status: corev1.ConditionFalse,
				// We expect the last transition for HealthyNode condition to wait 30 minutes
				LastTransitionTime: metav1.Time{Time: time.Now()},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)

			fakeClock.Step(27 * time.Minute)

			result := ExpectObjectReconciled(ctx, env.Client, healthController, node)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*3, time.Second))
		})
		It("should return the requeue interval for the time between now and when the nodeClaim termination time", func() {
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:   "BadNode",
				Status: corev1.ConditionFalse,
				// We expect the last transition for HealthyNode condition to wait 30 minutes
				LastTransitionTime: metav1.Time{Time: time.Now()},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)

			fakeClock.Step(27 * time.Minute)

			result := ExpectObjectReconciled(ctx, env.Client, healthController, node)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*3, time.Second))
		})
	})

	Context("Forceful termination", func() {
		It("should ignore node disruption budgets", func() {
			// Blocking disruption budgets
			nodePool.Spec.Disruption = v1.Disruption{
				Budgets: []v1.Budget{
					{
						Nodes: "0",
					},
				},
			}
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:               "BadNode",
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.DeletionTimestamp).ToNot(BeNil())
		})
		It("should ignore do-not-disrupt on a node", func() {
			node.Annotations = map[string]string{v1.DoNotDisruptAnnotationKey: "true"}
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:               "BadNode",
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, node)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.DeletionTimestamp).ToNot(BeNil())
		})
		It("should ignore unhealthy nodes if more then 20% of the nodes are unhealthy in a nodepool", func() {
			ExpectApplied(ctx, env.Client, nodePool)
			nodeClaims, nodes := test.NodeClaimsAndNodes(10, v1.NodeClaim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{v1.TerminationFinalizer}}})
			for i := range 3 {
				nodes[i].Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
					Type:               "BadNode",
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
				})
			}
			for i := range nodes {
				nodes[i].Labels[v1.NodePoolLabelKey] = nodePool.Name
				nodeClaims[i].Labels[v1.NodePoolLabelKey] = nodePool.Name
			}
			for i := range 10 {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}
			fakeClock.Step(60 * time.Minute)

			// Determine if we should delete unhealthy nodes
			nodeOne := nodes[0]
			nodeClaimOne := nodeClaims[0]
			result := ExpectObjectReconciled(ctx, env.Client, healthController, nodeOne)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*5, time.Second))
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaimOne)
			Expect(nodeClaim.DeletionTimestamp).To(BeNil())

			nodeTwo := nodes[1]
			nodeClaimTwo := nodeClaims[1]
			result = ExpectObjectReconciled(ctx, env.Client, healthController, nodeTwo)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*5, time.Second))
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaimTwo)
			Expect(nodeClaim.DeletionTimestamp).To(BeNil())
		})
		It("should ignore unhealthy nodes if more then 20% of the nodes are unhealthy in a cluster", func() {
			nodeClaims, nodes := test.NodeClaimsAndNodes(10, v1.NodeClaim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{v1.TerminationFinalizer}}})
			for i := range 3 {
				nodes[i].Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
					Type:               "BadNode",
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
				})
			}
			for i := range 10 {
				ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
			}

			fakeClock.Step(60 * time.Minute)

			// Determine if we should delete unhealthy nodes
			nodeOne := nodes[0]
			nodeClaimOne := nodeClaims[0]
			result := ExpectObjectReconciled(ctx, env.Client, healthController, nodeOne)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*5, time.Second))
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaimOne)
			Expect(nodeClaim.DeletionTimestamp).To(BeNil())

			nodeTwo := nodes[1]
			nodeClaimTwo := nodeClaims[1]
			result = ExpectObjectReconciled(ctx, env.Client, healthController, nodeTwo)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*5, time.Second))
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaimTwo)
			Expect(nodeClaim.DeletionTimestamp).To(BeNil())

			nodeThree := nodes[2]
			nodeClaimThree := nodeClaims[2]
			result = ExpectObjectReconciled(ctx, env.Client, healthController, nodeThree)
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Minute*5, time.Second))
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaimThree)
			Expect(nodeClaim.DeletionTimestamp).To(BeNil())
		})
		It("should consider round up when there is a low number of nodes for a nodepool", func() {
			nodeClaims := []*v1.NodeClaim{}
			nodes := []*corev1.Node{}
			for i := range 3 {
				nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{v1.TerminationFinalizer}}})
				if i == 0 {
					node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
						Type:               "BadNode",
						Status:             corev1.ConditionFalse,
						LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
					})
				}
				node.Labels[v1.NodePoolLabelKey] = nodePool.Name
				nodeClaim.Labels[v1.NodePoolLabelKey] = nodePool.Name
				nodeClaims = append(nodeClaims, nodeClaim)
				nodes = append(nodes, node)
				ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			}

			fakeClock.Step(60 * time.Minute)
			// Determine to delete unhealthy nodes
			ExpectObjectReconciled(ctx, env.Client, healthController, nodes[0])
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaims[0])
			Expect(nodeClaim.DeletionTimestamp).ToNot(BeNil())
		})
	})
	Context("Metrics", func() {
		It("should fire a karpenter_nodeclaims_disrupted_total metric when unhealthy", func() {
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
				Type:               "BadNode",
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			})
			fakeClock.Step(60 * time.Minute)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)

			ExpectObjectReconciled(ctx, env.Client, healthController, node)
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.DeletionTimestamp).ToNot(BeNil())

			ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 1, map[string]string{
				metrics.ReasonLabel:   metrics.UnhealthyReason,
				metrics.NodePoolLabel: nodePool.Name,
			})
			ExpectMetricCounterValue(health.NodeClaimsUnhealthyDisruptedTotal, 1, map[string]string{
				health.Condition:      pretty.ToSnakeCase(string(cloudProvider.RepairPolicies()[0].ConditionType)),
				metrics.NodePoolLabel: nodePool.Name,
			})
		})
	})
})
