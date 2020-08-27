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

package metrics

import (
	"k8s.io/api/autoscaling/v2beta2"
)

// Metric is exposed by the provider to the metrics server
type Metric struct {
	Name   string
	Labels map[string]string
	Value  float64
	v2beta2.MetricTargetType
}

// ObservedMetric contains the current value of the metric and the desired target configured by the user.
type ObservedMetric struct {
	Current Metric
	Target  Metric
}

// Producer interface for all metrics implementations
type Producer interface {
	// GetCurrentValues returns the current values for the set of metrics provided.
	GetCurrentValues() ([]Metric, error)
}
