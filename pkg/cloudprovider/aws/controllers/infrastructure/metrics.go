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

package infrastructure

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter/pkg/metrics"
)

const (
	subSystem = "aws_infrastructure_controller"
)

var (
	reconcileDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Subsystem: subSystem,
			Name:      "reconcile_duration_seconds",
			Help:      "Duration of scheduling process in seconds. Broken down by provisioner and error.",
			Buckets:   metrics.DurationBuckets(),
		},
	)
	healthy = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: subSystem,
			Name:      "healthy",
			Help:      "Whether the infrastructure that should be up for this controller is in a healthy state.",
		},
	)
)

func init() {
	crmetrics.Registry.MustRegister(reconcileDuration, healthy)
}
