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

package scheduling

import (
	"fmt"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
)

// Taints is a decorated alias type for []v1.Taint
type Taints []v1.Taint

// Tolerates returns true if the pod tolerates all taints.
func (ts Taints) Tolerates(pod *v1.Pod) (errs error) {
	for i := range ts {
		taint := Taint(ts[i])
		if !taint.Tolerates(pod) {
			errs = multierr.Append(errs, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return errs
}

// Taint is a decorated alias for v1.Taint
type Taint v1.Taint

func (t Taint) Tolerates(pod *v1.Pod) bool {
	tolerates := false
	for _, tol := range pod.Spec.Tolerations {
		taint := v1.Taint(t)
		tolerates = tolerates || toleratesTaintWithWildcard(tol, &taint)
	}
	return tolerates
}

func toleratesTaintWithWildcard(t v1.Toleration, taint *v1.Taint) bool {
	// Special case the wildcard scenario, where we tolerate the taint
	// if key and effect match (or are empty) and the operator is a valid value
	if taint.Value == "*" {
		if len(t.Effect) > 0 && t.Effect != taint.Effect {
			return false
		}
		if len(t.Key) > 0 && t.Key != taint.Key {
			return false
		}
		switch t.Operator {
		case "", v1.TolerationOpExists, v1.TolerationOpEqual:
			return true
		default:
			return false
		}
	}
	return t.ToleratesTaint(taint)
}
