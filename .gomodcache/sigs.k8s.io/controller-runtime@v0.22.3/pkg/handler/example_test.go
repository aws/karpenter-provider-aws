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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	mgr manager.Manager
	c   controller.Controller
)

// This example watches Pods and enqueues Requests with the Name and Namespace of the Pod from
// the Event (i.e. change caused by a Create, Update, Delete).
func ExampleEnqueueRequestForObject() {
	// controller is a controller.controller
	err := c.Watch(
		source.Kind(mgr.GetCache(), &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}),
	)
	if err != nil {
		// handle it
	}
}

// This example watches ReplicaSets and enqueues a Request containing the Name and Namespace of the
// owning (direct) Deployment responsible for the creation of the ReplicaSet.
func ExampleEnqueueRequestForOwner() {
	// controller is a controller.controller
	err := c.Watch(
		source.Kind(mgr.GetCache(),
			&appsv1.ReplicaSet{},
			handler.TypedEnqueueRequestForOwner[*appsv1.ReplicaSet](mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1.Deployment{}, handler.OnlyControllerOwner()),
		),
	)
	if err != nil {
		// handle it
	}
}

// This example watches Deployments and enqueues a Request contain the Name and Namespace of different
// objects (of Type: MyKind) using a mapping function defined by the user.
func ExampleEnqueueRequestsFromMapFunc() {
	// controller is a controller.controller
	err := c.Watch(
		source.Kind(mgr.GetCache(), &appsv1.Deployment{},
			handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, a *appsv1.Deployment) []reconcile.Request {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      a.Name + "-1",
						Namespace: a.Namespace,
					}},
					{NamespacedName: types.NamespacedName{
						Name:      a.Name + "-2",
						Namespace: a.Namespace,
					}},
				}
			}),
		),
	)
	if err != nil {
		// handle it
	}
}

// This example implements handler.EnqueueRequestForObject.
func ExampleFuncs() {
	// controller is a controller.controller
	err := c.Watch(
		source.Kind(mgr.GetCache(), &corev1.Pod{},
			handler.TypedFuncs[*corev1.Pod, reconcile.Request]{
				CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*corev1.Pod], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      e.Object.Name,
						Namespace: e.Object.Namespace,
					}})
				},
				UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*corev1.Pod], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      e.ObjectNew.Name,
						Namespace: e.ObjectNew.Namespace,
					}})
				},
				DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[*corev1.Pod], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      e.Object.Name,
						Namespace: e.Object.Namespace,
					}})
				},
				GenericFunc: func(ctx context.Context, e event.TypedGenericEvent[*corev1.Pod], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      e.Object.Name,
						Namespace: e.Object.Namespace,
					}})
				},
			},
		),
	)
	if err != nil {
		// handle it
	}
}
