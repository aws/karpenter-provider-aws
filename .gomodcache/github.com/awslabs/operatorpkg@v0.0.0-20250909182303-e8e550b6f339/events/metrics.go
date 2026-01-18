package events

import (
	pmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func eventTotalMetric(objectName string) pmetrics.CounterMetric {
	return pmetrics.NewPrometheusCounter(
		metrics.Registry,
		prometheus.CounterOpts{
			Namespace: pmetrics.Namespace,
			Subsystem: objectName,
			Name:      "event_total",
			Help:      "The total of events of a given type for an object.",
		},
		[]string{
			pmetrics.LabelType,
			pmetrics.LabelReason,
		},
	)
}
