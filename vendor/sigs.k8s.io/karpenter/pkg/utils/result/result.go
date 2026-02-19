/*
Copyright The Kubernetes Authors.

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
	"math"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Min returns the result that wants to requeue the soonest
func Min(results ...reconcile.Result) (result reconcile.Result) {
	min := time.Duration(math.MaxInt64)
	for _, r := range results {
		if r.IsZero() {
			continue
		}
		if r.RequeueAfter < min {
			min = r.RequeueAfter
			result.RequeueAfter = min
			//nolint:staticcheck
			result.Requeue = true
		}
	}
	return
}
