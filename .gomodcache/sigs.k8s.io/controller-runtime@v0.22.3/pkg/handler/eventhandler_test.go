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

package handler_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/controller/priorityqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Eventhandler", func() {
	var q workqueue.TypedRateLimitingInterface[reconcile.Request]
	var instance handler.EnqueueRequestForObject
	var pod *corev1.Pod
	var mapper meta.RESTMapper
	BeforeEach(func() {
		q = &controllertest.Queue{TypedInterface: workqueue.NewTyped[reconcile.Request]()}
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "biz", Name: "baz"},
		}
		Expect(cfg).NotTo(BeNil())

		httpClient, err := rest.HTTPClientFor(cfg)
		Expect(err).ShouldNot(HaveOccurred())
		mapper, err = apiutil.NewDynamicRESTMapper(cfg, httpClient)
		Expect(err).ShouldNot(HaveOccurred())
	})

	Describe("EnqueueRequestForObject", func() {
		It("should enqueue a Request with the Name / Namespace of the object in the CreateEvent.", func(ctx SpecContext) {
			evt := event.CreateEvent{
				Object: pod,
			}
			instance.Create(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))

			req, _ := q.Get()
			Expect(req.NamespacedName).To(Equal(types.NamespacedName{Namespace: "biz", Name: "baz"}))
		})

		It("should enqueue a Request with the Name / Namespace of the object in the DeleteEvent.", func(ctx SpecContext) {
			evt := event.DeleteEvent{
				Object: pod,
			}
			instance.Delete(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))

			req, _ := q.Get()
			Expect(req.NamespacedName).To(Equal(types.NamespacedName{Namespace: "biz", Name: "baz"}))
		})

		It("should enqueue a Request with the Name / Namespace of one object in the UpdateEvent.",
			func(ctx SpecContext) {
				newPod := pod.DeepCopy()
				newPod.Name = "baz2"
				newPod.Namespace = "biz2"

				evt := event.UpdateEvent{
					ObjectOld: pod,
					ObjectNew: newPod,
				}
				instance.Update(ctx, evt, q)
				Expect(q.Len()).To(Equal(1))

				req, _ := q.Get()
				Expect(req.NamespacedName).To(Equal(types.NamespacedName{Namespace: "biz2", Name: "baz2"}))
			})

		It("should enqueue a Request with the Name / Namespace of the object in the GenericEvent.", func(ctx SpecContext) {
			evt := event.GenericEvent{
				Object: pod,
			}
			instance.Generic(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))
			req, _ := q.Get()
			Expect(req.NamespacedName).To(Equal(types.NamespacedName{Namespace: "biz", Name: "baz"}))
		})

		Context("for a runtime.Object without Object", func() {
			It("should do nothing if the Object is missing for a CreateEvent.", func(ctx SpecContext) {
				evt := event.CreateEvent{
					Object: nil,
				}
				instance.Create(ctx, evt, q)
				Expect(q.Len()).To(Equal(0))
			})

			It("should do nothing if the Object is missing for a UpdateEvent.", func(ctx SpecContext) {
				newPod := pod.DeepCopy()
				newPod.Name = "baz2"
				newPod.Namespace = "biz2"

				evt := event.UpdateEvent{
					ObjectNew: newPod,
					ObjectOld: nil,
				}
				instance.Update(ctx, evt, q)
				Expect(q.Len()).To(Equal(1))
				req, _ := q.Get()
				Expect(req.NamespacedName).To(Equal(types.NamespacedName{Namespace: "biz2", Name: "baz2"}))

				evt.ObjectNew = nil
				evt.ObjectOld = pod
				instance.Update(ctx, evt, q)
				Expect(q.Len()).To(Equal(1))
				req, _ = q.Get()
				Expect(req.NamespacedName).To(Equal(types.NamespacedName{Namespace: "biz", Name: "baz"}))
			})

			It("should do nothing if the Object is missing for a DeleteEvent.", func(ctx SpecContext) {
				evt := event.DeleteEvent{
					Object: nil,
				}
				instance.Delete(ctx, evt, q)
				Expect(q.Len()).To(Equal(0))
			})

			It("should do nothing if the Object is missing for a GenericEvent.", func(ctx SpecContext) {
				evt := event.GenericEvent{
					Object: nil,
				}
				instance.Generic(ctx, evt, q)
				Expect(q.Len()).To(Equal(0))
			})
		})
	})

	Describe("EnqueueRequestsFromMapFunc", func() {
		It("should enqueue a Request with the function applied to the CreateEvent.", func(ctx SpecContext) {
			req := []reconcile.Request{}
			instance := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				defer GinkgoRecover()
				Expect(a).To(Equal(pod))
				req = []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"},
					},
					{
						NamespacedName: types.NamespacedName{Namespace: "biz", Name: "baz"},
					},
				}
				return req
			})

			evt := event.CreateEvent{
				Object: pod,
			}
			instance.Create(ctx, evt, q)
			Expect(q.Len()).To(Equal(2))

			i1, _ := q.Get()
			i2, _ := q.Get()
			Expect([]interface{}{i1, i2}).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}},
				reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: "biz", Name: "baz"}},
			))
		})

		It("should enqueue a Request with the function applied to the DeleteEvent.", func(ctx SpecContext) {
			req := []reconcile.Request{}
			instance := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				defer GinkgoRecover()
				Expect(a).To(Equal(pod))
				req = []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"},
					},
					{
						NamespacedName: types.NamespacedName{Namespace: "biz", Name: "baz"},
					},
				}
				return req
			})

			evt := event.DeleteEvent{
				Object: pod,
			}
			instance.Delete(ctx, evt, q)
			Expect(q.Len()).To(Equal(2))

			i1, _ := q.Get()
			i2, _ := q.Get()
			Expect([]interface{}{i1, i2}).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}},
				reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: "biz", Name: "baz"}},
			))
		})

		It("should enqueue a Request with the function applied to both objects in the UpdateEvent.",
			func(ctx SpecContext) {
				newPod := pod.DeepCopy()

				req := []reconcile.Request{}

				instance := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
					defer GinkgoRecover()
					req = []reconcile.Request{
						{
							NamespacedName: types.NamespacedName{Namespace: "foo", Name: a.GetName() + "-bar"},
						},
						{
							NamespacedName: types.NamespacedName{Namespace: "biz", Name: a.GetName() + "-baz"},
						},
					}
					return req
				})

				evt := event.UpdateEvent{
					ObjectOld: pod,
					ObjectNew: newPod,
				}
				instance.Update(ctx, evt, q)
				Expect(q.Len()).To(Equal(2))

				i, _ := q.Get()
				Expect(i).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "baz-bar"}}))

				i, _ = q.Get()
				Expect(i).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "biz", Name: "baz-baz"}}))
			})

		It("should enqueue a Request with the function applied to the GenericEvent.", func(ctx SpecContext) {
			req := []reconcile.Request{}
			instance := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				defer GinkgoRecover()
				Expect(a).To(Equal(pod))
				req = []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"},
					},
					{
						NamespacedName: types.NamespacedName{Namespace: "biz", Name: "baz"},
					},
				}
				return req
			})

			evt := event.GenericEvent{
				Object: pod,
			}
			instance.Generic(ctx, evt, q)
			Expect(q.Len()).To(Equal(2))

			i1, _ := q.Get()
			i2, _ := q.Get()
			Expect([]interface{}{i1, i2}).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}},
				reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: "biz", Name: "baz"}},
			))
		})
	})

	Describe("EnqueueRequestForOwner", func() {
		It("should enqueue a Request with the Owner of the object in the CreateEvent.", func(ctx SpecContext) {
			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})

			pod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "foo-parent",
					Kind:       "ReplicaSet",
					APIVersion: "apps/v1",
				},
			}
			evt := event.CreateEvent{
				Object: pod,
			}
			instance.Create(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))

			i, _ := q.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo-parent"}}))
		})

		It("should enqueue a Request with the Owner of the object in the DeleteEvent.", func(ctx SpecContext) {
			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})

			pod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "foo-parent",
					Kind:       "ReplicaSet",
					APIVersion: "apps/v1",
				},
			}
			evt := event.DeleteEvent{
				Object: pod,
			}
			instance.Delete(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))

			i, _ := q.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo-parent"}}))
		})

		It("should enqueue a Request with the Owners of both objects in the UpdateEvent.", func(ctx SpecContext) {
			newPod := pod.DeepCopy()
			newPod.Name = pod.Name + "2"
			newPod.Namespace = pod.Namespace + "2"

			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})

			pod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "foo1-parent",
					Kind:       "ReplicaSet",
					APIVersion: "apps/v1",
				},
			}
			newPod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "foo2-parent",
					Kind:       "ReplicaSet",
					APIVersion: "apps/v1",
				},
			}
			evt := event.UpdateEvent{
				ObjectOld: pod,
				ObjectNew: newPod,
			}
			instance.Update(ctx, evt, q)
			Expect(q.Len()).To(Equal(2))

			i1, _ := q.Get()
			i2, _ := q.Get()
			Expect([]interface{}{i1, i2}).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo1-parent"}},
				reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: newPod.GetNamespace(), Name: "foo2-parent"}},
			))
		})

		It("should enqueue a Request with the one duplicate Owner of both objects in the UpdateEvent.", func(ctx SpecContext) {
			newPod := pod.DeepCopy()
			newPod.Name = pod.Name + "2"

			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})

			pod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "foo-parent",
					Kind:       "ReplicaSet",
					APIVersion: "apps/v1",
				},
			}
			newPod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "foo-parent",
					Kind:       "ReplicaSet",
					APIVersion: "apps/v1",
				},
			}
			evt := event.UpdateEvent{
				ObjectOld: pod,
				ObjectNew: newPod,
			}
			instance.Update(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))

			i, _ := q.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo-parent"}}))
		})

		It("should enqueue a Request with the Owner of the object in the GenericEvent.", func(ctx SpecContext) {
			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})
			pod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "foo-parent",
					Kind:       "ReplicaSet",
					APIVersion: "apps/v1",
				},
			}
			evt := event.GenericEvent{
				Object: pod,
			}
			instance.Generic(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))

			i, _ := q.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo-parent"}}))
		})

		It("should not enqueue a Request if there are no owners matching Group and Kind.", func(ctx SpecContext) {
			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{}, handler.OnlyControllerOwner())
			pod.OwnerReferences = []metav1.OwnerReference{
				{ // Wrong group
					Name:       "foo1-parent",
					Kind:       "ReplicaSet",
					APIVersion: "extensions/v1",
				},
				{ // Wrong kind
					Name:       "foo2-parent",
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			}
			evt := event.CreateEvent{
				Object: pod,
			}
			instance.Create(ctx, evt, q)
			Expect(q.Len()).To(Equal(0))
		})

		It("should enqueue a Request if there are owners matching Group "+
			"and Kind with a different version.", func(ctx SpecContext) {
			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &autoscalingv1.HorizontalPodAutoscaler{})
			pod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "foo-parent",
					Kind:       "HorizontalPodAutoscaler",
					APIVersion: "autoscaling/v2",
				},
			}
			evt := event.CreateEvent{
				Object: pod,
			}
			instance.Create(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))

			i, _ := q.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo-parent"}}))
		})

		It("should enqueue a Request for a owner that is cluster scoped", func(ctx SpecContext) {
			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &corev1.Node{})
			pod.OwnerReferences = []metav1.OwnerReference{
				{
					Name:       "node-1",
					Kind:       "Node",
					APIVersion: "v1",
				},
			}
			evt := event.CreateEvent{
				Object: pod,
			}
			instance.Create(ctx, evt, q)
			Expect(q.Len()).To(Equal(1))

			i, _ := q.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: "", Name: "node-1"}}))

		})

		It("should not enqueue a Request if there are no owners.", func(ctx SpecContext) {
			instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})
			evt := event.CreateEvent{
				Object: pod,
			}
			instance.Create(ctx, evt, q)
			Expect(q.Len()).To(Equal(0))
		})

		Context("with the Controller field set to true", func() {
			It("should enqueue reconcile.Requests for only the first the Controller if there are "+
				"multiple Controller owners.", func(ctx SpecContext) {
				instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{}, handler.OnlyControllerOwner())
				pod.OwnerReferences = []metav1.OwnerReference{
					{
						Name:       "foo1-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
					{
						Name:       "foo2-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
						Controller: ptr.To(true),
					},
					{
						Name:       "foo3-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
					{
						Name:       "foo4-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
						Controller: ptr.To(true),
					},
					{
						Name:       "foo5-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
				}
				evt := event.CreateEvent{
					Object: pod,
				}
				instance.Create(ctx, evt, q)
				Expect(q.Len()).To(Equal(1))
				i, _ := q.Get()
				Expect(i).To(Equal(reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo2-parent"}}))
			})

			It("should not enqueue reconcile.Requests if there are no Controller owners.", func(ctx SpecContext) {
				instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{}, handler.OnlyControllerOwner())
				pod.OwnerReferences = []metav1.OwnerReference{
					{
						Name:       "foo1-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
					{
						Name:       "foo2-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
					{
						Name:       "foo3-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
				}
				evt := event.CreateEvent{
					Object: pod,
				}
				instance.Create(ctx, evt, q)
				Expect(q.Len()).To(Equal(0))
			})

			It("should not enqueue reconcile.Requests if there are no owners.", func(ctx SpecContext) {
				instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{}, handler.OnlyControllerOwner())
				evt := event.CreateEvent{
					Object: pod,
				}
				instance.Create(ctx, evt, q)
				Expect(q.Len()).To(Equal(0))
			})
		})

		Context("with the Controller field set to false", func() {
			It("should enqueue a reconcile.Requests for all owners.", func(ctx SpecContext) {
				instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})
				pod.OwnerReferences = []metav1.OwnerReference{
					{
						Name:       "foo1-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
					{
						Name:       "foo2-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
					{
						Name:       "foo3-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
				}
				evt := event.CreateEvent{
					Object: pod,
				}
				instance.Create(ctx, evt, q)
				Expect(q.Len()).To(Equal(3))

				i1, _ := q.Get()
				i2, _ := q.Get()
				i3, _ := q.Get()
				Expect([]interface{}{i1, i2, i3}).To(ConsistOf(
					reconcile.Request{
						NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo1-parent"}},
					reconcile.Request{
						NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo2-parent"}},
					reconcile.Request{
						NamespacedName: types.NamespacedName{Namespace: pod.GetNamespace(), Name: "foo3-parent"}},
				))
			})
		})

		Context("with a nil object", func() {
			It("should do nothing.", func(ctx SpecContext) {
				instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})
				pod.OwnerReferences = []metav1.OwnerReference{
					{
						Name:       "foo1-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1",
					},
				}
				evt := event.CreateEvent{
					Object: nil,
				}
				instance.Create(ctx, evt, q)
				Expect(q.Len()).To(Equal(0))
			})
		})

		Context("with a nil OwnerType", func() {
			It("should panic", func() {
				Expect(func() {
					handler.EnqueueRequestForOwner(nil, nil, nil)
				}).To(Panic())
			})
		})

		Context("with an invalid APIVersion in the OwnerReference", func() {
			It("should do nothing.", func(ctx SpecContext) {
				instance := handler.EnqueueRequestForOwner(scheme.Scheme, mapper, &appsv1.ReplicaSet{})
				pod.OwnerReferences = []metav1.OwnerReference{
					{
						Name:       "foo1-parent",
						Kind:       "ReplicaSet",
						APIVersion: "apps/v1/fail",
					},
				}
				evt := event.CreateEvent{
					Object: pod,
				}
				instance.Create(ctx, evt, q)
				Expect(q.Len()).To(Equal(0))
			})
		})
	})

	Describe("Funcs", func() {
		failingFuncs := handler.TypedFuncs[client.Object, reconcile.Request]{
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

		It("should call CreateFunc for a CreateEvent if provided.", func(ctx SpecContext) {
			instance := failingFuncs
			evt := event.CreateEvent{
				Object: pod,
			}
			instance.CreateFunc = func(ctx context.Context, evt2 event.CreateEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Expect(q2).To(Equal(q))
				Expect(evt2).To(Equal(evt))
			}
			instance.Create(ctx, evt, q)
		})

		It("should NOT call CreateFunc for a CreateEvent if NOT provided.", func(ctx SpecContext) {
			instance := failingFuncs
			instance.CreateFunc = nil
			evt := event.CreateEvent{
				Object: pod,
			}
			instance.Create(ctx, evt, q)
		})

		It("should call UpdateFunc for an UpdateEvent if provided.", func(ctx SpecContext) {
			newPod := pod.DeepCopy()
			newPod.Name = pod.Name + "2"
			newPod.Namespace = pod.Namespace + "2"
			evt := event.UpdateEvent{
				ObjectOld: pod,
				ObjectNew: newPod,
			}

			instance := failingFuncs
			instance.UpdateFunc = func(ctx context.Context, evt2 event.UpdateEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Expect(q2).To(Equal(q))
				Expect(evt2).To(Equal(evt))
			}

			instance.Update(ctx, evt, q)
		})

		It("should NOT call UpdateFunc for an UpdateEvent if NOT provided.", func(ctx SpecContext) {
			newPod := pod.DeepCopy()
			newPod.Name = pod.Name + "2"
			newPod.Namespace = pod.Namespace + "2"
			evt := event.UpdateEvent{
				ObjectOld: pod,
				ObjectNew: newPod,
			}
			instance.Update(ctx, evt, q)
		})

		It("should call DeleteFunc for a DeleteEvent if provided.", func(ctx SpecContext) {
			instance := failingFuncs
			evt := event.DeleteEvent{
				Object: pod,
			}
			instance.DeleteFunc = func(ctx context.Context, evt2 event.DeleteEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Expect(q2).To(Equal(q))
				Expect(evt2).To(Equal(evt))
			}
			instance.Delete(ctx, evt, q)
		})

		It("should NOT call DeleteFunc for a DeleteEvent if NOT provided.", func(ctx SpecContext) {
			instance := failingFuncs
			instance.DeleteFunc = nil
			evt := event.DeleteEvent{
				Object: pod,
			}
			instance.Delete(ctx, evt, q)
		})

		It("should call GenericFunc for a GenericEvent if provided.", func(ctx SpecContext) {
			instance := failingFuncs
			evt := event.GenericEvent{
				Object: pod,
			}
			instance.GenericFunc = func(ctx context.Context, evt2 event.GenericEvent, q2 workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				defer GinkgoRecover()
				Expect(q2).To(Equal(q))
				Expect(evt2).To(Equal(evt))
			}
			instance.Generic(ctx, evt, q)
		})

		It("should NOT call GenericFunc for a GenericEvent if NOT provided.", func(ctx SpecContext) {
			instance := failingFuncs
			instance.GenericFunc = nil
			evt := event.GenericEvent{
				Object: pod,
			}
			instance.Generic(ctx, evt, q)
		})
	})

	Describe("WithLowPriorityWhenUnchanged", func() {
		handlerPriorityTests := []struct {
			name             string
			handler          func() handler.EventHandler
			after            time.Duration
			ratelimited      bool
			overridePriority int
		}{
			{
				name:    "WithLowPriorityWhenUnchanged wrapper",
				handler: func() handler.EventHandler { return handler.WithLowPriorityWhenUnchanged(customHandler{}) },
			},
			{
				name:    "EnqueueRequestForObject",
				handler: func() handler.EventHandler { return &handler.EnqueueRequestForObject{} },
			},
			{
				name: "EnqueueRequestForOwner",
				handler: func() handler.EventHandler {
					return handler.EnqueueRequestForOwner(
						scheme.Scheme,
						mapper,
						&corev1.Pod{},
					)
				},
			},
			{
				name: "TypedEnqueueRequestForOwner",
				handler: func() handler.EventHandler {
					return handler.TypedEnqueueRequestForOwner[client.Object](
						scheme.Scheme,
						mapper,
						&corev1.Pod{},
					)
				},
			},
			{
				name: "Funcs",
				handler: func() handler.EventHandler {
					return handler.TypedFuncs[client.Object, reconcile.Request]{
						CreateFunc: func(ctx context.Context, tce event.TypedCreateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							wq.Add(reconcile.Request{NamespacedName: types.NamespacedName{
								Namespace: tce.Object.GetNamespace(),
								Name:      tce.Object.GetName(),
							}})
						},
						UpdateFunc: func(ctx context.Context, tue event.TypedUpdateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
							wq.Add(reconcile.Request{NamespacedName: types.NamespacedName{
								Namespace: tue.ObjectNew.GetNamespace(),
								Name:      tue.ObjectNew.GetName(),
							}})
						},
					}
				},
			},
			{
				name: "EnqueueRequestsFromMapFunc",
				handler: func() handler.EventHandler {
					return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
						return []reconcile.Request{{NamespacedName: types.NamespacedName{
							Name:      obj.GetName(),
							Namespace: obj.GetNamespace(),
						}}}
					})
				},
			},
			{
				name: "WithLowPriorityWhenUnchanged - Add",
				handler: func() handler.EventHandler {
					return handler.WithLowPriorityWhenUnchanged(
						handler.TypedFuncs[client.Object, reconcile.Request]{
							CreateFunc: func(ctx context.Context, tce event.TypedCreateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
								wq.Add(reconcile.Request{NamespacedName: types.NamespacedName{
									Namespace: tce.Object.GetNamespace(),
									Name:      tce.Object.GetName(),
								}})
							},
							UpdateFunc: func(ctx context.Context, tue event.TypedUpdateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
								wq.Add(reconcile.Request{NamespacedName: types.NamespacedName{
									Namespace: tue.ObjectNew.GetNamespace(),
									Name:      tue.ObjectNew.GetName(),
								}})
							},
						})
				},
			},
			{
				name: "WithLowPriorityWhenUnchanged - AddAfter",
				handler: func() handler.EventHandler {
					return handler.WithLowPriorityWhenUnchanged(
						handler.TypedFuncs[client.Object, reconcile.Request]{
							CreateFunc: func(ctx context.Context, tce event.TypedCreateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
								wq.AddAfter(reconcile.Request{NamespacedName: types.NamespacedName{
									Namespace: tce.Object.GetNamespace(),
									Name:      tce.Object.GetName(),
								}}, time.Second)
							},
							UpdateFunc: func(ctx context.Context, tue event.TypedUpdateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
								wq.AddAfter(reconcile.Request{NamespacedName: types.NamespacedName{
									Namespace: tue.ObjectNew.GetNamespace(),
									Name:      tue.ObjectNew.GetName(),
								}}, time.Second)
							},
						})
				},
				after: time.Second,
			},
			{
				name: "WithLowPriorityWhenUnchanged - AddRateLimited",
				handler: func() handler.EventHandler {
					return handler.WithLowPriorityWhenUnchanged(
						handler.TypedFuncs[client.Object, reconcile.Request]{
							CreateFunc: func(ctx context.Context, tce event.TypedCreateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
								wq.AddRateLimited(reconcile.Request{NamespacedName: types.NamespacedName{
									Namespace: tce.Object.GetNamespace(),
									Name:      tce.Object.GetName(),
								}})
							},
							UpdateFunc: func(ctx context.Context, tue event.TypedUpdateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
								wq.AddRateLimited(reconcile.Request{NamespacedName: types.NamespacedName{
									Namespace: tue.ObjectNew.GetNamespace(),
									Name:      tue.ObjectNew.GetName(),
								}})
							},
						})
				},
				ratelimited: true,
			},
			{
				name: "WithLowPriorityWhenUnchanged - AddWithOpts priority is retained",
				handler: func() handler.EventHandler {
					return handler.WithLowPriorityWhenUnchanged(
						handler.TypedFuncs[client.Object, reconcile.Request]{
							CreateFunc: func(ctx context.Context, tce event.TypedCreateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
								if pq, isPQ := wq.(priorityqueue.PriorityQueue[reconcile.Request]); isPQ {
									pq.AddWithOpts(priorityqueue.AddOpts{Priority: ptr.To(100)}, reconcile.Request{NamespacedName: types.NamespacedName{
										Namespace: tce.Object.GetNamespace(),
										Name:      tce.Object.GetName(),
									}})
									return
								}
								wq.Add(reconcile.Request{NamespacedName: types.NamespacedName{
									Namespace: tce.Object.GetNamespace(),
									Name:      tce.Object.GetName(),
								}})
							},
							UpdateFunc: func(ctx context.Context, tue event.TypedUpdateEvent[client.Object], wq workqueue.TypedRateLimitingInterface[reconcile.Request]) {
								if pq, isPQ := wq.(priorityqueue.PriorityQueue[reconcile.Request]); isPQ {
									pq.AddWithOpts(priorityqueue.AddOpts{Priority: ptr.To(100)}, reconcile.Request{NamespacedName: types.NamespacedName{
										Namespace: tue.ObjectNew.GetNamespace(),
										Name:      tue.ObjectNew.GetName(),
									}})
									return
								}
								wq.Add(reconcile.Request{NamespacedName: types.NamespacedName{
									Namespace: tue.ObjectNew.GetNamespace(),
									Name:      tue.ObjectNew.GetName(),
								}})
							},
						})
				},
				overridePriority: 100,
			},
		}
		for _, test := range handlerPriorityTests {
			When("handler is "+test.name, func() {
				It("should lower the priority of a create request for an object that was part of the initial list", func(ctx SpecContext) {
					actualOpts := priorityqueue.AddOpts{}
					var actualRequests []reconcile.Request
					wq := &fakePriorityQueue{
						addWithOpts: func(o priorityqueue.AddOpts, items ...reconcile.Request) {
							actualOpts = o
							actualRequests = items
						},
					}

					test.handler().Create(ctx, event.CreateEvent{
						Object: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name:              "my-pod",
							CreationTimestamp: metav1.Now(),
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
						IsInInitialList: true,
					}, wq)

					expected := handler.LowPriority
					if test.overridePriority != 0 {
						expected = test.overridePriority
					}

					Expect(actualOpts).To(Equal(priorityqueue.AddOpts{
						Priority:    ptr.To(expected),
						After:       test.after,
						RateLimited: test.ratelimited,
					}))
					Expect(actualRequests).To(Equal([]reconcile.Request{{NamespacedName: types.NamespacedName{Name: "my-pod"}}}))
				})

				It("should not lower the priority of a create request for an object that was not part of the initial list", func(ctx SpecContext) {
					actualOpts := priorityqueue.AddOpts{}
					var actualRequests []reconcile.Request
					wq := &fakePriorityQueue{
						addWithOpts: func(o priorityqueue.AddOpts, items ...reconcile.Request) {
							actualOpts = o
							actualRequests = items
						},
					}

					test.handler().Create(ctx, event.CreateEvent{
						Object: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name:              "my-pod",
							CreationTimestamp: metav1.Now(),
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
						IsInInitialList: false,
					}, wq)

					var expectedPriority *int
					if test.overridePriority != 0 {
						expectedPriority = &test.overridePriority
					}

					Expect(actualOpts).To(Equal(priorityqueue.AddOpts{After: test.after, RateLimited: test.ratelimited, Priority: expectedPriority}))
					Expect(actualRequests).To(Equal([]reconcile.Request{{NamespacedName: types.NamespacedName{Name: "my-pod"}}}))
				})

				It("should lower the priority of an update request with unchanged RV", func(ctx SpecContext) {
					actualOpts := priorityqueue.AddOpts{}
					var actualRequests []reconcile.Request
					wq := &fakePriorityQueue{
						addWithOpts: func(o priorityqueue.AddOpts, items ...reconcile.Request) {
							actualOpts = o
							actualRequests = items
						},
					}

					test.handler().Update(ctx, event.UpdateEvent{
						ObjectOld: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name: "my-pod",
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
						ObjectNew: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name: "my-pod",
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
					}, wq)

					expectedPriority := handler.LowPriority
					if test.overridePriority != 0 {
						expectedPriority = test.overridePriority
					}

					Expect(actualOpts).To(Equal(priorityqueue.AddOpts{After: test.after, RateLimited: test.ratelimited, Priority: ptr.To(expectedPriority)}))
					Expect(actualRequests).To(Equal([]reconcile.Request{{NamespacedName: types.NamespacedName{Name: "my-pod"}}}))
				})

				It("should not lower the priority of an update request with changed RV", func(ctx SpecContext) {
					actualOpts := priorityqueue.AddOpts{}
					var actualRequests []reconcile.Request
					wq := &fakePriorityQueue{
						addWithOpts: func(o priorityqueue.AddOpts, items ...reconcile.Request) {
							actualOpts = o
							actualRequests = items
						},
					}

					test.handler().Update(ctx, event.UpdateEvent{
						ObjectOld: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name: "my-pod",
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
						ObjectNew: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name:            "my-pod",
							ResourceVersion: "1",
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
					}, wq)

					var expectedPriority *int
					if test.overridePriority != 0 {
						expectedPriority = &test.overridePriority
					}
					Expect(actualOpts).To(Equal(priorityqueue.AddOpts{After: test.after, RateLimited: test.ratelimited, Priority: expectedPriority}))
					Expect(actualRequests).To(Equal([]reconcile.Request{{NamespacedName: types.NamespacedName{Name: "my-pod"}}}))
				})

				It("should have no effect on create if the workqueue is not a priorityqueue", func(ctx SpecContext) {
					test.handler().Create(ctx, event.CreateEvent{
						Object: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name: "my-pod",
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
					}, q)

					Expect(q.Len()).To(Equal(1))
					item, _ := q.Get()
					Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: "my-pod"}}))
				})

				It("should have no effect on Update if the workqueue is not a priorityqueue", func(ctx SpecContext) {
					test.handler().Update(ctx, event.UpdateEvent{
						ObjectOld: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name: "my-pod",
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
						ObjectNew: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
							Name: "my-pod",
							OwnerReferences: []metav1.OwnerReference{{
								Kind: "Pod",
								Name: "my-pod",
							}},
						}},
					}, q)

					Expect(q.Len()).To(Equal(1))
					item, _ := q.Get()
					Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: "my-pod"}}))
				})
			})
		}
	})
})

type fakePriorityQueue struct {
	workqueue.TypedRateLimitingInterface[reconcile.Request]
	addWithOpts func(o priorityqueue.AddOpts, items ...reconcile.Request)
}

func (f *fakePriorityQueue) Add(item reconcile.Request) {
	f.AddWithOpts(priorityqueue.AddOpts{}, item)
}

func (f *fakePriorityQueue) AddWithOpts(o priorityqueue.AddOpts, items ...reconcile.Request) {
	f.addWithOpts(o, items...)
}
func (f *fakePriorityQueue) GetWithPriority() (item reconcile.Request, priority int, shutdown bool) {
	panic("GetWithPriority is not expected to be called")
}

// customHandler re-implements the basic enqueueRequestForObject logic
// to be able to test the WithLowPriorityWhenUnchanged wrapper
type customHandler struct{}

func (ch customHandler) Create(ctx context.Context, evt event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: evt.Object.GetNamespace(),
		Name:      evt.Object.GetName(),
	}})
}
func (ch customHandler) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: evt.ObjectNew.GetNamespace(),
		Name:      evt.ObjectNew.GetName(),
	}})
}
func (ch customHandler) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: evt.Object.GetNamespace(),
		Name:      evt.Object.GetName(),
	}})
}
func (ch customHandler) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: evt.Object.GetNamespace(),
		Name:      evt.Object.GetName(),
	}})
}
