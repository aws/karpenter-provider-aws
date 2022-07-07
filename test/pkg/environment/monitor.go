package environment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Monitor is used to monitor the cluster state during a running test
type Monitor struct {
	ctx        context.Context
	kubeClient client.Client

	mu                 sync.RWMutex
	recordings         []recording
	nodesSeen          sets.String
	numberNodesAtReset int
}
type recording struct {
	nodes v1.NodeList
	pods  v1.PodList
}

func NewClusterMonitor(ctx context.Context, kubeClient client.Client) *Monitor {
	m := &Monitor{
		ctx:        ctx,
		kubeClient: kubeClient,
		nodesSeen:  sets.NewString(),
	}
	m.Reset()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				m.poll()
			}
		}
	}()
	return m
}

// Reset resets the cluster monitor prior to running a test.
func (m *Monitor) Reset() {
	m.mu.Lock()
	m.recordings = nil
	m.nodesSeen = map[string]sets.Empty{}
	m.mu.Unlock()
	m.poll()
	m.numberNodesAtReset = len(m.nodesSeen)
}

// RestartCount returns the containers and number of restarts for that container for all containers in the pods in the
// given namespace
func (m *Monitor) RestartCount() map[string]int {
	m.poll()

	m.mu.RLock()
	defer m.mu.RUnlock()
	restarts := map[string]int{}
	last := m.recordings[len(m.recordings)-1]
	for _, pod := range last.pods.Items {
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

// NodeCount returns the current number of nodes
func (m *Monitor) NodeCount() int {
	m.poll()
	m.mu.RLock()
	defer m.mu.RUnlock()
	last := m.recordings[len(m.recordings)-1]
	return len(last.nodes.Items)
}

// NodeCountAtReset returns the number of nodes that were running when the monitor was last reset, typically at the
// beginning of a test
func (m *Monitor) NodeCountAtReset() interface{} {
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

// RunningPods returns the number of running pods matching the given selector
func (m *Monitor) RunningPods(selector labels.Selector) int {
	m.poll()
	m.mu.RLock()
	defer m.mu.RUnlock()
	last := m.recordings[len(m.recordings)-1]
	count := 0
	for _, pod := range last.pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			count++
		}
	}
	return count
}

func (m *Monitor) poll() {
	var nodes v1.NodeList
	if err := m.kubeClient.List(m.ctx, &nodes); err != nil {
		logging.FromContext(m.ctx).Errorf("listing nodes, %s", err)
	}
	var pods v1.PodList
	if err := m.kubeClient.List(m.ctx, &pods); err != nil {
		logging.FromContext(m.ctx).Errorf("listing pods, %s", err)
	}
	m.record(nodes, pods)
}

func (m *Monitor) record(nodes v1.NodeList, pods v1.PodList) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordings = append(m.recordings, recording{
		nodes: nodes,
		pods:  pods,
	})

	for _, node := range nodes.Items {
		m.nodesSeen.Insert(node.Name)
	}
}
