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
	"fmt"
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusMetricsClient is a metrics client for Prometheus
type PrometheusMetricsClient struct {
	Client v1.API
}

// GetCurrentValue for the metric
func (c *PrometheusMetricsClient) GetCurrentValue(metric v1alpha1.Metric) (metrics.Metric, error) {
	value, _, err := c.Client.Query(context.Background(), metric.Prometheus.Query, time.Now())
	if err != nil {
		return metrics.Metric{}, fmt.Errorf("request failed for query %s, %w", metric.Prometheus.Query, err)
	}
	if err := c.validateResponseType(value); err != nil {
		return metrics.Metric{}, fmt.Errorf("invalid response for query %s, %w", metric.Prometheus.Query, err)
	}
	return metrics.Metric{Value: float64(value.(model.Vector)[0].Value)}, nil
}

func (c *PrometheusMetricsClient) validateResponseType(value model.Value) error {
	vector, ok := value.(model.Vector)
	if !ok {
		return fmt.Errorf("expected %s and got %s", model.ValVector, value.Type())
	}
	if vector.Len() != 1 {
		return fmt.Errorf("expected instant vector and got vector: %s", log.Pretty(value))
	}
	return nil
}
