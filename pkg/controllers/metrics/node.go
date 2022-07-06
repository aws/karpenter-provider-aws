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

// TODO: (Optimization) Don't reset gauges on every scrape

package metrics

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	resourceType     = "resource_type"
	nodeName         = "node_name"
	nodeProvisioner  = "provisioner"
	nodeZone         = "zone"
	nodeArchitecture = "arch"
	nodeCapacityType = "capacity_type"
	nodeInstanceType = "instance_type"
	nodePhase        = "phase"
	provisionerName  = "provisioner"
)

var (
	allocatableGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "allocatable",
			Help:      "Node allocatable are the resources allocatable by nodes. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.",
		},
		nodeLabelNames(),
	)
	podRequestsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_pod_requests",
			Help:      "Node total pod requests are the resources requested by non-DaemonSet pods bound to nodes.  Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.",
		},
		nodeLabelNames(),
	)
	podLimitsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_pod_limits",
			Help:      "Node total pod limits are the resources specified by non-DaemonSet pod limits. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.",
		},
		nodeLabelNames(),
	)
	daemonRequestsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_daemon_requests",
			Help:      "Node total daemon requests are the resource requested by DaemonSet pods bound to nodes. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.",
		},
		nodeLabelNames(),
	)
	daemonLimitsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_daemon_limits",
			Help:      "Node total pod limits are the resources specified by DaemonSet pod limits. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.",
		},
		nodeLabelNames(),
	)
	overheadGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "system_overhead",
			Help:      "Node system daemon overhead are the resources reserved for system overhead, the difference between the node's capacity and allocatable values are reported by the status. Labeled by provisioner name, node name, zone, architecture, capacity type, instance type, node phase and resource type.",
		},
		nodeLabelNames(),
	)
)

func nodeLabelNames() []string {
	return []string{
		resourceType,
		nodeName,
		nodeProvisioner,
		nodeZone,
		nodeArchitecture,
		nodeCapacityType,
		nodeInstanceType,
		nodePhase,
	}
}

type nodeScraper struct {
	cluster *state.Cluster
}

func newNodeCollector(cluster *state.Cluster) *nodeScraper {
	return &nodeScraper{cluster: cluster}
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
		crmetrics.Registry.Register(gauge)
	}
}

func (ns *nodeScraper) update(ctx context.Context) {
	ns.reset()

	ns.cluster.ForEachNode(func(n *state.Node) bool {
		podRequests := resources.Subtract(n.PodTotalRequests, n.DaemonSetRequested)
		podLimits := resources.Subtract(n.PodTotalLimits, n.DaemonSetLimits)
		// podRequests := n.DaemonSetRequested
		// podLimits := n.DaemonSetLimits
		allocatable := n.Node.Status.Capacity
		if len(n.Node.Status.Allocatable) > 0 {
			allocatable = n.Node.Status.Allocatable
		}

		// Populate  metrics
		for gaugeVec, resourceList := range map[*prometheus.GaugeVec]v1.ResourceList{
			overheadGaugeVec:       ns.getSystemOverhead(n.Node),
			podRequestsGaugeVec:    podRequests,
			podLimitsGaugeVec:      podLimits,
			daemonRequestsGaugeVec: n.DaemonSetRequested,
			daemonLimitsGaugeVec:   n.DaemonSetLimits,
			allocatableGaugeVec:    allocatable,
		} {
			if err := ns.set(resourceList, n.Node, gaugeVec); err != nil {
				logging.FromContext(ctx).Errorf("Failed to generate gauge: %s", err)
			}
		}

		return true
	})
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

// set sets the value for the node gauge
func (ns *nodeScraper) set(resourceList v1.ResourceList, node *v1.Node, gaugeVec *prometheus.GaugeVec) error {
	for resourceName, quantity := range resourceList {
		resourceTypeName := strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_")
		labels := ns.getNodeLabels(node, resourceTypeName)

		gauge, err := gaugeVec.GetMetricWith(labels)
		if err != nil {
			return fmt.Errorf("generate new gauge: %w", err)
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
	metricLabels[nodeZone] = node.Labels[v1.LabelTopologyZone]
	metricLabels[nodeArchitecture] = node.Labels[v1.LabelArchStable]
	if capacityType, ok := node.Labels[v1alpha5.LabelCapacityType]; !ok {
		metricLabels[nodeCapacityType] = "N/A"
	} else {
		metricLabels[nodeCapacityType] = capacityType
	}
	metricLabels[nodeInstanceType] = node.Labels[v1.LabelInstanceTypeStable]
	metricLabels[nodePhase] = string(node.Status.Phase)
	return metricLabels
}
