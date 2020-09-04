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

package v1alpha1

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Metric is modeled after https://godoc.org/k8s.io/api/autoscaling/v2beta2#MetricSpec
// +optional
type Metric struct {
	// type is the type of metric source.  It should be one of "Object",
	// "Replicas" or "Resource", each mapping to a matching field in the object.
	Type MetricSourceType `json:"type"`
	// +optional
	Prometheus *PrometheusMetricSource `json:"prometheus,omitempty"`
}

// PrometheusMetricSource defines a metric in Prometheus
type PrometheusMetricSource struct {
	Query  string       `json:"query"`
	Target MetricTarget `json:"target"`
}

// MetricTarget defines the target value, average value, or average utilization of a specific metric
type MetricTarget struct {
	// type represents whether the metric type is Utilization, Value, or AverageValue
	Type MetricTargetType `json:"type"`
	// value is the target value of the metric (as a quantity).
	// +optional
	Value *resource.Quantity `json:"value,omitempty"`
	// averageValue is the target value of the average of the
	// metric across all relevant pods (as a quantity)
	// +optional
	AverageValue *resource.Quantity `json:"averageValue,omitempty"`
	// averageUtilization is the target value of the average of the
	// resource metric across all relevant pods, represented as a percentage of
	// the requested value of the resource for the pods.
	// Currently only valid for Resource metric source type
	// +optional
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
}

// MetricTargetType specifies the type of metric being targeted, and should be either "Value", "AverageValue", or "Utilization"
type MetricTargetType string

// Enum for MetricTargetType
const (
	UtilizationMetricType  MetricTargetType = "Utilization"
	ValueMetricType        MetricTargetType = "Value"
	AverageValueMetricType MetricTargetType = "AverageValue"
)

// MetricSourceType indicates the type of metric.
type MetricSourceType string

// MetricSourceType enum definition
const (
	PrometheusMetricSourceType MetricSourceType = "PrometheusMetricSourceType"
)

// GetTarget returns the target of the metric source.
func (m *Metric) GetTarget() MetricTarget {
	switch m.Type {
	case PrometheusMetricSourceType:
		return m.Prometheus.Target
	}
	zap.S().Fatalf("Unrecognized metric type while retrieving target for %v", m)
	return MetricTarget{}
}
