/*
Copyright The Kubernetes Authors.

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

package termination

import (
	"time"

	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const dayDuration = time.Hour * 24

var (
	DurationSeconds = opmetrics.NewPrometheusSummary(
		crmetrics.Registry,
		prometheus.SummaryOpts{
			Namespace:  metrics.Namespace,
			Subsystem:  metrics.NodeSubsystem,
			Name:       "termination_duration_seconds",
			Help:       "The time taken between a node's deletion request and the removal of its finalizer",
			Objectives: metrics.SummaryObjectives(),
		},
		[]string{metrics.NodePoolLabel},
	)
	NodesDrainedTotal = opmetrics.NewPrometheusCounter(
		crmetrics.Registry,
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "drained_total",
			Help:      "The total number of nodes drained by Karpenter",
		},
		[]string{metrics.NodePoolLabel},
	)
	NodeLifetimeDurationSeconds = opmetrics.NewPrometheusHistogram(
		crmetrics.Registry,
		prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "lifetime_duration_seconds",
			Help:      "The lifetime duration of the nodes since creation.",
			Buckets: []float64{

				(time.Minute * 15).Seconds(),
				(time.Minute * 30).Seconds(),
				(time.Minute * 45).Seconds(),

				time.Hour.Seconds(),
				(time.Hour * 2).Seconds(),
				(time.Hour * 4).Seconds(),
				(time.Hour * 6).Seconds(),
				(time.Hour * 8).Seconds(),
				(time.Hour * 10).Seconds(),
				(time.Hour * 12).Seconds(),
				(time.Hour * 16).Seconds(),
				(time.Hour * 20).Seconds(),

				dayDuration.Seconds(),
				(dayDuration * 2).Seconds(),
				(dayDuration * 3).Seconds(),
				(dayDuration * 5).Seconds(),
				(dayDuration * 10).Seconds(),
				(dayDuration * 15).Seconds(),
				(dayDuration * 20).Seconds(),
				(dayDuration * 25).Seconds(),
				(dayDuration * 30).Seconds(),
			},
		},
		[]string{metrics.NodePoolLabel},
	)
)
