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
	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const (
	interruptionSubsystem = "interruption"
	messageTypeLabel      = "message_type"
)

var (
	ReceivedMessages = opmetrics.NewPrometheusCounter(
		crmetrics.Registry,
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: interruptionSubsystem,
			Name:      "received_messages_total",
			Help:      "Count of messages received from the SQS queue. Broken down by message type and whether the message was actionable.",
		},
		[]string{messageTypeLabel},
	)
	DeletedMessages = opmetrics.NewPrometheusCounter(
		crmetrics.Registry,
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: interruptionSubsystem,
			Name:      "deleted_messages_total",
			Help:      "Count of messages deleted from the SQS queue.",
		},
		[]string{},
	)
	MessageLatency = opmetrics.NewPrometheusHistogram(
		crmetrics.Registry,
		prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Subsystem: interruptionSubsystem,
			Name:      "message_queue_duration_seconds",
			Help:      "Amount of time an interruption message is on the queue before it is processed by karpenter.",
			Buckets:   metrics.DurationBuckets(),
		},
		[]string{},
	)
)
