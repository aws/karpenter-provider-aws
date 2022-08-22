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
type Taints []Taint

type Taint struct {
	v1.Taint
	Flexible bool
}

// Tolerates returns true if the pod tolerates all taints.
func (ts Taints) Tolerates(pod *v1.Pod) (errs error) {
	for i := range ts {
		taint := ts[i]
		tolerates := false
		for _, t := range pod.Spec.Tolerations {
			tolerates = tolerates || t.ToleratesTaint(&taint)
		}
		if !tolerates {
			// See if we can loosen the taint on the node template to be more flexible
			// If there are multiple toleration values to choose from, we pick randomly
			if taint.Value == "*" {
				for _, t := range pod.Spec.Tolerations {
					// If there was a toleration with an empty key, pod would have tolerated already
					// If there was a toleration with an empty effect, pod would have tolerated already
					if t.Key == taint.Key && t.Effect == taint.Effect {
						ts[i].Value = t.Value
						tolerates = true
						break
					}
				}
			}
			// If after attempting to loosen, we still don't tolerate, then we should log an error for not tolerating
			if !tolerates {
				errs = multierr.Append(errs, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
			}
		}
	}
	return errs
}
