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

package expectations

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/karpenter/pkg/test"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega" //nolint:revive,stylecheck
	prometheus "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
)

const (
	ReconcilerPropagationTime = 10 * time.Second
	RequestInterval           = 1 * time.Second
)

func ExpectPodExists(ctx context.Context, c client.Client, name string, namespace string) *v1.Pod {
	pod := &v1.Pod{}
	Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, pod)).To(Succeed())
	return pod
}

func ExpectNodeExists(ctx context.Context, c client.Client, name string) *v1.Node {
	node := &v1.Node{}
	Expect(c.Get(ctx, client.ObjectKey{Name: name}, node)).To(Succeed())
	return node
}

func ExpectNotFound(ctx context.Context, c client.Client, objects ...client.Object) {
	for _, object := range objects {
		Eventually(func() bool {
			return errors.IsNotFound(c.Get(ctx, types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}, object))
		}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
			return fmt.Sprintf("expected %s to be deleted, but it still exists", object.GetSelfLink())
		})
	}
}

func ExpectScheduled(ctx context.Context, c client.Client, pod *v1.Pod) *v1.Node {
	p := ExpectPodExists(ctx, c, pod.Name, pod.Namespace)
	Expect(p.Spec.NodeName).ToNot(BeEmpty(), fmt.Sprintf("expected %s/%s to be scheduled", pod.Namespace, pod.Name))
	return ExpectNodeExists(ctx, c, p.Spec.NodeName)
}

func ExpectNotScheduled(ctx context.Context, c client.Client, pod *v1.Pod) *v1.Pod {
	p := ExpectPodExists(ctx, c, pod.Name, pod.Namespace)
	Eventually(p.Spec.NodeName).Should(BeEmpty(), fmt.Sprintf("expected %s/%s to not be scheduled", pod.Namespace, pod.Name))
	return p
}

func ExpectApplied(ctx context.Context, c client.Client, objects ...client.Object) {
	for _, object := range objects {
		current := object.DeepCopyObject().(client.Object)
		statuscopy := object.DeepCopyObject().(client.Object) // Snapshot the status, since create/update may override
		deletecopy := object.DeepCopyObject().(client.Object) // Snapshot the status, since create/update may override
		// Create or Update
		if err := c.Get(ctx, client.ObjectKeyFromObject(current), current); err != nil {
			if errors.IsNotFound(err) {
				Expect(c.Create(ctx, object)).To(Succeed())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		} else {
			object.SetResourceVersion(current.GetResourceVersion())
			Expect(c.Update(ctx, object)).To(Succeed())
		}
		// Update status
		statuscopy.SetResourceVersion(object.GetResourceVersion())
		Expect(c.Status().Update(ctx, statuscopy)).To(Or(Succeed(), MatchError("the server could not find the requested resource"))) // Some objects do not have a status
		// Delete if timestamp set
		if deletecopy.GetDeletionTimestamp() != nil {
			Expect(c.Delete(ctx, deletecopy)).To(Succeed())
		}
	}
}

func ExpectDeleted(ctx context.Context, c client.Client, objects ...client.Object) {
	for _, object := range objects {
		if err := c.Delete(ctx, object, &client.DeleteOptions{GracePeriodSeconds: ptr.Int64(0)}); !errors.IsNotFound(err) {
			Expect(err).To(BeNil())
		}
		ExpectNotFound(ctx, c, object)
	}
}

func ExpectCleanedUp(ctx context.Context, c client.Client) {
	wg := sync.WaitGroup{}
	namespaces := &v1.NamespaceList{}
	Expect(c.List(ctx, namespaces)).To(Succeed())
	nodes := &v1.NodeList{}
	Expect(c.List(ctx, nodes)).To(Succeed())
	for i := range nodes.Items {
		nodes.Items[i].SetFinalizers([]string{})
		Expect(c.Update(ctx, &nodes.Items[i])).To(Succeed())
	}
	for _, object := range []client.Object{
		&v1.Pod{},
		&v1.Node{},
		&appsv1.DaemonSet{},
		&v1beta1.PodDisruptionBudget{},
		&v1.PersistentVolumeClaim{},
		&v1.PersistentVolume{},
		&storagev1.StorageClass{},
		&v1alpha5.Provisioner{},
	} {
		for _, namespace := range namespaces.Items {
			wg.Add(1)
			go func(object client.Object, namespace string) {
				Expect(c.DeleteAllOf(ctx, object, client.InNamespace(namespace),
					&client.DeleteAllOfOptions{DeleteOptions: client.DeleteOptions{GracePeriodSeconds: ptr.Int64(0)}})).ToNot(HaveOccurred())
				wg.Done()
			}(object, namespace.Name)
		}
	}
	wg.Wait()
}

func ExpectProvisioned(ctx context.Context, c client.Client, controller *provisioning.Controller, pods ...*v1.Pod) (result []*v1.Pod) {
	ExpectProvisionedNoBinding(ctx, c, controller, pods...)

	recorder := controller.Recorder().(*test.EventRecorder)
	recorder.ForEachBinding(func(pod *v1.Pod, node *v1.Node) {
		ExpectManualBinding(ctx, c, pod, node)
	})
	// reset bindings so we don't try to bind these same pods again if a new provisioning is performed in the same test
	recorder.ResetBindings()

	// Update objects after reconciling
	for _, pod := range pods {
		result = append(result, ExpectPodExists(ctx, c, pod.GetName(), pod.GetNamespace()))
	}
	return
}

func ExpectProvisionedNoBinding(ctx context.Context, c client.Client, controller *provisioning.Controller, pods ...*v1.Pod) (result []*v1.Pod) {
	// Persist objects
	for _, pod := range pods {
		ExpectApplied(ctx, c, pod)
	}

	// shuffle the pods to try to detect any issues where we rely on pod order within a batch, we shuffle a copy of
	// the slice so we can return the provisioned pods in the same order that the test supplied them for consistency
	unorderedPods := append([]*v1.Pod{}, pods...)
	r := rand.New(rand.NewSource(ginkgo.GinkgoRandomSeed())) //nolint
	r.Shuffle(len(unorderedPods), func(i, j int) { unorderedPods[i], unorderedPods[j] = unorderedPods[j], unorderedPods[i] })
	for _, pod := range unorderedPods {
		_, _ = controller.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(pod)})
	}

	controller.TriggerAndWait() //nolint , method is deprecated and used for unit testing only

	// Update objects after reconciling
	for _, pod := range pods {
		result = append(result, ExpectPodExists(ctx, c, pod.GetName(), pod.GetNamespace()))
	}
	return
}

func ExpectReconcileSucceeded(ctx context.Context, reconciler reconcile.Reconciler, key client.ObjectKey) reconcile.Result {
	result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
	Expect(err).ToNot(HaveOccurred())
	return result
}

func ExpectMetric(prefix string) *prometheus.MetricFamily {
	metrics, err := metrics.Registry.Gather()
	Expect(err).To(BeNil())
	var selected *prometheus.MetricFamily
	for _, mf := range metrics {
		if mf.GetName() == prefix {
			selected = mf
		}
	}
	Expect(selected).ToNot(BeNil(), fmt.Sprintf("expected to find a '%s' metric", prefix))
	return selected
}
func ExpectManualBinding(ctx context.Context, c client.Client, pod *v1.Pod, node *v1.Node) {
	Expect(c.Create(ctx, &v1.Binding{
		TypeMeta:   pod.TypeMeta,
		ObjectMeta: pod.ObjectMeta,
		Target: v1.ObjectReference{
			Name: node.Name,
		},
	})).To(Succeed())
}
