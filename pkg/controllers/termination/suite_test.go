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

package termination

import (
	"context"
	"testing"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/test"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Termination")
}

var controller *Controller
var env = test.NewEnvironment(func(e *test.Environment) {
	cloudProvider := &fake.CloudProvider{}
	registry.RegisterOrDie(cloudProvider)
	controller = NewController(
		e.Client,
		corev1.NewForConfigOrDie(e.Config),
		cloudProvider,
	)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Termination", func() {
	var ctx context.Context
	var node *v1.Node

	BeforeEach(func() {
		ctx = context.Background()
		node = test.NodeWith(test.NodeOptions{Finalizers: []string{v1alpha2.KarpenterFinalizer}})
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Reconciliation", func() {
		It("should terminate deleted nodes", func() {
			ExpectCreated(env.Client, node)
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(env.Client, node.Name)
			ExpectReconcileSucceeded(controller, node)
			ExpectNotFound(env.Client, node)
		})
		It("should not evict pods that tolerate unschedulable taint", func() {
			podEvict := test.Pod(test.PodOptions{NodeName: node.Name})
			podSkip := test.Pod(test.PodOptions{
				NodeName:    node.Name,
				Tolerations: []v1.Toleration{{Key: v1.TaintNodeUnschedulable, Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoSchedule}},
			})
			ExpectCreated(env.Client, node, podEvict, podSkip)

			// Trigger Termination Controller
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(env.Client, node.Name)
			ExpectReconcileSucceeded(controller, node)

			// Expect podToEvict to be evicting, and delete it
			podEvict = ExpectPodExists(env.Client, podEvict.Name, podEvict.Namespace)
			Expect(podEvict.GetObjectMeta().GetDeletionTimestamp().IsZero()).To(BeFalse())
			ExpectDeleted(env.Client, podEvict)
			// Expect podToSkip to not be evicting
			podSkip = ExpectPodExists(env.Client, podSkip.Name, podSkip.Namespace)
			Expect(podSkip.GetObjectMeta().GetDeletionTimestamp().IsZero()).To(BeTrue())

			// Reconcile to delete node
			node = ExpectNodeExists(env.Client, node.Name)
			ExpectReconcileSucceeded(controller, node)
			ExpectNotFound(env.Client, node)
		})
		It("should not terminate nodes that have a do-not-evict pod", func() {
			podEvict := test.Pod(test.PodOptions{NodeName: node.Name})
			podNoEvict := test.Pod(test.PodOptions{
				NodeName:    node.Name,
				Annotations: map[string]string{v1alpha2.KarpenterDoNotEvictPodAnnotation: "true"},
			})

			ExpectCreated(env.Client, node, podEvict, podNoEvict)

			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(env.Client, node.Name)
			ExpectReconcileSucceeded(controller, node)

			// Expect node to exist, but be cordoned
			node = ExpectNodeExists(env.Client, node.Name)
			Expect(node.Spec.Unschedulable).To(Equal(true))

			// Expect pods to not be evicting
			podEvict = ExpectPodExists(env.Client, podEvict.Name, podEvict.Namespace)
			Expect(podEvict.GetObjectMeta().GetDeletionTimestamp().IsZero()).To(BeTrue())
			podNoEvict = ExpectPodExists(env.Client, podNoEvict.Name, podNoEvict.Namespace)
			Expect(podNoEvict.GetObjectMeta().GetDeletionTimestamp().IsZero()).To(BeTrue())

			// Delete do-not-evict pod
			ExpectDeleted(env.Client, podNoEvict)

			// Reconcile node to evict pod
			node = ExpectNodeExists(env.Client, node.Name)
			ExpectReconcileSucceeded(controller, node)
			pod := ExpectPodExists(env.Client, podEvict.Name, podEvict.Namespace)
			Expect(pod.GetObjectMeta().GetDeletionTimestamp().IsZero()).To(BeFalse())

			// Delete pod to simulate successful eviction
			ExpectDeleted(env.Client, pod)

			// Terminate Node
			node = ExpectNodeExists(env.Client, node.Name)
			ExpectReconcileSucceeded(controller, node)
			ExpectNotFound(env.Client, node)
		})
	})
})
