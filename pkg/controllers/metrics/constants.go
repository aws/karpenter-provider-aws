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
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/metrics"
	v1 "k8s.io/api/core/v1"
)

const (
	controllerName  = "Metrics"
	requeueInterval = 10 * time.Second

	metricNamespace         = metrics.KarpenterNamespace
	metricSubsystemCapacity = "capacity"
	metricSubsystemPods     = "pods"

	metricLabelArch         = "arch"
	metricLabelInstanceType = "instancetype"
	metricLabelOS           = "os"
	metricLabelPhase        = "phase"
	metricLabelProvisioner  = metrics.ProvisionerLabel
	metricLabelZone         = "zone"

	nodeLabelArch         = v1.LabelArchStable
	nodeLabelInstanceType = v1.LabelInstanceTypeStable
	nodeLabelOS           = v1.LabelOSStable
	nodeLabelZone         = v1.LabelTopologyZone

	nodeConditionTypeReady = v1.NodeReady
)

var (
	nodeLabelProvisioner = v1alpha4.ProvisionerNameLabelKey

	knownValuesForNodeLabels = v1alpha4.WellKnownLabels
)
