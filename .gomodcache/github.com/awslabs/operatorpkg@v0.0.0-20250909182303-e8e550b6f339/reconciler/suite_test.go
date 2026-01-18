package reconciler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconciler")
}

// MockRateLimiter is a mock implementation of workqueue.TypedRateLimiter for testing
type MockRateLimiter[K comparable] struct {
	whenFunc        func(K) time.Duration
	numRequeues     map[K]int
	backoffDuration time.Duration
}

func (m *MockRateLimiter[K]) When(key K) time.Duration {
	if m.whenFunc != nil {
		return m.whenFunc(key)
	}
	// Default implementation
	if m.numRequeues == nil {
		m.numRequeues = make(map[K]int)
	}
	m.numRequeues[key] += 1
	return m.backoffDuration
}

func (m *MockRateLimiter[K]) NumRequeues(key K) int {
	return m.numRequeues[key]
}

func (m *MockRateLimiter[K]) Forget(key K) {
	delete(m.numRequeues, key)
}

// MockReconciler is a mock implementation of Reconciler for testing
type MockReconciler struct {
	reconcileFunc func(context.Context, reconcile.Request) (reconciler.Result, error)
	result        reconciler.Result
	err           error
}

func (m *MockReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconciler.Result, error) {
	if m.reconcileFunc != nil {
		return m.reconcileFunc(ctx, req)
	}
	return m.result, m.err
}

var _ = Describe("Reconciler", func() {
	It("should return the result without backoff", func() {
		mockReconciler := &MockReconciler{
			result: reconciler.Result{},
		}

		reconciler := reconciler.AsReconciler(mockReconciler)

		result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(0 * time.Second))
	})
	It("should return the result with backoff when Requeue is set", func() {
		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				Requeue: true,
			},
		}

		reconciler := reconciler.AsReconciler(mockReconciler)

		result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(5 * time.Millisecond))
	})
	It("should return the result with backoff when both RequeueAfter and Requeue are set", func() {
		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				RequeueAfter: 10 * time.Second,
				Requeue:      true,
			},
		}

		reconciler := reconciler.AsReconciler(mockReconciler)

		result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(10 * time.Second))
	})
	It("should return the result with backoff when RequeueAfter is set and Requeue is false", func() {
		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				RequeueAfter: 10 * time.Second,
				Requeue:      false,
			},
		}

		reconciler := reconciler.AsReconciler(mockReconciler)

		result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(10 * time.Second))
	})
	It("should return the result with backoff when RequeueAfter is set to zero and Requeue is true", func() {
		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				RequeueAfter: 0 * time.Second,
				Requeue:      true,
			},
		}

		reconciler := reconciler.AsReconciler(mockReconciler)

		result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(5 * time.Millisecond))
	})
	It("should return the result without backoff when RequeueAfter is set to zero and Requeue is false", func() {
		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				RequeueAfter: 0 * time.Second,
				Requeue:      false,
			},
		}

		reconciler := reconciler.AsReconciler(mockReconciler)

		result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(0 * time.Millisecond))
	})
	It("should return the error without processing backoff", func() {
		expectedErr := errors.New("test error")
		mockReconciler := &MockReconciler{
			result: reconciler.Result{Requeue: true},
			err:    expectedErr,
		}

		reconciler := reconciler.AsReconciler(mockReconciler)

		result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})

		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(expectedErr))
		Expect(result.RequeueAfter).To(BeZero())
	})
	It("should use custom rate limiter for backoff", func() {
		mockRateLimiter := &MockRateLimiter[reconcile.Request]{
			backoffDuration: 10 * time.Second,
		}

		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				Requeue: true,
			},
		}

		reconciler := reconciler.AsReconcilerWithRateLimiter(mockReconciler, mockRateLimiter)

		req := reconcile.Request{}
		result, err := reconciler.Reconcile(context.Background(), req)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(10 * time.Second))
		Expect(mockRateLimiter.NumRequeues(req)).To(Equal(1))
	})
	It("should rate limit distinct items", func() {
		mockRateLimiter := &MockRateLimiter[reconcile.Request]{
			backoffDuration: 10 * time.Second,
		}

		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				Requeue: true,
			},
		}

		reconciler := reconciler.AsReconcilerWithRateLimiter(mockReconciler, mockRateLimiter)

		req1 := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "req1",
				Namespace: "",
			},
		}
		result1, err1 := reconciler.Reconcile(context.Background(), req1)
		req2 := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "req2",
				Namespace: "",
			},
		}
		result2, err2 := reconciler.Reconcile(context.Background(), req2)

		Expect(err1).NotTo(HaveOccurred())
		Expect(result1.RequeueAfter).To(Equal(10 * time.Second))
		Expect(err2).NotTo(HaveOccurred())
		Expect(result2.RequeueAfter).To(Equal(10 * time.Second))
		Expect(mockRateLimiter.NumRequeues(req1)).To(Equal(1))
		Expect(mockRateLimiter.NumRequeues(req2)).To(Equal(1))
	})
	It("should implement exponential backoff on repeated calls", func() {
		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				Requeue: true,
			},
		}
		// Multiple calls to the same controller should show increasing delays
		delays := make([]time.Duration, 5)
		reconciler := reconciler.AsReconciler(mockReconciler)

		for i := range 5 {
			result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			delays[i] = result.RequeueAfter
		}

		initialDelay := 5 * time.Millisecond
		Expect(delays[0]).To(BeNumerically("==", initialDelay))
		for i := 1; i < len(delays); i++ {
			initialDelay *= 2
			Expect(delays[i]).To(BeNumerically("==", initialDelay))
			Expect(delays[i]).To(BeNumerically(">", delays[i-1]),
				"Delay at index %d (%v) should be >= delay at index %d (%v)",
				i, delays[i], i-1, delays[i-1])
		}
	})
	It("should forget an item when reconcile succeeds", func() {
		mockReconciler := &MockReconciler{
			result: reconciler.Result{
				Requeue: false,
			},
		}
		// Multiple calls to the same controller should show zero requeues
		reconciler := reconciler.AsReconciler(mockReconciler)

		for i := 0; i < 5; i++ {
			result, err := reconciler.Reconcile(context.Background(), reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
		}
	})
	It("should return a result with RequeueAfter that is scoped to a controller", func() {
		// Test with different controllers to ensure they're handled independently
		controller1 := &MockReconciler{
			result: reconciler.Result{Requeue: true},
		}
		controller2 := &MockReconciler{
			result: reconciler.Result{Requeue: true},
		}

		reconciler1 := reconciler.AsReconciler(controller1)
		reconciler2 := reconciler.AsReconciler(controller2)

		// Each controller should get its own rate limiting
		result1, err1 := reconciler1.Reconcile(context.Background(), reconcile.Request{})
		result2, err2 := reconciler2.Reconcile(context.Background(), reconcile.Request{})

		Expect(err1).NotTo(HaveOccurred())
		Expect(err2).NotTo(HaveOccurred())
		Expect(result1.RequeueAfter).To(BeNumerically(">=", 0))
		Expect(result2.RequeueAfter).To(BeNumerically(">=", 0))
		Expect(result1.RequeueAfter).To(Equal(result2.RequeueAfter))

		result2, err2 = reconciler2.Reconcile(context.Background(), reconcile.Request{})
		Expect(err2).NotTo(HaveOccurred())
		Expect(result1.RequeueAfter).NotTo(Equal(result2.RequeueAfter))
	})
})
