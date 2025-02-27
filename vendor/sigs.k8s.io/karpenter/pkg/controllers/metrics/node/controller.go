/*
Copyright The Kubernetes Authors.

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

package node

import (
	"context"
	"strings"
	"time"

	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

const (
	resourceType = "resource_type"
	nodeName     = "node_name"
	nodePhase    = "phase"
)

var (
	Allocatable = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "allocatable",
			Help:      "Node allocatable are the resources allocatable by nodes.",
		},
		nodeLabelNamesWithResourceType(),
	)
	TotalPodRequests = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "total_pod_requests",
			Help:      "Node total pod requests are the resources requested by pods bound to nodes, including the DaemonSet pods.",
		},
		nodeLabelNamesWithResourceType(),
	)
	TotalPodLimits = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "total_pod_limits",
			Help:      "Node total pod limits are the resources specified by pod limits, including the DaemonSet pods.",
		},
		nodeLabelNamesWithResourceType(),
	)
	TotalDaemonRequests = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "total_daemon_requests",
			Help:      "Node total daemon requests are the resource requested by DaemonSet pods bound to nodes.",
		},
		nodeLabelNamesWithResourceType(),
	)
	TotalDaemonLimits = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "total_daemon_limits",
			Help:      "Node total daemon limits are the resources specified by DaemonSet pod limits.",
		},
		nodeLabelNamesWithResourceType(),
	)
	SystemOverhead = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "system_overhead",
			Help:      "Node system daemon overhead are the resources reserved for system overhead, the difference between the node's capacity and allocatable values are reported by the status.",
		},
		nodeLabelNamesWithResourceType(),
	)
	Lifetime = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.NodeSubsystem,
			Name:      "current_lifetime_seconds",
			Help:      "Node age in seconds",
		},
		nodeLabelNames(),
	)
	ClusterUtilization = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: "cluster",
			Name:      "utilization_percent",
			Help:      "Utilization of allocatable resources by pod requests",
		},
		[]string{resourceType},
	)
	wellKnownLabels = getWellKnownLabels()
)

func nodeLabelNamesWithResourceType() []string {
	return append(
		nodeLabelNames(),
		resourceType,
	)
}

func nodeLabelNames() []string {
	return append(
		// WellKnownLabels includes the nodepool label, so we don't need to add it as its own item here.
		// If we do, prometheus will panic since there would be duplicate labels.
		sets.New(lo.Values(wellKnownLabels)...).UnsortedList(),
		nodeName,
		nodePhase,
	)
}

type Controller struct {
	cluster     *state.Cluster
	metricStore *metrics.Store
}

func NewController(cluster *state.Cluster) *Controller {
	return &Controller{
		cluster:     cluster,
		metricStore: metrics.NewStore(),
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "metrics.node") //nolint:ineffassign,staticcheck

	nodes := lo.Reject(c.cluster.Nodes(), func(n *state.StateNode, _ int) bool {
		return n.Node == nil
	})

	// Build per-node metrics
	metricsMap := lo.SliceToMap(nodes, func(n *state.StateNode) (string, []*metrics.StoreMetric) {
		return client.ObjectKeyFromObject(n.Node).String(), buildMetrics(n)
	})

	// Build cluster level metric
	metricsMap["clusterUtilization"] = buildClusterUtilizationMetric(nodes)

	c.metricStore.ReplaceAll(metricsMap)

	return reconcile.Result{RequeueAfter: time.Second * 5}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("metrics.node").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}

func buildClusterUtilizationMetric(nodes state.StateNodes) []*metrics.StoreMetric {

	// Aggregate resources allocated/utilized for all the nodes and pods inside the nodes
	allocatableAggregate, utilizedAggregate := corev1.ResourceList{}, corev1.ResourceList{}

	for _, node := range nodes {
		resources.MergeInto(allocatableAggregate, node.Allocatable())
		resources.MergeInto(utilizedAggregate, node.PodRequests())
	}

	res := make([]*metrics.StoreMetric, 0, len(allocatableAggregate))

	for resourceName, allocatableResource := range allocatableAggregate {

		if allocatableResource.Value() == 0 {
			// This zero check may be unnecessary. I'm erring towards caution.
			continue
		}

		utilizedResource := utilizedAggregate[resourceName]

		// Typecast to float before the calculation to maximize resolution
		utilizationPercentage := 100 * lo.Ternary(
			resourceName == corev1.ResourceCPU,
			float64(utilizedResource.MilliValue())/float64(allocatableResource.MilliValue()),
			float64(utilizedResource.Value())/float64(allocatableResource.Value()))

		res = append(res, &metrics.StoreMetric{
			GaugeMetric: ClusterUtilization,
			Value:       utilizationPercentage,
			Labels:      map[string]string{resourceType: resourceNameToString(resourceName)},
		})
	}

	return res
}

func buildMetrics(n *state.StateNode) (res []*metrics.StoreMetric) {
	for gaugeMetric, resourceList := range map[opmetrics.GaugeMetric]corev1.ResourceList{
		SystemOverhead:      resources.Subtract(n.Node.Status.Capacity, n.Node.Status.Allocatable),
		TotalPodRequests:    n.PodRequests(),
		TotalPodLimits:      n.PodLimits(),
		TotalDaemonRequests: n.DaemonSetRequests(),
		TotalDaemonLimits:   n.DaemonSetLimits(),
		Allocatable:         n.Node.Status.Allocatable,
	} {
		for resourceName, quantity := range resourceList {
			res = append(res, &metrics.StoreMetric{
				GaugeMetric: gaugeMetric,
				Value:       lo.Ternary(resourceName == corev1.ResourceCPU, float64(quantity.MilliValue())/float64(1000), float64(quantity.Value())),
				Labels:      getNodeLabelsWithResourceType(n.Node, resourceNameToString(resourceName)),
			})
		}
	}
	return append(res,
		&metrics.StoreMetric{
			GaugeMetric: Lifetime,
			Value:       time.Since(n.Node.GetCreationTimestamp().Time).Seconds(),
			Labels:      getNodeLabels(n.Node),
		})
}

func getNodeLabelsWithResourceType(node *corev1.Node, resourceTypeName string) prometheus.Labels {
	metricLabels := getNodeLabels(node)
	metricLabels[resourceType] = resourceTypeName
	return metricLabels
}

func getNodeLabels(node *corev1.Node) prometheus.Labels {
	metricLabels := map[string]string{}
	metricLabels[nodeName] = node.Name
	metricLabels[nodePhase] = string(node.Status.Phase)

	// Populate well known labels
	for wellKnownLabel, label := range wellKnownLabels {
		metricLabels[label] = node.Labels[wellKnownLabel]
	}
	return metricLabels
}

func getWellKnownLabels() map[string]string {
	labels := make(map[string]string)
	for wellKnownLabel := range v1.WellKnownLabels {
		if parts := strings.Split(wellKnownLabel, "/"); len(parts) == 2 {
			label := parts[1]
			// Reformat label names to be consistent with Prometheus naming conventions (snake_case)
			label = strings.ReplaceAll(strings.ToLower(label), "-", "_")
			labels[wellKnownLabel] = label
		}
	}
	return labels
}

func resourceNameToString(resourceName corev1.ResourceName) string {
	return strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_")
}
