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

package reservedcapacity

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricNamespace      = "karpenter"
	MetricSubsystem      = "metrics_producer_reserved_capacity"
	MetricLabelName      = "name"
	MetricLabelNamespace = "namespace"
)

type MetricType string

const (
	Reserved    MetricType = "reserved"
	Capacity    MetricType = "capacity"
	Utilization MetricType = "utilization"
)

var (
	Gauges = map[v1.ResourceName]map[MetricType]*prometheus.GaugeVec{
		v1.ResourceCPU: {
			Capacity:    GaugeVec(v1.ResourceCPU, Capacity),
			Reserved:    GaugeVec(v1.ResourceCPU, Reserved),
			Utilization: GaugeVec(v1.ResourceCPU, Utilization),
		},
		v1.ResourcePods: {
			Capacity:    GaugeVec(v1.ResourcePods, Capacity),
			Reserved:    GaugeVec(v1.ResourcePods, Reserved),
			Utilization: GaugeVec(v1.ResourcePods, Utilization),
		},
		v1.ResourceMemory: {
			Capacity:    GaugeVec(v1.ResourceMemory, Capacity),
			Reserved:    GaugeVec(v1.ResourceMemory, Reserved),
			Utilization: GaugeVec(v1.ResourceMemory, Utilization),
		},
	}
)

// GaugeVec instantiates a parameterizable Prometheus GaugeVec that can generate
// gauges for the provided "subsystem". In Prometheus, each metric is modeled as
// a gauge with a name formatted as ${NAMESPACE}_${SUBSYSTEM}_${NAME}. Each
// gauge is parameterized by labels, which for our metrics producers, will be
// labeled by resource name and namespace.
func GaugeVec(resourceName v1.ResourceName, metricType MetricType) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: MetricNamespace,
			Subsystem: MetricSubsystem, // refers to the suffix of the metric, or more specific descriptor of the name.
			Name:      fmt.Sprintf("%s_%s", resourceName, metricType),
			Help:      "Metric computed by a karpenter metrics producer corresponding to name and namespace labels",
		}, []string{MetricLabelName, MetricLabelNamespace},
	)
}

func init() {
	for _, gaugeMap := range Gauges {
		for _, gauge := range gaugeMap {
			metrics.Registry.MustRegister(gauge)
		}
	}
}
