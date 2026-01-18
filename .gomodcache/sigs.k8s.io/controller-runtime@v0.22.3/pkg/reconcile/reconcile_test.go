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

package reconcile_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type mockObjectReconciler struct {
	reconcileFunc func(context.Context, *corev1.ConfigMap) (reconcile.Result, error)
}

func (r *mockObjectReconciler) Reconcile(ctx context.Context, cm *corev1.ConfigMap) (reconcile.Result, error) {
	return r.reconcileFunc(ctx, cm)
}

var _ = Describe("reconcile", func() {
	Describe("Result", func() {
		It("IsZero should return true if empty", func() {
			var res *reconcile.Result
			Expect(res.IsZero()).To(BeTrue())
			res2 := &reconcile.Result{}
			Expect(res2.IsZero()).To(BeTrue())
			res3 := reconcile.Result{}
			Expect(res3.IsZero()).To(BeTrue())
		})

		It("IsZero should return false if Requeue is set to true", func() {
			res := reconcile.Result{Requeue: true}
			Expect(res.IsZero()).To(BeFalse())
		})

		It("IsZero should return false if RequeueAfter is set to true", func() {
			res := reconcile.Result{RequeueAfter: 1 * time.Second}
			Expect(res.IsZero()).To(BeFalse())
		})
	})

	Describe("Func", func() {
		It("should call the function with the request and return a nil error.", func(ctx SpecContext) {
			request := reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "foo", Namespace: "bar"},
			}
			result := reconcile.Result{
				Requeue: true,
			}

			instance := reconcile.Func(func(_ context.Context, r reconcile.Request) (reconcile.Result, error) {
				defer GinkgoRecover()
				Expect(r).To(Equal(request))

				return result, nil
			})
			actualResult, actualErr := instance.Reconcile(ctx, request)
			Expect(actualResult).To(Equal(result))
			Expect(actualErr).NotTo(HaveOccurred())
		})

		It("should call the function with the request and return an error.", func(ctx SpecContext) {
			request := reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "foo", Namespace: "bar"},
			}
			result := reconcile.Result{
				Requeue: false,
			}
			err := fmt.Errorf("hello world")

			instance := reconcile.Func(func(_ context.Context, r reconcile.Request) (reconcile.Result, error) {
				defer GinkgoRecover()
				Expect(r).To(Equal(request))

				return result, err
			})
			actualResult, actualErr := instance.Reconcile(ctx, request)
			Expect(actualResult).To(Equal(result))
			Expect(actualErr).To(Equal(err))
		})

		It("should allow unwrapping inner error from terminal error", func() {
			inner := apierrors.NewGone("")
			terminalError := reconcile.TerminalError(inner)

			Expect(apierrors.IsGone(terminalError)).To(BeTrue())
		})

		It("should handle nil terminal errors properly", func() {
			err := reconcile.TerminalError(nil)
			Expect(err.Error()).To(Equal("nil terminal error"))
		})
	})

	Describe("AsReconciler", func() {
		var testenv *envtest.Environment
		var testClient client.Client

		BeforeEach(func() {
			testenv = &envtest.Environment{}

			cfg, err := testenv.Start()
			Expect(err).NotTo(HaveOccurred())

			testClient, err = client.New(cfg, client.Options{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(testenv.Stop()).NotTo(HaveOccurred())
		})

		Context("with an existing object", func() {
			var key client.ObjectKey

			BeforeEach(func(ctx SpecContext) {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test",
					},
				}
				key = client.ObjectKeyFromObject(cm)

				err := testClient.Create(ctx, cm)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should Get the object and call the ObjectReconciler", func(ctx SpecContext) {
				var actual *corev1.ConfigMap
				reconciler := reconcile.AsReconciler(testClient, &mockObjectReconciler{
					reconcileFunc: func(ctx context.Context, cm *corev1.ConfigMap) (reconcile.Result, error) {
						actual = cm
						return reconcile.Result{}, nil
					},
				})

				res, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(BeZero())
				Expect(actual).NotTo(BeNil())
				Expect(actual.ObjectMeta.Name).To(Equal(key.Name))
				Expect(actual.ObjectMeta.Namespace).To(Equal(key.Namespace))
			})
		})

		Context("with an object that doesn't exist", func() {
			It("should not call the ObjectReconciler", func(ctx SpecContext) {
				called := false
				reconciler := reconcile.AsReconciler(testClient, &mockObjectReconciler{
					reconcileFunc: func(ctx context.Context, cm *corev1.ConfigMap) (reconcile.Result, error) {
						called = true
						return reconcile.Result{}, nil
					},
				})

				key := types.NamespacedName{Namespace: "default", Name: "fake-obj"}
				res, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(BeZero())
				Expect(called).To(BeFalse())
			})
		})
	})
})
