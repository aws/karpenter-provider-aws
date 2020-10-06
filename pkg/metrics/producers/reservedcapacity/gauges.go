package reservedcapacity

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricPrefix         = "karpenter"
	MetricNamespace      = "metrics_producer_reserved_capacity"
	MetricLabelName      = "name"
	MetricLabelNamespace = "namespace"
)

var (
	CpuGaugeVec    = GaugeVec("cpu")
	MemoryGaugeVec = GaugeVec("memory")
	PodsGaugeVec   = GaugeVec("pods")
)

// GaugeVec instantiates a parameterizable Prometheus GaugeVec that can generate
// gauges for the provided "subsystem". In Prometheus, each metric is modeled as
// a gauge with a name formatted as ${NAME}_${NAMESPACE}_${SUBSYSTEM}. Each
// gauge is parameterized by labels, which for our metrics producers, will be
// labeled by resource name and namespace.
func GaugeVec(subsystem string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      MetricPrefix,
			Namespace: MetricNamespace,
			Subsystem: subsystem, // refers to the suffix of the metric, or more specific descriptor of the name.
			Help:      "Metric computed by a karpenter metrics producer corresponding to name and namespace labels",
		}, []string{MetricLabelName, MetricLabelNamespace},
	)
}

func init() {
	metrics.Registry.MustRegister(
		CpuGaugeVec,
		MemoryGaugeVec,
		PodsGaugeVec,
	)
}
