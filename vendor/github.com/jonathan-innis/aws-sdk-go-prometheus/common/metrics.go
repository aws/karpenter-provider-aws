package common

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
)

var (
	labels        = []string{"service", "action", "code"}
	TotalRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aws_sdk_go_request_total",
		Help: "The total number of AWS SDK Go requests",
	}, labels)

	TotalRequestAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aws_sdk_go_request_attempt_total",
		Help: "The total number of AWS SDK Go request attempts",
	}, labels)

	RequestLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aws_sdk_go_request_duration_seconds",
		Help:    "Latency of AWS SDK Go requests",
		Buckets: prometheus.ExponentialBuckets(0.01, 1.5, 20),
	}, labels)

	RequestAttemptLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aws_sdk_go_request_attempt_duration_seconds",
		Help:    "Latency of AWS SDK Go request attempts",
		Buckets: prometheus.ExponentialBuckets(0.01, 1.5, 20),
	}, labels)

	RetryCount = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "aws_sdk_go_request_retry_count",
		Help: "The total number of AWS SDK Go retry attempts per request",
		Buckets: []float64{
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		},
	}, labels)
)

func RequestLabels(service string, action string, statusCode int) prometheus.Labels {
	return prometheus.Labels{
		"service": service,
		"action":  action,
		"code":    fmt.Sprint(statusCode),
	}
}

func MustRegisterMetrics(registry prometheus.Registerer) {
	for _, c := range []prometheus.Collector{TotalRequests, TotalRequestAttempts, RequestLatency, RequestAttemptLatency, RetryCount} {
		lo.Must0(registry.Register(c))
	}
}
