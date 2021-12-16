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
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	nodeName         = "name"
	nodeProvisioner  = "provisioner"
	nodeZone         = "zone"
	nodeArchitecture = "arch"
	nodeCapacityType = "capacitytype"
	nodeInstanceType = "instancetype"
	nodePhase        = "phase"
	nodeLabels       = "nodeLabels"
)

func getLabelNames() []string {
	return []string{
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
	KubeClient   client.Client
	CoreV1Client corev1client.CoreV1Interface
	LabelsMap    map[types.NamespacedName]*prometheus.Labels
	GaugeVecMap  map[string]*prometheus.GaugeVec
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1client.CoreV1Interface) *Controller {
	newcontroller := Controller{
		KubeClient:   kubeClient,
		CoreV1Client: coreV1Client,
		LabelsMap:    make(map[types.NamespacedName]*prometheus.Labels),
		GaugeVecMap:  make(map[string]*prometheus.GaugeVec),
	}
	numPodGaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "numberofpods",
			Help:      "Auto-generated gauge: numberofpods",
		},
		getLabelNames(),
	)
	newcontroller.GaugeVecMap["numberofpods"] = numPodGaugeVec
	crmetrics.Registry.MustRegister(numPodGaugeVec)

	overheadGaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "nodes",
			Name:      "overhead",
			Help:      "Auto-generated gauge: overhead",
		},
		getLabelNames(),
	)
	newcontroller.GaugeVecMap["overhead"] = overheadGaugeVec
	crmetrics.Registry.MustRegister(overheadGaugeVec)

	return &newcontroller
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
				c.deleteGaguges(labels)
			} else {
				logging.FromContext(ctx).Errorf("Failed to delete gauge: failed to locate labels")
			}
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Remove the previous gauge after node labels are updated
	if labels, ok := c.LabelsMap[req.NamespacedName]; ok {
		c.deleteGaguges(labels)
	}
	newlabels, err := c.generateLabels(node)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate labels: %s", err.Error())
		return reconcile.Result{}, err
	}

	if err := c.updateGaguges(ctx, node, newlabels); err != nil {
		logging.FromContext(ctx).Errorf("Failed to update gauges: %s", err.Error())
		return reconcile.Result{}, err
	}
	c.LabelsMap[req.NamespacedName] = newlabels

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
					logging.FromContext(ctx).Errorf("Failed to list nodes when mapping expiration watch events, %s", err.Error())
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
		WithOptions(controller.Options{MaxConcurrentReconciles: 10000}).
		Complete(c)
	return err
}

func (c *Controller) deleteGaguges(labels *prometheus.Labels) {
	for _, gaugeVec := range c.GaugeVecMap {
		gaugeVec.Delete(*labels)
	}
}

// generateLabels creates the labels using the current state of the pod
func (c *Controller) generateLabels(node *v1.Node) (*prometheus.Labels, error) {
	metricLabels := prometheus.Labels{}

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
	return &metricLabels, nil
}

func (c *Controller) updateGaguges(ctx context.Context, node *v1.Node, labels *prometheus.Labels) error {
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
	if err := c.insertGaugeValues(overheads, "overheads", labels); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %w", err)
	}
	// Populate total request metrics
	if err := c.insertGaugeValues(reqs, "totalrequests", labels); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %w", err)
	}
	// Populate total limits metrics
	if err := c.insertGaugeValues(limits, "totallimits", labels); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %w", err)
	}
	// Populate allocatable metrics
	if err := c.insertGaugeValues(allocatable, "allocatable", labels); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %w", err)
	}
	// Add total number of pods
	gaugeVec := c.GaugeVecMap["numberofpods"]
	gauge, err := gaugeVec.GetMetricWith(*labels)
	if err != nil {
		return fmt.Errorf("generate new gauge: %w", err)
	}
	gauge.Set(float64(podlist.Size()))

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

func (c *Controller) insertGaugeValues(resources map[v1.ResourceName]resource.Quantity, prefix string, labels *prometheus.Labels) error {
	for resourceName, quantity := range resources {
		gaugeVecKey := prefix + "_" + strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_")
		gaugeVec, ok := c.GaugeVecMap[gaugeVecKey]
		if !ok {
			gaugeVec = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: "karpenter",
					Subsystem: "nodes",
					Name:      gaugeVecKey,
					Help:      "Auto-generated gauge: " + gaugeVecKey,
				},
				getLabelNames(),
			)
			c.GaugeVecMap[gaugeVecKey] = gaugeVec
			crmetrics.Registry.MustRegister(gaugeVec)
		}
		gauge, err := gaugeVec.GetMetricWith(*labels)
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
