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

package scheduling

import (
	"context"
	"fmt"
	"sort"

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/utils/resources"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/injection"
)

var schedulingDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: "allocation_controller",
		Name:      "scheduling_duration_seconds",
		Help:      "Duration of scheduling process in seconds. Broken down by provisioner and error.",
		Buckets:   metrics.DurationBuckets(),
	},
	[]string{metrics.ProvisionerLabel},
)

func init() {
	crmetrics.Registry.MustRegister(schedulingDuration)
}

type Scheduler struct {
	kubeClient  client.Client
	preferences *Preferences
}

func NewScheduler(kubeClient client.Client) *Scheduler {
	return &Scheduler{
		kubeClient:  kubeClient,
		preferences: NewPreferences(),
	}
}

func (s *Scheduler) Solve(ctx context.Context, constraints *v1alpha5.Constraints, instanceTypes []cloudprovider.InstanceType, pods []*v1.Pod) ([]*Node, error) {
	defer metrics.Measure(schedulingDuration.WithLabelValues(injection.GetNamespacedName(ctx).Name))()

	sort.Slice(pods, byCPUAndMemoryDescending(pods))
	sort.Slice(instanceTypes, byPrice(instanceTypes))

	topology := NewTopology(s.kubeClient, &constraints.Requirements)
	if err := topology.Initialize(ctx, pods...); err != nil {
		return nil, fmt.Errorf("tracking topology counts, %w", err)
	}

	daemonOverhead, err := s.getDaemonOverhead(ctx, constraints)
	if err != nil {
		return nil, err
	}

	// We loop and retrying to schedule to unschedulable pods as long as we are making progress.  This solves a few
	// issues including pods with affinity to another pod in the batch. We could topo-sort to solve this, but it wouldn't
	// solve the problem of scheduling pods where a particular order is needed to prevent a max-skew violation. E.g. if we
	// had 5xA pods and 5xB pods were they have a zonal topology spread, but A can only go in one zone and B in another.
	// We need to schedule them alternating, A, B, A, B, .... and this solution also solves that as well.
	var nodes []*Node
	progressing := true
	errors := map[*v1.Pod]error{}
	for len(pods) > 0 && progressing {
		var unschedulable []*v1.Pod
		for _, pod := range pods {
			progressing = false
			// Relax preferences if pod has previously failed to schedule.
			if s.preferences.Relax(ctx, pod) {
				topology.Relax(pod)
				progressing = true
			}

			// Use existing node or create a node one
			node := s.scheduleExisting(pod, nodes, topology)
			if node == nil {
				node = NewNode(constraints, daemonOverhead, instanceTypes)
				if err := node.Add(topology, pod); err != nil {
					unschedulable = append(unschedulable, pod)
					errors[pod] = err
					continue
				}
				nodes = append(nodes, node)
			}

			// Record topology decision for future pods
			if err := topology.Record(pod, node.Constraints.Requirements); err != nil {
				return nil, fmt.Errorf("recording topology decision, %w", err)
			}
			progressing = true
		}
		pods = unschedulable
	}

	// Any remaining pods have failed to schedule
	for _, pod := range pods {
		logging.FromContext(ctx).With("pod", client.ObjectKeyFromObject(pod)).Errorf("Scheduling pod, %s", errors[pod])
	}
	return nodes, nil
}

func (s *Scheduler) scheduleExisting(pod *v1.Pod, nodes []*Node, topology *Topology) *Node {
	// Try nodes in ascending order of number of pods to more evenly distribute nodes, 100ms at 2000 nodes.
	sort.Slice(nodes, func(a, b int) bool { return len(nodes[a].Pods) < len(nodes[b].Pods) })
	for _, node := range nodes {
		if err := node.Add(topology, pod); err == nil {
			return node
		}
	}
	return nil
}

func (s *Scheduler) getDaemonOverhead(ctx context.Context, constraints *v1alpha5.Constraints) (v1.ResourceList, error) {
	daemonSetList := &appsv1.DaemonSetList{}
	if err := s.kubeClient.List(ctx, daemonSetList); err != nil {
		return nil, fmt.Errorf("listing daemonsets, %w", err)
	}
	var daemons []*v1.Pod
	for _, daemonSet := range daemonSetList.Items {
		p := &v1.Pod{Spec: daemonSet.Spec.Template.Spec}
		if err := constraints.Taints.Tolerates(p); err != nil {
			continue
		}
		if err := constraints.Requirements.Compatible(v1alpha5.NewPodRequirements(p)); err != nil {
			continue
		}
		daemons = append(daemons, p)
	}
	return resources.RequestsForPods(daemons...), nil
}

func byPrice(instanceTypes []cloudprovider.InstanceType) func(i int, j int) bool {
	return func(i, j int) bool {
		return instanceTypes[i].Price() < instanceTypes[j].Price()
	}
}

func byCPUAndMemoryDescending(pods []*v1.Pod) func(i int, j int) bool {
	return func(i, j int) bool {
		lhs := resources.RequestsForPods(pods[i])
		rhs := resources.RequestsForPods(pods[j])

		cpuCmp := resources.Cmp(lhs[v1.ResourceCPU], rhs[v1.ResourceCPU])
		if cpuCmp < 0 {
			// LHS has less CPU, so it should be sorted after
			return false
		} else if cpuCmp > 0 {
			return true
		}
		memCmp := resources.Cmp(lhs[v1.ResourceMemory], rhs[v1.ResourceMemory])

		if memCmp < 0 {
			return false
		} else if memCmp > 0 {
			return true
		}
		return false
	}
}
