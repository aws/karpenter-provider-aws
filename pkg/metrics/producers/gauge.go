package producers

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricNamespace      = "karpenter"
	MetricLabelName      = "name"
	MetricLabelNamespace = "namespace"
)

type MetricType string

var Gauges = make(map[string]map[MetricType]*prometheus.GaugeVec)

// GaugeVec instantiates a parameterizable Prometheus GaugeVec that can generate
// gauges for the provided "subsystem". In Prometheus, each metric is modeled as
// a gauge with a name formatted as ${NAMESPACE}_${SUBSYSTEM}_${NAME}. Each
// gauge is parameterized by labels, which for our metrics producers, will be
// labeled by resource name and namespace.
func RegisterNewGauge(subsystem string, name MetricType) *prometheus.GaugeVec {
	vec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: MetricNamespace,
			Subsystem: subsystem, // refers to the suffix of the metric, or more specific descriptor of the name.
			Name:      string(name),
			Help:      "Metric computed by a karpenter metrics producer corresponding to name and namespace labels",
		}, []string{MetricLabelName, MetricLabelNamespace},
	)
	metrics.Registry.MustRegister(vec)
	Gauges[subsystem][name] = vec
	return vec
}
