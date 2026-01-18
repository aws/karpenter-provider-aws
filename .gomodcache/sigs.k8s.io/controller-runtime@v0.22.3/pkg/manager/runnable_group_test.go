package manager

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ = Describe("runnables", func() {
	errCh := make(chan error)

	It("should be able to create a new runnables object", func() {
		Expect(newRunnables(defaultBaseContext, errCh)).ToNot(BeNil())
	})

	It("should add HTTP servers to the appropriate group", func() {
		server := &Server{}
		r := newRunnables(defaultBaseContext, errCh)
		Expect(r.Add(server)).To(Succeed())
		Expect(r.HTTPServers.startQueue).To(HaveLen(1))
		Expect(r.Others.startQueue).To(BeEmpty())
	})

	It("should add caches to the appropriate group", func() {
		cache := &cacheProvider{cache: &informertest.FakeInformers{Error: fmt.Errorf("expected error")}}
		r := newRunnables(defaultBaseContext, errCh)
		Expect(r.Add(cache)).To(Succeed())
		Expect(r.Caches.startQueue).To(HaveLen(1))
		Expect(r.Others.startQueue).To(BeEmpty())
	})

	It("should add webhooks to the appropriate group", func() {
		webhook := webhook.NewServer(webhook.Options{})
		r := newRunnables(defaultBaseContext, errCh)
		Expect(r.Add(webhook)).To(Succeed())
		Expect(r.Webhooks.startQueue).To(HaveLen(1))
		Expect(r.Others.startQueue).To(BeEmpty())
	})

	It("should add any runnable to the leader election group", func() {
		err := errors.New("runnable func")
		runnable := RunnableFunc(func(c context.Context) error {
			return err
		})

		r := newRunnables(defaultBaseContext, errCh)
		Expect(r.Add(runnable)).To(Succeed())
		Expect(r.LeaderElection.startQueue).To(HaveLen(1))
		Expect(r.Others.startQueue).To(BeEmpty())
	})

	It("should add WarmupRunnable to the Warmup and LeaderElection group", func() {
		warmupRunnable := newWarmupRunnableFunc(
			func(c context.Context) error {
				<-c.Done()
				return nil
			},
			func(c context.Context) error { return nil },
		)

		r := newRunnables(defaultBaseContext, errCh)
		Expect(r.Add(warmupRunnable)).To(Succeed())
		Expect(r.Warmup.startQueue).To(HaveLen(1))
		Expect(r.LeaderElection.startQueue).To(HaveLen(1))
		Expect(r.Others.startQueue).To(BeEmpty())
	})

	It("should add WarmupRunnable that doesn't needs leader election to warmup group only", func() {
		warmupRunnable := newLeaderElectionAndWarmupRunnable(
			func(c context.Context) error {
				<-c.Done()
				return nil
			},
			func(c context.Context) error { return nil },
			false,
		)

		r := newRunnables(defaultBaseContext, errCh)
		Expect(r.Add(warmupRunnable)).To(Succeed())

		Expect(r.Warmup.startQueue).To(HaveLen(1))
		Expect(r.LeaderElection.startQueue).To(BeEmpty())
		Expect(r.Others.startQueue).To(HaveLen(1))
	})

	It("should add WarmupRunnable that needs leader election to Warmup and LeaderElection group, not Others", func() {
		warmupRunnable := newLeaderElectionAndWarmupRunnable(
			func(c context.Context) error {
				<-c.Done()
				return nil
			},
			func(c context.Context) error { return nil },
			true,
		)

		r := newRunnables(defaultBaseContext, errCh)
		Expect(r.Add(warmupRunnable)).To(Succeed())

		Expect(r.Warmup.startQueue).To(HaveLen(1))
		Expect(r.LeaderElection.startQueue).To(HaveLen(1))
		Expect(r.Others.startQueue).To(BeEmpty())
	})

	It("should execute the Warmup function when Warmup group is started", func(ctx SpecContext) {
		var warmupExecuted atomic.Bool

		warmupRunnable := newWarmupRunnableFunc(
			func(c context.Context) error {
				<-c.Done()
				return nil
			},
			func(c context.Context) error {
				warmupExecuted.Store(true)
				return nil
			},
		)

		r := newRunnables(defaultBaseContext, errCh)
		Expect(r.Add(warmupRunnable)).To(Succeed())

		// Start the Warmup group
		Expect(r.Warmup.Start(ctx)).To(Succeed())

		// Verify warmup function was called
		Expect(warmupExecuted.Load()).To(BeTrue())
	})

	It("should propagate errors from Warmup function to error channel", func(ctx SpecContext) {
		expectedErr := fmt.Errorf("expected warmup error")

		warmupRunnable := newWarmupRunnableFunc(
			func(c context.Context) error {
				<-c.Done()
				return nil
			},
			func(c context.Context) error { return expectedErr },
		)

		testErrChan := make(chan error, 1)
		r := newRunnables(defaultBaseContext, testErrChan)
		Expect(r.Add(warmupRunnable)).To(Succeed())

		// Start the Warmup group in a goroutine
		go func() {
			Expect(r.Warmup.Start(ctx)).To(Succeed())
		}()

		// Error from Warmup should be sent to error channel
		var receivedErr error
		Eventually(func() error {
			select {
			case receivedErr = <-testErrChan:
				return receivedErr
			default:
				return nil
			}
		}).Should(Equal(expectedErr))
	})
})

var _ = Describe("runnableGroup", func() {
	errCh := make(chan error)

	It("should be able to add new runnables before it starts", func(ctx SpecContext) {
		rg := newRunnableGroup(defaultBaseContext, errCh)
		Expect(rg.Add(RunnableFunc(func(c context.Context) error {
			<-ctx.Done()
			return nil
		}), nil)).To(Succeed())

		Expect(rg.Started()).To(BeFalse())
	})

	It("should be able to add new runnables before and after start", func(ctx SpecContext) {
		rg := newRunnableGroup(defaultBaseContext, errCh)
		Expect(rg.Add(RunnableFunc(func(c context.Context) error {
			<-ctx.Done()
			return nil
		}), nil)).To(Succeed())
		Expect(rg.Start(ctx)).To(Succeed())
		Expect(rg.Started()).To(BeTrue())
		Expect(rg.Add(RunnableFunc(func(c context.Context) error {
			<-ctx.Done()
			return nil
		}), nil)).To(Succeed())
	})

	It("should be able to add new runnables before and after start concurrently", func(ctx SpecContext) {
		rg := newRunnableGroup(defaultBaseContext, errCh)

		go func() {
			defer GinkgoRecover()
			<-time.After(50 * time.Millisecond)
			Expect(rg.Start(ctx)).To(Succeed())
		}()

		for i := 0; i < 20; i++ {
			go func(i int) {
				defer GinkgoRecover()

				<-time.After(time.Duration(i) * 10 * time.Millisecond)
				Expect(rg.Add(RunnableFunc(func(c context.Context) error {
					<-ctx.Done()
					return nil
				}), nil)).To(Succeed())
			}(i)
		}
	})

	It("should be able to close the group and wait for all runnables to finish", func(specCtx SpecContext) {
		ctx, cancel := context.WithCancel(specCtx)

		exited := ptr.To(int64(0))
		rg := newRunnableGroup(defaultBaseContext, errCh)
		for i := 0; i < 10; i++ {
			Expect(rg.Add(RunnableFunc(func(c context.Context) error {
				defer atomic.AddInt64(exited, 1)
				<-ctx.Done()
				<-time.After(time.Duration(i) * 10 * time.Millisecond)
				return nil
			}), nil)).To(Succeed())
		}
		Expect(rg.Start(ctx)).To(Succeed())

		// Cancel the context, asking the runnables to exit.
		cancel()
		rg.StopAndWait(specCtx)

		Expect(rg.Add(RunnableFunc(func(c context.Context) error {
			return nil
		}), nil)).ToNot(Succeed())

		Expect(atomic.LoadInt64(exited)).To(BeNumerically("==", 10))
	})

	It("should be able to wait for all runnables to be ready at different intervals", func(specCtx SpecContext) {
		ctx, cancel := context.WithTimeout(specCtx, 1*time.Second)
		defer cancel()
		rg := newRunnableGroup(defaultBaseContext, errCh)

		go func() {
			defer GinkgoRecover()
			<-time.After(50 * time.Millisecond)
			Expect(rg.Start(ctx)).To(Succeed())
		}()

		for i := 0; i < 20; i++ {
			go func(i int) {
				defer GinkgoRecover()

				Expect(rg.Add(RunnableFunc(func(c context.Context) error {
					<-ctx.Done()
					return nil
				}), func(_ context.Context) bool {
					<-time.After(time.Duration(i) * 10 * time.Millisecond)
					return true
				})).To(Succeed())
			}(i)
		}
	})

	It("should be able to handle adding runnables while stopping", func(specCtx SpecContext) {
		ctx, cancel := context.WithTimeout(specCtx, 10*time.Second)
		defer cancel()
		rg := newRunnableGroup(defaultBaseContext, errCh)

		go func() {
			defer GinkgoRecover()
			<-time.After(1 * time.Millisecond)
			Expect(rg.Start(ctx)).To(Succeed())
		}()
		go func() {
			defer GinkgoRecover()
			<-time.After(1 * time.Millisecond)
			ctx, cancel := context.WithCancel(ctx)
			cancel()
			rg.StopAndWait(ctx)
		}()

		for i := 0; i < 200; i++ {
			go func(i int) {
				defer GinkgoRecover()

				<-time.After(time.Duration(i) * time.Microsecond)
				Expect(rg.Add(RunnableFunc(func(c context.Context) error {
					<-ctx.Done()
					return nil
				}), func(_ context.Context) bool {
					return true
				})).To(SatisfyAny(
					Succeed(),
					Equal(errRunnableGroupStopped),
				))
			}(i)
		}
	})

	It("should not turn ready if some readiness check fail", func(specCtx SpecContext) {
		ctx, cancel := context.WithTimeout(specCtx, 2*time.Second)
		defer cancel()
		rg := newRunnableGroup(defaultBaseContext, errCh)

		go func() {
			defer GinkgoRecover()
			<-time.After(50 * time.Millisecond)
			Expect(rg.Start(ctx)).To(Succeed())
		}()

		for i := 0; i < 20; i++ {
			go func(i int) {
				defer GinkgoRecover()

				Expect(rg.Add(RunnableFunc(func(c context.Context) error {
					<-ctx.Done()
					return nil
				}), func(_ context.Context) bool {
					<-time.After(time.Duration(i) * 10 * time.Millisecond)
					return i%2 == 0 // Return false readiness all uneven indexes.
				})).To(Succeed())
			}(i)
		}
	})
})

var _ warmupRunnable = &warmupRunnableFunc{}

func newWarmupRunnableFunc(
	startFunc func(context.Context) error,
	warmupFunc func(context.Context) error,
) *warmupRunnableFunc {
	return &warmupRunnableFunc{
		startFunc:  startFunc,
		warmupFunc: warmupFunc,
	}
}

// warmupRunnableFunc is a helper struct that implements WarmupRunnable
// for testing purposes.
type warmupRunnableFunc struct {
	startFunc  func(context.Context) error
	warmupFunc func(context.Context) error
}

func (r *warmupRunnableFunc) Start(ctx context.Context) error {
	return r.startFunc(ctx)
}

func (r *warmupRunnableFunc) Warmup(ctx context.Context) error {
	return r.warmupFunc(ctx)
}

var _ LeaderElectionRunnable = &leaderElectionAndWarmupRunnable{}
var _ warmupRunnable = &leaderElectionAndWarmupRunnable{}

// leaderElectionAndWarmupRunnable implements both WarmupRunnable and LeaderElectionRunnable
type leaderElectionAndWarmupRunnable struct {
	*warmupRunnableFunc
	needLeaderElection bool
}

func newLeaderElectionAndWarmupRunnable(
	startFunc func(context.Context) error,
	warmupFunc func(context.Context) error,
	needLeaderElection bool,
) *leaderElectionAndWarmupRunnable {
	return &leaderElectionAndWarmupRunnable{
		warmupRunnableFunc: &warmupRunnableFunc{
			startFunc:  startFunc,
			warmupFunc: warmupFunc,
		},
		needLeaderElection: needLeaderElection,
	}
}

func (r leaderElectionAndWarmupRunnable) NeedLeaderElection() bool {
	return r.needLeaderElection
}
