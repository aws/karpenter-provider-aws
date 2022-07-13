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
	"sort"
	"strings"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	resourceType    = "resource_type"
	nodeName        = "node_name"
	nodeProvisioner = "provisioner"
	nodePhase       = "phase"
)

var (
	allocatableGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "allocatable",
			Help:      "Node allocatable are the resources allocatable by nodes.",
		},
		nodeLabelNames(),
	)

	podRequestsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_pod_requests",
			Help:      "Node total pod requests are the resources requested by non-DaemonSet pods bound to nodes.",
		},
		nodeLabelNames(),
	)

	podLimitsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_pod_limits",
			Help:      "Node total pod limits are the resources specified by non-DaemonSet pod limits.",
		},
		nodeLabelNames(),
	)

	daemonRequestsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_daemon_limits",
			Help:      "Node total daemon requests are the resource requested by DaemonSet pods bound to nodes.",
		},
		nodeLabelNames(),
	)

	daemonLimitsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_daemon_requests",
			Help:      "Node total daemon limits are the resources specified by DaemonSet pod limits.",
		},
		nodeLabelNames(),
	)

	overheadGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "system_overhead",
			Help:      "Node system daemon overhead are the resources reserved for system overhead, the difference between the node's capacity and allocatable values are reported by the status.",
		},
		nodeLabelNames(),
	)

	wellKnownLabels = getWellKnownLabels()
)

func nodeLabelNames() []string {
	return append(
		lo.Values(wellKnownLabels),
		resourceType,
		nodeName,
		nodeProvisioner,
		nodePhase,
	)
}

func forEachGaugeVec(f func(*prometheus.GaugeVec)) {
	for _, gauge := range []*prometheus.GaugeVec{
		allocatableGaugeVec,
		podRequestsGaugeVec,
		podLimitsGaugeVec,
		daemonRequestsGaugeVec,
		daemonLimitsGaugeVec,
		overheadGaugeVec,
	} {
		f(gauge)
	}
}

func init() {
	forEachGaugeVec(func(g *prometheus.GaugeVec) {
		metrics.Registry.MustRegister(g)
	})
}

type NodeScraper struct {
	cluster       *state.Cluster
	gaugeLabelMap map[*prometheus.GaugeVec]map[string]prometheus.Labels
}

func NewNodeScraper(cluster *state.Cluster) *NodeScraper {
	return &NodeScraper{
		cluster: cluster,
		gaugeLabelMap: func() map[*prometheus.GaugeVec]map[string]prometheus.Labels {
			m := make(map[*prometheus.GaugeVec]map[string]prometheus.Labels)
			forEachGaugeVec(func(g *prometheus.GaugeVec) {
				m[g] = make(map[string]prometheus.Labels)
			})
			return m
		}(),
	}
}

func (ns *NodeScraper) Scrape(ctx context.Context) {
	gaugeLabelSet := make(map[*prometheus.GaugeVec]sets.String)
	forEachGaugeVec(func(g *prometheus.GaugeVec) {
		gaugeLabelSet[g] = sets.NewString()
	})

	// Populate metrics
	ns.cluster.ForEachNode(func(n *state.Node) bool {
		for gaugeVec, resourceList := range map[*prometheus.GaugeVec]v1.ResourceList{
			overheadGaugeVec:       ns.getSystemOverhead(n.Node),
			podRequestsGaugeVec:    resources.Subtract(n.PodTotalRequests, n.DaemonSetRequested),
			podLimitsGaugeVec:      resources.Subtract(n.PodTotalLimits, n.DaemonSetLimits),
			daemonRequestsGaugeVec: n.DaemonSetRequested,
			daemonLimitsGaugeVec:   n.DaemonSetLimits,
			allocatableGaugeVec:    n.Node.Status.Allocatable,
		} {
			for _, labels := range ns.set(gaugeVec, n.Node, resourceList) {
				ns.gaugeLabelMap[gaugeVec][labelsToString(labels)] = labels
				gaugeLabelSet[gaugeVec].Insert(labelsToString(labels))
			}
		}
		return true
	})

	// Remove stale gauges
	forEachGaugeVec(func(g *prometheus.GaugeVec) {
		for labelsKey := range sets.NewString(lo.Keys(ns.gaugeLabelMap[g])...).Difference(gaugeLabelSet[g]) {
			g.Delete(ns.gaugeLabelMap[g][labelsKey])
		}
	})
}

// set sets the value for the node gauge and returns a slice of the labels for the gauges set
func (ns *NodeScraper) set(gaugeVec *prometheus.GaugeVec, node *v1.Node, resourceList v1.ResourceList) []prometheus.Labels {
	gaugeLabels := []prometheus.Labels{}
	for resourceName, quantity := range resourceList {
		// Reformat resource type to be consistent with Prometheus naming conventions (snake_case)
		resourceLabels := ns.getNodeLabels(node, strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_"))
		gaugeLabels = append(gaugeLabels, resourceLabels)
		if resourceName == v1.ResourceCPU {
			gaugeVec.With(resourceLabels).Set(float64(quantity.MilliValue()) / float64(1000))
		} else {
			gaugeVec.With(resourceLabels).Set(float64(quantity.Value()))
		}
	}
	return gaugeLabels
}

func (ns *NodeScraper) getSystemOverhead(node *v1.Node) v1.ResourceList {
	systemOverhead := v1.ResourceList{}
	if len(node.Status.Allocatable) > 0 {
		// calculating system daemons overhead
		for resourceName, quantity := range node.Status.Allocatable {
			overhead := node.Status.Capacity[resourceName]
			overhead.Sub(quantity)
			systemOverhead[resourceName] = overhead
		}
	}
	return systemOverhead
}

func (ns *NodeScraper) getNodeLabels(node *v1.Node, resourceTypeName string) prometheus.Labels {
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
			// Reformat label names to be consistent with Prometheus naming conventions (snake_case)
			label = strings.ReplaceAll(strings.ToLower(string(label)), "-", "_")
			labels[wellKnownLabel] = label
		}
	}
	return labels
}

func labelsToString(labels prometheus.Labels) string {
	keyValues := lo.Entries(labels)
	sort.Slice(keyValues, func(i, j int) bool {
		return keyValues[i].Key < keyValues[j].Key
	})

	return strings.Join(lo.Map(keyValues, func(entry lo.Entry[string, string], _ int) string {
		return fmt.Sprintf("%s=\"%s\"", entry.Key, entry.Value)
	}), ",")
}
