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

import "github.com/prometheus/client_golang/prometheus"

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
