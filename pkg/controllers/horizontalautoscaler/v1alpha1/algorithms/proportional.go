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
	"math"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"go.uber.org/zap"
)

// Proportional calculates desired replicas as a simple proportion of the observed metrics.
type Proportional struct {
	// Spec defines autoscaling rules for the object
	Spec v1alpha1.HorizontalAutoscalerSpec
}

// GetDesiredReplicas returns the autoscaler's recommendation.
// This function mirrors the implementation of HPA's autoscaling algorithm
// https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details
func (a *Proportional) GetDesiredReplicas(metric Metric, replicas int32) int32 {
	ratio := (metric.Metric.Value / metric.TargetValue)
	proportional := float64(replicas) * ratio
	switch metric.TargetType {
	// Proportional
	case v1alpha1.ValueMetricType:
		return int32(math.Ceil(proportional))
	// Proportional average, divided by number of replicas
	case v1alpha1.AverageValueMetricType:
		return int32(math.Ceil(ratio))
	// Proportional percentage, multiplied by 100
	case v1alpha1.UtilizationMetricType:
		return int32(math.Ceil(proportional * 100))
	default:
		zap.S().Errorf("Unexpected TargetType %s for ", metric.TargetType)
		return replicas
	}
}
