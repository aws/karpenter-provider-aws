package reconciler

import (
	"context"
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Result adds Requeue functionality back to reconcile results.
type Result struct {
	RequeueAfter time.Duration
	Requeue      bool
}

// Reconciler defines the interface for standard reconcilers
type Reconciler interface {
	Reconcile(ctx context.Context, req reconcile.Request) (Result, error)
}

// AsReconciler creates a reconciler with a default rate-limiter
func AsReconciler(reconciler Reconciler) reconcile.Reconciler {
	return AsReconcilerWithRateLimiter(
		reconciler,
		workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
	)
}

// AsReconcilerWithRateLimiter creates a reconciler with a custom rate-limiter
func AsReconcilerWithRateLimiter(
	reconciler Reconciler,
	rateLimiter workqueue.TypedRateLimiter[reconcile.Request],
) reconcile.Reconciler {
	return reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		result, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			return reconcile.Result{}, err
		}
		if result.RequeueAfter > 0 {
			rateLimiter.Forget(req)
			return reconcile.Result{RequeueAfter: result.RequeueAfter}, nil
		}
		if result.Requeue {
			return reconcile.Result{RequeueAfter: rateLimiter.When(req)}, nil
		}
		rateLimiter.Forget(req)
		return reconcile.Result{}, nil
	})
}
