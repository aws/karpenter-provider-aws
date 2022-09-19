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

package environment

import (
	"context"
	"fmt"
	"math"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/resources"
)

// Monitor is used to monitor the cluster state during a running test
type Monitor struct {
	ctx        context.Context
	kubeClient client.Client

	mu                 sync.RWMutex
	nodesSeen          sets.String
	numberNodesAtReset int
}
type state struct {
	pods         v1.PodList
	nodes        map[string]*v1.Node        // node name -> node
	nodePods     map[string][]*v1.Pod       // node name -> pods bound to the node
	nodeRequests map[string]v1.ResourceList // node name -> sum of pod resource requests
}

func NewMonitor(ctx context.Context, kubeClient client.Client) *Monitor {
	m := &Monitor{
		ctx:        ctx,
		kubeClient: kubeClient,
		nodesSeen:  sets.NewString(),
	}
	m.Reset()
	return m
}

// Reset resets the cluster monitor prior to running a test.
func (m *Monitor) Reset() {
	m.mu.Lock()
	m.nodesSeen = map[string]sets.Empty{}
	m.mu.Unlock()
	m.poll()
	m.numberNodesAtReset = len(m.nodesSeen)
}

// RestartCount returns the containers and number of restarts for that container for all containers in the pods in the
// given namespace
func (m *Monitor) RestartCount() map[string]int {
	st := m.poll()

	m.mu.RLock()
	defer m.mu.RUnlock()
	restarts := map[string]int{}
	for _, pod := range st.pods.Items {
		if pod.Namespace != "karpenter" {
			continue
		}
		for _, cs := range pod.Status.ContainerStatuses {
			name := fmt.Sprintf("%s/%s", pod.Name, cs.Name)
			restarts[name] = int(cs.RestartCount)
		}
	}
	return restarts
}

// GetNodes returns the most recent recording of nodes
func (m *Monitor) GetNodes() []v1.Node {
	var nodes []v1.Node
	for _, n := range m.poll().nodes {
		nodes = append(nodes, *n)
	}
	return nodes
}

// NodeCount returns the current number of nodes
func (m *Monitor) NodeCount() int {
	return len(m.poll().nodes)
}

// NodeCountAtReset returns the number of nodes that were running when the monitor was last reset, typically at the
// beginning of a test
func (m *Monitor) NodeCountAtReset() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.numberNodesAtReset
}

// TotalNodesSeen returns the total number of unique nodes ever seen since the last reset.
func (m *Monitor) TotalNodesSeen() int {
	m.poll()
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.nodesSeen)
}

// CreatedNodes returns the number of nodes created since the last reset
func (m *Monitor) CreatedNodes() int {
	return m.TotalNodesSeen() - m.numberNodesAtReset
}

func (m *Monitor) GetCreatedNodes() []v1.Node {
	return lo.Filter(m.GetNodes(), func(node v1.Node, _ int) bool {
		return node.Labels[v1alpha5.ProvisionerNameLabelKey] != ""
	})
}

// RunningPods returns the number of running pods matching the given selector
func (m *Monitor) RunningPods(selector labels.Selector) []*v1.Pod {
	var pods []*v1.Pod
	for _, pod := range m.poll().pods.Items {
		pod := pod
		if pod.Status.Phase != v1.PodRunning {
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
	var nodes v1.NodeList
	if err := m.kubeClient.List(m.ctx, &nodes); err != nil {
		logging.FromContext(m.ctx).Errorf("listing nodes, %s", err)
	}
	var pods v1.PodList
	if err := m.kubeClient.List(m.ctx, &pods); err != nil {
		logging.FromContext(m.ctx).Errorf("listing pods, %s", err)
	}

	m.mu.Lock()
	for _, node := range nodes.Items {
		m.nodesSeen.Insert(node.Name)
	}
	m.mu.Unlock()

	st := state{
		nodes:        map[string]*v1.Node{},
		pods:         pods,
		nodePods:     map[string][]*v1.Pod{},
		nodeRequests: map[string]v1.ResourceList{},
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

func (m *Monitor) AvgUtilization(resource v1.ResourceName) float64 {
	utilization := m.nodeUtilization(resource)
	sum := 0.0
	for _, v := range utilization {
		sum += v
	}
	return sum / float64(len(utilization))
}

func (m *Monitor) MinUtilization(resource v1.ResourceName) float64 {
	min := math.MaxFloat64
	for _, v := range m.nodeUtilization(resource) {
		min = math.Min(v, min)
	}
	return min
}

func (m *Monitor) nodeUtilization(resource v1.ResourceName) []float64 {
	st := m.poll()
	var utilization []float64
	for nodeName, requests := range st.nodeRequests {
		allocatable := st.nodes[nodeName].Status.Allocatable[resource]
		// skip any nodes we didn't launch
		if _, ok := st.nodes[nodeName].Labels[v1alpha5.ProvisionerNameLabelKey]; !ok {
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
