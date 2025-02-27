package publisher

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	smithy "github.com/aws/smithy-go/middleware"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/common"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

// WithPrometheusMetrics wraps an aws.Config, injecting prometheus metric firing
// into the middleware to track request count totals, latencies, and retry counts
func WithPrometheusMetrics(cfg aws.Config, r prometheus.Registerer) aws.Config {
	p := NewPrometheusPublisher(r)

	// See https://aws.github.io/aws-sdk-go-v2/docs/middleware/#attaching-middleware-to-all-clients for
	// more detail on attaching middleware to all clients associated with a config
	cfg.APIOptions = append(cfg.APIOptions, func(s *smithy.Stack) error {
		return middleware.WithMetricMiddlewares(p, http.DefaultClient)(s)
	})
	return cfg
}

// PrometheusPublisher is a MetricPublisher implementation that publishes metrics to the Prometheus registry.
type PrometheusPublisher struct {
	registry prometheus.Registerer
}

// NewPrometheusPublisher creates a new PrometheusPublisher with the specified namespace and serializer.
func NewPrometheusPublisher(r prometheus.Registerer) *PrometheusPublisher {
	common.MustRegisterMetrics(r)
	return &PrometheusPublisher{
		registry: r,
	}
}

// PostRequestMetrics publishes request metrics to the prometheus registry.
func (p *PrometheusPublisher) PostRequestMetrics(data *awsmetrics.MetricData) error {
	common.TotalRequests.With(common.RequestLabels(data.ServiceID, data.OperationName, data.StatusCode)).Inc()
	common.RequestLatency.With(common.RequestLabels(data.ServiceID, data.OperationName, data.StatusCode)).Observe(float64(data.APICallDuration.Milliseconds()) / 1000.0)
	common.RetryCount.With(common.RequestLabels(data.ServiceID, data.OperationName, data.StatusCode)).Observe(float64(data.RetryCount))

	for _, attempt := range data.Attempts {
		common.TotalRequestAttempts.With(common.RequestLabels(data.ServiceID, data.OperationName, attempt.StatusCode)).Inc()
		common.RequestAttemptLatency.With(common.RequestLabels(data.ServiceID, data.OperationName, data.StatusCode)).Observe(float64(attempt.ServiceCallDuration.Milliseconds()) / 1000.0)
	}
	return nil
}

// PostStreamMetrics publishes the stream metrics to the prometheus registry.
func (p *PrometheusPublisher) PostStreamMetrics(data *awsmetrics.MetricData) error {
	common.TotalRequests.With(common.RequestLabels(data.ServiceID, data.OperationName, data.StatusCode)).Inc()
	return nil
}
