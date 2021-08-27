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

// Tolerates returns an error if the pod does not tolerate the taints
// https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/#concepts
func Tolerates(pod *v1.Pod, taints ...v1.Taint) error {
	var err error
	for _, taint := range taints {
		tolerates := false
		for _, t := range pod.Spec.Tolerations {
			tolerates = tolerates || t.ToleratesTaint(&taint)
		}
		if !tolerates {
			err = multierr.Append(err, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return err
}

func HasTaint(taints []v1.Taint, key string) bool {
	for _, taint := range taints {
		if taint.Key == key {
			return true
		}
	}
	return false
}
