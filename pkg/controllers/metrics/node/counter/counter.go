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

package counter

import (
	"context"
	"reflect"
	"strings"

	karpenterapi "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	coreapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricArchLabel                    = "arch"
	metricConditionDiskPressureLabel   = "diskpressure"
	metricConditionMemoryPressureLabel = "memorypressure"
	metricConditionPIDPressureLabel    = "pidpressure"
	metricConditionReadyLabel          = "ready"
	metricInstanceTypeLabel            = "instancetype"
	metricTopologyRegionLabel          = "region"
	metricTopologyZoneLabel            = "zone"
	metricOsLabel                      = "os"
)

var (
	nodeCountGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.KarpenterNamespace,
			Subsystem: "cluster",
			Name:      "node_count",
			Help:      "Count of cluster nodes. Broken out by topology and status.",
		},
		[]string{
			metrics.ProvisionerLabel,
			metricArchLabel,
			metricConditionDiskPressureLabel,
			metricConditionMemoryPressureLabel,
			metricConditionPIDPressureLabel,
			metricConditionReadyLabel,
			metricInstanceTypeLabel,
			metricTopologyRegionLabel,
			metricTopologyZoneLabel,
			metricOsLabel,
		},
	)

	prometheusLabelsFor = make(map[types.NamespacedName]prometheus.Labels)

	conditionTypeToMetricLabel = map[coreapi.NodeConditionType]string{
		coreapi.NodeDiskPressure:   metricConditionDiskPressureLabel,
		coreapi.NodeMemoryPressure: metricConditionMemoryPressureLabel,
		coreapi.NodePIDPressure:    metricConditionPIDPressureLabel,
		coreapi.NodeReady:          metricConditionReadyLabel,
	}
)

func init() {
	crmetrics.Registry.MustRegister(nodeCountGaugeVec)
}

// UpdateCount updates the emitted metric based on the node's current status relative to the
// past status. If the data for `node` cannot be populated then `nil` should be passed as the
// argument.
func UpdateCount(ctx context.Context, name types.NamespacedName, node *coreapi.Node) {
	currLabels := getLabels(node)
	pastLabels, isKnown := prometheusLabelsFor[name]
	switch {
	case !isKnown && node != nil:
		handleCreatedNode(ctx, name, currLabels)
	case isKnown && node == nil:
		handleDeletedNode(ctx, name, pastLabels)
	case isKnown && node != nil:
		handleUpdatedNode(ctx, name, currLabels, pastLabels)
	default: // An unknown node was deleted.
	}
}

func handleCreatedNode(ctx context.Context, name types.NamespacedName, labels prometheus.Labels) {
	prometheusLabelsFor[name] = labels

	if err := incrementNodeCount(labels); err != nil {
		logging.FromContext(ctx).Warnf("Failed to update count metric for new node [labels=%s]: error=%s", labels, err.Error())
	}
}

func handleDeletedNode(ctx context.Context, name types.NamespacedName, labels prometheus.Labels) {
	delete(prometheusLabelsFor, name)

	if err := decrementNodeCount(labels); err != nil {
		logging.FromContext(ctx).Warnf("Failed to update count metric for deleted node [labels=%s]: error=%s", labels, err.Error())
	}
}

func handleUpdatedNode(ctx context.Context, name types.NamespacedName, currLabels prometheus.Labels, pastLabels prometheus.Labels) {
	// Only report node updates that affect tracked dimensions.
	if reflect.DeepEqual(currLabels, pastLabels) {
		return
	}

	prometheusLabelsFor[name] = currLabels

	if err := decrementNodeCount(pastLabels); err != nil {
		logging.FromContext(ctx).Warnf("Failed to decrement previous count for updated node [labels=%s]: error=%s", pastLabels, err.Error())
	}
	if err := incrementNodeCount(currLabels); err != nil {
		logging.FromContext(ctx).Warnf("Failed to increment current count for updated node [labels=%s]: error=%s", currLabels, err.Error())
	}
}

func getLabels(node *coreapi.Node) prometheus.Labels {
	labels := make(prometheus.Labels)
	if node == nil {
		return labels
	}

	labels[metrics.ProvisionerLabel] = node.Labels[karpenterapi.ProvisionerNameLabelKey]
	labels[metricArchLabel] = node.Labels[coreapi.LabelArchStable]
	labels[metricInstanceTypeLabel] = node.Labels[coreapi.LabelInstanceTypeStable]
	labels[metricTopologyRegionLabel] = node.Labels[coreapi.LabelTopologyRegion]
	labels[metricTopologyZoneLabel] = node.Labels[coreapi.LabelTopologyZone]
	labels[metricOsLabel] = node.Labels[coreapi.LabelOSStable]

	for _, c := range node.Status.Conditions {
		if labelName, found := conditionTypeToMetricLabel[c.Type]; found {
			labels[labelName] = strings.ToLower(string(c.Status))
		}
	}

	return labels
}

func incrementNodeCount(labels prometheus.Labels) error {
	return updateNodeCount(labels, prometheus.Gauge.Inc)
}

func decrementNodeCount(labels prometheus.Labels) error {
	return updateNodeCount(labels, prometheus.Gauge.Dec)
}

func updateNodeCount(labels prometheus.Labels, update func(prometheus.Gauge)) error {
	nodeCount, err := nodeCountGaugeVec.GetMetricWith(labels)
	if err != nil {
		return err
	}

	update(nodeCount)
	return nil
}
