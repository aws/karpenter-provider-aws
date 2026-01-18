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

package controller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/goleak"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/controller/priorityqueue"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/internal/controller/metrics"
	"sigs.k8s.io/controller-runtime/pkg/internal/log"
	"sigs.k8s.io/controller-runtime/pkg/leaderelection"
	fakeleaderelection "sigs.k8s.io/controller-runtime/pkg/leaderelection/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type TestRequest struct {
	Key string
}

const testControllerName = "testcontroller"

var _ = Describe("controller", func() {
	var fakeReconcile *fakeReconciler
	var ctrl *Controller[reconcile.Request]
	var queue *controllertest.Queue
	var reconciled chan reconcile.Request
	var request = reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"},
	}

	BeforeEach(func() {
		reconciled = make(chan reconcile.Request)
		fakeReconcile = &fakeReconciler{
			Requests: reconciled,
			results:  make(chan fakeReconcileResultPair, 10 /* chosen by the completely scientific approach of guessing */),
		}
		queue = &controllertest.Queue{
			TypedInterface: workqueue.NewTyped[reconcile.Request](),
		}
		ctrl = New[reconcile.Request](Options[reconcile.Request]{
			MaxConcurrentReconciles: 1,
			Do:                      fakeReconcile,
			NewQueue: func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				return queue
			},
			LogConstructor: func(_ *reconcile.Request) logr.Logger {
				return log.RuntimeLog.WithName("controller").WithName("test")
			},
		})
	})

	Describe("Reconciler", func() {
		It("should call the Reconciler function", func(ctx SpecContext) {
			ctrl.Do = reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
				return reconcile.Result{Requeue: true}, nil
			})
			result, err := ctrl.Reconcile(ctx,
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{Requeue: true}))
		})

		It("should not recover panic if RecoverPanic is false", func(ctx SpecContext) {
			defer func() {
				Expect(recover()).ShouldNot(BeNil())
			}()
			ctrl.RecoverPanic = ptr.To(false)
			ctrl.Do = reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
				var res *reconcile.Result
				return *res, nil
			})
			_, _ = ctrl.Reconcile(ctx,
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})
		})

		It("should recover panic if RecoverPanic is true by default", func(ctx SpecContext) {
			defer func() {
				Expect(recover()).To(BeNil())
			}()
			// RecoverPanic defaults to true.
			ctrl.Do = reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
				var res *reconcile.Result
				return *res, nil
			})
			_, err := ctrl.Reconcile(ctx,
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("[recovered]"))
		})

		It("should recover panic if RecoverPanic is true", func(ctx SpecContext) {
			defer func() {
				Expect(recover()).To(BeNil())
			}()
			ctrl.RecoverPanic = ptr.To(true)
			ctrl.Do = reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
				var res *reconcile.Result
				return *res, nil
			})
			_, err := ctrl.Reconcile(ctx,
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("[recovered]"))
		})

		It("should time out if ReconciliationTimeout is set", func(ctx SpecContext) {
			ctrl.ReconciliationTimeout = time.Duration(1) // One nanosecond
			ctrl.Do = reconcile.Func(func(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
				<-ctx.Done()
				return reconcile.Result{}, ctx.Err()
			})
			_, err := ctrl.Reconcile(ctx,
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(context.DeadlineExceeded))
		})

		It("should not configure a timeout if ReconciliationTimeout is zero", func(ctx SpecContext) {
			ctrl.Do = reconcile.Func(func(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
				defer GinkgoRecover()

				_, ok := ctx.Deadline()
				Expect(ok).To(BeFalse())
				return reconcile.Result{}, nil
			})
			_, err := ctrl.Reconcile(ctx,
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Start", func() {
		It("should return an error if there is an error waiting for the informers", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = time.Second
			f := false
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Kind(&informertest.FakeInformers{Synced: &f}, &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}),
			}
			ctrl.Name = "foo"
			err := ctrl.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to wait for foo caches to sync"))
		})

		It("should error when cache sync timeout occurs", func(ctx SpecContext) {
			c, err := cache.New(cfg, cache.Options{})
			Expect(err).NotTo(HaveOccurred())
			c = &cacheWithIndefinitelyBlockingGetInformer{c}

			ctrl.CacheSyncTimeout = time.Second
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Kind(c, &appsv1.Deployment{}, &handler.TypedEnqueueRequestForObject[*appsv1.Deployment]{}),
			}
			ctrl.Name = testControllerName

			err = ctrl.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to wait for testcontroller caches to sync kind source: *v1.Deployment: timed out waiting for cache to be synced"))
		})

		It("should not error when controller Start context is cancelled during Sources WaitForSync", func(specCtx SpecContext) {
			ctrl.CacheSyncTimeout = 1 * time.Second

			sourceSynced := make(chan struct{})
			c, err := cache.New(cfg, cache.Options{})
			Expect(err).NotTo(HaveOccurred())
			c = &cacheWithIndefinitelyBlockingGetInformer{c}
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				&singnallingSourceWrapper{
					SyncingSource: source.Kind[client.Object](c, &appsv1.Deployment{}, &handler.EnqueueRequestForObject{}),
					cacheSyncDone: sourceSynced,
				},
			}
			ctrl.Name = testControllerName

			ctx, cancel := context.WithCancel(specCtx)
			go func() {
				defer GinkgoRecover()
				err = ctrl.Start(ctx)
				Expect(err).To(Succeed())
			}()

			cancel()
			<-sourceSynced
		})

		It("should error when Start() is blocking forever", func(specCtx SpecContext) {
			ctrl.CacheSyncTimeout = time.Second

			controllerDone := make(chan struct{})
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					<-controllerDone
					return ctx.Err()
				})}

			ctx, cancel := context.WithTimeout(specCtx, 10*time.Second)
			defer cancel()

			err := ctrl.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Please ensure that its Start() method is non-blocking"))

			close(controllerDone)
		})

		It("should not error when cache sync timeout is of sufficiently high", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second

			sourceSynced := make(chan struct{})
			c := &informertest.FakeInformers{}
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				&singnallingSourceWrapper{
					SyncingSource: source.Kind[client.Object](c, &appsv1.Deployment{}, &handler.EnqueueRequestForObject{}),
					cacheSyncDone: sourceSynced,
				},
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).To(Succeed())
			}()

			<-sourceSynced
		})

		It("should process events from source.Channel", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second
			// channel to be closed when event is processed
			processed := make(chan struct{})
			// source channel
			ch := make(chan event.GenericEvent, 1)

			// event to be sent to the channel
			p := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"},
			}
			evt := event.GenericEvent{
				Object: p,
			}

			ins := source.Channel(
				ch,
				handler.Funcs{
					GenericFunc: func(ctx context.Context, evt event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						close(processed)
					},
				},
			)

			// send the event to the channel
			ch <- evt

			ctrl.startWatches = []source.TypedSource[reconcile.Request]{ins}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).To(Succeed())
			}()
			<-processed
		})

		It("should error when channel source is not specified", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second

			ins := source.Channel[string](nil, nil)
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{ins}

			e := ctrl.Start(ctx)
			Expect(e).To(HaveOccurred())
			Expect(e.Error()).To(ContainSubstring("must specify Channel.Source"))
		})

		It("should call Start on sources with the appropriate EventHandler, Queue, and Predicates", func(specCtx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second
			started := false
			ctx, cancel := context.WithCancel(specCtx)
			src := source.Func(func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				defer GinkgoRecover()
				Expect(q).To(Equal(ctrl.Queue))

				started = true
				cancel() // Cancel the context so ctrl.Start() doesn't block forever
				return nil
			})
			Expect(ctrl.Watch(src)).NotTo(HaveOccurred())

			err := ctrl.Start(ctx)
			Expect(err).To(Succeed())
			Expect(started).To(BeTrue())
		})

		It("should return an error if there is an error starting sources", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second
			err := fmt.Errorf("Expected Error: could not start source")
			src := source.Func(func(context.Context,
				workqueue.TypedRateLimitingInterface[reconcile.Request],
			) error {
				defer GinkgoRecover()
				return err
			})
			Expect(ctrl.Watch(src)).To(Succeed())
			Expect(ctrl.Start(ctx)).To(Equal(err))
		})

		It("should return an error if it gets started more than once", func(specCtx SpecContext) {
			// Use a cancelled context so Start doesn't block
			ctx, cancel := context.WithCancel(specCtx)
			cancel()
			Expect(ctrl.Start(ctx)).To(Succeed())
			err := ctrl.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("controller was started more than once. This is likely to be caused by being added to a manager multiple times"))
		})

		It("should check for correct TypedSyncingSource if custom types are used", func(specCtx SpecContext) {
			queue := &priorityQueueWrapper[TestRequest]{
				TypedRateLimitingInterface: &controllertest.TypedQueue[TestRequest]{
					TypedInterface: workqueue.NewTyped[TestRequest](),
				}}
			ctrl := New[TestRequest](Options[TestRequest]{
				NewQueue: func(string, workqueue.TypedRateLimiter[TestRequest]) workqueue.TypedRateLimitingInterface[TestRequest] {
					return queue
				},
				LogConstructor: func(*TestRequest) logr.Logger {
					return log.RuntimeLog.WithName("controller").WithName("test")
				},
			})
			ctrl.CacheSyncTimeout = time.Second
			src := &bisignallingSource[TestRequest]{
				startCall: make(chan workqueue.TypedRateLimitingInterface[TestRequest]),
				startDone: make(chan error, 1),
				waitCall:  make(chan struct{}),
				waitDone:  make(chan error, 1),
			}
			ctrl.startWatches = []source.TypedSource[TestRequest]{src}
			ctrl.Name = "foo"
			ctx, cancel := context.WithCancel(specCtx)
			defer cancel()
			startCh := make(chan error)
			go func() {
				defer GinkgoRecover()
				startCh <- ctrl.Start(ctx)
			}()
			Eventually(src.startCall).Should(Receive(Equal(queue)))
			src.startDone <- nil
			Eventually(src.waitCall).Should(BeClosed())
			src.waitDone <- nil
			cancel()
			Eventually(startCh).Should(Receive(Succeed()))
		})
	})

	Describe("startEventSourcesAndQueueLocked", func() {
		It("should return nil when no sources are provided", func(ctx SpecContext) {
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{}
			err := ctrl.startEventSourcesAndQueueLocked(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should initialize controller queue when called", func(ctx SpecContext) {
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{}
			err := ctrl.startEventSourcesAndQueueLocked(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ctrl.Queue).NotTo(BeNil())
		})

		It("should return an error if a source fails to start", func(ctx SpecContext) {
			expectedErr := fmt.Errorf("failed to start source")
			src := source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				// Return the error immediately so we don't get a timeout
				return expectedErr
			})

			// Set a sufficiently long timeout to avoid timeouts interfering with the error being returned
			ctrl.CacheSyncTimeout = 5 * time.Second
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{src}
			err := ctrl.startEventSourcesAndQueueLocked(ctx)
			Expect(err).To(Equal(expectedErr))
		})

		It("should return an error if a source fails to sync", func(ctx SpecContext) {
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Kind(&informertest.FakeInformers{Synced: ptr.To(false)}, &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}),
			}
			ctrl.Name = "test-controller"
			ctrl.CacheSyncTimeout = 5 * time.Second

			err := ctrl.startEventSourcesAndQueueLocked(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to wait for test-controller caches to sync"))
		})

		It("should not return an error when sources start and sync successfully", func(ctx SpecContext) {
			// Create a source that starts and syncs successfully
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Kind(&informertest.FakeInformers{Synced: ptr.To(true)}, &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}),
			}
			ctrl.Name = "test-controller"
			ctrl.CacheSyncTimeout = 5 * time.Second

			err := ctrl.startEventSourcesAndQueueLocked(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error when context is cancelled during source sync", func(ctx SpecContext) {
			sourceCtx, sourceCancel := context.WithCancel(ctx)
			defer sourceCancel()

			ctrl.CacheSyncTimeout = 5 * time.Second

			// Create a bisignallingSource to control the test flow
			src := &bisignallingSource[reconcile.Request]{
				startCall: make(chan workqueue.TypedRateLimitingInterface[reconcile.Request]),
				startDone: make(chan error, 1),
				waitCall:  make(chan struct{}),
				waitDone:  make(chan error, 1),
			}

			ctrl.startWatches = []source.TypedSource[reconcile.Request]{src}

			// Start the sources in a goroutine
			startErrCh := make(chan error)
			go func() {
				defer GinkgoRecover()
				startErrCh <- ctrl.startEventSourcesAndQueueLocked(sourceCtx)
			}()

			// Allow source to start successfully
			Eventually(src.startCall).Should(Receive())
			src.startDone <- nil

			// Wait for WaitForSync to be called
			Eventually(src.waitCall).Should(BeClosed())

			// Return context.Canceled from WaitForSync
			src.waitDone <- context.Canceled

			// Also cancel the context
			sourceCancel()

			// We expect to receive the context.Canceled error
			err := <-startErrCh
			Expect(err).To(MatchError(context.Canceled))
		})

		It("should timeout if source Start blocks for too long", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 1 * time.Millisecond

			// Create a source that blocks forever in Start
			blockingSrc := source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				<-ctx.Done()
				return ctx.Err()
			})

			ctrl.startWatches = []source.TypedSource[reconcile.Request]{blockingSrc}

			err := ctrl.startEventSourcesAndQueueLocked(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("timed out waiting for source"))
		})

		It("should only start sources once when called multiple times concurrently", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 1 * time.Millisecond

			var startCount atomic.Int32
			src := source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				startCount.Add(1)
				return nil
			})

			ctrl.startWatches = []source.TypedSource[reconcile.Request]{src}

			By("Calling startEventSourcesAndQueueLocked multiple times in parallel")
			var wg sync.WaitGroup
			for i := 1; i <= 5; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := ctrl.startEventSourcesAndQueueLocked(ctx)
					// All calls should return the same nil error
					Expect(err).NotTo(HaveOccurred())
				}()
			}

			wg.Wait()
			Expect(startCount.Load()).To(Equal(int32(1)), "Source should only be started once even when called multiple times")
		})

		It("should block subsequent calls from returning until the first call to startEventSourcesAndQueueLocked has returned", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 5 * time.Second

			// finishSourceChan is closed to unblock startEventSourcesAndQueueLocked from returning
			finishSourceChan := make(chan struct{})

			src := source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				<-finishSourceChan
				return nil
			})
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{src}

			By("Calling startEventSourcesAndQueueLocked asynchronously")
			wg := sync.WaitGroup{}
			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				wg.Add(1)
				Expect(ctrl.startEventSourcesAndQueueLocked(ctx)).To(Succeed())
			}()

			By("Calling startEventSourcesAndQueueLocked again")
			var didSubsequentCallComplete atomic.Bool
			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				wg.Add(1)
				Expect(ctrl.startEventSourcesAndQueueLocked(ctx)).To(Succeed())
				didSubsequentCallComplete.Store(true)
			}()

			// Assert that second call to startEventSourcesAndQueueLocked is blocked while source has not finished
			Consistently(didSubsequentCallComplete.Load).Should(BeFalse())

			By("Finishing source start + sync")
			finishSourceChan <- struct{}{}

			// Assert that second call to startEventSourcesAndQueueLocked is now complete
			Eventually(didSubsequentCallComplete.Load).Should(BeTrue(), "startEventSourcesAndQueueLocked should complete after source is started and synced")
			wg.Wait()
		})

		It("should reset c.startWatches to nil after returning and startedEventSourcesAndQueue", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 1 * time.Millisecond

			src := source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				return nil
			})

			ctrl.startWatches = []source.TypedSource[reconcile.Request]{src}

			err := ctrl.startEventSourcesAndQueueLocked(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ctrl.startWatches).To(BeNil(), "startWatches should be reset to nil after returning")
			Expect(ctrl.startedEventSourcesAndQueue).To(BeTrue(), "startedEventSourcesAndQueue should be set to true after startEventSourcesAndQueueLocked returns without error")
		})
	})

	Describe("Processing queue items from a Controller", func() {
		It("should call Reconciler if an item is enqueued", func(ctx SpecContext) {
			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()
			queue.Add(request)

			By("Invoking Reconciler")
			fakeReconcile.AddResult(reconcile.Result{}, nil)
			Expect(<-reconciled).To(Equal(request))

			By("Removing the item from the queue")
			Eventually(queue.Len).Should(Equal(0))
			Eventually(func() int { return queue.NumRequeues(request) }).Should(Equal(0))
		})

		It("should requeue a Request if there is an error and continue processing items", func(ctx SpecContext) {
			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			queue.Add(request)

			By("Invoking Reconciler which will give an error")
			fakeReconcile.AddResult(reconcile.Result{}, fmt.Errorf("expected error: reconcile"))
			Expect(<-reconciled).To(Equal(request))
			queue.AddedRateLimitedLock.Lock()
			Expect(queue.AddedRatelimited).To(Equal([]any{request}))
			queue.AddedRateLimitedLock.Unlock()

			By("Invoking Reconciler a second time without error")
			fakeReconcile.AddResult(reconcile.Result{}, nil)
			Expect(<-reconciled).To(Equal(request))

			By("Removing the item from the queue")
			Eventually(queue.Len).Should(Equal(0))
			Eventually(func() int { return queue.NumRequeues(request) }, 1.0).Should(Equal(0))
		})

		It("should not requeue a Request if there is a terminal error", func(ctx SpecContext) {
			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			queue.Add(request)

			By("Invoking Reconciler which will give an error")
			fakeReconcile.AddResult(reconcile.Result{}, reconcile.TerminalError(fmt.Errorf("expected error: reconcile")))
			Expect(<-reconciled).To(Equal(request))

			queue.AddedRateLimitedLock.Lock()
			Expect(queue.AddedRatelimited).To(BeEmpty())
			queue.AddedRateLimitedLock.Unlock()

			Expect(queue.Len()).Should(Equal(0))
		})

		// TODO(directxman12): we should ensure that backoff occurrs with error requeue

		It("should not reset backoff until there's a non-error result", func(ctx SpecContext) {
			dq := &DelegatingQueue{TypedRateLimitingInterface: ctrl.NewQueue("controller1", nil)}
			ctrl.NewQueue = func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				return dq
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			dq.Add(request)
			Expect(dq.getCounts()).To(Equal(countInfo{Trying: 1}))

			By("Invoking Reconciler which returns an error")
			fakeReconcile.AddResult(reconcile.Result{}, fmt.Errorf("something's wrong"))
			Expect(<-reconciled).To(Equal(request))
			Eventually(dq.getCounts).Should(Equal(countInfo{Trying: 1, AddRateLimited: 1}))

			By("Invoking Reconciler a second time with an error")
			fakeReconcile.AddResult(reconcile.Result{}, fmt.Errorf("another thing's wrong"))
			Expect(<-reconciled).To(Equal(request))

			Eventually(dq.getCounts).Should(Equal(countInfo{Trying: 1, AddRateLimited: 2}))

			By("Invoking Reconciler a third time, where it finally does not return an error")
			fakeReconcile.AddResult(reconcile.Result{}, nil)
			Expect(<-reconciled).To(Equal(request))

			Eventually(dq.getCounts).Should(Equal(countInfo{Trying: 0, AddRateLimited: 2}))

			By("Removing the item from the queue")
			Eventually(dq.Len).Should(Equal(0))
			Eventually(func() int { return dq.NumRequeues(request) }).Should(Equal(0))
		})

		It("should requeue a Request with rate limiting if the Result sets Requeue:true and continue processing items", func(ctx SpecContext) {
			dq := &DelegatingQueue{TypedRateLimitingInterface: ctrl.NewQueue("controller1", nil)}
			ctrl.NewQueue = func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				return dq
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			dq.Add(request)
			Expect(dq.getCounts()).To(Equal(countInfo{Trying: 1}))

			By("Invoking Reconciler which will ask for requeue")
			fakeReconcile.AddResult(reconcile.Result{Requeue: true}, nil)
			Expect(<-reconciled).To(Equal(request))
			Eventually(dq.getCounts).Should(Equal(countInfo{Trying: 1, AddRateLimited: 1}))

			By("Invoking Reconciler a second time without asking for requeue")
			fakeReconcile.AddResult(reconcile.Result{Requeue: false}, nil)
			Expect(<-reconciled).To(Equal(request))

			Eventually(dq.getCounts).Should(Equal(countInfo{Trying: 0, AddRateLimited: 1}))

			By("Removing the item from the queue")
			Eventually(dq.Len).Should(Equal(0))
			Eventually(func() int { return dq.NumRequeues(request) }).Should(Equal(0))
		})

		It("should retain the priority when the reconciler requests a requeue", func(ctx SpecContext) {
			q := &fakePriorityQueue{PriorityQueue: priorityqueue.New[reconcile.Request]("controller1")}
			ctrl.NewQueue = func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				return q
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			q.PriorityQueue.AddWithOpts(priorityqueue.AddOpts{Priority: ptr.To(10)}, request)

			By("Invoking Reconciler which will request a requeue")
			fakeReconcile.AddResult(reconcile.Result{Requeue: true}, nil)
			Expect(<-reconciled).To(Equal(request))
			Eventually(func() []priorityQueueAddition {
				q.lock.Lock()
				defer q.lock.Unlock()
				return q.added
			}).Should(Equal([]priorityQueueAddition{{
				AddOpts: priorityqueue.AddOpts{
					RateLimited: true,
					Priority:    ptr.To(10),
				},
				items: []reconcile.Request{request},
			}}))
		})

		It("should requeue a Request after a duration (but not rate-limitted) if the Result sets RequeueAfter (regardless of Requeue)", func(ctx SpecContext) {
			dq := &DelegatingQueue{TypedRateLimitingInterface: ctrl.NewQueue("controller1", nil)}
			ctrl.NewQueue = func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				return dq
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			dq.Add(request)
			Expect(dq.getCounts()).To(Equal(countInfo{Trying: 1}))

			By("Invoking Reconciler which will ask for requeue & requeueafter")
			fakeReconcile.AddResult(reconcile.Result{RequeueAfter: time.Millisecond * 100, Requeue: true}, nil)
			Expect(<-reconciled).To(Equal(request))
			Eventually(dq.getCounts).Should(Equal(countInfo{Trying: 0, AddAfter: 1}))

			By("Invoking Reconciler a second time asking for a requeueafter only")
			fakeReconcile.AddResult(reconcile.Result{RequeueAfter: time.Millisecond * 100}, nil)
			Expect(<-reconciled).To(Equal(request))

			Eventually(dq.getCounts).Should(Equal(countInfo{Trying: -1 /* we don't increment the count in addafter */, AddAfter: 2}))

			By("Removing the item from the queue")
			Eventually(dq.Len).Should(Equal(0))
			Eventually(func() int { return dq.NumRequeues(request) }).Should(Equal(0))
		})

		It("should retain the priority with RequeAfter", func(ctx SpecContext) {
			q := &fakePriorityQueue{PriorityQueue: priorityqueue.New[reconcile.Request]("controller1")}
			ctrl.NewQueue = func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				return q
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			q.PriorityQueue.AddWithOpts(priorityqueue.AddOpts{Priority: ptr.To(10)}, request)

			By("Invoking Reconciler which will ask for RequeueAfter")
			fakeReconcile.AddResult(reconcile.Result{RequeueAfter: time.Millisecond * 100}, nil)
			Expect(<-reconciled).To(Equal(request))
			Eventually(func() []priorityQueueAddition {
				q.lock.Lock()
				defer q.lock.Unlock()
				return q.added
			}).Should(Equal([]priorityQueueAddition{{
				AddOpts: priorityqueue.AddOpts{
					After:    time.Millisecond * 100,
					Priority: ptr.To(10),
				},
				items: []reconcile.Request{request},
			}}))
		})

		It("should perform error behavior if error is not nil, regardless of RequeueAfter", func(ctx SpecContext) {
			dq := &DelegatingQueue{TypedRateLimitingInterface: ctrl.NewQueue("controller1", nil)}
			ctrl.NewQueue = func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				return dq
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			dq.Add(request)
			Expect(dq.getCounts()).To(Equal(countInfo{Trying: 1}))

			By("Invoking Reconciler which will ask for requeueafter with an error")
			fakeReconcile.AddResult(reconcile.Result{RequeueAfter: time.Millisecond * 100}, fmt.Errorf("expected error: reconcile"))
			Expect(<-reconciled).To(Equal(request))
			Eventually(dq.getCounts).Should(Equal(countInfo{Trying: 1, AddRateLimited: 1}))

			By("Invoking Reconciler a second time asking for requeueafter without errors")
			fakeReconcile.AddResult(reconcile.Result{RequeueAfter: time.Millisecond * 100}, nil)
			Expect(<-reconciled).To(Equal(request))
			Eventually(dq.getCounts).Should(Equal(countInfo{AddAfter: 1, AddRateLimited: 1}))

			By("Removing the item from the queue")
			Eventually(dq.Len).Should(Equal(0))
			Eventually(func() int { return dq.NumRequeues(request) }).Should(Equal(0))
		})

		It("should retain the priority when there was an error", func(ctx SpecContext) {
			q := &fakePriorityQueue{PriorityQueue: priorityqueue.New[reconcile.Request]("controller1")}
			ctrl.NewQueue = func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
				return q
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
			}()

			q.PriorityQueue.AddWithOpts(priorityqueue.AddOpts{Priority: ptr.To(10)}, request)

			By("Invoking Reconciler which will return an error")
			fakeReconcile.AddResult(reconcile.Result{}, errors.New("oups, I did it again"))
			Expect(<-reconciled).To(Equal(request))
			Eventually(func() []priorityQueueAddition {
				q.lock.Lock()
				defer q.lock.Unlock()
				return q.added
			}).Should(Equal([]priorityQueueAddition{{
				AddOpts: priorityqueue.AddOpts{
					RateLimited: true,
					Priority:    ptr.To(10),
				},
				items: []reconcile.Request{request},
			}}))
		})

		PIt("should return if the queue is shutdown", func() {
			// TODO(community): write this test
		})

		PIt("should wait for informers to be synced before processing items", func() {
			// TODO(community): write this test
		})

		PIt("should create a new go routine for MaxConcurrentReconciles", func() {
			// TODO(community): write this test
		})

		Context("prometheus metric reconcile_total", func() {
			var reconcileTotal dto.Metric

			BeforeEach(func() {
				ctrlmetrics.ReconcileTotal.Reset()
				reconcileTotal.Reset()
			})

			It("should get updated on successful reconciliation", func(ctx SpecContext) {
				Expect(func() error {
					Expect(ctrlmetrics.ReconcileTotal.WithLabelValues(ctrl.Name, "success").Write(&reconcileTotal)).To(Succeed())
					if reconcileTotal.GetCounter().GetValue() != 0.0 {
						return fmt.Errorf("metric reconcile total not reset")
					}
					return nil
				}()).Should(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
				}()
				By("Invoking Reconciler which will succeed")
				queue.Add(request)

				fakeReconcile.AddResult(reconcile.Result{}, nil)
				Expect(<-reconciled).To(Equal(request))
				Eventually(func() error {
					Expect(ctrlmetrics.ReconcileTotal.WithLabelValues(ctrl.Name, "success").Write(&reconcileTotal)).To(Succeed())
					if actual := reconcileTotal.GetCounter().GetValue(); actual != 1.0 {
						return fmt.Errorf("metric reconcile total expected: %v and got: %v", 1.0, actual)
					}
					return nil
				}, 2.0).Should(Succeed())
			})

			It("should get updated on reconcile errors", func(ctx SpecContext) {
				Expect(func() error {
					Expect(ctrlmetrics.ReconcileTotal.WithLabelValues(ctrl.Name, "error").Write(&reconcileTotal)).To(Succeed())
					if reconcileTotal.GetCounter().GetValue() != 0.0 {
						return fmt.Errorf("metric reconcile total not reset")
					}
					return nil
				}()).Should(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
				}()
				By("Invoking Reconciler which will give an error")
				queue.Add(request)

				fakeReconcile.AddResult(reconcile.Result{}, fmt.Errorf("expected error: reconcile"))
				Expect(<-reconciled).To(Equal(request))
				Eventually(func() error {
					Expect(ctrlmetrics.ReconcileTotal.WithLabelValues(ctrl.Name, "error").Write(&reconcileTotal)).To(Succeed())
					if actual := reconcileTotal.GetCounter().GetValue(); actual != 1.0 {
						return fmt.Errorf("metric reconcile total expected: %v and got: %v", 1.0, actual)
					}
					return nil
				}, 2.0).Should(Succeed())
			})

			It("should get updated when reconcile returns with retry enabled", func(ctx SpecContext) {
				Expect(func() error {
					Expect(ctrlmetrics.ReconcileTotal.WithLabelValues(ctrl.Name, "retry").Write(&reconcileTotal)).To(Succeed())
					if reconcileTotal.GetCounter().GetValue() != 0.0 {
						return fmt.Errorf("metric reconcile total not reset")
					}
					return nil
				}()).Should(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
				}()

				By("Invoking Reconciler which will return result with Requeue enabled")
				queue.Add(request)

				fakeReconcile.AddResult(reconcile.Result{Requeue: true}, nil)
				Expect(<-reconciled).To(Equal(request))
				Eventually(func() error {
					Expect(ctrlmetrics.ReconcileTotal.WithLabelValues(ctrl.Name, "requeue").Write(&reconcileTotal)).To(Succeed())
					if actual := reconcileTotal.GetCounter().GetValue(); actual != 1.0 {
						return fmt.Errorf("metric reconcile total expected: %v and got: %v", 1.0, actual)
					}
					return nil
				}, 2.0).Should(Succeed())
			})

			It("should get updated when reconcile returns with retryAfter enabled", func(ctx SpecContext) {
				Expect(func() error {
					Expect(ctrlmetrics.ReconcileTotal.WithLabelValues(ctrl.Name, "retry_after").Write(&reconcileTotal)).To(Succeed())
					if reconcileTotal.GetCounter().GetValue() != 0.0 {
						return fmt.Errorf("metric reconcile total not reset")
					}
					return nil
				}()).Should(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
				}()
				By("Invoking Reconciler which will return result with requeueAfter enabled")
				queue.Add(request)

				fakeReconcile.AddResult(reconcile.Result{RequeueAfter: 5 * time.Hour}, nil)
				Expect(<-reconciled).To(Equal(request))
				Eventually(func() error {
					Expect(ctrlmetrics.ReconcileTotal.WithLabelValues(ctrl.Name, "requeue_after").Write(&reconcileTotal)).To(Succeed())
					if actual := reconcileTotal.GetCounter().GetValue(); actual != 1.0 {
						return fmt.Errorf("metric reconcile total expected: %v and got: %v", 1.0, actual)
					}
					return nil
				}, 2.0).Should(Succeed())
			})
		})

		Context("should update prometheus metrics", func() {
			It("should requeue a Request if there is an error and continue processing items", func(ctx SpecContext) {
				var reconcileErrs dto.Metric
				ctrlmetrics.ReconcileErrors.Reset()
				Expect(func() error {
					Expect(ctrlmetrics.ReconcileErrors.WithLabelValues(ctrl.Name).Write(&reconcileErrs)).To(Succeed())
					if reconcileErrs.GetCounter().GetValue() != 0.0 {
						return fmt.Errorf("metric reconcile errors not reset")
					}
					return nil
				}()).Should(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
				}()
				queue.Add(request)

				By("Invoking Reconciler which will give an error")
				fakeReconcile.AddResult(reconcile.Result{}, fmt.Errorf("expected error: reconcile"))
				Expect(<-reconciled).To(Equal(request))
				Eventually(func() error {
					Expect(ctrlmetrics.ReconcileErrors.WithLabelValues(ctrl.Name).Write(&reconcileErrs)).To(Succeed())
					if reconcileErrs.GetCounter().GetValue() != 1.0 {
						return fmt.Errorf("metrics not updated")
					}
					return nil
				}, 2.0).Should(Succeed())

				By("Invoking Reconciler a second time without error")
				fakeReconcile.AddResult(reconcile.Result{}, nil)
				Expect(<-reconciled).To(Equal(request))

				By("Removing the item from the queue")
				Eventually(queue.Len).Should(Equal(0))
				Eventually(func() int { return queue.NumRequeues(request) }).Should(Equal(0))
			})

			It("should add a reconcile time to the reconcile time histogram", func(ctx SpecContext) {
				var reconcileTime dto.Metric
				ctrlmetrics.ReconcileTime.Reset()

				Expect(func() error {
					histObserver := ctrlmetrics.ReconcileTime.WithLabelValues(ctrl.Name)
					hist := histObserver.(prometheus.Histogram)
					Expect(hist.Write(&reconcileTime)).To(Succeed())
					if reconcileTime.GetHistogram().GetSampleCount() != uint64(0) {
						return fmt.Errorf("metrics not reset")
					}
					return nil
				}()).Should(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(ctrl.Start(ctx)).NotTo(HaveOccurred())
				}()
				queue.Add(request)

				By("Invoking Reconciler")
				fakeReconcile.AddResult(reconcile.Result{}, nil)
				Expect(<-reconciled).To(Equal(request))

				By("Removing the item from the queue")
				Eventually(queue.Len).Should(Equal(0))
				Eventually(func() int { return queue.NumRequeues(request) }).Should(Equal(0))

				Eventually(func() error {
					histObserver := ctrlmetrics.ReconcileTime.WithLabelValues(ctrl.Name)
					hist := histObserver.(prometheus.Histogram)
					Expect(hist.Write(&reconcileTime)).To(Succeed())
					if reconcileTime.GetHistogram().GetSampleCount() == uint64(0) {
						return fmt.Errorf("metrics not updated")
					}
					return nil
				}, 2.0).Should(Succeed())
			})
		})
	})

	Describe("Warmup", func() {
		JustBeforeEach(func() {
			ctrl.EnableWarmup = ptr.To(true)
		})

		It("should track warmup status correctly with successful sync", func(ctx SpecContext) {
			// Setup controller with sources that complete successfully
			ctrl.CacheSyncTimeout = time.Second
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					return nil
				}),
			}

			Expect(ctrl.Warmup(ctx)).To(Succeed())
		})

		It("should return an error if there is an error waiting for the informers", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = time.Second
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Kind(&informertest.FakeInformers{Synced: ptr.To(false)}, &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}),
			}
			ctrl.Name = testControllerName
			err := ctrl.Warmup(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to wait for testcontroller caches to sync"))
		})

		It("should error when cache sync timeout occurs", func(ctx SpecContext) {
			c, err := cache.New(cfg, cache.Options{})
			Expect(err).NotTo(HaveOccurred())
			c = &cacheWithIndefinitelyBlockingGetInformer{c}

			ctrl.CacheSyncTimeout = time.Second
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Kind(c, &appsv1.Deployment{}, &handler.TypedEnqueueRequestForObject[*appsv1.Deployment]{}),
			}
			ctrl.Name = testControllerName

			err = ctrl.Warmup(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to wait for testcontroller caches to sync kind source: *v1.Deployment: timed out waiting for cache to be synced"))
		})

		It("should not error when controller Warmup context is cancelled during Sources WaitForSync", func(specCtx SpecContext) {
			ctrl.CacheSyncTimeout = 1 * time.Second

			sourceSynced := make(chan struct{})
			c, err := cache.New(cfg, cache.Options{})
			Expect(err).NotTo(HaveOccurred())
			c = &cacheWithIndefinitelyBlockingGetInformer{c}
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				&singnallingSourceWrapper{
					SyncingSource: source.Kind[client.Object](c, &appsv1.Deployment{}, &handler.EnqueueRequestForObject{}),
					cacheSyncDone: sourceSynced,
				},
			}
			ctrl.Name = testControllerName

			ctx, cancel := context.WithCancel(specCtx)
			go func() {
				defer GinkgoRecover()
				err = ctrl.Warmup(ctx)
				Expect(err).To(Succeed())
			}()

			cancel()
			<-sourceSynced
		})

		It("should error when Warmup() is blocking forever", func(specCtx SpecContext) {
			ctrl.CacheSyncTimeout = time.Second

			controllerDone := make(chan struct{})
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					<-controllerDone
					return ctx.Err()
				})}

			ctx, cancel := context.WithTimeout(specCtx, 10*time.Second)
			defer cancel()

			err := ctrl.Warmup(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Please ensure that its Start() method is non-blocking"))

			close(controllerDone)
		})

		It("should not error when cache sync timeout is of sufficiently high", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second

			sourceSynced := make(chan struct{})
			c := &informertest.FakeInformers{}
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				&singnallingSourceWrapper{
					SyncingSource: source.Kind[client.Object](c, &appsv1.Deployment{}, &handler.EnqueueRequestForObject{}),
					cacheSyncDone: sourceSynced,
				},
			}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Warmup(ctx)).To(Succeed())
			}()

			<-sourceSynced
		})

		It("should process events from source.Channel", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second
			// channel to be closed when event is processed
			processed := make(chan struct{})
			// source channel
			ch := make(chan event.GenericEvent, 1)

			// event to be sent to the channel
			p := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"},
			}
			evt := event.GenericEvent{
				Object: p,
			}

			ins := source.Channel(
				ch,
				handler.Funcs{
					GenericFunc: func(ctx context.Context, evt event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
						defer GinkgoRecover()
						close(processed)
					},
				},
			)

			// send the event to the channel
			ch <- evt

			ctrl.startWatches = []source.TypedSource[reconcile.Request]{ins}

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Warmup(ctx)).To(Succeed())
			}()
			<-processed
		})

		It("should error when channel source is not specified", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second

			ins := source.Channel[string](nil, nil)
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{ins}

			e := ctrl.Warmup(ctx)
			Expect(e).To(HaveOccurred())
			Expect(e.Error()).To(ContainSubstring("must specify Channel.Source"))
		})

		It("should call Start on sources with the appropriate EventHandler, Queue, and Predicates", func(specCtx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second
			started := false
			ctx, cancel := context.WithCancel(specCtx)
			src := source.Func(func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				defer GinkgoRecover()
				Expect(q).To(Equal(ctrl.Queue))

				started = true
				cancel() // Cancel the context so ctrl.Warmup() doesn't block forever
				return nil
			})
			Expect(ctrl.Watch(src)).NotTo(HaveOccurred())

			err := ctrl.Warmup(ctx)
			Expect(err).To(Succeed())
			Expect(started).To(BeTrue())
		})

		It("should return an error if there is an error starting sources", func(ctx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second
			err := fmt.Errorf("Expected Error: could not start source")
			src := source.Func(func(context.Context,
				workqueue.TypedRateLimitingInterface[reconcile.Request],
			) error {
				defer GinkgoRecover()
				return err
			})
			Expect(ctrl.Watch(src)).To(Succeed())

			Expect(ctrl.Warmup(ctx)).To(Equal(err))
		})

		It("should track warmup status correctly with unsuccessful sync", func(ctx SpecContext) {
			// Setup controller with sources that complete with error
			ctrl.CacheSyncTimeout = time.Second
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					return errors.New("sync error")
				}),
			}

			err := ctrl.Warmup(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("sync error"))
		})

		It("should call Start on sources with the appropriate non-nil queue", func(specCtx SpecContext) {
			ctrl.CacheSyncTimeout = 10 * time.Second
			started := false
			ctx, cancel := context.WithCancel(specCtx)
			src := source.Func(func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				defer GinkgoRecover()
				Expect(q).ToNot(BeNil())
				Expect(q).To(Equal(ctrl.Queue))

				started = true
				cancel() // Cancel the context so ctrl.Start() doesn't block forever
				return nil
			})
			Expect(ctrl.Watch(src)).To(Succeed())
			Expect(ctrl.Warmup(ctx)).To(Succeed())
			Expect(ctrl.Queue).ToNot(BeNil())
			Expect(started).To(BeTrue())
		})

		It("should return true if context is cancelled while waiting for source to start", func(specCtx SpecContext) {
			// Setup controller with sources that complete with error
			ctx, cancel := context.WithCancel(specCtx)
			defer cancel()

			ctrl.CacheSyncTimeout = time.Second
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					<-ctx.Done()
					return nil
				}),
			}

			// channel to prevent the goroutine from outliving the It test
			waitChan := make(chan struct{})

			// Invoked in a goroutine because Warmup will block
			go func() {
				defer GinkgoRecover()
				defer close(waitChan)
				Expect(ctrl.Warmup(ctx)).To(Succeed())
			}()

			cancel()
			<-waitChan
		})

		It("should be called before leader election runnables if warmup is enabled", func(specCtx SpecContext) {
			// This unit test exists to ensure that a warmup enabled controller will actually be
			// called in the warmup phase before the leader election runnables are started. It
			// catches regressions in the controller that would not implement warmupRunnable from
			// pkg/manager.
			ctx, cancel := context.WithCancel(specCtx)

			By("Creating a channel to track execution order")
			runnableExecutionOrderChan := make(chan string, 2)
			const nonWarmupRunnableName = "nonWarmupRunnable"
			const warmupRunnableName = "warmupRunnable"

			ctrl.CacheSyncTimeout = time.Second
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					runnableExecutionOrderChan <- warmupRunnableName
					return nil
				}),
			}

			nonWarmupCtrl := New[reconcile.Request](Options[reconcile.Request]{
				MaxConcurrentReconciles: 1,
				Do:                      fakeReconcile,
				NewQueue: func(string, workqueue.TypedRateLimiter[reconcile.Request]) workqueue.TypedRateLimitingInterface[reconcile.Request] {
					return queue
				},
				LogConstructor: func(_ *reconcile.Request) logr.Logger {
					return log.RuntimeLog.WithName("controller").WithName("test")
				},
				CacheSyncTimeout: time.Second,
				EnableWarmup:     ptr.To(false),
				LeaderElected:    ptr.To(true),
			})
			nonWarmupCtrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					runnableExecutionOrderChan <- nonWarmupRunnableName
					return nil
				}),
			}

			By("Creating a test resource lock with hooks")
			resourceLock, err := fakeleaderelection.NewResourceLock(nil, nil, leaderelection.Options{})
			Expect(err).ToNot(HaveOccurred())

			By("Creating a manager")
			testenv = &envtest.Environment{}
			cfg, err := testenv.Start()
			Expect(err).NotTo(HaveOccurred())
			m, err := manager.New(cfg, manager.Options{
				LeaderElection:                      true,
				LeaderElectionID:                    "some-leader-election-id",
				LeaderElectionNamespace:             "default",
				LeaderElectionResourceLockInterface: resourceLock,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Adding warmup and non-warmup controllers to the manager")
			Expect(m.Add(ctrl)).To(Succeed())
			Expect(m.Add(nonWarmupCtrl)).To(Succeed())

			By("Blocking leader election")
			resourceLockWithHooks, ok := resourceLock.(fakeleaderelection.ControllableResourceLockInterface)
			Expect(ok).To(BeTrue(), "resource lock should implement ResourceLockInterfaceWithHooks")
			resourceLockWithHooks.BlockLeaderElection()

			By("Starting the manager")
			waitChan := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				defer close(waitChan)
				Expect(m.Start(ctx)).To(Succeed())
			}()
			Expect(<-runnableExecutionOrderChan).To(Equal(warmupRunnableName))

			By("Unblocking leader election")
			resourceLockWithHooks.UnblockLeaderElection()
			<-m.Elected()
			Expect(<-runnableExecutionOrderChan).To(Equal(nonWarmupRunnableName))

			cancel()
			<-waitChan
		})

		It("should not cause a data race when called concurrently", func(ctx SpecContext) {

			ctrl.CacheSyncTimeout = time.Second

			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					return nil
				}),
			}

			var wg sync.WaitGroup
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					Expect(ctrl.Warmup(ctx)).To(Succeed())
				}()
			}

			wg.Wait()
		})

		It("should not cause a data race when called concurrently with Start and only start sources once", func(specCtx SpecContext) {
			ctx, cancel := context.WithCancel(specCtx)

			ctrl.CacheSyncTimeout = time.Second
			numWatches := 10

			var watchStartedCount atomic.Int32
			for range numWatches {
				ctrl.startWatches = append(ctrl.startWatches, source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					watchStartedCount.Add(1)
					return nil
				}))
			}

			By("calling Warmup and Start concurrently")
			blockOnStartChan := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).To(Succeed())
				close(blockOnStartChan)
			}()

			blockOnWarmupChan := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Warmup(ctx)).To(Succeed())
				close(blockOnWarmupChan)
			}()

			<-blockOnWarmupChan

			cancel()

			<-blockOnStartChan

			Expect(watchStartedCount.Load()).To(Equal(int32(numWatches)), "source should only be started once")
			Expect(ctrl.startWatches).To(BeNil(), "startWatches should be reset to nil after they are started")
		})

		It("should start sources added after Warmup is called", func(specCtx SpecContext) {
			ctx, cancel := context.WithCancel(specCtx)

			ctrl.CacheSyncTimeout = time.Second

			Expect(ctrl.Warmup(ctx)).To(Succeed())

			By("starting a watch after warmup is added")
			var didWatchStart atomic.Bool
			Expect(ctrl.Watch(source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				didWatchStart.Store(true)
				return nil
			}))).To(Succeed())

			waitChan := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).To(Succeed())
				close(waitChan)
			}()

			Eventually(didWatchStart.Load).Should(BeTrue(), "watch should be started if it is added after Warmup")

			cancel()
			<-waitChan
		})

		DescribeTable("should not leak goroutines when manager is stopped with warmup runnable",
			func(specContext SpecContext, leaderElection bool) {
				ctx, cancel := context.WithCancel(specContext)
				defer cancel()

				ctrl.CacheSyncTimeout = time.Second

				By("Creating a manager")
				testenv = &envtest.Environment{}
				cfg, err := testenv.Start()
				Expect(err).NotTo(HaveOccurred())
				m, err := manager.New(cfg, manager.Options{
					LeaderElection:          leaderElection,
					LeaderElectionID:        "some-leader-election-id",
					LeaderElectionNamespace: "default",
				})
				Expect(err).NotTo(HaveOccurred())

				ctrl.startWatches = []source.TypedSource[reconcile.Request]{
					source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
						<-ctx.Done()
						return nil
					}),
				}
				Expect(m.Add(ctrl)).To(Succeed())

				// ignore needs to go after the testenv.Start() call to ignore the apiserver
				// process
				currentGRs := goleak.IgnoreCurrent()
				waitChan := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).To(Succeed())
					close(waitChan)
				}()

				<-m.Elected()
				By("stopping the manager via context")
				cancel()

				Eventually(func() error { return goleak.Find(currentGRs) }).Should(Succeed())
				<-waitChan
			},
			Entry("and with leader election enabled", true),
			Entry("and without leader election enabled", false),
		)
	})

	Describe("Warmup with warmup disabled", func() {
		JustBeforeEach(func() {
			ctrl.EnableWarmup = ptr.To(false)
		})

		It("should not start sources when Warmup is called if warmup is disabled but start it when Start is called.", func(specCtx SpecContext) {
			// Setup controller with sources that complete successfully
			ctx, cancel := context.WithCancel(specCtx)

			ctrl.CacheSyncTimeout = time.Second
			var isSourceStarted atomic.Bool
			isSourceStarted.Store(false)
			ctrl.startWatches = []source.TypedSource[reconcile.Request]{
				source.Func(func(ctx context.Context, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
					isSourceStarted.Store(true)
					return nil
				}),
			}

			By("Calling Warmup when EnableWarmup is false")
			err := ctrl.Warmup(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(isSourceStarted.Load()).To(BeFalse())

			By("Calling Start when EnableWarmup is false")
			waitChan := make(chan struct{})

			go func() {
				defer GinkgoRecover()
				Expect(ctrl.Start(ctx)).To(Succeed())
				close(waitChan)
			}()
			Eventually(isSourceStarted.Load).Should(BeTrue())
			cancel()
			<-waitChan
		})
	})
})

var _ = Describe("ReconcileIDFromContext function", func() {
	It("should return an empty string if there is nothing in the context", func(ctx SpecContext) {
		reconcileID := ReconcileIDFromContext(ctx)

		Expect(reconcileID).To(Equal(types.UID("")))
	})

	It("should return the correct reconcileID from context", func(specContext SpecContext) {
		const expectedReconcileID = types.UID("uuid")
		ctx := addReconcileID(specContext, expectedReconcileID)
		reconcileID := ReconcileIDFromContext(ctx)

		Expect(reconcileID).To(Equal(expectedReconcileID))
	})
})

type DelegatingQueue struct {
	workqueue.TypedRateLimitingInterface[reconcile.Request]
	mu sync.Mutex

	countAddRateLimited int
	countAdd            int
	countAddAfter       int
}

func (q *DelegatingQueue) AddRateLimited(item reconcile.Request) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.countAddRateLimited++
	q.TypedRateLimitingInterface.AddRateLimited(item)
}

func (q *DelegatingQueue) AddAfter(item reconcile.Request, d time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.countAddAfter++
	q.TypedRateLimitingInterface.AddAfter(item, d)
}

func (q *DelegatingQueue) Add(item reconcile.Request) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.countAdd++

	q.TypedRateLimitingInterface.Add(item)
}

func (q *DelegatingQueue) Forget(item reconcile.Request) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.countAdd--

	q.TypedRateLimitingInterface.Forget(item)
}

type countInfo struct {
	Trying, AddAfter, AddRateLimited int
}

func (q *DelegatingQueue) getCounts() countInfo {
	q.mu.Lock()
	defer q.mu.Unlock()

	return countInfo{
		Trying:         q.countAdd,
		AddAfter:       q.countAddAfter,
		AddRateLimited: q.countAddRateLimited,
	}
}

type fakeReconcileResultPair struct {
	Result reconcile.Result
	Err    error
}

type fakeReconciler struct {
	Requests chan reconcile.Request
	results  chan fakeReconcileResultPair
}

func (f *fakeReconciler) AddResult(res reconcile.Result, err error) {
	f.results <- fakeReconcileResultPair{Result: res, Err: err}
}

func (f *fakeReconciler) Reconcile(_ context.Context, r reconcile.Request) (reconcile.Result, error) {
	res := <-f.results
	if f.Requests != nil {
		f.Requests <- r
	}
	return res.Result, res.Err
}

type singnallingSourceWrapper struct {
	cacheSyncDone chan struct{}
	source.SyncingSource
}

func (s *singnallingSourceWrapper) Start(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
	err := s.SyncingSource.Start(ctx, q)
	if err != nil {
		// WaitForSync will never be called if this errors, so close the channel to prevent deadlocks in tests
		close(s.cacheSyncDone)
	}
	return err
}

func (s *singnallingSourceWrapper) WaitForSync(ctx context.Context) error {
	defer func() {
		close(s.cacheSyncDone)
	}()
	return s.SyncingSource.WaitForSync(ctx)
}

var _ cache.Cache = &cacheWithIndefinitelyBlockingGetInformer{}

// cacheWithIndefinitelyBlockingGetInformer has a GetInformer implementation that blocks indefinitely or until its
// context is cancelled.
// We need it as a workaround for testenvs lack of support for a secure apiserver, because the insecure port always
// implies the allow all authorizer, so we can not simulate rbac issues with it. They are the usual cause of the real
// caches GetInformer blocking showing this behavior.
// TODO: Remove this once envtest supports a secure apiserver.
type cacheWithIndefinitelyBlockingGetInformer struct {
	cache.Cache
}

func (c *cacheWithIndefinitelyBlockingGetInformer) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	<-ctx.Done()
	return nil, errors.New("GetInformer timed out")
}

type bisignallingSource[T comparable] struct {
	// receives the queue that is passed to Start
	startCall chan workqueue.TypedRateLimitingInterface[T]
	// passes an error to return from Start
	startDone chan error
	// closed when WaitForSync is called
	waitCall chan struct{}
	// passes an error to return from WaitForSync
	waitDone chan error
}

var _ source.TypedSyncingSource[int] = (*bisignallingSource[int])(nil)

func (t *bisignallingSource[T]) Start(ctx context.Context, q workqueue.TypedRateLimitingInterface[T]) error {
	select {
	case t.startCall <- q:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-t.startDone:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *bisignallingSource[T]) WaitForSync(ctx context.Context) error {
	close(t.waitCall)
	select {
	case err := <-t.waitDone:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type priorityQueueAddition struct {
	priorityqueue.AddOpts
	items []reconcile.Request
}

type fakePriorityQueue struct {
	priorityqueue.PriorityQueue[reconcile.Request]

	lock  sync.Mutex
	added []priorityQueueAddition
}

func (f *fakePriorityQueue) AddWithOpts(o priorityqueue.AddOpts, items ...reconcile.Request) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.added = append(f.added, priorityQueueAddition{AddOpts: o, items: items})
}
