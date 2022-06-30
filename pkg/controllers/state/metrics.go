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

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	podutil "github.com/aws/karpenter/pkg/utils/pod"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	resourceType        = "resource_type"
	nodeName            = "node_name"
	nodeProvisioner     = "provisioner"
	nodeZone            = "zone"
	nodeArchitecture    = "arch"
	nodeCapacityType    = "capacity_type"
	nodeInstanceType    = "instance_type"
	nodePhase           = "phase"
	podName             = "name"
	podNameSpace        = "namespace"
	ownerSelfLink       = "owner"
	podHostName         = "node"
	podProvisioner      = "provisioner"
	podHostZone         = "zone"
	podHostArchitecture = "arch"
	podHostCapacityType = "capacity_type"
	podHostInstanceType = "instance_type"
	podPhase            = "phase"
	provisionerName     = "provisioner"
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

	prometheusMetrics = []*prometheus.GaugeVec{
		allocatableGaugeVec,
		podRequestsGaugeVec,
		podLimitsGaugeVec,
		daemonRequestsGaugeVec,
		daemonLimitsGaugeVec,
		overheadGaugeVec,
		podGaugeVec,
		limitGaugeVec,
		usageGaugeVec,
		usagePctGaugeVec,
	}
)

func init() {
	for _, m := range prometheusMetrics {
		crmetrics.Registry.MustRegister(m)
	}
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

func ResetMetrics() {
	for _, vec := range prometheusMetrics {
		vec.Reset()
	}
}

// Gets the labels for a given pod based on current cluster state
func (c *PodController) getPodLabels(ctx context.Context, pod *v1.Pod) prometheus.Labels {
	metricLabels := prometheus.Labels{}
	metricLabels[podName] = pod.GetName()
	metricLabels[podNameSpace] = pod.GetNamespace()
	// Selflink has been deprecated after v.1.20
	// Manually generate the selflink for the first owner reference
	// Currently we do not support multiple owner references
	selflink := ""
	if len(pod.GetOwnerReferences()) > 0 {
		ownerreference := pod.GetOwnerReferences()[0]
		selflink = fmt.Sprintf("/apis/%s/namespaces/%s/%ss/%s", ownerreference.APIVersion, pod.Namespace, strings.ToLower(ownerreference.Kind), ownerreference.Name)
	}
	metricLabels[ownerSelfLink] = selflink
	metricLabels[podHostName] = pod.Spec.NodeName
	metricLabels[podPhase] = string(pod.Status.Phase)
	node := &v1.Node{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, node); err != nil {
		metricLabels[podHostZone] = "N/A"
		metricLabels[podHostArchitecture] = "N/A"
		metricLabels[podHostCapacityType] = "N/A"
		metricLabels[podHostInstanceType] = "N/A"
		if provisionerName, ok := pod.Spec.NodeSelector[v1alpha5.ProvisionerNameLabelKey]; ok {
			metricLabels[podProvisioner] = provisionerName
		} else {
			metricLabels[podProvisioner] = "N/A"
		}
	} else {
		metricLabels[podHostZone] = node.Labels[v1.LabelTopologyZone]
		metricLabels[podHostArchitecture] = node.Labels[v1.LabelArchStable]
		if capacityType, ok := node.Labels[v1alpha5.LabelCapacityType]; !ok {
			metricLabels[podHostCapacityType] = "N/A"
		} else {
			metricLabels[podHostCapacityType] = capacityType
		}
		metricLabels[podHostInstanceType] = node.Labels[v1.LabelInstanceTypeStable]
		if provisionerName, ok := node.Labels[v1alpha5.ProvisionerNameLabelKey]; !ok {
			metricLabels[podProvisioner] = "N/A"
		} else {
			metricLabels[podProvisioner] = provisionerName
		}
	}
	return metricLabels
}

func (c *PodController) record(ctx context.Context, pod *v1.Pod) {
	logging.FromContext(ctx).Infof("Recording object: %s", client.ObjectKeyFromObject(pod).String())
	labels := c.getPodLabels(ctx, pod)
	podGaugeVec.With(labels).Set(float64(1))
	c.labelsMap.Store(client.ObjectKeyFromObject(pod), labels)
}

// labels creates the labels using the current state of the pod
func (c *NodeController) getNodeLabels(node *v1.Node, resourceTypeName string) prometheus.Labels {
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
		labels := c.getNodeLabels(node, resourceTypeName)
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

func (c *ProvisionerController) cleanup(provisionerName types.NamespacedName) {
	if labelSet, ok := c.labelMap.Load(provisionerName); ok {
		for _, labels := range labelSet.([]prometheus.Labels) {
			limitGaugeVec.Delete(labels)
			usageGaugeVec.Delete(labels)
			usagePctGaugeVec.Delete(labels)
		}
	}
	c.labelMap.Store(provisionerName, []prometheus.Labels{})
}

func (c *ProvisionerController) getProvisionerLabels(provisioner *v1alpha5.Provisioner, resourceTypeName string) prometheus.Labels {
	metricLabels := prometheus.Labels{}
	metricLabels[resourceType] = resourceTypeName
	metricLabels[provisionerName] = provisioner.Name
	return metricLabels
}

func (c *ProvisionerController) record(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	if provisioner.Spec.Limits == nil {
		return nil
	}

	if err := c.set(provisioner.Spec.Limits.Resources, provisioner, limitGaugeVec); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %s", err)
	}

	if err := c.set(provisioner.Status.Resources, provisioner, usageGaugeVec); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %s", err)
	}

	usage := v1.ResourceList{}
	for k, v := range provisioner.Spec.Limits.Resources {
		limitValue := v.AsApproximateFloat64()
		usedValue := provisioner.Status.Resources[k]
		if limitValue == 0 {
			usage[k] = *resource.NewQuantity(100, resource.DecimalSI)
		} else {
			usage[k] = *resource.NewQuantity(int64(usedValue.AsApproximateFloat64()/limitValue*100), resource.DecimalSI)
		}
	}

	if err := c.set(usage, provisioner, usagePctGaugeVec); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %s", err)
	}

	return nil
}

// set sets the value for the node gauge
func (c *ProvisionerController) set(resourceList v1.ResourceList, provisioner *v1alpha5.Provisioner, gaugeVec *prometheus.GaugeVec) error {
	for resourceName, quantity := range resourceList {
		resourceTypeName := strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_")
		labels := c.getProvisionerLabels(provisioner, resourceTypeName)

		provisionerName := types.NamespacedName{Name: provisioner.Name}
		existingLabels, _ := c.labelMap.LoadOrStore(provisionerName, []prometheus.Labels{})
		existingLabels = append(existingLabels.([]prometheus.Labels), labels)
		c.labelMap.Store(provisionerName, existingLabels)

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
