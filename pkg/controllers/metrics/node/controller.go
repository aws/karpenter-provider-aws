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

package node

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	resourceType     = "resource_type"
	nodeName         = "name"
	nodeProvisioner  = "provisioner"
	nodeZone         = "zone"
	nodeArchitecture = "arch"
	nodeCapacityType = "capacity_type"
	nodeInstanceType = "instance_type"
	nodePhase        = "phase"
	nodeLabels       = "node_labels"
)

var (
	allocatableGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "allocatable",
			Help:      "Node allocatable",
		},
		getLabelNames(),
	)
	requestsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_requests",
			Help:      "Node total requests",
		},
		getLabelNames(),
	)
	limitsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_limits",
			Help:      "Node total limits",
		},
		getLabelNames(),
	)
	overheadGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "total_overhead",
			Help:      "Node total overhead",
		},
		getLabelNames(),
	)
)

func init() {
	crmetrics.Registry.MustRegister(allocatableGaugeVec)
	crmetrics.Registry.MustRegister(requestsGaugeVec)
	crmetrics.Registry.MustRegister(limitsGaugeVec)
	crmetrics.Registry.MustRegister(overheadGaugeVec)
}

func getLabelNames() []string {
	return []string{
		resourceType,
		nodeName,
		nodeProvisioner,
		nodeZone,
		nodeArchitecture,
		nodeCapacityType,
		nodeInstanceType,
		nodePhase,
		nodeLabels,
	}
}

type Controller struct {
	KubeClient client.Client
	LabelsMap  map[types.NamespacedName][]prometheus.Labels
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		KubeClient: kubeClient,
		LabelsMap:  make(map[types.NamespacedName][]prometheus.Labels),
	}
}

// Reconcile executes a termination control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("nodemetrics").With("node", req.Name))
	// Retrieve node from reconcile request
	node := &v1.Node{}
	if err := c.KubeClient.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			// Remove gauge due to node deletion
			if labels, ok := c.LabelsMap[req.NamespacedName]; ok {
				c.deleteGauges(labels)
			} else {
				logging.FromContext(ctx).Debugf("Failed to delete gauge: failed to locate labels")
			}
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Remove the previous gauge after node labels are updated
	if labelSlice, ok := c.LabelsMap[req.NamespacedName]; ok {
		c.deleteGauges(labelSlice)
	}
	c.LabelsMap[req.NamespacedName] = []prometheus.Labels{}

	if err := c.updateGauges(ctx, node); err != nil {
		logging.FromContext(ctx).Debugf("Failed to update gauges: %s", err.Error())
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	err := controllerruntime.
		NewControllerManagedBy(m).
		Named("nodemetrics").
		For(&v1.Node{}).
		Watches(
			// Reconcile all nodes related to a provisioner when it changes.
			&source.Kind{Type: &v1alpha5.Provisioner{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				nodes := &v1.NodeList{}
				if err := c.KubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{v1alpha5.ProvisionerNameLabelKey: o.GetName()})); err != nil {
					logging.FromContext(ctx).Debugf("Failed to list nodes when mapping expiration watch events, %s", err.Error())
					return requests
				}
				for _, node := range nodes.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
				}
				return requests
			}),
		).
		Watches(
			// Reconcile node when a pod assigned to it changes.
			&source.Kind{Type: &v1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				if name := o.(*v1.Pod).Spec.NodeName; name != "" {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
				}
				return requests
			}),
		).
		Complete(c)
	return err
}

func (c *Controller) deleteGauges(labelSlice []prometheus.Labels) {
	for _, labels := range labelSlice {
		allocatableGaugeVec.Delete(labels)
		requestsGaugeVec.Delete(labels)
		limitsGaugeVec.Delete(labels)
		overheadGaugeVec.Delete(labels)
	}
}

// generateLabels creates the labels using the current state of the pod
func (c *Controller) generateLabels(node *v1.Node, resourceTypeName string) (prometheus.Labels, error) {
	metricLabels := prometheus.Labels{}
	metricLabels[resourceType] = resourceTypeName
	metricLabels[nodeName] = node.GetName()
	metricLabels[nodeProvisioner] = node.Labels[v1alpha5.ProvisionerNameLabelKey]
	metricLabels[nodeZone] = node.Labels[v1.LabelTopologyZone]
	metricLabels[nodeArchitecture] = node.Labels[v1.LabelArchStable]
	metricLabels[nodeCapacityType] = node.Labels[v1alpha5.LabelCapacityType]
	metricLabels[nodeInstanceType] = node.Labels[v1.LabelInstanceTypeStable]
	metricLabels[nodePhase] = string(node.Status.Phase)
	// Add node labels
	labels, err := json.Marshal(node.GetLabels())
	if err != nil {
		return nil, fmt.Errorf("marshal pod labels: %w", err)
	}
	metricLabels[nodeLabels] = string(labels)
	return metricLabels, nil
}

func (c *Controller) updateGauges(ctx context.Context, node *v1.Node) error {

	// Calculate total requests and limits from all pods
	podlist := &v1.PodList{}
	if err := c.KubeClient.List(ctx, podlist, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return fmt.Errorf("listing pods on node %s, %w", node.Name, err)
	}
	reqs, limits := getPodsTotalRequestsAndLimits(podlist.Items)
	allocatable := node.Status.Capacity
	overheads := calculateDaemonSetOverhead(podlist.Items)
	if len(node.Status.Allocatable) > 0 {
		allocatable = node.Status.Allocatable
		// calculating system daemons overhead
		for resourceName, quantity := range allocatable {
			overhead := node.Status.Capacity[resourceName]
			overhead.Sub(quantity)
			overheads[resourceName] = overhead
		}
	}

	// Populate overhead metrics
	if err := c.insertGaugeValues(overheads, node, overheadGaugeVec); err != nil {
		logging.FromContext(ctx).Debugf("Failed to generate gauge: %w", err)
	}
	// Populate total request metrics
	if err := c.insertGaugeValues(reqs, node, requestsGaugeVec); err != nil {
		logging.FromContext(ctx).Debugf("Failed to generate gauge: %w", err)
	}
	// Populate total limits metrics
	if err := c.insertGaugeValues(limits, node, limitsGaugeVec); err != nil {
		logging.FromContext(ctx).Debugf("Failed to generate gauge: %w", err)
	}
	// Populate allocatable metrics
	if err := c.insertGaugeValues(allocatable, node, allocatableGaugeVec); err != nil {
		logging.FromContext(ctx).Debugf("Failed to generate gauge: %w", err)
	}

	return nil
}

func (c *Controller) insertGaugeValues(resources map[v1.ResourceName]resource.Quantity, node *v1.Node, gaugeVec *prometheus.GaugeVec) error {

	for resourceName, quantity := range resources {
		resourceTypeName := strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_")
		labels, err := c.generateLabels(node, resourceTypeName)
		// Register the set of labels that are generated for node
		nodeNamespacedName := types.NamespacedName{Name: node.Name}
		c.LabelsMap[nodeNamespacedName] = append(c.LabelsMap[nodeNamespacedName], labels)
		if err != nil {
			return fmt.Errorf("generate new labels: %w", err)
		}
		gauge, err := gaugeVec.GetMetricWith(labels)
		if err != nil {
			return fmt.Errorf("generate new gauge: %w", err)
		}
		if resourceName == v1.ResourceCPU {
			gauge.Set(float64(quantity.MilliValue()))
		} else {
			gauge.Set(float64(quantity.Value()))
		}
	}
	return nil
}

func calculateDaemonSetOverhead(pods []v1.Pod) v1.ResourceList {
	overheads := v1.ResourceList{}
	// calculating daemonset overhead
	daemonSetPods := []v1.Pod{}
	for _, pod := range pods {
		if pod.GetOwnerReferences()[0].Kind == "DaemonSet" {
			daemonSetPods = append(daemonSetPods, pod)
		}
	}
	daemonSetRequests, _ := getPodsTotalRequestsAndLimits(daemonSetPods)
	addResourceQuantity(daemonSetRequests, overheads)
	return overheads
}

// GetPodsTotalRequestsAndLimits calculates the total resource requests and limits for the pods.
// If pod overhead is non-nil, the pod overhead is added to the
// total container resource requests and to the total container limits which have a non-zero quantity.
func getPodsTotalRequestsAndLimits(pods []v1.Pod) (reqs map[v1.ResourceName]resource.Quantity, limits map[v1.ResourceName]resource.Quantity) {
	reqs, limits = map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
	for _, pod := range pods {
		// Excluding pods that are completed or failed
		if pod.Status.Phase == v1.PodFailed || pod.Status.Phase == v1.PodSucceeded {
			continue
		}
		for _, container := range pod.Spec.Containers {
			// Calculate Resource Requests
			addResourceQuantity(container.Resources.Requests, reqs)
			// Calculate Resource Limits
			addResourceQuantity(container.Resources.Limits, limits)
		}
		// Add overhead for running a pod to the sum of requests and to non-zero limits:
		if pod.Spec.Overhead != nil {
			// Calculate Resource Requests
			addResourceQuantity(pod.Spec.Overhead, reqs)
			// Calculate Resource Requests
			// Add to limits only when non-zero
			for resourceName, quantity := range pod.Spec.Overhead {
				if value, ok := limits[resourceName]; ok && !value.IsZero() {
					value.Add(quantity)
					limits[resourceName] = value
				}
			}
		}
	}
	return
}

func addResourceQuantity(resources v1.ResourceList, resourceMap map[v1.ResourceName]resource.Quantity) {
	for resourceName, quantity := range resources {
		if value, ok := resourceMap[resourceName]; !ok {
			resourceMap[resourceName] = quantity.DeepCopy()
		} else {
			value.Add(quantity)
			resourceMap[resourceName] = value
		}
	}

}
