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

package controller_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/priorityqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	internalcontroller "sigs.k8s.io/controller-runtime/pkg/internal/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ = Describe("controller.Controller", func() {
	rec := reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
		return reconcile.Result{}, nil
	})

	Describe("New", func() {
		It("should return an error if Name is not Specified", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())
			c, err := controller.New("", m, controller.Options{Reconciler: rec})
			Expect(c).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("must specify Name for Controller"))
		})

		It("should return an error if Reconciler is not Specified", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("foo", m, controller.Options{})
			Expect(c).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("must specify Reconciler"))
		})

		It("should return an error if two controllers are registered with the same name", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c1, err := controller.New("c3", m, controller.Options{Reconciler: rec})
			Expect(err).NotTo(HaveOccurred())
			Expect(c1).ToNot(BeNil())

			c2, err := controller.New("c3", m, controller.Options{Reconciler: rec})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("controller with name c3 already exists"))
			Expect(c2).To(BeNil())
		})

		It("should return an error if two controllers are registered with the same name and SkipNameValidation is set to false on the manager", func() {
			m, err := manager.New(cfg, manager.Options{
				Controller: config.Controller{
					SkipNameValidation: ptr.To(false),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			c1, err := controller.New("c4", m, controller.Options{Reconciler: rec})
			Expect(err).NotTo(HaveOccurred())
			Expect(c1).ToNot(BeNil())

			c2, err := controller.New("c4", m, controller.Options{Reconciler: rec})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("controller with name c4 already exists"))
			Expect(c2).To(BeNil())
		})

		It("should not return an error if two controllers are registered with the same name and SkipNameValidation is set on the manager", func() {
			m, err := manager.New(cfg, manager.Options{
				Controller: config.Controller{
					SkipNameValidation: ptr.To(true),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			c1, err := controller.New("c5", m, controller.Options{Reconciler: rec})
			Expect(err).NotTo(HaveOccurred())
			Expect(c1).ToNot(BeNil())

			c2, err := controller.New("c5", m, controller.Options{Reconciler: rec})
			Expect(err).NotTo(HaveOccurred())
			Expect(c2).ToNot(BeNil())
		})

		It("should not return an error if two controllers are registered with the same name and SkipNameValidation is set on the controller", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c1, err := controller.New("c6", m, controller.Options{Reconciler: rec})
			Expect(err).NotTo(HaveOccurred())
			Expect(c1).ToNot(BeNil())

			c2, err := controller.New("c6", m, controller.Options{Reconciler: rec, SkipNameValidation: ptr.To(true)})
			Expect(err).NotTo(HaveOccurred())
			Expect(c2).ToNot(BeNil())
		})

		It("should not return an error if two controllers are registered with different names", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c1, err := controller.New("c1", m, controller.Options{Reconciler: rec})
			Expect(err).NotTo(HaveOccurred())
			Expect(c1).ToNot(BeNil())

			c2, err := controller.New("c2", m, controller.Options{Reconciler: rec})
			Expect(err).NotTo(HaveOccurred())
			Expect(c2).ToNot(BeNil())
		})

		It("should not leak goroutines when stopped", func(specCtx SpecContext) {
			currentGRs := goleak.IgnoreCurrent()

			ctx, cancel := context.WithCancel(specCtx)
			watchChan := make(chan event.GenericEvent, 1)
			watch := source.Channel(watchChan, &handler.EnqueueRequestForObject{})
			watchChan <- event.GenericEvent{Object: &corev1.Pod{}}

			reconcileStarted := make(chan struct{})
			controllerFinished := make(chan struct{})
			rec := reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
				defer GinkgoRecover()
				close(reconcileStarted)
				// Make sure reconciliation takes a moment and is not quicker than the controllers
				// shutdown.
				time.Sleep(50 * time.Millisecond)
				// Explicitly test this on top of the leakdetection, as the latter uses Eventually
				// so might succeed even when the controller does not wait for all reconciliations
				// to finish.
				Expect(controllerFinished).NotTo(BeClosed())
				return reconcile.Result{}, nil
			})

			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-0", m, controller.Options{Reconciler: rec})
			Expect(c.Watch(watch)).To(Succeed())
			Expect(err).NotTo(HaveOccurred())

			go func() {
				defer GinkgoRecover()
				Expect(m.Start(ctx)).To(Succeed())
				close(controllerFinished)
			}()

			<-reconcileStarted
			cancel()
			<-controllerFinished

			// force-close keep-alive connections.  These'll time anyway (after
			// like 30s or so) but force it to speed up the tests.
			clientTransport.CloseIdleConnections()
			Eventually(func() error { return goleak.Find(currentGRs) }).Should(Succeed())
		})

		It("should not create goroutines if never started", func() {
			currentGRs := goleak.IgnoreCurrent()

			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			_, err = controller.New("new-controller-1", m, controller.Options{Reconciler: rec})
			Expect(err).NotTo(HaveOccurred())

			// force-close keep-alive connections.  These'll time anyway (after
			// like 30s or so) but force it to speed up the tests.
			clientTransport.CloseIdleConnections()
			Eventually(func() error { return goleak.Find(currentGRs) }).Should(Succeed())
		})

		It("should default RateLimiter and NewQueue if not specified", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-2", m, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.RateLimiter).NotTo(BeNil())
			Expect(ctrl.NewQueue).NotTo(BeNil())
		})

		It("should not override RateLimiter and NewQueue if specified", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			customRateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](5*time.Millisecond, 1000*time.Second)
			customNewQueueCalled := false
			customNewQueue := func(controllerName string, rateLimiter workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				customNewQueueCalled = true
				return nil
			}

			c, err := controller.New("new-controller-3", m, controller.Options{
				Reconciler:  reconcile.Func(nil),
				RateLimiter: customRateLimiter,
				NewQueue:    customNewQueue,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.RateLimiter).To(BeIdenticalTo(customRateLimiter))
			ctrl.NewQueue("controller1", nil)
			Expect(customNewQueueCalled).To(BeTrue(), "Expected customNewQueue to be called")
		})

		It("should default RecoverPanic from the manager", func() {
			m, err := manager.New(cfg, manager.Options{Controller: config.Controller{RecoverPanic: ptr.To(true)}})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-4", m, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.RecoverPanic).NotTo(BeNil())
			Expect(*ctrl.RecoverPanic).To(BeTrue())
		})

		It("should not override RecoverPanic on the controller", func() {
			m, err := manager.New(cfg, manager.Options{Controller: config.Controller{RecoverPanic: ptr.To(true)}})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller", m, controller.Options{
				RecoverPanic: ptr.To(false),
				Reconciler:   reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.RecoverPanic).NotTo(BeNil())
			Expect(*ctrl.RecoverPanic).To(BeFalse())
		})

		It("should default NeedLeaderElection from the manager", func() {
			m, err := manager.New(cfg, manager.Options{Controller: config.Controller{NeedLeaderElection: ptr.To(true)}})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-5", m, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.NeedLeaderElection()).To(BeTrue())
		})

		It("should not override NeedLeaderElection on the controller", func() {
			m, err := manager.New(cfg, manager.Options{Controller: config.Controller{NeedLeaderElection: ptr.To(true)}})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-6", m, controller.Options{
				NeedLeaderElection: ptr.To(false),
				Reconciler:         reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.NeedLeaderElection()).To(BeFalse())
		})

		It("Should default MaxConcurrentReconciles from the manager if set", func() {
			m, err := manager.New(cfg, manager.Options{Controller: config.Controller{MaxConcurrentReconciles: 5}})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-7", m, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.MaxConcurrentReconciles).To(BeEquivalentTo(5))
		})

		It("Should default MaxConcurrentReconciles to 1 if unset", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-8", m, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.MaxConcurrentReconciles).To(BeEquivalentTo(1))
		})

		It("Should leave MaxConcurrentReconciles if set", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-9", m, controller.Options{
				Reconciler:              reconcile.Func(nil),
				MaxConcurrentReconciles: 5,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.MaxConcurrentReconciles).To(BeEquivalentTo(5))
		})

		It("Should default CacheSyncTimeout from the manager if set", func() {
			m, err := manager.New(cfg, manager.Options{Controller: config.Controller{CacheSyncTimeout: 5}})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-10", m, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.CacheSyncTimeout).To(BeEquivalentTo(5))
		})

		It("Should default CacheSyncTimeout to 2 minutes if unset", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-11", m, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.CacheSyncTimeout).To(BeEquivalentTo(2 * time.Minute))
		})

		It("Should leave CacheSyncTimeout if set", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-12", m, controller.Options{
				Reconciler:       reconcile.Func(nil),
				CacheSyncTimeout: 5,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.CacheSyncTimeout).To(BeEquivalentTo(5))
		})

		It("should default NeedLeaderElection on the controller to true", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-13", m, controller.Options{
				Reconciler: rec,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.NeedLeaderElection()).To(BeTrue())
		})

		It("should allow for setting leaderElected to false", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-14", m, controller.Options{
				NeedLeaderElection: ptr.To(false),
				Reconciler:         rec,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.NeedLeaderElection()).To(BeFalse())
		})

		It("should implement manager.LeaderElectionRunnable", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-15", m, controller.Options{
				Reconciler: rec,
			})
			Expect(err).NotTo(HaveOccurred())

			_, ok := c.(manager.LeaderElectionRunnable)
			Expect(ok).To(BeTrue())
		})

		It("should configure a priority queue if UsePriorityQueue is set", func() {
			m, err := manager.New(cfg, manager.Options{
				Controller: config.Controller{UsePriorityQueue: ptr.To(true)},
			})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-16", m, controller.Options{
				Reconciler: rec,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			q := ctrl.NewQueue("foo", nil)
			_, ok = q.(priorityqueue.PriorityQueue[reconcile.Request])
			Expect(ok).To(BeTrue())
		})

		It("should not configure a priority queue if UsePriorityQueue is not set", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("new-controller-17", m, controller.Options{
				Reconciler: rec,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			q := ctrl.NewQueue("foo", nil)
			_, ok = q.(priorityqueue.PriorityQueue[reconcile.Request])
			Expect(ok).To(BeFalse())
		})

		It("should set EnableWarmup correctly", func() {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			// Test with EnableWarmup set to true
			ctrlWithWarmup, err := controller.New("warmup-enabled-ctrl", m, controller.Options{
				Reconciler:   reconcile.Func(nil),
				EnableWarmup: ptr.To(true),
			})
			Expect(err).NotTo(HaveOccurred())

			internalCtrlWithWarmup, ok := ctrlWithWarmup.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())
			Expect(internalCtrlWithWarmup.EnableWarmup).To(HaveValue(BeTrue()))

			// Test with EnableWarmup set to false
			ctrlWithoutWarmup, err := controller.New("warmup-disabled-ctrl", m, controller.Options{
				Reconciler:   reconcile.Func(nil),
				EnableWarmup: ptr.To(false),
			})
			Expect(err).NotTo(HaveOccurred())

			internalCtrlWithoutWarmup, ok := ctrlWithoutWarmup.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())
			Expect(internalCtrlWithoutWarmup.EnableWarmup).To(HaveValue(BeFalse()))

			// Test with EnableWarmup not set (should default to nil)
			ctrlWithDefaultWarmup, err := controller.New("warmup-default-ctrl", m, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			internalCtrlWithDefaultWarmup, ok := ctrlWithDefaultWarmup.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())
			Expect(internalCtrlWithDefaultWarmup.EnableWarmup).To(BeNil())
		})

		It("should inherit EnableWarmup from manager config", func() {
			// Test with manager default setting EnableWarmup to true
			managerWithWarmup, err := manager.New(cfg, manager.Options{
				Controller: config.Controller{
					EnableWarmup: ptr.To(true),
				},
			})
			Expect(err).NotTo(HaveOccurred())
			ctrlInheritingWarmup, err := controller.New("inherit-warmup-enabled", managerWithWarmup, controller.Options{
				Reconciler: reconcile.Func(nil),
			})
			Expect(err).NotTo(HaveOccurred())

			internalCtrlInheritingWarmup, ok := ctrlInheritingWarmup.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())
			Expect(internalCtrlInheritingWarmup.EnableWarmup).To(HaveValue(BeTrue()))

			// Test that explicit controller setting overrides manager setting
			ctrlOverridingWarmup, err := controller.New("override-warmup-disabled", managerWithWarmup, controller.Options{
				Reconciler:   reconcile.Func(nil),
				EnableWarmup: ptr.To(false),
			})
			Expect(err).NotTo(HaveOccurred())

			internalCtrlOverridingWarmup, ok := ctrlOverridingWarmup.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())
			Expect(internalCtrlOverridingWarmup.EnableWarmup).To(HaveValue(BeFalse()))
		})

		It("should default ReconciliationTimeout from manager if unset", func() {
			m, err := manager.New(cfg, manager.Options{
				Controller: config.Controller{ReconciliationTimeout: 30 * time.Second},
			})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("mgr-reconciliation-timeout", m, controller.Options{
				Reconciler: rec,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.ReconciliationTimeout).To(Equal(30 * time.Second))
		})

		It("should not override an existing ReconciliationTimeout", func() {
			m, err := manager.New(cfg, manager.Options{
				Controller: config.Controller{ReconciliationTimeout: 30 * time.Second},
			})
			Expect(err).NotTo(HaveOccurred())

			c, err := controller.New("ctrl-reconciliation-timeout", m, controller.Options{
				Reconciler:            rec,
				ReconciliationTimeout: time.Minute,
			})
			Expect(err).NotTo(HaveOccurred())

			ctrl, ok := c.(*internalcontroller.Controller[reconcile.Request])
			Expect(ok).To(BeTrue())

			Expect(ctrl.ReconciliationTimeout).To(Equal(time.Minute))
		})
	})
})
