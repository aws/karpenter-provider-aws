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

package nodemetrics

import (
	"context"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	nodename         = "name"
	nodeprovisioner  = "provisioner"
	nodezone         = "zone"
	nodearchitecture = "arch"
	nodecapacitype   = "capacitytype"
	nodeinstancetype = "instancetype"
	nodephase        = "phase"
)

func getLabelNames() []string {
	return []string{
		nodename,
		nodeprovisioner,
		nodezone,
		nodearchitecture,
		nodecapacitype,
		nodeinstancetype,
		nodephase,
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
	newlabels := c.generateLabels(ctx, node)

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
func (c *Controller) generateLabels(ctx context.Context, node *v1.Node) *prometheus.Labels {
	metricLabels := prometheus.Labels{}

	metricLabels[nodename] = node.GetName()
	metricLabels[nodeprovisioner] = node.Labels[v1alpha5.ProvisionerNameLabelKey]
	metricLabels[nodezone] = node.Labels[v1.LabelTopologyZone]
	metricLabels[nodearchitecture] = node.Labels[v1.LabelArchStable]
	metricLabels[nodecapacitype] = node.Labels[v1alpha5.LabelCapacityType]
	metricLabels[nodeinstancetype] = node.Labels[v1.LabelInstanceTypeStable]
	metricLabels[nodephase] = string(node.Status.Phase)
	return &metricLabels
}

func (c *Controller) updateGaguges(ctx context.Context, node *v1.Node, labels *prometheus.Labels) error {
	// Calculate total requests and limits from all pods
	podlist := &v1.PodList{}
	if err := c.KubeClient.List(ctx, podlist, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return fmt.Errorf("listing pods on node %s, %w", node.Name, err)
	}
	reqs, limits := getPodsTotalRequestsAndLimits(podlist.Items)
	allocatable := node.Status.Capacity
	overheads := v1.ResourceList{}
	if len(node.Status.Allocatable) > 0 {
		allocatable = node.Status.Allocatable
		// calculating system daemons overhead
		for resourceName, quantity := range allocatable {
			overhead := node.Status.Capacity[resourceName]
			overhead.Sub(quantity)
			overheads[resourceName] = overhead
		}
	}

	// calculating daemonset overhead
	daemonSetPods := []v1.Pod{}
	for _, pod := range podlist.Items {
		if pod.GetOwnerReferences()[0].Kind == "DaemonSet" {
			daemonSetPods = append(daemonSetPods, pod)
		}
	}

	daemonSetRequests, _ := getPodsTotalRequestsAndLimits(daemonSetPods)
	for resourceName, quantity := range daemonSetRequests {
		overhead := overheads[resourceName]
		overhead.Add(quantity)
		overheads[resourceName] = overhead
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
		return fmt.Errorf("generate new gauge: %s", err.Error())
	}
	gauge.Set(float64(podlist.Size()))

	return nil
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
			return fmt.Errorf("generate new gauge: %s", err.Error())
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
			for resourceName, quantity := range container.Resources.Requests {
				if value, ok := reqs[resourceName]; !ok {
					reqs[resourceName] = quantity.DeepCopy()
				} else {
					value.Add(quantity)
					reqs[resourceName] = value
				}
			}
			// Calculate Resource Limits
			for resourceName, quantity := range container.Resources.Limits {
				if value, ok := limits[resourceName]; !ok {
					limits[resourceName] = quantity.DeepCopy()
				} else {
					value.Add(quantity)
					limits[resourceName] = value
				}
			}
		}
		// Add overhead for running a pod to the sum of requests and to non-zero limits:
		if pod.Spec.Overhead != nil {
			// Calculate Resource Requests
			for resourceName, quantity := range pod.Spec.Overhead {
				if value, ok := reqs[resourceName]; !ok {
					reqs[resourceName] = quantity.DeepCopy()
				} else {
					value.Add(quantity)
					reqs[resourceName] = value
				}
			}
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
