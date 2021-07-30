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
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/controllers/node"
	"github.com/awslabs/karpenter/pkg/test"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	. "knative.dev/pkg/logging/testing"
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
	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Readiness", func() {
		It("should not remove the readiness taint if not ready", func() {
			node := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionUnknown,
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: randomdata.SillyName()},
				Taints: []v1.Taint{
					{Key: v1alpha3.NotReadyTaintKey, Effect: v1.TaintEffectNoSchedule},
					{Key: randomdata.SillyName(), Effect: v1.TaintEffectNoSchedule},
				},
			})
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Spec.Taints).To(Equal(node.Spec.Taints))
		})
		It("should remove the readiness taint if ready", func() {
			node := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionTrue,
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: randomdata.SillyName()},
				Taints: []v1.Taint{
					{Key: v1alpha3.NotReadyTaintKey, Effect: v1.TaintEffectNoSchedule},
					{Key: randomdata.SillyName(), Effect: v1.TaintEffectNoSchedule},
				},
			})
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Spec.Taints).ToNot(Equal([]v1.Taint{node.Spec.Taints[1]}))
		})
		It("should do nothing if ready and the readiness taint does not exist", func() {
			node := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionTrue,
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: randomdata.SillyName()},
				Taints: []v1.Taint{
					{Key: randomdata.SillyName(), Effect: v1.TaintEffectNoSchedule},
				},
			})
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Spec.Taints).To(Equal(node.Spec.Taints))
		})
		It("should do nothing if not owned by a provisioner", func() {
			node := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionTrue,
				Taints: []v1.Taint{
					{Key: v1alpha3.NotReadyTaintKey, Effect: v1.TaintEffectNoSchedule},
					{Key: randomdata.SillyName(), Effect: v1.TaintEffectNoSchedule},
				},
			})
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Spec.Taints).To(Equal(node.Spec.Taints))
		})
	})
	Context("Finalizer", func() {
		It("should add the termination finalizer if missing", func() {
			node := test.Node(test.NodeOptions{
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: randomdata.SillyName()},
				Finalizers: []string{"fake.com/finalizer"},
			})
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Finalizers).To(ConsistOf(node.Finalizers[0], v1alpha3.TerminationFinalizer))
		})
		It("should do nothing if terminating", func() {
			node := test.Node(test.NodeOptions{
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: randomdata.SillyName()},
				Finalizers: []string{"fake.com/finalizer"},
			})
			ExpectCreatedWithStatus(env.Client, node)
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Finalizers).To(Equal(node.Finalizers))
		})
		It("should do nothing if the termination finalizer already exists", func() {
			node := test.Node(test.NodeOptions{
				Labels:      map[string]string{v1alpha3.ProvisionerNameLabelKey: randomdata.SillyName()},
				Finalizers: []string{v1alpha3.TerminationFinalizer, "fake.com/finalizer"},
			})
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Finalizers).To(Equal(node.Finalizers))
		})
		It("should do nothing if the not owned by a provisioner", func() {
			node := test.Node(test.NodeOptions{
				Finalizers: []string{"fake.com/finalizer"},
			})
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Finalizers).To(Equal(node.Finalizers))
		})
	})
})
