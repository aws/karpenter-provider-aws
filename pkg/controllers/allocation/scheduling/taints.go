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

type Taints []v1.Taint

func (ts Taints) WithPod(pod *v1.Pod) Taints {
	for _, toleration := range pod.Spec.Tolerations {
		// Only OpEqual is supported. OpExists does not make sense for
		// provisioning -- in theory we could create a taint on the node with a
		// random string, but it's unclear use case this would accomplish.
		if toleration.Operator != v1.TolerationOpEqual {
			continue
		}
		var generated []v1.Taint
		// Use effect if defined, otherwise taint all effects
		if toleration.Effect != "" {
			generated = []v1.Taint{{Key: toleration.Key, Value: toleration.Value, Effect: toleration.Effect}}
		} else {
			generated = []v1.Taint{
				{Key: toleration.Key, Value: toleration.Value, Effect: v1.TaintEffectNoSchedule},
				{Key: toleration.Key, Value: toleration.Value, Effect: v1.TaintEffectNoExecute},
			}
		}
		// Only add taints that do not already exist on constraints
		for _, taint := range generated {
			if !ts.Has(taint) {
				ts = append(ts, taint)
			}
		}
	}
	return ts
}

// Has returns true if taints has a taint for the given key
func (ts Taints) Has(taint v1.Taint) bool {
	for _, t := range ts {
		if t.Key == taint.Key && t.Effect == taint.Effect {
			return true
		}
	}
	return false
}

// Tolerates returns true if the pod tolerates all taints
func (ts Taints) Tolerates(pod *v1.Pod) (errs error) {
	for i := range ts {
		taint := ts[i]
		tolerates := false
		for _, t := range pod.Spec.Tolerations {
			tolerates = tolerates || t.ToleratesTaint(&taint)
		}
		if !tolerates {
			errs = multierr.Append(errs, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return errs
}
