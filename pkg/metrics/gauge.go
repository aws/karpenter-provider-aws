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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricNamespace      = "karpenter"
	MetricLabelName      = "name"
	MetricLabelNamespace = "namespace"
)

var Gauges = make(map[string]map[string]*prometheus.GaugeVec)

// GaugeVec instantiates a parameterizable Prometheus GaugeVec that can generate
// gauges for the provided "subsystem". In Prometheus, each metric is modeled as
// a gauge with a name formatted as ${NAMESPACE}_${SUBSYSTEM}_${NAME}. Each
// gauge is parameterized by labels, which for our metrics producers, will be
// labeled by resource name and namespace.
func RegisterNewGauge(subsystem string, name string) {
	vec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: MetricNamespace,
			Subsystem: subsystem, // refers to the suffix of the metric, or more specific descriptor of the name.
			Name:      name,
			Help:      "Metric computed by a karpenter metrics producer corresponding to name and namespace labels",
		}, []string{MetricLabelName, MetricLabelNamespace},
	)
	metrics.Registry.MustRegister(vec)
	if Gauges[subsystem] == nil {
		Gauges[subsystem] = make(map[string]*prometheus.GaugeVec)
	}

	Gauges[subsystem][name] = vec
}
