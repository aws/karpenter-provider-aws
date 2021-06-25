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

package pod

import (
	"fmt"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func FailedToSchedule(pod *v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodScheduled && condition.Reason == v1.PodReasonUnschedulable {
			return true
		}
	}
	return false
}

// IsSchedulable returns true if the pod can schedule to the node
func IsSchedulable(pod *v1.PodSpec, node *v1.Node) bool {
	// Tolerate Taints
	if err := ToleratesTaints(pod, node.Spec.Taints...); err != nil {
		return false
	}
	// Match Node Selector labels
	if !labels.SelectorFromSet(pod.NodeSelector).Matches(labels.Set(node.Labels)) {
		return false
	}
	// TODO, support node affinity
	return true
}

// ToleratesAllTaints returns an error if the pod does not tolerate the taints
// https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/#concepts
func ToleratesTaints(spec *v1.PodSpec, taints ...v1.Taint) error {
	var err error
	for _, taint := range taints {
		taintValid := false
		for _, toleration := range spec.Tolerations {
			if toleration.ToleratesTaint(&taint) {
				taintValid = true
				continue
			}
		}
		if !taintValid {
			err = multierr.Append(err, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return err
}
