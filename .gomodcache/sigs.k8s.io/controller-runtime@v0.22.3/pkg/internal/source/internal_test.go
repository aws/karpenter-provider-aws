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

package internal_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	internal "sigs.k8s.io/controller-runtime/pkg/internal/source"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ = Describe("Internal", func() {
	var instance *internal.EventHandler[client.Object, reconcile.Request]
	var funcs, setfuncs *handler.Funcs
	var set bool
	BeforeEach(func(ctx SpecContext) {
		funcs = &handler.Funcs{
			CreateFunc: func(context.Context, event.CreateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Fail("Did not expect CreateEvent to be called.")
			},
			DeleteFunc: func(context.Context, event.DeleteEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Fail("Did not expect DeleteEvent to be called.")
			},
			UpdateFunc: func(context.Context, event.UpdateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Fail("Did not expect UpdateEvent to be called.")
			},
			GenericFunc: func(context.Context, event.GenericEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Fail("Did not expect GenericEvent to be called.")
			},
		}

		setfuncs = &handler.Funcs{
			CreateFunc: func(context.Context, event.CreateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				set = true
			},
			DeleteFunc: func(context.Context, event.DeleteEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				set = true
			},
			UpdateFunc: func(context.Context, event.UpdateEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				set = true
			},
			GenericFunc: func(context.Context, event.GenericEvent, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				set = true
			},
		}
		instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, funcs, nil)
	})

	Describe("EventHandler", func() {
		var pod, newPod *corev1.Pod

		BeforeEach(func() {
			pod = &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "test", Image: "test"}},
				},
			}
			newPod = pod.DeepCopy()
			newPod.Labels = map[string]string{"foo": "bar"}
		})

		It("should create a CreateEvent", func(ctx SpecContext) {
			funcs.CreateFunc = func(ctx context.Context, evt event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
			}
			instance.OnAdd(pod, false)
		})

		It("should used Predicates to filter CreateEvents", func(ctx SpecContext) {
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return false }},
			})
			set = false
			instance.OnAdd(pod, false)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return true }},
			})
			instance.OnAdd(pod, false)
			Expect(set).To(BeTrue())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return true }},
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return false }},
			})
			instance.OnAdd(pod, false)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return false }},
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return true }},
			})
			instance.OnAdd(pod, false)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return true }},
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return true }},
			})
			instance.OnAdd(pod, false)
			Expect(set).To(BeTrue())
		})

		It("should not call Create EventHandler if the object is not a runtime.Object", func() {
			instance.OnAdd(&metav1.ObjectMeta{}, false)
		})

		It("should not call Create EventHandler if the object does not have metadata", func() {
			instance.OnAdd(FooRuntimeObject{}, false)
		})

		It("should create an UpdateEvent", func(ctx SpecContext) {
			funcs.UpdateFunc = func(ctx context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Expect(evt.ObjectOld).To(Equal(pod))
				Expect(evt.ObjectNew).To(Equal(newPod))
			}
			instance.OnUpdate(pod, newPod)
		})

		It("should used Predicates to filter UpdateEvents", func(ctx SpecContext) {
			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{UpdateFunc: func(updateEvent event.UpdateEvent) bool { return false }},
			})
			instance.OnUpdate(pod, newPod)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{UpdateFunc: func(event.UpdateEvent) bool { return true }},
			})
			instance.OnUpdate(pod, newPod)
			Expect(set).To(BeTrue())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{UpdateFunc: func(event.UpdateEvent) bool { return true }},
				predicate.Funcs{UpdateFunc: func(event.UpdateEvent) bool { return false }},
			})
			instance.OnUpdate(pod, newPod)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{UpdateFunc: func(event.UpdateEvent) bool { return false }},
				predicate.Funcs{UpdateFunc: func(event.UpdateEvent) bool { return true }},
			})
			instance.OnUpdate(pod, newPod)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return true }},
				predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return true }},
			})
			instance.OnUpdate(pod, newPod)
			Expect(set).To(BeTrue())
		})

		It("should not call Update EventHandler if the object is not a runtime.Object", func() {
			instance.OnUpdate(&metav1.ObjectMeta{}, &corev1.Pod{})
			instance.OnUpdate(&corev1.Pod{}, &metav1.ObjectMeta{})
		})

		It("should not call Update EventHandler if the object does not have metadata", func() {
			instance.OnUpdate(FooRuntimeObject{}, &corev1.Pod{})
			instance.OnUpdate(&corev1.Pod{}, FooRuntimeObject{})
		})

		It("should create a DeleteEvent", func() {
			funcs.DeleteFunc = func(ctx context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
			}
			instance.OnDelete(pod)
		})

		It("should used Predicates to filter DeleteEvents", func(ctx SpecContext) {
			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return false }},
			})
			instance.OnDelete(pod)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return true }},
			})
			instance.OnDelete(pod)
			Expect(set).To(BeTrue())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return true }},
				predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return false }},
			})
			instance.OnDelete(pod)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return false }},
				predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return true }},
			})
			instance.OnDelete(pod)
			Expect(set).To(BeFalse())

			set = false
			instance = internal.NewEventHandler(ctx, &controllertest.Queue{}, setfuncs, []predicate.Predicate{
				predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return true }},
				predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return true }},
			})
			instance.OnDelete(pod)
			Expect(set).To(BeTrue())
		})

		It("should not call Delete EventHandler if the object is not a runtime.Object", func() {
			instance.OnDelete(&metav1.ObjectMeta{})
		})

		It("should not call Delete EventHandler if the object does not have metadata", func() {
			instance.OnDelete(FooRuntimeObject{})
		})

		It("should create a DeleteEvent from a tombstone", func() {
			tombstone := cache.DeletedFinalStateUnknown{
				Obj: pod,
			}
			funcs.DeleteFunc = func(ctx context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
				Expect(evt.DeleteStateUnknown).Should(BeTrue())
			}

			instance.OnDelete(tombstone)
		})

		It("should ignore tombstone objects without meta", func() {
			tombstone := cache.DeletedFinalStateUnknown{Obj: Foo{}}
			instance.OnDelete(tombstone)
		})
		It("should ignore objects without meta", func() {
			instance.OnAdd(Foo{}, false)
			instance.OnUpdate(Foo{}, Foo{})
			instance.OnDelete(Foo{})
		})
	})

	Describe("Kind", func() {
		It("should return kind source type", func() {
			kind := internal.Kind[*corev1.Pod, reconcile.Request]{
				Type: &corev1.Pod{},
			}
			Expect(kind.String()).Should(Equal("kind source: *v1.Pod"))
		})
	})
})

type Foo struct{}

var _ runtime.Object = FooRuntimeObject{}

type FooRuntimeObject struct{}

func (FooRuntimeObject) GetObjectKind() schema.ObjectKind { return nil }
func (FooRuntimeObject) DeepCopyObject() runtime.Object   { return nil }
