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
	"sort"
	"sync"

	"github.com/aws/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"

	"knative.dev/pkg/logging"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	podutils "github.com/aws/karpenter/pkg/utils/pod"
	"github.com/aws/karpenter/pkg/utils/resources"
)

// Cluster maintains cluster state that is often needed but expensive to compute.
type Cluster struct {
	ctx           context.Context
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider

	// Pod Specific Tracking
	antiAffinityPods sync.Map // mapping of pod namespaced name to *v1.Pod of pods that have required anti affinities

	// Node Status & Pod -> Node Binding
	mu       sync.RWMutex
	nodes    map[string]*Node                // node name -> node
	bindings map[types.NamespacedName]string // pod namespaced named -> node name
}

func NewCluster(ctx context.Context, client client.Client, cp cloudprovider.CloudProvider) *Cluster {
	c := &Cluster{
		ctx:           ctx,
		kubeClient:    client,
		cloudProvider: cp,
		nodes:         map[string]*Node{},
		bindings:      map[types.NamespacedName]string{},
	}
	return c
}

// Node is a cached version of a node in the cluster that maintains state which is expensive to compute every time it's
// needed.  This currently contains node utilization across all the allocatable resources, but will soon be used to
// compute topology information.
type Node struct {
	Node *v1.Node
	// Capacity is the total amount of resources on the node.  The available resources are the capacity minus overhead
	// minus anything allocated to pods.
	Capacity v1.ResourceList
	// Available is the total amount of resources that are available on the node.  This is the Allocatable minus the
	// resources requested by all pods bound to the node.
	Available v1.ResourceList
	// DaemonSetRequested is the total amount of resources that have been requested by daemon sets.  This allows users
	// of the Node to identify the remaining resources that we expect future daemonsets to consume.  This is already
	// included in the calculation for Available.
	DaemonSetRequested v1.ResourceList
	// HostPort usage of all pods that are bound to the node
	HostPortUsage *HostPortUsage
	// Provisioner is the provisioner used to create the node.
	Provisioner *v1alpha5.Provisioner

	podRequests  map[types.NamespacedName]v1.ResourceList
	InstanceType cloudprovider.InstanceType
}

// ForPodsWithAntiAffinity calls the supplied function once for each pod with required anti affinity terms that is
// currently bound to a node. The pod returned may not be up-to-date with respect to status, however since the
// anti-affinity terms can't be modified, they will be correct.
func (c *Cluster) ForPodsWithAntiAffinity(fn func(p *v1.Pod, n *v1.Node) bool) {
	c.antiAffinityPods.Range(func(key, value interface{}) bool {
		pod := value.(*v1.Pod)
		c.mu.RLock()
		defer c.mu.RUnlock()
		nodeName, ok := c.bindings[client.ObjectKeyFromObject(pod)]
		if !ok {
			return true
		}
		node, ok := c.nodes[nodeName]
		if !ok {
			// if we receive the node deletion event before the pod deletion event, this can happen
			return true
		}
		return fn(pod, node.Node)
	})
}

// ForEachNode calls the supplied function once per node object that is being tracked. It is not safe to store the
// state.Node object, it should be only accessed from within the function provided to this method.
func (c *Cluster) ForEachNode(f func(n *Node) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var nodes []*Node
	for _, node := range c.nodes {
		nodes = append(nodes, node)
	}
	// sort nodes by creation time so we provide a consistent ordering
	sort.Slice(nodes, func(a, b int) bool {
		return nodes[a].Node.CreationTimestamp.Time.Before(nodes[b].Node.CreationTimestamp.Time)
	})

	for _, node := range nodes {
		if !f(node) {
			return
		}
	}
}

func (c *Cluster) newNode(node *v1.Node) *Node {
	n := &Node{
		Node:          node,
		HostPortUsage: NewHostPortUsage(),
		podRequests:   map[types.NamespacedName]v1.ResourceList{},
	}

	// store the provisioner if it exists
	if provisionerName, ok := node.Labels[v1alpha5.ProvisionerNameLabelKey]; ok {
		var provisioner v1alpha5.Provisioner
		if err := c.kubeClient.Get(c.ctx, client.ObjectKey{Name: provisionerName}, &provisioner); err != nil {
			logging.FromContext(c.ctx).Errorf("getting provisioner, %s", err)
		} else {
			n.Provisioner = &provisioner
		}
	}

	var err error

	n.InstanceType, err = c.getInstanceType(c.ctx, n.Provisioner, node.Labels[v1.LabelInstanceTypeStable])
	if err != nil {
		logging.FromContext(c.ctx).Errorf("getting instance type, %s", err)
	}

	var pods v1.PodList
	if err := c.kubeClient.List(c.ctx, &pods, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		logging.FromContext(c.ctx).Errorf("listing pods, %s", err)
	}
	var requested []v1.ResourceList
	var daemonsetRequested []v1.ResourceList
	for i := range pods.Items {
		pod := &pods.Items[i]
		requests := resources.RequestsForPods(pod)
		podKey := client.ObjectKeyFromObject(pod)

		n.podRequests[podKey] = requests
		c.bindings[podKey] = n.Node.Name
		if podutils.IsOwnedByDaemonSet(pod) {
			daemonsetRequested = append(daemonsetRequested, requests)
		}
		requested = append(requested, requests)
		if err := n.HostPortUsage.Add(pod); err != nil {
			logging.FromContext(c.ctx).Errorf("inconsistent state, error tracking host port usage on node %s, %s", n.Node.Name, err)
		}
	}

	n.DaemonSetRequested = resources.Merge(daemonsetRequested...)
	n.Capacity = n.Node.Status.Capacity
	// if the capacity hasn't been reported yet, fall back to what the instance type reports so we can track
	// limits
	if len(n.Capacity) == 0 && n.InstanceType != nil {
		n.Capacity = n.InstanceType.Resources()
	}
	n.Available = resources.Subtract(c.getNodeAllocatable(node, n.Provisioner), resources.Merge(requested...))
	return n
}

// getNodeAllocatable gets the allocatable resources for the node.
func (c *Cluster) getNodeAllocatable(node *v1.Node, provisioner *v1alpha5.Provisioner) v1.ResourceList {
	instanceType, err := c.getInstanceType(c.ctx, provisioner, node.Labels[v1.LabelInstanceTypeStable])
	if err != nil {
		logging.FromContext(c.ctx).Errorf("error finding instance type, %s", err)
		return node.Status.Allocatable
	}

	// If the node is ready, don't take into consideration possible kubelet resource  zeroing.  This is to handle the
	// case where a node comes up with a resource and the hardware fails in some way so that the device-plugin zeros
	// out the resource.  We don't want to assume that it will always come back.  The instance type may be nil if
	// the node was created from a provisioner that has since been deleted.
	if instanceType == nil || node.Labels[v1alpha5.LabelNodeInitialized] == "true" {
		return node.Status.Allocatable
	}

	allocatable := v1.ResourceList{}
	for k, v := range node.Status.Allocatable {
		allocatable[k] = v
	}
	for resourceName, quantity := range instanceType.Resources() {
		// kubelet will zero out both the capacity and allocatable for an extended resource on startup
		if resources.IsZero(node.Status.Capacity[resourceName]) &&
			resources.IsZero(node.Status.Allocatable[resourceName]) &&
			!quantity.IsZero() {
			allocatable[resourceName] = quantity
		}
	}
	return allocatable
}

func (c *Cluster) deleteNode(nodeName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nodes, nodeName)
}

// updateNode is called for every node reconciliation
func (c *Cluster) updateNode(node *v1.Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes[node.Name] = c.newNode(node)
}

// deletePod is called when the pod has been deleted
func (c *Cluster) deletePod(podKey types.NamespacedName) {
	c.antiAffinityPods.Delete(podKey)
	c.updateNodeUsageFromPodDeletion(podKey)
}

func (c *Cluster) updateNodeUsageFromPodDeletion(podKey types.NamespacedName) {
	c.mu.Lock()
	defer c.mu.Unlock()
	nodeName, bindingKnown := c.bindings[podKey]
	if !bindingKnown {
		// we didn't think the pod was bound, so we weren't tracking it and don't need to do anything
		return
	}

	delete(c.bindings, podKey)
	n, ok := c.nodes[nodeName]
	if !ok {
		// we weren't tracking the node yet, so nothing to do
		return
	}
	// pod has been deleted so our available capacity increases by the resources that had been
	// requested by the pod
	n.Available = resources.Merge(n.Available, n.podRequests[podKey])
	delete(n.podRequests, podKey)
	n.HostPortUsage.DeletePod(podKey)

	// We can't easily track the changes to the DaemonsetRequested here as we no longer have the pod.  We could keep up
	// with this separately, but if a daemonset pod is being deleted, it usually means the node is going down.  In the
	// worst case we will resync to correct this.
}

// updatePod is called every time the pod is reconciled
func (c *Cluster) updatePod(pod *v1.Pod) {
	c.updateNodeUsageFromPod(pod)
	c.updatePodAntiAffinities(pod)
}

func (c *Cluster) updatePodAntiAffinities(pod *v1.Pod) {
	// We intentionally don't track inverse anti-affinity preferences. We're not
	// required to enforce them so it just adds complexity for very little
	// value. The problem with them comes from the relaxation process, the pod
	// we are relaxing is not the pod with the anti-affinity term.
	if podKey := client.ObjectKeyFromObject(pod); podutils.HasRequiredPodAntiAffinity(pod) {
		c.antiAffinityPods.Store(podKey, pod)
	} else {
		c.antiAffinityPods.Delete(podKey)
	}
}

// updateNodeUsageFromPod is called every time a reconcile event occurs for the pod. If the pods binding has changed
// (unbound to bound), we need to update the resource requests on the node.
func (c *Cluster) updateNodeUsageFromPod(pod *v1.Pod) {
	// nothing to do if the pod isn't bound, checking early allows avoiding unnecessary locking
	if pod.Spec.NodeName == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	podKey := client.ObjectKeyFromObject(pod)
	oldNodeName, bindingKnown := c.bindings[podKey]
	if bindingKnown {
		if oldNodeName == pod.Spec.NodeName {
			// we are already tracking the pod binding, so nothing to update
			return
		}
		// the pod has switched nodes, this can occur if a pod name was re-used and it was deleted/re-created rapidly,
		// binding to a different node the second time
		logging.FromContext(c.ctx).Infof("pod %s has moved from node %s to %s", podKey, oldNodeName, pod.Spec.NodeName)
		n, ok := c.nodes[oldNodeName]
		if ok {
			// we were tracking the old node, so we need to reduce its capacity by the amount of the pod that has
			// left it
			delete(c.bindings, podKey)
			n.Available = resources.Merge(n.Available, n.podRequests[podKey])
			n.HostPortUsage.DeletePod(podKey)
			delete(n.podRequests, podKey)
		}
	}

	// we have noticed that the pod is bound to a node and didn't know about the binding before
	n, ok := c.nodes[pod.Spec.NodeName]
	if !ok {
		var node v1.Node
		if err := c.kubeClient.Get(c.ctx, client.ObjectKey{Name: pod.Spec.NodeName}, &node); err != nil {
			logging.FromContext(c.ctx).Errorf("getting node, %s", err)
		}

		// node didn't exist, but creating it will pick up this newly bound pod as well
		n = c.newNode(&node)
		c.nodes[pod.Spec.NodeName] = n
		return
	}

	// sum the newly bound pod's requests into the existing node and record the binding
	podRequests := resources.RequestsForPods(pod)
	// our available capacity goes down by the amount that the pod had requested
	n.Available = resources.Subtract(n.Available, podRequests)
	// if it's a daemonset, we track what it has requested separately
	if podutils.IsOwnedByDaemonSet(pod) {
		n.DaemonSetRequested = resources.Merge(n.DaemonSetRequested, podRequests)
	}
	if err := n.HostPortUsage.Add(pod); err != nil {
		logging.FromContext(c.ctx).Errorf("inconsistent state, error tracking host port usage on node %s, %s", n.Node.Name, err)
	}
	n.podRequests[podKey] = podRequests
	c.bindings[podKey] = n.Node.Name
}

func (c *Cluster) getInstanceType(ctx context.Context, provisioner *v1alpha5.Provisioner, instanceTypeName string) (cloudprovider.InstanceType, error) {
	if provisioner == nil || provisioner.Spec.Provider == nil {
		// no provisioner means we cant lookup the instance type
		return nil, nil
	}
	instanceTypes, err := c.cloudProvider.GetInstanceTypes(ctx, provisioner.Spec.Provider)
	if err != nil {
		return nil, err
	}
	for _, it := range instanceTypes {
		if it.Name() == instanceTypeName {
			return it, nil
		}
	}
	return nil, fmt.Errorf("instance type '%s' not found", instanceTypeName)
}
