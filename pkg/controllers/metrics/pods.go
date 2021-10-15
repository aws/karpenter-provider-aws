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

	provisionablePodCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystemPods,
			Name:      "provisionable_count",
			Help:      "Total count of pods that are eligible for provisioning.",
		},
	)

	podCountByPhaseProvisioner = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystemPods,
			Name:      "count",
			Help:      "Total pod count by phase and provisioner.",
		},
		[]string{
			metricLabelPhase,
			metricLabelProvisioner,
		},
	)

	runningPodCountByProvisionerZone = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystemPods,
			Name:      "running_count",
			Help:      "Total running pod count by provisioner and zone.",
		},
		[]string{
			metricLabelProvisioner,
			metricLabelZone,
		},
	)
)

func init() {
	crmetrics.Registry.MustRegister(provisionablePodCount)
	crmetrics.Registry.MustRegister(podCountByPhaseProvisioner)
	crmetrics.Registry.MustRegister(runningPodCountByProvisionerZone)
}

func publishPodCounts(provisioner string, podsByZone map[string][]v1.Pod, provisionablePods []*v1.Pod) error {
	countByPhase := make(map[v1.PodPhase]int, len(phaseValues))
	zoneValues := knownValuesForNodeLabels[nodeLabelZone]
	countByZone := make(map[string]int, len(zoneValues))

	for zone, pods := range podsByZone {
		countByZone[zone] = len(pods)

		for _, pod := range pods {
			countByPhase[pod.Status.Phase] += 1
		}
	}

	errors := make([]error, 0, len(phaseValues)+len(zoneValues))

	provisionablePodCount.Set(float64(len(provisionablePods)))
	for _, phase := range phaseValues {
		metricLabels := prometheus.Labels{
			metricLabelPhase:       strings.ToLower(string(phase)),
			metricLabelProvisioner: provisioner,
		}
		errors = append(errors, publishCount(podCountByPhaseProvisioner, metricLabels, countByPhase[phase]))
	}
	for _, zone := range zoneValues {
		metricLabels := prometheus.Labels{
			metricLabelProvisioner: provisioner,
			metricLabelZone:        zone,
		}
		errors = append(errors, publishCount(runningPodCountByProvisionerZone, metricLabels, countByZone[zone]))
	}

	return multierr.Combine(errors...)
}
