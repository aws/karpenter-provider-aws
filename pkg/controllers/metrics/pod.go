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
	"context"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	podName             = "name"
	podNameSpace        = "namespace"
	ownerSelfLink       = "owner"
	podHostName         = "node"
	podProvisioner      = "provisioner"
	podHostZone         = "zone"
	podHostArchitecture = "arch"
	podHostCapacityType = "capacity_type"
	podHostInstanceType = "instance_type"
	podPhase            = "phase"
)

var (
	podGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "pods",
			Name:      "state",
			Help:      "Pod state is the current state of pods. This metric can be used several ways as it is labeled by the pod name, namespace, owner, node, provisioner name, zone, architecture, capacity type, instance type and pod phase.",
		},
		podLabelNames(),
	)
)

func podLabelNames() []string {
	return []string{
		podName,
		podNameSpace,
		ownerSelfLink,
		podHostName,
		podProvisioner,
		podHostZone,
		podHostArchitecture,
		podHostCapacityType,
		podHostInstanceType,
		podPhase,
	}
}

type podMetrics struct {
	cluster *state.Cluster
	labels  []prometheus.Labels
}

func (pm *podMetrics) init(ctx context.Context) {
	logging.FromContext(ctx).Infof("Starting pod metrics collector")
	crmetrics.Registry.Register(podGaugeVec)
}

func (pm *podMetrics) update(ctx context.Context) {
	// logging.FromContext(ctx).Infof("Running pod metrics collector")
	// Clear existing labels
	// for _, l := range pm.labels {
	// 	podGaugeVec.Delete(l)
	// }
	podGaugeVec.Reset()

	pm.labels = pm.updatePodLabels()
	for _, l := range pm.labels {
		podGaugeVec.With(l).Set(float64(1))
	}

	// logging.FromContext(ctx).Infof("Completed pod metrics collector")
}

func (pm *podMetrics) updatePodLabels() []prometheus.Labels {
	labels := make(map[types.NamespacedName]prometheus.Labels)
	bindings := make(map[string][]types.NamespacedName)

	// Populate default labels and derivable labels for each pod and generate bindings map
	pm.cluster.ForEachPod(func(p *v1.Pod) bool {
		label := prometheus.Labels{}
		label[podName] = p.Name
		label[podNameSpace] = p.Namespace
		label[ownerSelfLink] = p.SelfLink
		label[podHostName] = p.Spec.NodeName
		label[podPhase] = string(p.Status.Phase)

		// Default values, will be overwritten if node exists
		label[podHostZone] = "N/A"
		label[podHostArchitecture] = "N/A"
		label[podHostCapacityType] = "N/A"
		label[podHostInstanceType] = "N/A"
		if provisionerName, ok := p.Spec.NodeSelector[v1alpha5.ProvisionerNameLabelKey]; ok {
			label[podProvisioner] = provisionerName
		} else {
			label[podProvisioner] = "N/A"
		}

		labels[types.NamespacedName{Namespace: p.Namespace, Name: p.Name}] = label
		bindings[p.Spec.NodeName] = append(bindings[p.Spec.NodeName], types.NamespacedName{Namespace: p.Namespace, Name: p.Name})

		return true
	})

	// Update labels with values from cached nodes
	pm.cluster.ForEachNode(func(n *state.Node) bool {
		if pods, ok := bindings[n.Node.Name]; ok {
			for _, pod := range pods {
				label := labels[pod]
				label[podHostZone] = n.Node.Labels[v1.LabelTopologyZone]
				label[podHostArchitecture] = n.Node.Labels[v1.LabelArchStable]
				if capacityType, ok := n.Node.Labels[v1alpha5.LabelCapacityType]; !ok {
					label[podHostCapacityType] = "N/A"
				} else {
					label[podHostCapacityType] = capacityType
				}
				label[podHostInstanceType] = n.Node.Labels[v1.LabelInstanceTypeStable]
				if provisionerName, ok := n.Node.Labels[v1alpha5.ProvisionerNameLabelKey]; !ok {
					label[podProvisioner] = "N/A"
				} else {
					label[podProvisioner] = provisionerName
				}
				labels[pod] = label
			}
		}
		return true
	})

	labelList := make([]prometheus.Labels, 0, len(labels))
	for _, label := range labels {
		labelList = append(labelList, label)
	}
	return labelList
}

func (pm *podMetrics) reset() {
	podGaugeVec.Reset()
}
