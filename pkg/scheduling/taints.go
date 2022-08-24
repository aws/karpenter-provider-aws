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

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

// Taints is a decorated alias type for []v1.Taint
type Taints []v1.Taint

// Tolerates returns true if the pod tolerates all taints.
func (ts Taints) Tolerates(pod *v1.Pod) (errs error) {
	for i := range ts {
		taint := ts[i]
		tolerates := false
		for _, t := range pod.Spec.Tolerations {
			tolerates = tolerates || toleratesTaint(t, &taint)
		}
		if !tolerates {
			errs = multierr.Append(errs, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return errs
}

// Copied from https://github.com/kubernetes/api/blob/master/core/v1/toleration.go
// with added handling for wildcard character in the taint
func toleratesTaint(t v1.Toleration, taint *v1.Taint) bool {
	if len(t.Effect) > 0 && t.Effect != taint.Effect {
		return false
	}

	if len(t.Key) > 0 && t.Key != taint.Key {
		return false
	}

	// TODO: Use proper defaulting when Toleration becomes a field of PodSpec
	switch t.Operator {
	// empty operator means Equal
	case "", v1.TolerationOpEqual:
		// This line is edited to handle the case where a value is still flexible
		// It will be constrained in the next step if there is a toleration that has
		// an operator=Equal
		return t.Value == taint.Value || taint.Value == v1alpha5.TaintWildcardValue
	case v1.TolerationOpExists:
		return true
	default:
		return false
	}
}
