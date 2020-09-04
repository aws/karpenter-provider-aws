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

package algorithms

import (
	"github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
)

// Proportional calculates desired replicas as a simple proportion of the observed metrics.
type Proportional struct {
	// Spec defines autoscaling rules for the object
	Spec v1alpha1.HorizontalAutoscalerSpec
}

// GetDesiredReplicas returns the autoscalers recommendation
func (a *Proportional) GetDesiredReplicas(metric Metric, replicas int32) int32 {
	return replicas
}
