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
	interruptionSubsystem  = "interruption"
	messageTypeLabel       = "message_type"
	actionTypeLabel        = "action_type"
	terminationReasonLabel = "interruption"
)

var (
	receivedMessages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: interruptionSubsystem,
			Name:      "received_messages",
			Help:      "Count of messages received from the SQS queue. Broken down by message type and whether the message was actionable.",
		},
		[]string{messageTypeLabel},
	)
	deletedMessages = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: interruptionSubsystem,
			Name:      "deleted_messages",
			Help:      "Count of messages deleted from the SQS queue.",
		},
	)
	messageLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Subsystem: interruptionSubsystem,
			Name:      "message_latency_time_seconds",
			Help:      "Length of time between message creation in queue and an action taken on the message by the controller.",
			Buckets:   metrics.DurationBuckets(),
		},
	)
	actionsPerformed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: interruptionSubsystem,
			Name:      "actions_performed",
			Help:      "Number of notification actions performed. Labeled by action",
		},
		[]string{actionTypeLabel},
	)
)

func init() {
	crmetrics.Registry.MustRegister(receivedMessages, deletedMessages, messageLatency, actionsPerformed)
}
