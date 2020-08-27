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
package proportional

import (
	"github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics"
)

// Autoscaler calculates desired replicas as a simple proportion of the observed metrics.
type Autoscaler struct {
	Spec v1alpha1.HorizontalAutoscalerSpec
}

// GetDesiredReplicas returns the autoscalers recommendation
func (a *Autoscaler) GetDesiredReplicas(metrics []metrics.ObservedMetric, replicas int32) int32 {
	return replicas
}
