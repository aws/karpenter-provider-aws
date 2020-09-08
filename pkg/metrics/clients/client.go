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
	"github.com/prometheus/client_golang/api"
)

// MetricsClient interface for all metrics implementations
type MetricsClient interface {
	// GetCurrentValues returns the current values for the set of metrics provided.
	GetCurrentValue(v1alpha1.Metrics) (float64, error)
}

// MetricsClientFactory instantiates metrics clients
type MetricsClientFactory struct {
	PrometheusClient api.Client
}

// NewPrometheusMetricsClient instantiates a metrics producer
func (m *MetricsClientFactory) NewPrometheusMetricsClient() MetricsClient {
	return &PrometheusMetricsClient{}
}
