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
