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

package polling

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/aws/karpenter/pkg/metrics"
)

func (t *Controller) healthyMetric() prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: t.r.MetricsSubsystemName(),
			Name:      "healthy",
			Help:      "Whether the controller is in a healthy state.",
		},
	)
}

func (t *Controller) activeMetric() prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: t.r.MetricsSubsystemName(),
			Name:      "active",
			Help:      "Whether the controller is active.",
		},
	)
}

func (t *Controller) triggeredCountMetric() prometheus.Counter {
	return prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: t.r.MetricsSubsystemName(),
			Name:      "trigger_count",
			Help:      "A counter of the number of times this controller has been triggered.",
		},
	)
}
