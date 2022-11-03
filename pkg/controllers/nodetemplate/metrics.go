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
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter-core/pkg/metrics"
)

const subSystem = "nodetemplate_infrastructure"

var (
	infrastructureCreateDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Subsystem: subSystem,
			Name:      "create_time_seconds",
			Help:      "Length of time to create infrastructure.",
			Buckets:   metrics.DurationBuckets(),
		},
	)
	infrastructureDeleteDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Subsystem: subSystem,
			Name:      "delete_time_seconds",
			Help:      "Length of time to delete infrastructure.",
			Buckets:   metrics.DurationBuckets(),
		},
	)
)

func init() {
	crmetrics.Registry.MustRegister(infrastructureCreateDuration, infrastructureDeleteDuration)
}
