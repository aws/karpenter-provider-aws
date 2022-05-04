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
	"sort"
	"sync"

	"github.com/aws/karpenter/pkg/utils/sets"

	"k8s.io/apimachinery/pkg/util/clock"

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
	ctx        context.Context
	clock      clock.Clock
	kubeClient client.Client

	// Pod Specific Tracking
	antiAffinityPods sync.Map // mapping of pod namespaced name to *v1.Pod of pods that have required anti affinities

	// Node Status & Pod -> Node Binding
	mu                sync.RWMutex
	nodes             map[string]*Node                // node name -> node
	inflightNodes     map[string]*Node                // node name -> inflight node
	bindings          map[types.NamespacedName]string // pod namespaced named -> node name
	extendedResources map[string]v1.ResourceList      // node name -> extended resources
}

func NewCluster(ctx context.Context, clock clock.Clock, client client.Client) *Cluster {
	return &Cluster{
		ctx:               ctx,
		clock:             clock,
		kubeClient:        client,
		nodes:             map[string]*Node{},
		inflightNodes:     map[string]*Node{},
		bindings:          map[types.NamespacedName]string{},
		extendedResources: map[string]v1.ResourceList{},
	}
}

// Node is a cached version of a node in the cluster that maintains state which is expensive to compute every time it's
// needed.  This currently contains node utilization across all the allocatable resources, but will soon be used to
// compute topology information.
type Node struct {
	Node *v1.Node
	// InFlightNode is true if we've launched a node, but the node object hasn't been created by kubelet yet.
	InFlightNode bool
	// Available is the total amount of resources that are available on the node.  This is the Allocatable minus the
	// resources requested by all pods bound to the node.
	Available v1.ResourceList
	// DaemonSetRequested is the total amount of resources that have been requested by daemon sets.  This allows users
	// of the Node to identify the remaining resources that we expect future daemonsets to consume.  This is already
	// included in the calculation for Available.
	DaemonSetRequested v1.ResourceList

	Provisioner *v1alpha5.Provisioner

	podRequests map[types.NamespacedName]v1.ResourceList
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
	seen := sets.NewSet()
	for _, node := range c.nodes {
		nodes = append(nodes, node)
		seen.Insert(node.Node.Name)
	}

	// only look at in-flight nodes for which we don't have a real node object
	for _, node := range c.inflightNodes {
		if seen.Has(node.Node.Name) {
			continue
		}
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

// newNode constructs a new node. This assumes the mutex is already locked.
func (c *Cluster) newNode(node *v1.Node) *Node {
	n := &Node{
		Node:        node,
		podRequests: map[types.NamespacedName]v1.ResourceList{},
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
	}

	n.DaemonSetRequested = resources.Merge(daemonsetRequested...)
	n.Available = resources.Subtract(c.getNodeAllocatable(node, n.Provisioner), resources.Merge(requested...))
	return n
}

// getNodeAllocatable gets the allocatable resources for the node. This assumes the mutex is already locked.
func (c *Cluster) getNodeAllocatable(node *v1.Node, provisioner *v1alpha5.Provisioner) v1.ResourceList {
	allocatable := node.Status.Allocatable
	// If the node is ready, don't take into consideration possible kubelet resource  zeroing.  This is to handle the
	// case where a node comes up with a resource and the hardware fails in some way so that the device-plugin zeros
	// out the resource.  We don't want to assume that it will always come back.
	if c.nodeIsReady(node, provisioner) {
		// once the node is ready, we can delete any extended resource knowledge
		delete(c.extendedResources, node.Name)
		return allocatable
	}

	extendedResources, ok := c.extendedResources[node.Name]
	if !ok {
		return allocatable
	}

	allocatable = v1.ResourceList{}
	for k, v := range node.Status.Allocatable {
		allocatable[k] = v
	}
	for resourceName, quantity := range extendedResources {
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
	delete(c.extendedResources, nodeName)
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
	n.podRequests[podKey] = podRequests
	c.bindings[podKey] = n.Node.Name
}

// IsNodeReady returns true if:
// a) its current status is set to Ready
// b) all the startup taints have been removed from the node
// c) all extended resources have been registered
// This method handles both nil provisioners and nodes without extended resources gracefully.
func (c *Cluster) IsNodeReady(node *v1.Node, provisioner *v1alpha5.Provisioner) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nodeIsReady(node, provisioner)
}

var kubletNotReadyTaint = &v1.Taint{
	Key:    "node.kubernetes.io/not-ready",
	Effect: v1.TaintEffectNoExecute,
}

// nodeIsReady is the internal readiness check method that is called with the mutex locked.
func (c *Cluster) nodeIsReady(node *v1.Node, provisioner *v1alpha5.Provisioner) bool {
	// fast checks first
	if GetCondition(node.Status.Conditions, v1.NodeReady).Status != v1.ConditionTrue {
		return false
	}
	// this taint is removed by the node controller when the condition above is true, but by looking for it we avoid
	// thinking the node is ready before the taint is actually removed
	for _, taint := range node.Spec.Taints {
		if taint.MatchTaint(kubletNotReadyTaint) {
			return false
		}
	}

	return isStartupTaintRemoved(node, provisioner) && c.isExtendedResourceRegistered(node)
}

// isExtendedResourceRegistered returns true if there are no extended resources on the node, or they have all been
// registered by device plugins. This assumes the mutex is already locked.
func (c *Cluster) isExtendedResourceRegistered(node *v1.Node) bool {
	extendedResources, ok := c.extendedResources[node.Name]
	if !ok {
		return true
	}

	for resourceName, quantity := range extendedResources {
		// kubelet will zero out both the capacity and allocatable for an extended resource on startup, so if our
		// annotation says the resource should be there, but it's zero'd in both then the device plugin hasn't
		// registered it yet
		if resources.IsZero(node.Status.Capacity[resourceName]) &&
			resources.IsZero(node.Status.Allocatable[resourceName]) &&
			!quantity.IsZero() {
			return false
		}
	}
	return true
}

func (c *Cluster) updateInflightNode(node *v1alpha5.InFlightNode) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n := &Node{
		Node:               node.ToNode(),
		InFlightNode:       true,
		Available:          resources.Subtract(node.Spec.Capacity, node.Spec.Overhead),
		DaemonSetRequested: v1.ResourceList{},
		podRequests:        map[types.NamespacedName]v1.ResourceList{},
	}

	c.inflightNodes[node.Name] = n

	// store the provisioner if it exists
	if provisionerName, ok := node.Labels[v1alpha5.ProvisionerNameLabelKey]; ok {
		var provisioner v1alpha5.Provisioner
		if err := c.kubeClient.Get(c.ctx, client.ObjectKey{Name: provisionerName}, &provisioner); err != nil {
			logging.FromContext(c.ctx).Errorf("getting provisioner, %s", err)
		} else {
			n.Provisioner = &provisioner
		}
	}

	// record the non-zero extended resources so we are aware for in-flight nodes and of the final expected state
	// while device plugins are starting up
	nonZeroExtendedResources := v1.ResourceList{}
	for name, quantity := range node.Spec.Capacity {
		if resources.IsExtended(name) {
			if !quantity.IsZero() {
				nonZeroExtendedResources[name] = quantity
			}
		}
	}
	if len(nonZeroExtendedResources) != 0 {
		c.extendedResources[node.Name] = nonZeroExtendedResources
	}
}

func (c *Cluster) deleteInflightNode(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.inflightNodes, name)
}
