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
	"sync"
	"time"

	"knative.dev/pkg/logging"

	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	podutils "github.com/aws/karpenter/pkg/utils/pod"
	"github.com/aws/karpenter/pkg/utils/resources"
)

// Cluster maintains cluster state that is often needed but expensive to compute.
type Cluster struct {
	ctx         context.Context
	rateLimiter *rate.Limiter
	kubeClient  client.Client

	// Pod Specific Tracking
	knownPods        sync.Map // mapping of pod namespaced name to struct{} used to track if we've seen a pod before
	antiAffinityPods sync.Map // mapping of pod namespaced name to *v1.Pod of pods that have required anti affinities

	// Node Status & Pod -> Node Binding
	mu       sync.RWMutex
	nodes    map[string]*Node                // node name -> node
	bindings map[types.NamespacedName]string // pod namespaced named -> pod
}

func NewCluster(ctx context.Context, client client.Client) *Cluster {
	s := &Cluster{
		ctx:         ctx,
		kubeClient:  client,
		rateLimiter: rate.NewLimiter(rate.Every(10*time.Second), 1),
		nodes:       map[string]*Node{},
		bindings:    map[types.NamespacedName]string{},
	}
	return s
}

// Node is a cached version of a node in the cluster that maintains state which is expensive to compute every time it's
// needed.  This currently contains node utilization across all of the allocatable resources, but will soon be used to
// compute topology information.
type Node struct {
	Node        *v1.Node
	requested   v1.ResourceList
	allocatable v1.ResourceList

	podRequests map[types.NamespacedName]v1.ResourceList
}

// Requested returns the total amount of resources requested by pods that have been bound to the node.
func (n Node) Requested() v1.ResourceList {
	return n.requested
}

// ForPodsWithAntiAffinity calls the supplied function once for each pod with required anti affinity terms that is
// currently bound to a node. The pod returned may not be up to date with respect to status, however since the
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
	for _, node := range c.nodes {
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
		logging.FromContext(c.ctx).Errorf("listing pods, %c", err)
	}
	var requested []v1.ResourceList
	for i := range pods.Items {
		requests := resources.RequestsForPods(&pods.Items[i])
		podKey := client.ObjectKeyFromObject(&pods.Items[i])

		n.podRequests[podKey] = requests
		c.bindings[podKey] = n.Node.Name
		requested = append(requested, requests)
	}
	n.requested = resources.Merge(requested...)
	n.allocatable = node.Status.Allocatable
	return n
}

func (c *Cluster) handleNodeDeletion(nodeName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nodes, nodeName)
}

// handleNodeUpdate is called for every node reconciliation
func (c *Cluster) handleNodeUpdate(node *v1.Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.nodes[node.Name]

	// are we already tracking this node?
	if !ok {
		c.nodes[node.Name] = c.newNode(node)
	} else {
		// just update the node object so we can track label changes, etc.
		c.nodes[node.Name].Node = node
	}
}

// handlePodDeletion is called when the pod has been deleted
func (c *Cluster) handlePodDeletion(podKey types.NamespacedName) {
	c.knownPods.Delete(podKey)
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
	n.requested = resources.Subtract(n.requested, n.podRequests[podKey])
	delete(n.podRequests, podKey)
}

// handlePodUpdate is called every time the pod is reconciled
func (c *Cluster) handlePodUpdate(pod *v1.Pod) {
	c.updateNodeUsageFromPod(pod)
	c.updatePodAntiAffinities(pod)
}

func (c *Cluster) updatePodAntiAffinities(pod *v1.Pod) {
	podKey := client.ObjectKeyFromObject(pod)
	if _, known := c.knownPods.Load(podKey); known {
		return
	}

	c.knownPods.Store(podKey, struct{}{})
	// We intentionally don't track inverse anti-affinity preferences. We're not
	// required to enforce them so it just adds complexity for very little
	// value. The problem with them comes from the relaxation process, the pod
	// we are relaxing is not the pod with the anti-affinity term.
	if podutils.HasRequiredPodAntiAffinity(pod) {
		c.antiAffinityPods.Store(podKey, pod)
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
	nodeName, bindingKnown := c.bindings[podKey]
	if bindingKnown {
		if nodeName != pod.Spec.NodeName {
			logging.FromContext(c.ctx).Errorf("internal tracking error, pod node name changed from %c to %c", nodeName, pod.Spec.NodeName)
		}
		// we are already tracking the pod binding, so nothing to update
		return
	}

	// we have noticed that the pod is bound to a node and didn't know about the binding before
	n, ok := c.nodes[pod.Spec.NodeName]
	if !ok {
		var node v1.Node
		if err := c.kubeClient.Get(c.ctx, client.ObjectKey{Name: pod.Spec.NodeName}, &node); err != nil {
			logging.FromContext(c.ctx).Errorf("getting node, %c", err)
		}

		// node didn't exist, but creating it will pick up this newly bound pod as well
		n = c.newNode(&node)
		c.nodes[pod.Spec.NodeName] = n
		return
	}

	// sum the newly bound pod's requests into the existing node and record the binding
	podRequests := resources.RequestsForPods(pod)
	n.requested = resources.Merge(n.requested, podRequests)
	n.podRequests[podKey] = podRequests
	c.bindings[podKey] = n.Node.Name
}
