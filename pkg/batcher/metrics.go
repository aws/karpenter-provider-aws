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

package batcher

import (
	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const (
	batcherSubsystem = "cloudprovider_batcher"
	batcherNameLabel = "batcher"
)

// SizeBuckets returns a []float64 of default threshold values for size histograms.
// Each returned slice is new and may be modified without impacting other bucket definitions.
func SizeBuckets() []float64 {
	return []float64{1, 2, 4, 5, 10, 15, 20, 25, 30, 40, 50, 60, 70, 80, 90, 100, 125, 150, 175, 200,
		225, 250, 275, 300, 350, 400, 450, 500, 550, 600, 700, 800, 900, 1000}
}

var (
	BatchWindowDuration = opmetrics.NewPrometheusHistogram(crmetrics.Registry, prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: batcherSubsystem,
		Name:      "batch_time_seconds",
		Help:      "Duration of the batching window per batcher",
		Buckets:   metrics.DurationBuckets(),
	}, []string{batcherNameLabel})
	BatchSize = opmetrics.NewPrometheusHistogram(crmetrics.Registry, prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: batcherSubsystem,
		Name:      "batch_size",
		Help:      "Size of the request batch per batcher",
		Buckets:   SizeBuckets(),
	}, []string{batcherNameLabel})
)
