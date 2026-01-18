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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

var _ = Describe("Source", func() {
	Describe("Kind", func() {
		var c chan struct{}
		var p *corev1.Pod
		var ic *informertest.FakeInformers

		BeforeEach(func() {
			ic = &informertest.FakeInformers{}
			c = make(chan struct{})
			p = &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "test"},
					},
				},
			}
		})

		Context("for a Pod resource", func() {
			It("should provide a Pod CreateEvent", func(ctx SpecContext) {
				c := make(chan struct{})
				p := &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test", Image: "test"},
						},
					},
				}

				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				instance := source.Kind(ic, &corev1.Pod{}, handler.TypedFuncs[*corev1.Pod, reconcile.Request]{
					CreateFunc: func(ctx context.Context, evt event.TypedCreateEvent[*corev1.Pod], q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Expect(q2).To(Equal(q))
						Expect(evt.Object).To(Equal(p))
						close(c)
					},
					UpdateFunc: func(context.Context, event.TypedUpdateEvent[*corev1.Pod], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected UpdateEvent")
					},
					DeleteFunc: func(context.Context, event.TypedDeleteEvent[*corev1.Pod], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected DeleteEvent")
					},
					GenericFunc: func(context.Context, event.TypedGenericEvent[*corev1.Pod], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected GenericEvent")
					},
				})
				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())
				Expect(instance.WaitForSync(ctx)).NotTo(HaveOccurred())

				i, err := ic.FakeInformerFor(ctx, &corev1.Pod{})
				Expect(err).NotTo(HaveOccurred())

				i.Add(p)
				<-c
			})

			It("should provide a Pod UpdateEvent", func(ctx SpecContext) {
				p2 := p.DeepCopy()
				p2.SetLabels(map[string]string{"biz": "baz"})

				ic := &informertest.FakeInformers{}
				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				instance := source.Kind(ic, &corev1.Pod{}, handler.TypedFuncs[*corev1.Pod, reconcile.Request]{
					CreateFunc: func(ctx context.Context, evt event.TypedCreateEvent[*corev1.Pod], q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected CreateEvent")
					},
					UpdateFunc: func(ctx context.Context, evt event.TypedUpdateEvent[*corev1.Pod], q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Expect(q2).To(BeIdenticalTo(q))
						Expect(evt.ObjectOld).To(Equal(p))

						Expect(evt.ObjectNew).To(Equal(p2))

						close(c)
					},
					DeleteFunc: func(context.Context, event.TypedDeleteEvent[*corev1.Pod], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected DeleteEvent")
					},
					GenericFunc: func(context.Context, event.TypedGenericEvent[*corev1.Pod], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected GenericEvent")
					},
				})
				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())
				Expect(instance.WaitForSync(ctx)).NotTo(HaveOccurred())

				i, err := ic.FakeInformerFor(ctx, &corev1.Pod{})
				Expect(err).NotTo(HaveOccurred())

				i.Update(p, p2)
				<-c
			})

			It("should provide a Pod DeletedEvent", func(ctx SpecContext) {
				c := make(chan struct{})
				p := &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test", Image: "test"},
						},
					},
				}

				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				instance := source.Kind(ic, &corev1.Pod{}, handler.TypedFuncs[*corev1.Pod, reconcile.Request]{
					CreateFunc: func(context.Context, event.TypedCreateEvent[*corev1.Pod], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected DeleteEvent")
					},
					UpdateFunc: func(context.Context, event.TypedUpdateEvent[*corev1.Pod], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected UpdateEvent")
					},
					DeleteFunc: func(ctx context.Context, evt event.TypedDeleteEvent[*corev1.Pod], q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Expect(q2).To(BeIdenticalTo(q))
						Expect(evt.Object).To(Equal(p))
						close(c)
					},
					GenericFunc: func(context.Context, event.TypedGenericEvent[*corev1.Pod], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						Fail("Unexpected GenericEvent")
					},
				})
				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())
				Expect(instance.WaitForSync(ctx)).NotTo(HaveOccurred())

				i, err := ic.FakeInformerFor(ctx, &corev1.Pod{})
				Expect(err).NotTo(HaveOccurred())

				i.Delete(p)
				<-c
			})
		})

		It("should return an error from Start cache was not provided", func(ctx SpecContext) {
			instance := source.Kind(nil, &corev1.Pod{}, nil)
			err := instance.Start(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must create Kind with a non-nil cache"))
		})

		It("should return an error from Start if a type was not provided", func(ctx SpecContext) {
			instance := source.Kind[client.Object](ic, nil, nil)
			err := instance.Start(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must create Kind with a non-nil object"))
		})
		It("should return an error from Start if a handler was not provided", func(ctx SpecContext) {
			instance := source.Kind(ic, &corev1.Pod{}, nil)
			err := instance.Start(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must create Kind with non-nil handler"))
		})

		It("should return an error if syncing fails", func(ctx SpecContext) {
			f := false
			instance := source.Kind[client.Object](&informertest.FakeInformers{Synced: &f}, &corev1.Pod{}, &handler.EnqueueRequestForObject{})
			Expect(instance.Start(ctx, nil)).NotTo(HaveOccurred())
			err := instance.WaitForSync(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cache did not sync"))

		})

		Context("for a Kind not in the cache", func() {
			It("should return an error when WaitForSync is called", func(specContext SpecContext) {
				ic.Error = fmt.Errorf("test error")
				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})

				ctx, cancel := context.WithTimeout(specContext, 2*time.Second)
				defer cancel()

				instance := source.Kind(ic, &corev1.Pod{}, handler.TypedFuncs[*corev1.Pod, reconcile.Request]{})
				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())
				Eventually(instance.WaitForSync).WithArguments(ctx).Should(HaveOccurred())
			})
		})

		It("should return an error if syncing fails", func(ctx SpecContext) {
			f := false
			instance := source.Kind[client.Object](&informertest.FakeInformers{Synced: &f}, &corev1.Pod{}, &handler.EnqueueRequestForObject{})
			Expect(instance.Start(ctx, nil)).NotTo(HaveOccurred())
			err := instance.WaitForSync(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cache did not sync"))

		})
	})

	Describe("Func", func() {
		It("should be called from Start", func(ctx SpecContext) {
			run := false
			instance := source.Func(func(
				context.Context,
				workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				run = true
				return nil
			})
			Expect(instance.Start(ctx, nil)).NotTo(HaveOccurred())
			Expect(run).To(BeTrue())

			expected := fmt.Errorf("expected error: Func")
			instance = source.Func(func(
				context.Context,
				workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				return expected
			})
			Expect(instance.Start(ctx, nil)).To(Equal(expected))
		})
	})

	Describe("Channel", func() {
		var ch chan event.GenericEvent

		BeforeEach(func() {
			ch = make(chan event.GenericEvent)
		})

		AfterEach(func() {
			close(ch)
		})

		Context("for a source", func() {
			It("should provide a GenericEvent", func(ctx SpecContext) {
				ch := make(chan event.GenericEvent)
				c := make(chan struct{})
				p := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"},
				}
				evt := event.GenericEvent{
					Object: p,
				}
				// Event that should be filtered out by predicates
				invalidEvt := event.GenericEvent{}

				// Predicate to filter out empty event
				prct := predicate.Funcs{
					GenericFunc: func(e event.GenericEvent) bool {
						return e.Object != nil
					},
				}

				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				instance := source.Channel(
					ch,
					handler.Funcs{
						CreateFunc: func(context.Context, event.CreateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected CreateEvent")
						},
						UpdateFunc: func(context.Context, event.UpdateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected UpdateEvent")
						},
						DeleteFunc: func(context.Context, event.DeleteEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected DeleteEvent")
						},
						GenericFunc: func(ctx context.Context, evt event.GenericEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							// The empty event should have been filtered out by the predicates,
							// and will not be passed to the handler.
							Expect(q2).To(BeIdenticalTo(q))
							Expect(evt.Object).To(Equal(p))
							close(c)
						},
					},
					source.WithPredicates[client.Object, reconcile.Request](prct),
				)
				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())

				ch <- invalidEvt
				ch <- evt
				<-c
			})
			It("should get pending events processed once channel unblocked", func(ctx SpecContext) {
				ch := make(chan event.GenericEvent)
				unblock := make(chan struct{})
				processed := make(chan struct{})
				evt := event.GenericEvent{}
				eventCount := 0

				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				// Add a handler to get distribution blocked
				instance := source.Channel(
					ch,
					handler.Funcs{
						CreateFunc: func(context.Context, event.CreateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected CreateEvent")
						},
						UpdateFunc: func(context.Context, event.UpdateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected UpdateEvent")
						},
						DeleteFunc: func(context.Context, event.DeleteEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected DeleteEvent")
						},
						GenericFunc: func(ctx context.Context, evt event.GenericEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							// Block for the first time
							if eventCount == 0 {
								<-unblock
							}
							eventCount++

							if eventCount == 3 {
								close(processed)
							}
						},
					},
				)
				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())

				// Write 3 events into the source channel.
				// The 1st should be passed into the generic func of the handler;
				// The 2nd should be fetched out of the source channel, and waiting to write into dest channel;
				// The 3rd should be pending in the source channel.
				ch <- evt
				ch <- evt
				ch <- evt

				// Validate none of the events have been processed.
				Expect(eventCount).To(Equal(0))

				close(unblock)

				<-processed

				// Validate all of the events have been processed.
				Expect(eventCount).To(Equal(3))
			})
			It("should be able to cope with events in the channel before the source is started", func(ctx SpecContext) {
				ch := make(chan event.GenericEvent, 1)
				processed := make(chan struct{})
				evt := event.GenericEvent{}
				ch <- evt

				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				// Add a handler to get distribution blocked
				instance := source.Channel(
					ch,
					handler.Funcs{
						CreateFunc: func(context.Context, event.CreateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected CreateEvent")
						},
						UpdateFunc: func(context.Context, event.UpdateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected UpdateEvent")
						},
						DeleteFunc: func(context.Context, event.DeleteEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected DeleteEvent")
						},
						GenericFunc: func(ctx context.Context, evt event.GenericEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()

							close(processed)
						},
					},
				)

				err := instance.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())

				<-processed
			})
			It("should stop when the source channel is closed", func(ctx SpecContext) {
				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				// if we didn't stop, we'd start spamming the queue with empty
				// messages as we "received" a zero-valued GenericEvent from
				// the source channel

				By("creating a channel with one element, then closing it")
				ch := make(chan event.GenericEvent, 1)
				evt := event.GenericEvent{}
				ch <- evt
				close(ch)

				By("feeding that channel to a channel source")
				processed := make(chan struct{})
				defer close(processed)
				src := source.Channel(
					ch,
					handler.Funcs{
						CreateFunc: func(context.Context, event.CreateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected CreateEvent")
						},
						UpdateFunc: func(context.Context, event.UpdateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected UpdateEvent")
						},
						DeleteFunc: func(context.Context, event.DeleteEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()
							Fail("Unexpected DeleteEvent")
						},
						GenericFunc: func(ctx context.Context, evt event.GenericEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							defer GinkgoRecover()

							processed <- struct{}{}
						},
					},
				)

				err := src.Start(ctx, q)
				Expect(err).NotTo(HaveOccurred())

				By("expecting to only get one event")
				Eventually(processed).Should(Receive())
				Consistently(processed).ShouldNot(Receive())
			})
			It("should get error if no source specified", func(ctx SpecContext) {
				q := workqueue.NewTypedRateLimitingQueueWithConfig(
					workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
					workqueue.TypedRateLimitingQueueConfig[reconcile.Request]{
						Name: "test",
					})
				instance := source.Channel[string](nil, nil /*no source specified*/)
				err := instance.Start(ctx, q)
				Expect(err).To(Equal(fmt.Errorf("must specify Channel.Source")))
			})
		})
	})
})
