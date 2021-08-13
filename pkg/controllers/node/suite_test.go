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

package node_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/controllers/node"
	"github.com/awslabs/karpenter/pkg/test"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ctx context.Context
var controller *node.Controller
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Node")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		controller = node.NewController(e.Client)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Controller", func() {
	var provisioner *v1alpha3.Provisioner
	BeforeEach(func() {
		provisioner = &v1alpha3.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.DefaultProvisioner.Name},
			Spec:       v1alpha3.ProvisionerSpec{},
		}
	})

	AfterEach(func() {
		node.Now = time.Now
		ExpectCleanedUp(env.Client)
	})

	Context("Expiration", func() {
		It("should ignore nodes without TTLSecondsUntilExpired", func() {
			n := test.Node(test.NodeOptions{
				Finalizers: []string{v1alpha3.TerminationFinalizer},
				Labels: map[string]string{
					v1alpha3.ProvisionerNameLabelKey: provisioner.Name,
				},
			})
			ExpectCreated(env.Client, provisioner, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.DeletionTimestamp.IsZero()).To(BeTrue())
		})
		It("should ignore nodes without a provisioner", func() {
			n := test.Node(test.NodeOptions{Finalizers: []string{v1alpha3.TerminationFinalizer}})
			ExpectCreated(env.Client, provisioner, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.DeletionTimestamp.IsZero()).To(BeTrue())
		})
		It("should terminate nodes after expiry", func() {
			provisioner.Spec.TTLSecondsUntilExpired = ptr.Int64(30)
			n := test.Node(test.NodeOptions{
				Finalizers: []string{v1alpha3.TerminationFinalizer},
				Labels: map[string]string{
					v1alpha3.ProvisionerNameLabelKey: provisioner.Name,
				},
			})
			ExpectCreated(env.Client, provisioner, n)

			// Should still exist
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))
			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.DeletionTimestamp.IsZero()).To(BeTrue())

			// Simulate time passing
			node.Now = func() time.Time {
				return time.Now().Add(time.Duration(*provisioner.Spec.TTLSecondsUntilExpired) * time.Second)
			}
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))
			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.DeletionTimestamp.IsZero()).To(BeFalse())
		})
	})

	Context("Readiness", func() {
		It("should not remove the readiness taint if not ready", func() {
			n := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionUnknown,
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				Taints: []v1.Taint{
					{Key: v1alpha3.NotReadyTaintKey, Effect: v1.TaintEffectNoSchedule},
					{Key: randomdata.SillyName(), Effect: v1.TaintEffectNoSchedule},
				},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.Spec.Taints).To(Equal(n.Spec.Taints))
		})
		It("should remove the readiness taint if ready", func() {
			n := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionTrue,
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				Taints: []v1.Taint{
					{Key: v1alpha3.NotReadyTaintKey, Effect: v1.TaintEffectNoSchedule},
					{Key: randomdata.SillyName(), Effect: v1.TaintEffectNoSchedule},
				},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.Spec.Taints).ToNot(Equal([]v1.Taint{n.Spec.Taints[1]}))
		})
		It("should do nothing if ready and the readiness taint does not exist", func() {
			n := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionTrue,
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				Taints:      []v1.Taint{{Key: randomdata.SillyName(), Effect: v1.TaintEffectNoSchedule}},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.Spec.Taints).To(Equal(n.Spec.Taints))
		})
		It("should do nothing if not owned by a provisioner", func() {
			n := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionTrue,
				Taints: []v1.Taint{
					{Key: v1alpha3.NotReadyTaintKey, Effect: v1.TaintEffectNoSchedule},
					{Key: randomdata.SillyName(), Effect: v1.TaintEffectNoSchedule},
				},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.Spec.Taints).To(Equal(n.Spec.Taints))
		})
	})
	Context("Liveness", func() {
		It("should terminate nodes if NodeStatusNeverUpdated after 5 minutes", func() {
			n := test.Node(test.NodeOptions{
				Finalizers:  []string{v1alpha3.TerminationFinalizer},
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				ReadyStatus: v1.ConditionUnknown,
				ReadyReason: "NodeStatusNeverUpdated",
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)

			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			// Expect node not be deleted
			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.DeletionTimestamp.IsZero()).To(BeTrue())

			// Simulate time passing and a n failing to join
			node.Now = func() time.Time { return time.Now().Add(node.LivenessTimeout) }
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.DeletionTimestamp.IsZero()).To(BeFalse())
		})
		It("should terminate nodes if we never hear anything after 5 minutes", func() {
			n := test.Node(test.NodeOptions{
				Finalizers:  []string{v1alpha3.TerminationFinalizer},
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				ReadyStatus: v1.ConditionUnknown,
				ReadyReason: "",
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)

			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			// Expect node not be deleted
			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.DeletionTimestamp.IsZero()).To(BeTrue())

			// Simulate time passing and a n failing to join
			node.Now = func() time.Time { return time.Now().Add(node.LivenessTimeout) }
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.DeletionTimestamp.IsZero()).To(BeFalse())
		})
	})
	Describe("Emptiness", func() {
		It("should not TTL nodes that have ready status unknown", func() {
			provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(30)
			node := test.Node(test.NodeOptions{
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				ReadyStatus: v1.ConditionUnknown,
			})

			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			node = ExpectNodeExists(env.Client, node.Name)
			Expect(node.Annotations).ToNot(HaveKey(v1alpha3.EmptinessTimestampAnnotationKey))
		})
		It("should not TTL nodes that have ready status false", func() {
			provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(30)
			node := test.Node(test.NodeOptions{
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				ReadyStatus: v1.ConditionFalse,
			})

			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			node = ExpectNodeExists(env.Client, node.Name)
			Expect(node.Annotations).ToNot(HaveKey(v1alpha3.EmptinessTimestampAnnotationKey))
		})
		It("should label nodes as underutilized and add TTL", func() {
			provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(30)
			node := test.Node(test.NodeOptions{
				Labels: map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			node = ExpectNodeExists(env.Client, node.Name)
			Expect(node.Annotations).To(HaveKey(v1alpha3.EmptinessTimestampAnnotationKey))
		})
		It("should remove labels from non-empty nodes", func() {
			provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(30)
			node := test.Node(test.NodeOptions{
				Labels: map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				Annotations: map[string]string{
					v1alpha3.EmptinessTimestampAnnotationKey: time.Now().Add(100 * time.Second).Format(time.RFC3339),
				},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, node)
			ExpectCreatedWithStatus(env.Client, test.Pod(test.PodOptions{
				Name:       strings.ToLower(randomdata.SillyName()),
				Namespace:  provisioner.Namespace,
				NodeName:   node.Name,
				Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}},
			}))
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			node = ExpectNodeExists(env.Client, node.Name)
			Expect(node.Annotations).ToNot(HaveKey(v1alpha3.EmptinessTimestampAnnotationKey))
		})
		It("should terminate empty nodes past their TTL", func() {
			provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(30)
			node := test.Node(test.NodeOptions{
				Finalizers: []string{v1alpha3.TerminationFinalizer},
				Labels:     map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				Annotations: map[string]string{
					v1alpha3.EmptinessTimestampAnnotationKey: time.Now().Add(-100 * time.Second).Format(time.RFC3339),
				},
			})
			ExpectCreated(env.Client, provisioner, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			node = ExpectNodeExists(env.Client, node.Name)
			Expect(node.DeletionTimestamp.IsZero()).To(BeFalse())
		})
	})
	Context("Finalizer", func() {
		It("should add the termination finalizer if missing", func() {
			n := test.Node(test.NodeOptions{
				Labels:     map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				Finalizers: []string{"fake.com/finalizer"},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.Finalizers).To(ConsistOf(n.Finalizers[0], v1alpha3.TerminationFinalizer))
		})
		It("should do nothing if terminating", func() {
			n := test.Node(test.NodeOptions{
				Labels:     map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				Finalizers: []string{"fake.com/finalizer"},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)
			Expect(env.Client.Delete(ctx, n)).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.Finalizers).To(Equal(n.Finalizers))
		})
		It("should do nothing if the termination finalizer already exists", func() {
			n := test.Node(test.NodeOptions{
				Labels:     map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name},
				Finalizers: []string{v1alpha3.TerminationFinalizer, "fake.com/finalizer"},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.Finalizers).To(Equal(n.Finalizers))
		})
		It("should do nothing if the not owned by a provisioner", func() {
			n := test.Node(test.NodeOptions{
				Finalizers: []string{"fake.com/finalizer"},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, n)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(n))

			n = ExpectNodeExists(env.Client, n.Name)
			Expect(n.Finalizers).To(Equal(n.Finalizers))
		})
	})
})
