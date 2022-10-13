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

	"github.com/aws/karpenter/pkg/metrics"
)

const (
	subsystem = "aws_notification_controller"
)

var (
	healthy = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: subsystem,
			Name:      "healthy",
			Help:      "Whether the infrastructure provisioned by the controller is healthy.",
		},
	)
	active = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: subsystem,
			Name:      "active",
			Help:      "Whether the infrastructure reconciliation is currently active. This is based on AWSNodeTemplate reconciliation and us ref-counting more than 1 AWSNodeTemplate.",
		},
	)
)
