package result

import (
	"context"

	"go.uber.org/multierr"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RetryIfError logs any errors and requeues if not nil. Supports multierr unwrapping.
func RetryIfError(ctx context.Context, err error) (reconcile.Result, error) {
	for _, err := range multierr.Errors(err) {
		logging.FromContext(ctx).Errorf("Failed allocation, %s", err.Error())
	}
	return reconcile.Result{Requeue: err != nil}, nil
}
