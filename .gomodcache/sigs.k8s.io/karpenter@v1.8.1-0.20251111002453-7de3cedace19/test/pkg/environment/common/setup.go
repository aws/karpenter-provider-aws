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

package common

import (
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/apis/v1alpha1"
	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/utils/pod"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
	"sigs.k8s.io/karpenter/test/pkg/debug"
)

const TestingFinalizer = "testing/finalizer"

var (
	ObjectListsToPrint = []client.ObjectList{
		&corev1.PodList{},
		&v1.NodePoolList{},
		&corev1.NodeList{},
		&v1.NodeClaimList{},
	}
	CleanableObjects = []client.Object{
		&corev1.Pod{},
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&policyv1.PodDisruptionBudget{},
		&corev1.PersistentVolumeClaim{},
		&corev1.PersistentVolume{},
		&storagev1.StorageClass{},
		&v1.NodePool{},
		&corev1.LimitRange{},
		&schedulingv1.PriorityClass{},
		&corev1.Node{},
		&v1.NodeClaim{},
		&v1alpha1.NodeOverlay{},
		&admissionregistrationv1.ValidatingAdmissionPolicy{},
		&admissionregistrationv1.ValidatingAdmissionPolicyBinding{},
	}
)

// nolint:gocyclo
func (env *Environment) BeforeEach() {
	debug.BeforeEach(env.Context, env.Config, env.Client)
	// Expect this cluster to be clean for test runs to execute successfully
	env.ExpectCleanCluster()
	env.TimeIntervalCollector.Start(debug.StageE2E)

	env.Monitor.Reset()
	env.StartingNodeCount = env.Monitor.NodeCountAtReset()
}

func (env *Environment) ExpectCleanCluster() {
	var nodes corev1.NodeList
	Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
	for _, node := range nodes.Items {
		if len(node.Spec.Taints) == 0 && !node.Spec.Unschedulable {
			Fail(fmt.Sprintf("expected system pool node %s to be tainted", node.Name))
		}
	}
	var pods corev1.PodList
	Expect(env.Client.List(env.Context, &pods)).To(Succeed())
	for i := range pods.Items {
		Expect(pod.IsProvisionable(&pods.Items[i])).To(BeFalse(),
			fmt.Sprintf("expected to have no provisionable pods, found %s/%s", pods.Items[i].Namespace, pods.Items[i].Name))
		Expect(pods.Items[i].Namespace).ToNot(Equal("default"),
			fmt.Sprintf("expected no pods in the `default` namespace, found %s/%s", pods.Items[i].Namespace, pods.Items[i].Name))
	}
	for _, obj := range []client.Object{&v1.NodePool{}, env.DefaultNodeClass.DeepCopy()} {
		metaList := &metav1.PartialObjectMetadataList{}
		gvk := lo.Must(apiutil.GVKForObject(obj, env.Client.Scheme()))
		metaList.SetGroupVersionKind(gvk)
		Expect(env.Client.List(env.Context, metaList, client.Limit(1))).To(Succeed())
		Expect(metaList.Items).To(HaveLen(0), fmt.Sprintf("expected no %s to exist", gvk.Kind))
	}
}

func (env *Environment) Cleanup() {
	if !CurrentSpecReport().Failure.IsZero() {
		env.PrintCluster()
	}
	env.TimeIntervalCollector.Start(debug.StageAfterEach)
	env.CleanupObjects(CleanableObjects...)
	env.eventuallyExpectScaleDown()
	env.ExpectNoCrashes()
	env.TimeIntervalCollector.End(debug.StageAfterEach)
}

func (env *Environment) AfterEach() {
	debug.AfterEach(env.Context)
	env.TimeIntervalCollector.Record(CurrentSpecReport().LeafNodeText)
	env.printControllerLogs(&corev1.PodLogOptions{Container: "controller"})
}

func (env *Environment) PrintCluster() {
	nodeClassList := unstructured.UnstructuredList{}
	nodeClassList.SetGroupVersionKind(env.DefaultNodeClass.GroupVersionKind())
	for _, obj := range append(ObjectListsToPrint, nodeClassList.DeepCopy()) {
		gvk := lo.Must(apiutil.GVKForObject(obj, env.Client.Scheme()))
		By(fmt.Sprintf("printing %s(s)", gvk.Kind))
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		Expect(env.Client.List(env, list, client.HasLabels([]string{test.DiscoveryLabel}))).To(Succeed())
		for _, item := range list.Items {
			fmt.Println(pretty.Concise(item))
			fmt.Println()
		}
		fmt.Println("---------------------------")
	}
}

func (env *Environment) CleanupObjects(cleanableObjects ...client.Object) {
	time.Sleep(time.Second) // wait one second to let the caches get up-to-date for deletion
	wg := sync.WaitGroup{}
	version, err := env.KubeClient.Discovery().ServerVersion()
	Expect(err).To(BeNil())
	for _, obj := range append(cleanableObjects, env.DefaultNodeClass.DeepCopy()) {
		if version.Minor < "30" &&
			obj.GetObjectKind().GroupVersionKind().Kind == "ValidatingAdmissionPolicy" &&
			obj.GetObjectKind().GroupVersionKind().Kind == "ValidatingAdmissionPolicyBinding" {
			continue
		}

		wg.Add(1)
		go func(obj client.Object) {
			defer wg.Done()
			defer GinkgoRecover()
			Eventually(func(g Gomega) {
				// This only gets the metadata for the objects since we don't need all the details of the objects
				metaList := &metav1.PartialObjectMetadataList{}
				metaList.SetGroupVersionKind(lo.Must(apiutil.GVKForObject(obj, env.Client.Scheme())))
				g.Expect(env.Client.List(env, metaList, client.HasLabels([]string{test.DiscoveryLabel}))).To(Succeed())
				// Limit the concurrency of these calls to 50 workers per object so that we try to limit how aggressively we
				// are deleting so that we avoid getting client-side throttled
				workqueue.ParallelizeUntil(env, 50, len(metaList.Items), func(i int) {
					defer GinkgoRecover()
					g.Expect(env.ExpectTestingFinalizerRemoved(&metaList.Items[i])).To(Succeed())
					g.Expect(client.IgnoreNotFound(env.Client.Delete(env, &metaList.Items[i],
						client.PropagationPolicy(metav1.DeletePropagationForeground),
						&client.DeleteOptions{GracePeriodSeconds: lo.ToPtr(int64(0))}))).To(Succeed())
				})
				// If the deletes eventually succeed, we should have no elements here at the end of the test
				g.Expect(env.Client.List(env, metaList, client.HasLabels([]string{test.DiscoveryLabel}), client.Limit(1))).To(Succeed())
				g.Expect(metaList.Items).To(HaveLen(0))
			}).Should(Succeed())
		}(obj)
	}
	wg.Wait()
}

func (env *Environment) ExpectTestingFinalizerRemoved(obj client.Object) error {
	metaObj := &metav1.PartialObjectMetadata{}
	metaObj.SetGroupVersionKind(lo.Must(apiutil.GVKForObject(obj, env.Client.Scheme())))
	if err := env.Client.Get(env, client.ObjectKeyFromObject(obj), metaObj); err != nil {
		return client.IgnoreNotFound(err)
	}
	deepCopy := metaObj.DeepCopy()
	metaObj.Finalizers = lo.Reject(metaObj.Finalizers, func(finalizer string, _ int) bool {
		return finalizer == TestingFinalizer
	})

	if !equality.Semantic.DeepEqual(metaObj, deepCopy) {
		// If the Group is the "core" APIs, then we can strategic merge patch
		// CRDs do not currently have support for strategic merge patching, so we can't blindly do it
		// https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#advanced-features-and-flexibility:~:text=Yes-,strategic%2Dmerge%2Dpatch,-The%20new%20endpoints
		if metaObj.GroupVersionKind().Group == "" {
			return client.IgnoreNotFound(env.Client.Patch(env, metaObj, client.StrategicMergeFrom(deepCopy)))
		}
		return client.IgnoreNotFound(env.Client.Patch(env, metaObj, client.MergeFrom(deepCopy)))
	}
	return nil
}
