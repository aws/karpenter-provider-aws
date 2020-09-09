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
	"github.com/ellistarn/karpenter/pkg/metrics"
)

// Algorithm defines an interface for all autoscaling algorithm implementations
// These algorithms are black boxes and may use different strategies to compute desired replicas.
type Algorithm interface {
	GetDesiredReplicas(metric Metric, replicas int32) int32
}

// Metric contains both desired and observed values for a Metric.
type Metric struct {
	metrics.Metric
	TargetType  v1alpha1.MetricTargetType
	TargetValue float64
}

// For returns the autoscaling algorithm for the given spec.
func For(spec v1alpha1.HorizontalAutoscalerSpec) Algorithm {
	// For now, our default implementation will be Proportional.
	// TODO, investigate Spec.Behaviors.Algorithm or something similar to control this.
	return &Proportional{
		Spec: spec,
	}
}
