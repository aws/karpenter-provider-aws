/*
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

package result

import (
	"context"
	"math"
	"time"

	"go.uber.org/multierr"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RetryIfError logs any errors and requeues if not nil. Supports multierr unwrapping.
func RetryIfError(ctx context.Context, err error) (reconcile.Result, error) {
	for _, err := range multierr.Errors(err) {
		logging.FromContext(ctx).Errorf("Failed reconciliation, %s", err.Error())
	}
	return reconcile.Result{Requeue: err != nil}, nil
}

// MinResult returns the result that wants to requeue the soonest
func MinResult(results ...reconcile.Result) (result reconcile.Result) {
	min := time.Duration(math.MaxInt64)
	for _, r := range results {
		if r.IsZero() {
			continue
		}
		if r.RequeueAfter < min {
			min = r.RequeueAfter
			result.RequeueAfter = min
			result.Requeue = true
		}
	}
	return
}
