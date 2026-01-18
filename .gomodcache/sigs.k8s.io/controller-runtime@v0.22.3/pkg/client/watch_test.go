/*
Copyright 2020 The Kubernetes Authors.

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

package client_test

import (
	"context"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ClientWithWatch", func() {
	var dep *appsv1.Deployment
	var count uint64 = 0
	var replicaCount int32 = 2
	var ns = "kube-public"

	BeforeEach(func(ctx SpecContext) {
		atomic.AddUint64(&count, 1)
		dep = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("watch-deployment-name-%v", count), Namespace: ns, Labels: map[string]string{"app": fmt.Sprintf("bar-%v", count)}},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicaCount,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
				},
			},
		}

		var err error
		dep, err = clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		deleteDeployment(ctx, dep, ns)
	})

	Describe("NewWithWatch", func() {
		It("should return a new Client", func(ctx SpecContext) {
			cl, err := client.NewWithWatch(cfg, client.Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(cl).NotTo(BeNil())
		})

		watchSuite := func(ctx context.Context, through client.ObjectList, expectedType client.Object, checkGvk bool) {
			cl, err := client.NewWithWatch(cfg, client.Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(cl).NotTo(BeNil())

			watchInterface, err := cl.Watch(ctx, through, &client.ListOptions{
				FieldSelector: fields.OneTermEqualSelector("metadata.name", dep.Name),
				Namespace:     dep.Namespace,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(watchInterface).NotTo(BeNil())

			defer watchInterface.Stop()

			event, ok := <-watchInterface.ResultChan()
			Expect(ok).To(BeTrue())
			Expect(event.Type).To(BeIdenticalTo(watch.Added))
			Expect(event.Object).To(BeAssignableToTypeOf(expectedType))

			// The metadata client doesn't set GVK so we just use the
			// name and UID as a proxy to confirm that we got the right
			// object.
			metaObject, ok := event.Object.(metav1.Object)
			Expect(ok).To(BeTrue())
			Expect(metaObject.GetName()).To(Equal(dep.Name))
			Expect(metaObject.GetUID()).To(Equal(dep.UID))

			if checkGvk {
				runtimeObject := event.Object
				gvk := runtimeObject.GetObjectKind().GroupVersionKind()
				Expect(gvk).To(Equal(schema.GroupVersionKind{
					Group:   "apps",
					Kind:    "Deployment",
					Version: "v1",
				}))
			}
		}

		It("should receive a create event when watching the typed object", func(ctx SpecContext) {
			watchSuite(ctx, &appsv1.DeploymentList{}, &appsv1.Deployment{}, false)
		})

		It("should receive a create event when watching the unstructured object", func(ctx SpecContext) {
			u := &unstructured.UnstructuredList{}
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Kind:    "Deployment",
				Version: "v1",
			})
			watchSuite(ctx, u, &unstructured.Unstructured{}, true)
		})

		It("should receive a create event when watching the metadata object", func(ctx SpecContext) {
			m := &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"}}
			watchSuite(ctx, m, &metav1.PartialObjectMetadata{}, false)
		})
	})

})
