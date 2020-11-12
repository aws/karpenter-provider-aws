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
	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/awslabs/karpenter/pkg/metrics"
	"github.com/awslabs/karpenter/pkg/utils/log"

	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func NewFactoryOrDie(prometheusURI string) *Factory {
	client, err := api.NewClient(api.Config{Address: prometheusURI})
	log.PanicIfError(err, "Failed to instantiate metrics client factory")
	return &Factory{
		PrometheusClient: prometheusv1.NewAPI(client),
	}
}

// Factory instantiates metrics clients
type Factory struct {
	PrometheusClient prometheusv1.API
}

// For returns a metrics client for the given source type
func (m *Factory) For(metric v1alpha1.Metric) metrics.Client {
	if metric.Prometheus != nil {
		return m.NewPrometheusMetricsClient()
	}
	log.InvariantViolated("Failed to instantiate metrics client, no metric type specified")
	return nil
}

// NewPrometheusMetricsClient instantiates a metrics producer
func (m *Factory) NewPrometheusMetricsClient() metrics.Client {
	return &PrometheusMetricsClient{
		Client: m.PrometheusClient,
	}
}
