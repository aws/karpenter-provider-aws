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
	"fmt"
	"strings"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	resourceType    = "resource_type"
	nodeName        = "node_name"
	nodeProvisioner = "provisioner"
	nodePhase       = "phase"
)

var (
	allocatableGaugeVec    = newNodeGaugeVec("allocatable", "Node allocatable are the resources allocatable by nodes. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.")
	podRequestsGaugeVec    = newNodeGaugeVec("total_pod_requests", "Node total pod requests are the resources requested by non-DaemonSet pods bound to nodes.  Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.")
	podLimitsGaugeVec      = newNodeGaugeVec("total_pod_limits", "Node total pod limits are the resources specified by non-DaemonSet pod limits. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.")
	daemonRequestsGaugeVec = newNodeGaugeVec("total_daemon_requests", "Node total daemon requests are the resource requested by DaemonSet pods bound to nodes. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.")
	daemonLimitsGaugeVec   = newNodeGaugeVec("total_daemon_limits", "Node total pod limits are the resources specified by DaemonSet pod limits. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.")
	overheadGaugeVec       = newNodeGaugeVec("system_overhead", "Node system daemon overhead are the resources reserved for system overhead, the difference between the node's capacity and allocatable values are reported by the status. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.")
	wellKnownLabels        = getWellKnownLabels()
)

func newNodeGaugeVec(name, help string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      name,
			Help:      help,
		},
		nodeLabelNames(),
	)
}

func nodeLabelNames() []string {
	labels := []string{
		resourceType,
		nodeName,
		nodeProvisioner,
		nodePhase, // NOTE: deprecated
	}

	for _, l := range wellKnownLabels {
		labels = append(labels, l)
	}

	return labels
}

type nodeScraper struct {
	cluster  *state.Cluster
	labelMap map[string]map[*prometheus.GaugeVec][]prometheus.Labels
}

func newNodeCollector(cluster *state.Cluster) *nodeScraper {
	return &nodeScraper{
		cluster:  cluster,
		labelMap: make(map[string]map[*prometheus.GaugeVec][]prometheus.Labels),
	}
}

func (ns *nodeScraper) getName() string {
	return "nodes"
}

func (ns *nodeScraper) init(ctx context.Context) {
	for _, gauge := range []*prometheus.GaugeVec{
		allocatableGaugeVec,
		podRequestsGaugeVec,
		podLimitsGaugeVec,
		daemonRequestsGaugeVec,
		daemonLimitsGaugeVec,
		overheadGaugeVec,
	} {
		crmetrics.Registry.MustRegister(gauge)
	}
}

func (ns *nodeScraper) update(ctx context.Context) {
	nodes := make(map[string]struct{})
	ns.cluster.ForEachNode(func(n *state.Node) bool {
		if _, ok := ns.labelMap[n.Node.Name]; !ok {
			logging.FromContext(ctx).Debugf("Tracking new node: %s", n.Node.Name)
			ns.labelMap[n.Node.Name] = make(map[*prometheus.GaugeVec][]prometheus.Labels)
		}
		nodes[n.Node.Name] = struct{}{}

		podRequests := resources.Subtract(n.PodTotalRequests, n.DaemonSetRequested)
		podLimits := resources.Subtract(n.PodTotalLimits, n.DaemonSetLimits)
		allocatable := n.Node.Status.Capacity
		if len(n.Node.Status.Allocatable) > 0 {
			allocatable = n.Node.Status.Allocatable
		}

		// Populate  metrics
		var err error
		for gaugeVec, resourceList := range map[*prometheus.GaugeVec]v1.ResourceList{
			overheadGaugeVec:       ns.getSystemOverhead(n.Node),
			podRequestsGaugeVec:    podRequests,
			podLimitsGaugeVec:      podLimits,
			daemonRequestsGaugeVec: n.DaemonSetRequested,
			daemonLimitsGaugeVec:   n.DaemonSetLimits,
			allocatableGaugeVec:    allocatable,
		} {
			err = multierr.Append(err, ns.set(resourceList, n.Node, gaugeVec))
		}

		if err != nil {
			logging.FromContext(ctx).Errorf("Failed to generate gauges for %s: %s", n.Node.Name, err)
		}

		return true
	})

	ns.cleanup(ctx, nodes)
}

func (ns *nodeScraper) reset() {
	for _, gauge := range []*prometheus.GaugeVec{
		allocatableGaugeVec,
		podRequestsGaugeVec,
		podLimitsGaugeVec,
		daemonRequestsGaugeVec,
		daemonLimitsGaugeVec,
		overheadGaugeVec,
	} {
		gauge.Reset()
	}
}

func (ns *nodeScraper) cleanup(ctx context.Context, existingNodes map[string]struct{}) {
	nodesToRemove := []string{}
	for nodeName := range ns.labelMap {
		if _, ok := existingNodes[nodeName]; !ok {
			nodesToRemove = append(nodesToRemove, nodeName)
		}
	}

	// Remove all gauges associated with removed node
	for _, node := range nodesToRemove {
		gaugeMap := ns.labelMap[node]
		for gaugeVec, labelSet := range gaugeMap {
			for _, labels := range labelSet {
				gaugeVec.Delete(labels)
			}
		}
		delete(ns.labelMap, node)
	}

	if len(nodesToRemove) > 0 {
		logging.FromContext(ctx).Debugf("Removed the following node gauges: %s", strings.Join(nodesToRemove, ", "))
	}
}

// set sets the value for the node gauge
func (ns *nodeScraper) set(resourceList v1.ResourceList, node *v1.Node, gaugeVec *prometheus.GaugeVec) error {
	for resourceName, quantity := range resourceList {
		resourceTypeName := strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_")
		labels := ns.getNodeLabels(node, resourceTypeName)
		ns.labelMap[node.Name][gaugeVec] = append(ns.labelMap[node.Name][gaugeVec], labels)

		gauge, err := gaugeVec.GetMetricWith(labels)
		if err != nil {
			return fmt.Errorf("generating new gauge: %w", err)
		}
		if resourceName == v1.ResourceCPU {
			gauge.Set(float64(quantity.MilliValue()) / float64(1000))
		} else {
			gauge.Set(float64(quantity.Value()))
		}
	}
	return nil
}

func (ns *nodeScraper) getSystemOverhead(node *v1.Node) v1.ResourceList {
	systemOverheads := v1.ResourceList{}
	if len(node.Status.Allocatable) > 0 {
		// calculating system daemons overhead
		for resourceName, quantity := range node.Status.Allocatable {
			overhead := node.Status.Capacity[resourceName]
			overhead.Sub(quantity)
			systemOverheads[resourceName] = overhead
		}
	}
	return systemOverheads
}

func (ns *nodeScraper) getNodeLabels(node *v1.Node, resourceTypeName string) prometheus.Labels {
	metricLabels := prometheus.Labels{}
	metricLabels[resourceType] = resourceTypeName
	metricLabels[nodeName] = node.GetName()
	if provisionerName, ok := node.Labels[v1alpha5.ProvisionerNameLabelKey]; !ok {
		metricLabels[nodeProvisioner] = "N/A"
	} else {
		metricLabels[nodeProvisioner] = provisionerName
	}
	metricLabels[nodePhase] = string(node.Status.Phase)

	// Populate well known labels
	for wellKnownLabel, label := range wellKnownLabels {
		if value, ok := node.Labels[wellKnownLabel]; !ok {
			metricLabels[label] = "N/A"
		} else {
			metricLabels[label] = value
		}
	}

	return metricLabels
}

func getWellKnownLabels() map[string]string {
	labels := make(map[string]string)
	for wellKnownLabel := range v1alpha5.WellKnownLabels {
		if parts := strings.Split(wellKnownLabel, "/"); len(parts) == 2 {
			label := parts[1]
			label = strings.ReplaceAll(strings.ToLower(string(label)), "-", "_")
			labels[wellKnownLabel] = label
		}
	}
	return labels
}
