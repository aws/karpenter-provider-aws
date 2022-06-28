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

package state

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	resourceType        = "k_resource_type"
	nodeName            = "k_node_name"
	nodeProvisioner     = "k_provisioner"
	nodeZone            = "k_zone"
	nodeArchitecture    = "k_arch"
	nodeCapacityType    = "k_capacity_type"
	nodeInstanceType    = "k_instance_type"
	nodePhase           = "k_phase"
	podName             = "k_name"
	podNameSpace        = "k_namespace"
	ownerSelfLink       = "k_owner"
	podHostName         = "k_node"
	podProvisioner      = "k_provisioner"
	podHostZone         = "k_zone"
	podHostArchitecture = "k_arch"
	podHostCapacityType = "k_capacity_type"
	podHostInstanceType = "k_instance_type"
	podPhase            = "k_phase"
	provisionerName     = "k_provisioner"
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

	// Pod Metrics
	podGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "pods",
			Name:      "state",
			Help:      "Pod state is the current state of pods. This metric can be used several ways as it is labeled by the pod name, namespace, owner, node, provisioner name, zone, architecture, capacity type, instance type and pod phase.",
		},
		podLabelNames(),
	)

	// Provisioner Metrics
	limitGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "limit",
			Help:      "The Provisioner Limits are the limits specified on the provisioner that restrict the quantity of resources provisioned. Labeled by provisioner name and resource type.",
		},
		provisionerLabelNames(),
	)
	usageGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "usage",
			Help:      "The Provisioner Usage is the amount of resources that have been provisioned by a particular provisioner. Labeled by provisioner name and resource type.",
		},
		provisionerLabelNames(),
	)
	usagePctGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "usage_pct",
			Help:      "The Provisioner Usage Percentage is the percentage of each resource used based on the resources provisioned and the limits that have been configured in the range [0,100].  Labeled by provisioner name and resource type.",
		},
		provisionerLabelNames(),
	)
)

func init() {
	// crmetrics.Registry.MustRegister(allocatableGaugeVec)
	// crmetrics.Registry.MustRegister(podRequestsGaugeVec)
	// crmetrics.Registry.MustRegister(podLimitsGaugeVec)
	// crmetrics.Registry.MustRegister(daemonRequestsGaugeVec)
	// crmetrics.Registry.MustRegister(daemonLimitsGaugeVec)
	// crmetrics.Registry.MustRegister(overheadGaugeVec)

	crmetrics.Registry.MustRegister(podGaugeVec)

	// crmetrics.Registry.MustRegister(limitGaugeVec)
	// crmetrics.Registry.MustRegister(usageGaugeVec)
	// crmetrics.Registry.MustRegister(usagePctGaugeVec)
}

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

func provisionerLabelNames() []string {
	return []string{
		resourceType,
		provisionerName,
	}
}
