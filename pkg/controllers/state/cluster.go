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
	kubeClient client.Client

	// Pod Specific Tracking
	antiAffinityPods sync.Map // mapping of pod namespaced name to *v1.Pod of pods that have required anti affinities

	// Node Status & Pod -> Node Binding
	mu       sync.RWMutex
	nodes    map[string]*Node                // node name -> node
	bindings map[types.NamespacedName]string // pod namespaced named -> node name
}

func NewCluster(ctx context.Context, client client.Client) *Cluster {
	return &Cluster{
		ctx:        ctx,
		kubeClient: client,
		nodes:      map[string]*Node{},
		bindings:   map[types.NamespacedName]string{},
	}
}

// Node is a cached version of a node in the cluster that maintains state which is expensive to compute every time it's
// needed.  This currently contains node utilization across all the allocatable resources, but will soon be used to
// compute topology information.
type Node struct {
	Node *v1.Node
	// Available is the total amount of resources that are available on the node.  This is the Allocatable minus the
	// resources requested by all pods bound to the node.
	Available v1.ResourceList
	// DaemonSetRequested is the total amount of resources that have been requested by daemon sets.  This allows users
	// of the Node to identify the remaining resources that we expect future daemonsets to consume.  This is already
	// included in the calculation for Available.
	DaemonSetRequested v1.ResourceList

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
		Node:        node,
		podRequests: map[types.NamespacedName]v1.ResourceList{},
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
		if isOwnedByDaemonset(pod) {
			daemonsetRequested = append(daemonsetRequested, requests)
		}
		requested = append(requested, requests)
	}

	n.DaemonSetRequested = resources.Merge(daemonsetRequested...)
	n.Available = resources.Subtract(n.Node.Status.Allocatable, resources.Merge(requested...))
	return n
}

func isOwnedByDaemonset(pod *v1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
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
	if isOwnedByDaemonset(pod) {
		n.DaemonSetRequested = resources.Merge(n.DaemonSetRequested, podRequests)
	}
	n.podRequests[podKey] = podRequests
	c.bindings[podKey] = n.Node.Name
}
