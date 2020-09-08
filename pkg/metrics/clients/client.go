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

package clients

import (
	"github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics"
	"github.com/prometheus/client_golang/api"
	"go.uber.org/zap"
)

// MetricsClient interface for all metrics implementations
type MetricsClient interface {
	// GetCurrentValues returns the current values for the set of metrics provided.
	GetCurrentValue(v1alpha1.Metric) (metrics.Metric, error)
}

// Factory instantiates metrics clients
type Factory struct {
	PrometheusClient api.Client
}

// For returns a metrics client for the given source type
func (m *Factory) For(metricSourceType v1alpha1.MetricSourceType) MetricsClient {
	switch metricSourceType {
	case v1alpha1.PrometheusMetricSourceType:
		return m.NewPrometheusMetricsClient()
	}
	zap.S().Fatalf("Failed to instantiate metrics client: unexpected MetricsSourceType %s", metricSourceType)
	return nil
}

// NewPrometheusMetricsClient instantiates a metrics producer
func (m *Factory) NewPrometheusMetricsClient() MetricsClient {
	return &PrometheusMetricsClient{}
}
