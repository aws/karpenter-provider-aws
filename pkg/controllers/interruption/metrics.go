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

package interruption

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter-core/pkg/metrics"
)

const (
	subsystem              = "aws_interruption_controller"
	messageTypeLabel       = "message_type"
	actionableTypeLabel    = "actionable"
	actionTypeLabel        = "action_type"
	terminationReasonLabel = "interruption"
)

var (
	active = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: subsystem,
			Name:      "active",
			Help:      "Whether the message polling is currently active.",
		},
	)
	receivedMessages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: subsystem,
			Name:      "received_messages",
			Help:      "Count of messages received from the SQS queue. Broken down by message type and whether the message was actionable.",
		},
		[]string{messageTypeLabel},
	)
	deletedMessages = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: subsystem,
			Name:      "deleted_messages",
			Help:      "Count of messages deleted from the SQS queue.",
		},
	)
	actionsPerformed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: subsystem,
			Name:      "actions_performed",
			Help:      "Number of notification actions performed. Labeled by action",
		},
		[]string{actionTypeLabel},
	)
)

func init() {
	crmetrics.Registry.MustRegister(receivedMessages, deletedMessages, actionsPerformed)
}
