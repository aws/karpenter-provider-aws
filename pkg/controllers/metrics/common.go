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
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
)

const (
	controllerName = "metrics"

	metricSubsystemCapacity = "capacity"
	metricSubsystemPods     = "pods"

	metricLabelArch         = "arch"
	metricLabelInstanceType = "instancetype"
	metricLabelPhase        = "phase"
	metricLabelProvisioner  = metrics.ProvisionerLabel
	metricLabelZone         = "zone"

	nodeLabelArch         = v1.LabelArchStable
	nodeLabelInstanceType = v1.LabelInstanceTypeStable
	nodeLabelZone         = v1.LabelTopologyZone

	nodeConditionTypeReady = v1.NodeReady
)

var nodeLabelProvisioner = v1alpha5.ProvisionerNameLabelKey

func publishCount(gaugeVec *prometheus.GaugeVec, labels prometheus.Labels, count int) error {
	gauge, err := gaugeVec.GetMetricWith(labels)
	if err != nil {
		return err
	}
	gauge.Set(float64(count))
	return nil
}
