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

package termination_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/termination"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injectabletime"
	"github.com/aws/karpenter/pkg/utils/injection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	. "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"
)

var ctx context.Context
var controller *termination.Controller
var evictionQueue *termination.EvictionQueue
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Termination")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		ctx = cloudprovider.Inject(ctx, &fake.CloudProvider{})
		registry.RegisterOrDie(cloudprovider.Get(ctx))
		coreV1Client := corev1.NewForConfigOrDie(e.Config)
		evictionQueue = termination.NewEvictionQueue(ctx, coreV1Client)
		controller = &termination.Controller{
			KubeClient: e.Client,
			Terminator: &termination.Terminator{
				KubeClient:    injection.GetKubeClient(ctx),
				CoreV1Client:  injection.GetKubernetesInterface(ctx).CoreV1(),
				CloudProvider: cloudprovider.Get(ctx),
				EvictionQueue: evictionQueue,
			},
		}
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Termination", func() {
	var node *v1.Node

	BeforeEach(func() {
		node = test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{v1alpha5.TerminationFinalizer}}})
	})

	AfterEach(func() {
		ExpectCleanedUp(ctx, env.Client)
		injectabletime.Now = time.Now
	})

	Context("Reconciliation", func() {
		It("should delete nodes", func() {
			ExpectApplied(ctx, env.Client, node)
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)
		})
		It("should not evict pods that tolerate unschedulable taint", func() {
			podEvict := test.Pod(test.PodOptions{NodeName: node.Name})
			podSkip := test.Pod(test.PodOptions{
				NodeName:    node.Name,
				Tolerations: []v1.Toleration{{Key: v1.TaintNodeUnschedulable, Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoSchedule}},
			})
			ExpectApplied(ctx, env.Client, node, podEvict, podSkip)

			// Trigger Termination Controller
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			// Expect node to exist and be draining
			ExpectNodeDraining(env.Client, node.Name)

			// Expect podEvict to be evicting, and delete it
			ExpectEvicted(env.Client, podEvict)
			ExpectDeleted(ctx, env.Client, podEvict)

			// Reconcile to delete node
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)
		})
		It("should not delete nodes that have a do-not-evict pod", func() {
			podEvict := test.Pod(test.PodOptions{NodeName: node.Name})
			podNoEvict := test.Pod(test.PodOptions{
				NodeName:   node.Name,
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1alpha5.DoNotEvictPodAnnotationKey: "true"}},
			})

			ExpectApplied(ctx, env.Client, node, podEvict, podNoEvict)

			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			// Expect no pod to be enqueued for eviction
			ExpectNotEnqueuedForEviction(evictionQueue, podEvict, podNoEvict)

			// Expect node to exist and be draining
			ExpectNodeDraining(env.Client, node.Name)

			// Delete do-not-evict pod
			ExpectDeleted(ctx, env.Client, podNoEvict)

			// Reconcile node to evict pod
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			// Expect podEvict to be enqueued for eviction then be successful
			ExpectEvicted(env.Client, podEvict)

			// Delete pod to simulate successful eviction
			ExpectDeleted(ctx, env.Client, podEvict)

			// Reconcile to delete node
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)
		})
		It("should delete nodes that have do-not-evict on pods for which it does not apply", func() {
			ExpectApplied(ctx, env.Client, node)
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			pods := []*v1.Pod{
				test.Pod(test.PodOptions{
					NodeName:   node.Name,
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1alpha5.DoNotEvictPodAnnotationKey: "true"}},
				}),
				test.Pod(test.PodOptions{
					NodeName:    node.Name,
					Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
				}),
				test.Pod(test.PodOptions{
					NodeName:   node.Name,
					ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{Kind: "Node", APIVersion: "v1", Name: node.Name, UID: node.UID}}},
				}),
			}
			for _, pod := range pods {
				ExpectApplied(ctx, env.Client, pod)
			}
			// Trigger eviction
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			for _, pod := range pods {
				Expect(env.Client.Delete(ctx, pod, &client.DeleteOptions{GracePeriodSeconds: ptr.Int64(30)})).To(Succeed())
			}
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			// Simulate stuck terminating
			injectabletime.Now = func() time.Time { return time.Now().Add(1 * time.Minute) }
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)
		})
		It("should fail to evict pods that violate a PDB", func() {
			minAvailable := intstr.FromInt(1)
			labelSelector := map[string]string{randomdata.SillyName(): randomdata.SillyName()}
			pdb := test.PodDisruptionBudget(test.PDBOptions{
				Labels: labelSelector,
				// Don't let any pod evict
				MinAvailable: &minAvailable,
			})
			podNoEvict := test.Pod(test.PodOptions{
				NodeName:   node.Name,
				ObjectMeta: metav1.ObjectMeta{Labels: labelSelector},
				Phase:      v1.PodRunning,
			})

			ExpectApplied(ctx, env.Client, node, podNoEvict, pdb)

			// Trigger Termination Controller
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			// Expect node to exist and be draining
			ExpectNodeDraining(env.Client, node.Name)

			// Expect podNoEvict to fail eviction due to PDB, and be retried
			Eventually(func() int {
				return evictionQueue.NumRequeues(client.ObjectKeyFromObject(podNoEvict))
			}).Should(BeNumerically(">=", 1))

			// Delete pod to simulate successful eviction
			ExpectDeleted(ctx, env.Client, podNoEvict)
			ExpectNotFound(ctx, env.Client, podNoEvict)

			// Reconcile to delete node
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)
		})
		It("should evict non-critical pods first", func() {
			podEvict := test.Pod(test.PodOptions{NodeName: node.Name})
			podNodeCritical := test.Pod(test.PodOptions{NodeName: node.Name, PriorityClassName: "system-node-critical"})
			podClusterCritical := test.Pod(test.PodOptions{NodeName: node.Name, PriorityClassName: "system-cluster-critical"})

			ExpectApplied(ctx, env.Client, node, podEvict, podNodeCritical, podClusterCritical)

			// Trigger Termination Controller
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			// Expect node to exist and be draining
			ExpectNodeDraining(env.Client, node.Name)

			// Expect podEvict to be evicting, and delete it
			ExpectEvicted(env.Client, podEvict)
			ExpectDeleted(ctx, env.Client, podEvict)

			// Expect the critical pods to be evicted and deleted
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectEvicted(env.Client, podNodeCritical)
			ExpectDeleted(ctx, env.Client, podNodeCritical)
			ExpectEvicted(env.Client, podClusterCritical)
			ExpectDeleted(ctx, env.Client, podClusterCritical)

			// Reconcile to delete node
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)
		})

		It("should not evict static pods", func() {
			podEvict := test.Pod(test.PodOptions{NodeName: node.Name})
			ExpectApplied(ctx, env.Client, node, podEvict)

			podNoEvict := test.Pod(test.PodOptions{
				NodeName: node.Name,
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "v1",
						Kind:       "Node",
						Name:       node.Name,
						UID:        node.UID,
					}},
				},
			})
			ExpectApplied(ctx, env.Client, podNoEvict)

			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			// Expect mirror pod to not be queued for eviction
			ExpectNotEnqueuedForEviction(evictionQueue, podNoEvict)

			// Expect podEvict to be enqueued for eviction then be successful
			ExpectEvicted(env.Client, podEvict)

			// Expect node to exist and be draining
			ExpectNodeDraining(env.Client, node.Name)

			// Reconcile node to evict pod
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			// Delete pod to simulate successful eviction
			ExpectDeleted(ctx, env.Client, podEvict)

			// Reconcile to delete node
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)

		})
		It("should not delete nodes until all pods are deleted", func() {
			pods := []*v1.Pod{test.Pod(test.PodOptions{NodeName: node.Name}), test.Pod(test.PodOptions{NodeName: node.Name})}
			ExpectApplied(ctx, env.Client, node, pods[0], pods[1])

			// Trigger Termination Controller
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

			// Expect the pods to be evicted
			ExpectEvicted(env.Client, pods[0], pods[1])

			// Expect node to exist and be draining, but not deleted
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNodeDraining(env.Client, node.Name)

			ExpectDeleted(ctx, env.Client, pods[1])

			// Expect node to exist and be draining, but not deleted
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNodeDraining(env.Client, node.Name)

			ExpectDeleted(ctx, env.Client, pods[0])

			// Reconcile to delete node
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)
		})
		It("should wait for pods to terminate", func() {
			pod := test.Pod(test.PodOptions{NodeName: node.Name})
			ExpectApplied(ctx, env.Client, node, pod)

			// Before grace period, node should not delete
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNodeExists(ctx, env.Client, node.Name)
			ExpectEvicted(env.Client, pod)

			// After grace period, node should delete
			injectabletime.Now = func() time.Time { return time.Now().Add(30 * time.Second) }
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			ExpectNotFound(ctx, env.Client, node)
		})
	})
})

func ExpectNotEnqueuedForEviction(e *termination.EvictionQueue, pods ...*v1.Pod) {
	for _, pod := range pods {
		Expect(e.Contains(client.ObjectKeyFromObject(pod))).To(BeFalse())
	}
}

func ExpectEvicted(c client.Client, pods ...*v1.Pod) {
	for _, pod := range pods {
		Eventually(func() bool {
			return ExpectPodExists(ctx, c, pod.Name, pod.Namespace).GetDeletionTimestamp().IsZero()
		}, ReconcilerPropagationTime, RequestInterval).Should(BeFalse(), func() string {
			return fmt.Sprintf("expected %s/%s to be evicting, but it isn't", pod.Namespace, pod.Name)
		})
	}
}

func ExpectNodeDraining(c client.Client, nodeName string) *v1.Node {
	node := ExpectNodeExists(ctx, c, nodeName)
	Expect(node.Spec.Unschedulable).To(BeTrue())
	Expect(functional.ContainsString(node.Finalizers, v1alpha5.TerminationFinalizer)).To(BeTrue())
	Expect(node.DeletionTimestamp.IsZero()).To(BeFalse())
	return node
}
