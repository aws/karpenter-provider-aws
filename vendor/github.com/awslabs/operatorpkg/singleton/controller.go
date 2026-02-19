package singleton

import (
	"context"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// RequeueImmediately is a constant that allows for immediate RequeueAfter when you want to run your
	// singleton controller as hot as possible in a fast requeuing loop
	RequeueImmediately = 1 * time.Nanosecond
)

// Reconciler defines the interface for singleton reconcilers
type Reconciler interface {
	Reconcile(ctx context.Context) (reconciler.Result, error)
}
type reconcilerAdapter struct {
	Reconciler
}

func (r *reconcilerAdapter) Reconcile(ctx context.Context, _ reconcile.Request) (reconciler.Result, error) {
	return r.Reconciler.Reconcile(ctx)
}

// In response to Requeue: True being deprecated via: https://github.com/kubernetes-sigs/controller-runtime/pull/3107/files
// This uses a bucket and per item delay but the item will be the same because the key is the controller name.
// This implements the same behavior as Requeue: True.

// AsReconciler creates a controller-runtime reconciler from a singleton reconciler
func AsReconciler(rec Reconciler) reconcile.Reconciler {
	adapter := &reconcilerAdapter{Reconciler: rec}
	return reconciler.AsReconciler(adapter)
}

// Source creates a source for singleton controllers
func Source() source.Source {
	eventSource := make(chan event.GenericEvent, 1)
	eventSource <- event.GenericEvent{}
	return source.Channel(eventSource, handler.Funcs{
		GenericFunc: func(_ context.Context, _ event.GenericEvent, queue workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			queue.Add(reconcile.Request{})
		},
	})
}
