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

func GaugeVec(subsystem string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      MetricPrefix,
			Namespace: MetricNamespace,
			Subsystem: subsystem,
			Help:      "Metric computed by karpenter a metrics producer correspodning to name and namespace labels",
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
