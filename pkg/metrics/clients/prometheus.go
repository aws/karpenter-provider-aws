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
	"context"
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusMetricsClient is a metrics client for Prometheus
type PrometheusMetricsClient struct {
	Client v1.API
}

// GetCurrentValue for the metric
func (c *PrometheusMetricsClient) GetCurrentValue(metric v1alpha1.Metric) (metrics.Metric, error) {
	response, _, err := c.Client.Query(context.Background(), metric.Prometheus.Query, time.Now().Round(time.Hour))
	if err != nil {
		return metrics.Metric{}, errors.Wrapf(err, "Failed to query metric %s", metric.Prometheus.Query)
	}
	vector, ok := response.(model.Vector)
	if !ok {
		return metrics.Metric{}, errors.Errorf(
			"Invalid metric type for query %s: prometheus value type, expected %s and got %s",
			metric.Prometheus.Query,
			model.ValVector,
			response.Type(),
		)
	}

	if vector.Len() != 1 {
		return metrics.Metric{}, errors.New("Invalid metric type for query: expected instant vector and got vector")
	}

	return metrics.Metric{Value: float64(vector[0].Value)}, nil
}
