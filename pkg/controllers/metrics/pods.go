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

package metrics

import (
	"strings"

	"github.com/aws/karpenter/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	phaseValues = []v1.PodPhase{
		v1.PodFailed,
		v1.PodPending,
		v1.PodRunning,
		v1.PodSucceeded,
		v1.PodUnknown,
	}

	podCountByPhaseProvisioner = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metricSubsystemPods,
			Name:      "count",
			Help:      "Total pod count by phase and provisioner.",
		},
		[]string{
			metricLabelPhase,
			metricLabelProvisioner,
		},
	)
)

func init() {
	crmetrics.Registry.MustRegister(podCountByPhaseProvisioner)
}

func publishPodCounts(provisioner string, podList []v1.Pod) error {
	countByPhase := make(map[v1.PodPhase]int, len(phaseValues))

	for _, pod := range podList {
		countByPhase[pod.Status.Phase]++
	}

	errors := make([]error, 0, len(phaseValues))

	for _, phase := range phaseValues {
		metricLabels := prometheus.Labels{
			metricLabelPhase:       strings.ToLower(string(phase)),
			metricLabelProvisioner: provisioner,
		}
		errors = append(errors, publishCount(podCountByPhaseProvisioner, metricLabels, countByPhase[phase]))
	}

	return multierr.Combine(errors...)
}
