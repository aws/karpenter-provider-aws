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

package lifecycle_test

import (
	"context"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/karpenter/pkg/operator/logging"
	"sigs.k8s.io/karpenter/pkg/state/nodepoolhealth"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	nodeclaimlifecycle "sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/lifecycle"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

func init() {
	log.SetLogger(logging.NopLogger)
}

var (
	ctx                 context.Context
	nodeClaimController *nodeclaimlifecycle.Controller
	env                 *test.Environment
	fakeClock           *clock.FakeClock
	cloudProvider       *fake.CloudProvider
	recorder            *test.EventRecorder
	npState             *nodepoolhealth.State
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lifecycle")
}

func removeNodeClaimImmutabilityValidation(crds ...*apiextensionsv1.CustomResourceDefinition) []*apiextensionsv1.CustomResourceDefinition {
	for _, crd := range crds {
		if crd.Name != "nodeclaims.karpenter.sh" {
			continue
		}
		overrideProperties := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
		overrideProperties.XValidations = []apiextensionsv1.ValidationRule{}
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"] = overrideProperties
	}
	return crds
}

var _ = BeforeSuite(func() {
	fakeClock = clock.NewFakeClock(time.Now())
	recorder = test.NewEventRecorder()
	env = test.NewEnvironment(test.WithCRDs(removeNodeClaimImmutabilityValidation(apis.CRDs...)...), test.WithCRDs(v1alpha1.CRDs...), test.WithFieldIndexers(test.NodeProviderIDFieldIndexer(ctx)))
	ctx = options.ToContext(ctx, test.Options())

	cloudProvider = fake.NewCloudProvider()
	npState = nodepoolhealth.NewState()
	nodeClaimController = nodeclaimlifecycle.NewController(fakeClock, env.Client, cloudProvider, recorder, npState)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	fakeClock.SetTime(time.Now())
	ExpectCleanedUp(ctx, env.Client)
	cloudProvider.Reset()
})

var _ = Describe("Finalizer", func() {
	var nodePool *v1.NodePool

	BeforeEach(func() {
		recorder.Reset() // Reset the events that we captured during the run
		nodePool = test.NodePool()
	})
	Context("TerminationFinalizer", func() {
		It("should add the finalizer if it doesn't exist", func() {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			_, ok := lo.Find(nodeClaim.Finalizers, func(f string) bool {
				return f == v1.TerminationFinalizer
			})
			Expect(ok).To(BeTrue())
		})
		It("shouldn't add the finalizer to NodeClaims not managed by this instance of Karpenter", func() {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
				Spec: v1.NodeClaimSpec{
					NodeClassRef: &v1.NodeClassReference{
						Group: "karpenter.test.sh",
						Kind:  "UnmanagedNodeClass",
						Name:  "default",
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			_, ok := lo.Find(nodeClaim.Finalizers, func(f string) bool {
				return f == v1.TerminationFinalizer
			})
			Expect(ok).To(BeFalse())
		})
		It("should update status conditions to the latest generation when finalizing", func() {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
			ExpectMakeNodeClaimsInitialized(ctx, env.Client, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			// add a finalizer so we can make assertions about all status conditions
			ExpectDeletionTimestampSet(ctx, env.Client, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeLaunched).IsTrue()).To(BeTrue())
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeLaunched).ObservedGeneration).To(Equal(nodeClaim.Generation))
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).ObservedGeneration).To(Equal(nodeClaim.Generation))
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).IsTrue()).To(BeTrue())
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).ObservedGeneration).To(Equal(nodeClaim.Generation))
			Expect(nodeClaim.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
			Expect(nodeClaim.StatusConditions().Get(status.ConditionReady).ObservedGeneration).To(Equal(nodeClaim.Generation))

			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInstanceTerminating).IsTrue()).To(BeTrue())
			Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInstanceTerminating).ObservedGeneration).To(Equal(nodeClaim.Generation))
			ExpectFinalizersRemoved(ctx, env.Client, nodeClaim)
			ExpectDeleted(ctx, env.Client, nodeClaim)
		})
	})
	It("should update observedGeneration if generation increases after all conditions are marked True", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
						corev1.ResourcePods:   resource.MustParse("5"),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		ExpectMakeNodesReady(ctx, env.Client, node) // Remove the not-ready taint

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).IsUnknown()).To(BeTrue())

		node = ExpectExists(ctx, env.Client, node)
		node.Status.Capacity = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
			corev1.ResourcePods:   resource.MustParse("110"),
		}
		node.Status.Allocatable = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("8"),
			corev1.ResourceMemory: resource.MustParse("80Mi"),
			corev1.ResourcePods:   resource.MustParse("110"),
		}
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).IsTrue()).To(BeTrue())

		// Change a field to increase the generation
		nodeClaim.Spec.Taints = append(nodeClaim.Spec.Taints, corev1.Taint{Key: "test", Value: "value", Effect: corev1.TaintEffectNoSchedule})
		ExpectApplied(ctx, env.Client, nodeClaim)

		// Expect that when the object re-reconciles, all of the observedGenerations across all status condition match
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeLaunched).IsTrue()).To(BeTrue())
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeLaunched).ObservedGeneration).To(Equal(nodeClaim.Generation))
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).ObservedGeneration).To(Equal(nodeClaim.Generation))
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).IsTrue()).To(BeTrue())
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).ObservedGeneration).To(Equal(nodeClaim.Generation))

		Expect(nodeClaim.StatusConditions().Get(status.ConditionReady).IsTrue()).To(BeTrue())
		Expect(nodeClaim.StatusConditions().Get(status.ConditionReady).ObservedGeneration).To(Equal(nodeClaim.Generation))
	})
})
