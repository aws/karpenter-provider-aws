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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricNamespace      = "karpenter"
	MetricSubsystem      = "metrics_producer_reserved_capacity"
	MetricLabelName      = "name"
	MetricLabelNamespace = "namespace"
)

var (
	CpuUtilizationGaugeVec    = GaugeVec("cpu_utilization")
	MemoryUtilizationGaugeVec = GaugeVec("memory_utilization")
	PodUtilizationGaugeVec    = GaugeVec("pods_utilization")
)

// GaugeVec instantiates a parameterizable Prometheus GaugeVec that can generate
// gauges for the provided "subsystem". In Prometheus, each metric is modeled as
// a gauge with a name formatted as ${NAMESPACE}_${SUBSYSTEM}_${NAME}. Each
// gauge is parameterized by labels, which for our metrics producers, will be
// labeled by resource name and namespace.
func GaugeVec(name string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: MetricNamespace,
			Subsystem: MetricSubsystem, // refers to the suffix of the metric, or more specific descriptor of the name.
			Name:      name,
			Help:      "Metric computed by a karpenter metrics producer corresponding to name and namespace labels",
		}, []string{MetricLabelName, MetricLabelNamespace},
	)
}

func init() {
	metrics.Registry.MustRegister(
		CpuUtilizationGaugeVec,
		MemoryUtilizationGaugeVec,
		PodUtilizationGaugeVec,
	)
}
