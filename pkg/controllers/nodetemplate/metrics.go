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

package nodetemplate

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/aws/karpenter-core/pkg/metrics"
)

const (
	subsystem = "aws_notification_controller"
)

var (
	infrastructureHealthy = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: subsystem,
			Name:      "infrastructure_healthy",
			Help:      "Whether the infrastructure provisioned by the controller is healthy.",
		},
	)
	infrastructureActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: subsystem,
			Name:      "infrastructure_active",
			Help:      "Whether the infrastructure reconciliation is currently active.",
		},
	)
)
