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

package lifecycle

import (
	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

var InstanceTerminationDurationSeconds = opmetrics.NewPrometheusHistogram(
	crmetrics.Registry,
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: metrics.NodeClaimSubsystem,
		Name:      "instance_termination_duration_seconds",
		Help:      "Duration of CloudProvider Instance termination in seconds.",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 11), //The threshold values generated here are 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024
	},
	[]string{metrics.NodePoolLabel},
)

var NodeClaimTerminationDurationSeconds = opmetrics.NewPrometheusHistogram(
	crmetrics.Registry,
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: metrics.NodeClaimSubsystem,
		Name:      "termination_duration_seconds",
		Help:      "Duration of NodeClaim termination in seconds.",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 12)}, //The threshold values generated here are 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024. 2048
	[]string{metrics.NodePoolLabel},
)
