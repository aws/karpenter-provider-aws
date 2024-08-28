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

package common

import (
	"context"
	"fmt"
	"math"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/samber/lo"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

// Monitor is used to monitor the cluster state during a running test
type Monitor struct {
	ctx        context.Context
	kubeClient client.Client

	mu sync.RWMutex

	nodesAtReset map[string]*corev1.Node
}

type state struct {
	pods         corev1.PodList
	nodes        map[string]*corev1.Node        // node name -> node
	nodePods     map[string][]*corev1.Pod       // node name -> pods bound to the node
	nodeRequests map[string]corev1.ResourceList // node name -> sum of pod resource requests
}

func NewMonitor(ctx context.Context, kubeClient client.Client) *Monitor {
	m := &Monitor{
		ctx:          ctx,
		kubeClient:   kubeClient,
		nodesAtReset: map[string]*corev1.Node{},
	}
	m.Reset()
	return m
}

// Reset resets the cluster monitor prior to running a test.
func (m *Monitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := m.poll()
	m.nodesAtReset = deepCopyMap(st.nodes)
}

// RestartCount returns the containers and number of restarts for that container for all containers in the pods in the
// given namespace
func (m *Monitor) RestartCount(namespace string) map[string]int {
	st := m.poll()

	m.mu.RLock()
	defer m.mu.RUnlock()
	restarts := map[string]int{}
	for _, pod := range st.pods.Items {
		if pod.Namespace != namespace {
			continue
		}
		for _, cs := range pod.Status.ContainerStatuses {
			name := fmt.Sprintf("%s/%s", pod.Name, cs.Name)
			restarts[name] = int(cs.RestartCount)
		}
	}
	return restarts
}

// NodeCount returns the current number of nodes
func (m *Monitor) NodeCount() int {
	return len(m.poll().nodes)
}

// NodeCountAtReset returns the number of nodes that were running when the monitor was last reset, typically at the
// beginning of a test
func (m *Monitor) NodeCountAtReset() int {
	return len(m.NodesAtReset())
}

// CreatedNodeCount returns the number of nodes created since the last reset
func (m *Monitor) CreatedNodeCount() int {
	return m.NodeCount() - m.NodeCountAtReset()
}

// NodesAtReset returns a slice of nodes that the monitor saw at the last reset
func (m *Monitor) NodesAtReset() []*corev1.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return deepCopySlice(lo.Values(m.nodesAtReset))
}

// Nodes returns all the nodes on the cluster
func (m *Monitor) Nodes() []*corev1.Node {
	st := m.poll()
	return lo.Values(st.nodes)
}

// CreatedNodes returns the nodes that have been created since the last reset (essentially Nodes - NodesAtReset)
func (m *Monitor) CreatedNodes() []*corev1.Node {
	resetNodeNames := sets.NewString(lo.Map(m.NodesAtReset(), func(n *corev1.Node, _ int) string { return n.Name })...)
	return lo.Filter(m.Nodes(), func(n *corev1.Node, _ int) bool { return !resetNodeNames.Has(n.Name) })
}

// DeletedNodes returns the nodes that have been deleted since the last reset (essentially NodesAtReset - Nodes)
func (m *Monitor) DeletedNodes() []*corev1.Node {
	currentNodeNames := sets.NewString(lo.Map(m.Nodes(), func(n *corev1.Node, _ int) string { return n.Name })...)
	return lo.Filter(m.NodesAtReset(), func(n *corev1.Node, _ int) bool { return !currentNodeNames.Has(n.Name) })
}

// PendingPods returns the number of pending pods matching the given selector
func (m *Monitor) PendingPods(selector labels.Selector) []*corev1.Pod {
	var pods []*corev1.Pod
	for _, pod := range m.poll().pods.Items {
		if pod.Status.Phase != corev1.PodPending {
			continue
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			pods = append(pods, &pod)
		}
	}
	return pods
}

func (m *Monitor) PendingPodsCount(selector labels.Selector) int {
	return len(m.PendingPods(selector))
}

// RunningPods returns the number of running pods matching the given selector
func (m *Monitor) RunningPods(selector labels.Selector) []*corev1.Pod {
	var pods []*corev1.Pod
	for _, pod := range m.poll().pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			pods = append(pods, &pod)
		}
	}
	return pods
}

func (m *Monitor) RunningPodsCount(selector labels.Selector) int {
	return len(m.RunningPods(selector))
}

func (m *Monitor) poll() state {
	var nodes corev1.NodeList
	if err := m.kubeClient.List(m.ctx, &nodes); err != nil {
		log.FromContext(m.ctx).Error(err, "failed listing nodes")
	}
	var pods corev1.PodList
	if err := m.kubeClient.List(m.ctx, &pods); err != nil {
		log.FromContext(m.ctx).Error(err, "failing listing pods")
	}
	st := state{
		nodes:        map[string]*corev1.Node{},
		pods:         pods,
		nodePods:     map[string][]*corev1.Pod{},
		nodeRequests: map[string]corev1.ResourceList{},
	}
	for i := range nodes.Items {
		st.nodes[nodes.Items[i].Name] = &nodes.Items[i]
	}
	// collect pods per node
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Spec.NodeName == "" {
			continue
		}
		st.nodePods[pod.Spec.NodeName] = append(st.nodePods[pod.Spec.NodeName], pod)
	}

	for _, n := range nodes.Items {
		st.nodeRequests[n.Name] = resources.RequestsForPods(st.nodePods[n.Name]...)
	}
	return st
}

func (m *Monitor) AvgUtilization(resource corev1.ResourceName) float64 {
	utilization := m.nodeUtilization(resource)
	sum := 0.0
	for _, v := range utilization {
		sum += v
	}
	return sum / float64(len(utilization))
}

func (m *Monitor) MinUtilization(resource corev1.ResourceName) float64 {
	min := math.MaxFloat64
	for _, v := range m.nodeUtilization(resource) {
		min = math.Min(v, min)
	}
	return min
}

func (m *Monitor) nodeUtilization(resource corev1.ResourceName) []float64 {
	st := m.poll()
	var utilization []float64
	for nodeName, requests := range st.nodeRequests {
		allocatable := st.nodes[nodeName].Status.Allocatable[resource]
		// skip any nodes we didn't launch
		if st.nodes[nodeName].Labels[karpv1.NodePoolLabelKey] == "" {
			continue
		}
		if allocatable.IsZero() {
			continue
		}
		requested := requests[resource]
		utilization = append(utilization, requested.AsApproximateFloat64()/allocatable.AsApproximateFloat64())
	}
	return utilization
}

type copyable[T any] interface {
	DeepCopy() T
}

func deepCopyMap[K comparable, V copyable[V]](m map[K]V) map[K]V {
	ret := map[K]V{}
	for k, v := range m {
		ret[k] = v.DeepCopy()
	}
	return ret
}

func deepCopySlice[T copyable[T]](s []T) []T {
	var ret []T
	for _, elem := range s {
		ret = append(ret, elem.DeepCopy())
	}
	return ret
}
