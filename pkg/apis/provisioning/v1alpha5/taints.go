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

package v1alpha5

import (
	"fmt"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
)

// Taints is a decorated alias type for []v1.Taint
type Taints []v1.Taint

// Has returns true if taints has a taint for the given key and effect
func (ts Taints) Has(taint v1.Taint) bool {
	for _, t := range ts {
		if t.MatchTaint(&taint) {
			return true
		}
	}
	return false
}

// HasKey returns true if taints has a taint for the given key
func (ts Taints) HasKey(taintKey string) bool {
	for _, t := range ts {
		if t.Key == taintKey {
			return true
		}
	}
	return false
}

// Tolerates returns true if the pod tolerates all taints. 'additional' are extra tolerations for things like startup
// taints or the standard not-ready taint applied to nodes we have launched.
func (ts Taints) Tolerates(pod *v1.Pod, additional ...v1.Toleration) (errs error) {
	for i := range ts {
		taint := ts[i]
		tolerates := false
		for _, t := range pod.Spec.Tolerations {
			tolerates = tolerates || t.ToleratesTaint(&taint)
		}
		for _, t := range additional {
			tolerates = tolerates || t.ToleratesTaint(&taint)
		}
		if !tolerates {
			errs = multierr.Append(errs, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return errs
}

// TaintToToleration converts a taint to a toleration that tolerates the specified taint.
func TaintToToleration(taint v1.Taint) v1.Toleration {
	return v1.Toleration{
		Key:      taint.Key,
		Operator: v1.TolerationOpEqual,
		Value:    taint.Value,
		Effect:   taint.Effect,
	}
}
