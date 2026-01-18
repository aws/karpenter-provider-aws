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

package consistency_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/consistency"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var nodeClaimConsistencyController *consistency.Controller
var env *test.Environment
var fakeClock *clock.FakeClock
var cp *fake.CloudProvider
var recorder *test.EventRecorder

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Consistency")
}

var _ = BeforeSuite(func() {
	fakeClock = clock.NewFakeClock(time.Now())
	env = test.NewEnvironment(
		test.WithCRDs(apis.CRDs...),
		test.WithCRDs(v1alpha1.CRDs...),
		test.WithFieldIndexers(test.NodeClaimProviderIDFieldIndexer(ctx), test.NodeProviderIDFieldIndexer(ctx)),
	)
	ctx = options.ToContext(ctx, test.Options())
	cp = &fake.CloudProvider{}
	recorder = test.NewEventRecorder()
	nodeClaimConsistencyController = consistency.NewController(fakeClock, env.Client, cp, recorder)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {

	recorder.Reset()
})

var _ = AfterEach(func() {
	fakeClock.SetTime(time.Now())
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClaimController", func() {
	var nodePool *v1.NodePool

	BeforeEach(func() {
		nodePool = test.NodePool()
	})
	Context("Node Shape", func() {
		DescribeTable(
			"Node Shape",
			func(isNodeClaimManaged bool) {
				nodeClaimOpts := []v1.NodeClaim{{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            nodePool.Name,
							corev1.LabelInstanceTypeStable: "arm-instance-type",
							v1.NodeInitializedLabelKey:     "true",
						},
					},
					Spec: v1.NodeClaimSpec{
						Resources: v1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("8"),
								corev1.ResourceMemory: resource.MustParse("64Gi"),
								corev1.ResourcePods:   resource.MustParse("5"),
							},
						},
					},
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("16"),
							corev1.ResourceMemory: resource.MustParse("128Gi"),
							corev1.ResourcePods:   resource.MustParse("10"),
						},
					},
				}}
				if !isNodeClaimManaged {
					nodeClaimOpts = append(nodeClaimOpts, v1.NodeClaim{
						Spec: v1.NodeClaimSpec{
							NodeClassRef: &v1.NodeClassReference{
								Group: "karpenter.test.sh",
								Kind:  "UnmanagedNodeClass",
								Name:  "default",
							},
						},
					})
				}
				nodeClaim, node := test.NodeClaimAndNode(nodeClaimOpts...)
				nodeClaim.StatusConditions().SetUnknown(v1.ConditionTypeConsistentStateFound)
				ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
				ExpectMakeNodeClaimsInitialized(ctx, env.Client, nodeClaim)
				ExpectObjectReconciled(ctx, env.Client, nodeClaimConsistencyController, nodeClaim)
				nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
				if isNodeClaimManaged {
					Expect(nodeClaim.StatusConditions().IsTrue(v1.ConditionTypeConsistentStateFound)).To(BeTrue())
				} else {
					Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsistentStateFound).IsUnknown()).To(BeTrue())
				}
			},
			Entry("should set consistent state found condition to true if there are no consistency issues", true),
			Entry("should ignore NodeClaims which aren't managed by this instance of Karpenter", false),
		)
		It("should detect issues that launch with much fewer resources than expected", func() {
			nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: "arm-instance-type",
						v1.NodeInitializedLabelKey:     "true",
					},
				},
				Spec: v1.NodeClaimSpec{
					Resources: v1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("64Gi"),
							corev1.ResourcePods:   resource.MustParse("5"),
						},
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("16"),
						corev1.ResourceMemory: resource.MustParse("128Gi"),
						corev1.ResourcePods:   resource.MustParse("10"),
					},
				},
			})
			node.Status.Capacity = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("16"),
				corev1.ResourceMemory: resource.MustParse("64Gi"),
				corev1.ResourcePods:   resource.MustParse("10"),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
			ExpectMakeNodeClaimsInitialized(ctx, env.Client, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimConsistencyController, nodeClaim)
			Expect(recorder.DetectedEvent("expected 128Gi of resource memory, but found 64Gi (50.0% of expected)")).To(BeTrue())
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeConsistentStateFound).IsFalse()).To(BeTrue())
		})
	})
})
