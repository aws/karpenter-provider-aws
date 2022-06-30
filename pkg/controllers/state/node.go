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
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	podutil "github.com/aws/karpenter/pkg/utils/pod"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const nodeControllerName = "node-state"

// NodeController reconciles nodes for the purpose of maintaining state regarding nodes that is expensive to compute.
type NodeController struct {
	kubeClient client.Client
	cluster    *Cluster
	labelMap   sync.Map
}

// NewNodeController constructs a controller instance
func NewNodeController(kubeClient client.Client, cluster *Cluster) *NodeController {
	return &NodeController{
		kubeClient: kubeClient,
		cluster:    cluster,
	}
}

func (c *NodeController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(nodeControllerName).With("node", req.Name))

	c.cleanup(req.NamespacedName)

	node := &v1.Node{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			// notify cluster state of the node deletion
			c.cluster.deleteNode(req.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if err := c.cluster.updateNode(ctx, node); err != nil {
		return reconcile.Result{}, err
	}

	if err := c.record(ctx, node); err != nil {
		logging.FromContext(ctx).Errorf("Failed to update gauges: %s", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true, RequeueAfter: stateRetryPeriod}, nil
}

// TODO: Determine if additional watches conflict w/ cluster state
func (c *NodeController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(nodeControllerName).
		For(&v1.Node{}).
		Watches(
			// Reconcile all nodes related to a provisioner when it changes.
			&source.Kind{Type: &v1alpha5.Provisioner{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				nodes := &v1.NodeList{}
				if err := c.kubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{v1alpha5.ProvisionerNameLabelKey: o.GetName()})); err != nil {
					logging.FromContext(ctx).Errorf("Failed to list nodes when mapping expiration watch events, %s", err)
					return requests
				}
				for _, node := range nodes.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
				}
				return requests
			}),
		).
		Watches(
			// Reconcile nodes where pods have changed
			&source.Kind{Type: &v1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				if name := o.(*v1.Pod).Spec.NodeName; name != "" {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
				}
				return requests
			}),
		).
		Complete(c)
}

func (c *NodeController) cleanup(nodeNamespacedName types.NamespacedName) {
	if labelSet, ok := c.labelMap.Load(nodeNamespacedName); ok {
		for _, labels := range labelSet.([]prometheus.Labels) {
			allocatableGaugeVec.Delete(labels)
			podRequestsGaugeVec.Delete(labels)
			podLimitsGaugeVec.Delete(labels)
			daemonRequestsGaugeVec.Delete(labels)
			daemonLimitsGaugeVec.Delete(labels)
			overheadGaugeVec.Delete(labels)
		}
	}
	c.labelMap.Store(nodeNamespacedName, []prometheus.Labels{})
}

// labels creates the labels using the current state of the pod
func (c *NodeController) labels(node *v1.Node, resourceTypeName string) prometheus.Labels {
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

func (c *NodeController) record(ctx context.Context, node *v1.Node) error {
	podlist := &v1.PodList{}
	if err := c.kubeClient.List(ctx, podlist, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return fmt.Errorf("listing pods on node %s, %w", node.Name, err)
	}
	var daemons, pods []*v1.Pod
	for index := range podlist.Items {
		if podutil.IsOwnedByDaemonSet(&podlist.Items[index]) {
			daemons = append(daemons, &podlist.Items[index])
		} else {
			pods = append(pods, &podlist.Items[index])
		}
	}
	podRequest := resources.RequestsForPods(pods...)
	podLimits := resources.LimitsForPods(pods...)
	daemonRequest := resources.RequestsForPods(daemons...)
	daemonLimits := resources.LimitsForPods(daemons...)
	systemOverhead := getSystemOverhead(node)
	allocatable := node.Status.Capacity
	if len(node.Status.Allocatable) > 0 {
		allocatable = node.Status.Allocatable
	}
	// Populate  metrics
	for gaugeVec, resourceList := range map[*prometheus.GaugeVec]v1.ResourceList{
		overheadGaugeVec:       systemOverhead,
		podRequestsGaugeVec:    podRequest,
		podLimitsGaugeVec:      podLimits,
		daemonRequestsGaugeVec: daemonRequest,
		daemonLimitsGaugeVec:   daemonLimits,
		allocatableGaugeVec:    allocatable,
	} {
		if err := c.set(resourceList, node, gaugeVec); err != nil {
			logging.FromContext(ctx).Errorf("Failed to generate gauge: %s", err)
		}
	}
	return nil
}

func getSystemOverhead(node *v1.Node) v1.ResourceList {
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

// set sets the value for the node gauge
func (c *NodeController) set(resourceList v1.ResourceList, node *v1.Node, gaugeVec *prometheus.GaugeVec) error {
	for resourceName, quantity := range resourceList {
		resourceTypeName := strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_")
		labels := c.labels(node, resourceTypeName)
		// Register the set of labels that are generated for node
		nodeNamespacedName := types.NamespacedName{Name: node.Name}

		existingLabels, _ := c.labelMap.LoadOrStore(nodeNamespacedName, []prometheus.Labels{})
		existingLabels = append(existingLabels.([]prometheus.Labels), labels)
		c.labelMap.Store(nodeNamespacedName, existingLabels)

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
