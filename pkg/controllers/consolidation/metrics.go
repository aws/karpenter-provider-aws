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

package consolidation

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter/pkg/metrics"
)

func init() {
	crmetrics.Registry.MustRegister(consolidationDurationHistogram)
	crmetrics.Registry.MustRegister(consolidationReplacementNodeInitializedHistogram)
	crmetrics.Registry.MustRegister(consolidationNodesCreatedCounter)
	crmetrics.Registry.MustRegister(consolidationNodesTerminatedCounter)
	crmetrics.Registry.MustRegister(consolidationActionsPerformedCounter)
}

var consolidationDurationHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: "consolidation",
		Name:      "evaluation_duration_seconds",
		Help:      "Duration of the consolidation evaluation process in seconds.",
		Buckets:   metrics.DurationBuckets(),
	},
	[]string{"method"},
)

var consolidationReplacementNodeInitializedHistogram = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: "consolidation",
		Name:      "replacement_node_initialized_seconds",
		Help:      "Amount of time required for a replacement node to become initialized.",
		Buckets:   metrics.DurationBuckets(),
	})

var consolidationNodesCreatedCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: "consolidation",
		Name:      "nodes_created",
		Help:      "Number of nodes created in total by consolidation.",
	},
)
var consolidationNodesTerminatedCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: "consolidation",
		Name:      "nodes_terminated",
		Help:      "Number of nodes terminated in total by consolidation.",
	},
)
var consolidationActionsPerformedCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: "consolidation",
		Name:      "actions_performed",
		Help:      "Number of consolidation actions performed. Labeled by action.",
	},
	[]string{"action"},
)
