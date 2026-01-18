/*
Copyright 2018 The Kubernetes Authors.

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

package source_test

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var _ = Describe("Source", func() {
	var instance1, instance2 source.Source
	var obj client.Object
	var q workqueue.TypedRateLimitingInterface[reconcile.Request]
	var c1, c2 chan interface{}
	var ns string
	count := 0

	BeforeEach(func(ctx SpecContext) {
		// Create the namespace for the test
		ns = fmt.Sprintf("controller-source-kindsource-%v", count)
		count++
		_, err := clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		q = workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
			workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
				Name: "test",
			})
		c1 = make(chan interface{})
		c2 = make(chan interface{})
	})

	AfterEach(func(ctx SpecContext) {
		err := clientset.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
		close(c1)
		close(c2)
	})

	Describe("Kind", func() {
		Context("for a Deployment resource", func() {
			obj = &appsv1.Deployment{}

			It("should provide Deployment Events", func(ctx SpecContext) {
				var created, updated, deleted *appsv1.Deployment
				var err error

				// Get the client and Deployment used to create events
				client := clientset.AppsV1().Deployments(ns)
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "deployment-name"},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "nginx",
										Image: "nginx",
									},
								},
							},
						},
					},
				}

				// Create an event handler to verify the events
				newHandler := func(c chan interface{}) handler.Funcs {
					return handler.Funcs{
						CreateFunc: func(ctx context.Context, evt event.CreateEvent, rli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Expect(rli).To(Equal(q))
							c <- evt
						},
						UpdateFunc: func(ctx context.Context, evt event.UpdateEvent, rli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Expect(rli).To(Equal(q))
							c <- evt
						},
						DeleteFunc: func(ctx context.Context, evt event.DeleteEvent, rli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Expect(rli).To(Equal(q))
							c <- evt
						},
					}
				}
				handler1 := newHandler(c1)
				handler2 := newHandler(c2)

				// Create 2 instances
				instance1 = source.Kind(icache, obj, handler1)
				instance2 = source.Kind(icache, obj, handler2)
				Expect(instance1.Start(ctx, q)).To(Succeed())
				Expect(instance2.Start(ctx, q)).To(Succeed())

				By("Creating a Deployment and expecting the CreateEvent.")
				created, err = client.Create(ctx, deployment, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(created).NotTo(BeNil())

				// Check first CreateEvent
				evt := <-c1
				createEvt, ok := evt.(event.CreateEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.CreateEvent{}))
				Expect(createEvt.Object).To(Equal(created))

				// Check second CreateEvent
				evt = <-c2
				createEvt, ok = evt.(event.CreateEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.CreateEvent{}))
				Expect(createEvt.Object).To(Equal(created))

				By("Updating a Deployment and expecting the UpdateEvent.")
				updated = created.DeepCopy()
				updated.Labels = map[string]string{"biz": "buz"}
				updated, err = client.Update(ctx, updated, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Check first UpdateEvent
				evt = <-c1
				updateEvt, ok := evt.(event.UpdateEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.UpdateEvent{}))

				Expect(updateEvt.ObjectNew).To(Equal(updated))

				Expect(updateEvt.ObjectOld).To(Equal(created))

				// Check second UpdateEvent
				evt = <-c2
				updateEvt, ok = evt.(event.UpdateEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.UpdateEvent{}))

				Expect(updateEvt.ObjectNew).To(Equal(updated))

				Expect(updateEvt.ObjectOld).To(Equal(created))

				By("Deleting a Deployment and expecting the Delete.")
				deleted = updated.DeepCopy()
				err = client.Delete(ctx, created.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				deleted.SetResourceVersion("")
				evt = <-c1
				deleteEvt, ok := evt.(event.DeleteEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.DeleteEvent{}))
				deleteEvt.Object.SetResourceVersion("")
				Expect(deleteEvt.Object).To(Equal(deleted))

				evt = <-c2
				deleteEvt, ok = evt.(event.DeleteEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.DeleteEvent{}))
				deleteEvt.Object.SetResourceVersion("")
				Expect(deleteEvt.Object).To(Equal(deleted))
			})
		})

		// TODO(pwittrock): Write this test
		PContext("for a Foo CRD resource", func() {
			It("should provide Foo Events", func() {

			})
		})
	})

	Describe("Informer", func() {
		var c chan struct{}
		var rs *appsv1.ReplicaSet
		var depInformer toolscache.SharedIndexInformer
		var informerFactory kubeinformers.SharedInformerFactory
		var stopTest chan struct{}

		BeforeEach(func() {
			stopTest = make(chan struct{})
			informerFactory = kubeinformers.NewSharedInformerFactory(clientset, time.Second*30)
			depInformer = informerFactory.Apps().V1().ReplicaSets().Informer()
			informerFactory.Start(stopTest)
			Eventually(depInformer.HasSynced).Should(BeTrue())

			c = make(chan struct{})
			rs = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{Name: "informer-rs-name"},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			}
		})

		AfterEach(func() {
			close(stopTest)
		})

		Context("for a ReplicaSet resource", func() {
			It("should provide a ReplicaSet CreateEvent", func(ctx SpecContext) {
				c := make(chan struct{})

				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				instance := &source.Informer{
					Informer: depInformer,
					Handler: handler.Funcs{
						CreateFunc: func(ctx context.Context, evt event.CreateEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							var err error
							rs, err := clientset.AppsV1().ReplicaSets("default").Get(ctx, rs.Name, metav1.GetOptions{})
							Expect(err).NotTo(HaveOccurred())

							Expect(q2).To(BeIdenticalTo(q))
							Expect(evt.Object).To(Equal(rs))
							close(c)
						},
						UpdateFunc: func(context.Context, event.UpdateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected UpdateEvent")
						},
						DeleteFunc: func(context.Context, event.DeleteEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected DeleteEvent")
						},
						GenericFunc: func(context.Context, event.GenericEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected GenericEvent")
						},
					},
				}
				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())

				_, err = clientset.AppsV1().ReplicaSets("default").Create(ctx, rs, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				<-c
			})

			It("should provide a ReplicaSet UpdateEvent", func(ctx SpecContext) {
				var err error
				rs, err = clientset.AppsV1().ReplicaSets("default").Get(ctx, rs.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				rs2 := rs.DeepCopy()
				rs2.SetLabels(map[string]string{"biz": "baz"})

				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				instance := &source.Informer{
					Informer: depInformer,
					Handler: handler.Funcs{
						CreateFunc: func(ctx context.Context, evt event.CreateEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						},
						UpdateFunc: func(ctx context.Context, evt event.UpdateEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							var err error
							rs2, err := clientset.AppsV1().ReplicaSets("default").Get(ctx, rs.Name, metav1.GetOptions{})
							Expect(err).NotTo(HaveOccurred())

							Expect(q2).To(Equal(q))
							Expect(evt.ObjectOld).To(Equal(rs))

							Expect(evt.ObjectNew).To(Equal(rs2))

							close(c)
						},
						DeleteFunc: func(context.Context, event.DeleteEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected DeleteEvent")
						},
						GenericFunc: func(context.Context, event.GenericEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected GenericEvent")
						},
					},
				}
				err = instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())

				_, err = clientset.AppsV1().ReplicaSets("default").Update(ctx, rs2, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())
				<-c
			})

			It("should provide a ReplicaSet DeletedEvent", func(ctx SpecContext) {
				c := make(chan struct{})

				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				instance := &source.Informer{
					Informer: depInformer,
					Handler: handler.Funcs{
						CreateFunc: func(context.Context, event.CreateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						},
						UpdateFunc: func(context.Context, event.UpdateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						},
						DeleteFunc: func(ctx context.Context, evt event.DeleteEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Expect(q2).To(Equal(q))
							Expect(evt.Object.GetName()).To(Equal(rs.Name))
							close(c)
						},
						GenericFunc: func(context.Context, event.GenericEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected GenericEvent")
						},
					},
				}
				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())

				err = clientset.AppsV1().ReplicaSets("default").Delete(ctx, rs.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
				<-c
			})
		})
	})
})
