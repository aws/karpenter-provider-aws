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
	kubeClient client.Client
	topology   *Topology
}

func NewScheduler(kubeClient client.Client) *Scheduler {
	return &Scheduler{
		kubeClient: kubeClient,
		topology:   &Topology{kubeClient: kubeClient},
	}
}

func (s *Scheduler) Solve(ctx context.Context, provisioner *v1alpha5.Provisioner, instanceTypes []cloudprovider.InstanceType, pods []*v1.Pod) ([]*Node, error) {
	defer metrics.Measure(schedulingDuration.WithLabelValues(injection.GetNamespacedName(ctx).Name))()
	constraints := provisioner.Spec.Constraints.DeepCopy()

	sort.Slice(pods, byCPUAndMemoryDescending(pods))
	sort.Slice(instanceTypes, byPrice(instanceTypes))

	// Inject temporarily adds specific NodeSelectors to pods, which are then
	// used by scheduling logic. This isn't strictly necessary, but is a useful
	// trick to avoid passing topology decisions through the scheduling code. It
	// lets us treat TopologySpreadConstraints as just-in-time NodeSelectors.
	if err := s.topology.Inject(ctx, constraints, pods); err != nil {
		return nil, fmt.Errorf("injecting topology, %w", err)
	}

	nodeSet, err := NewNodeSet(ctx, constraints, s.kubeClient)
	if err != nil {
		return nil, fmt.Errorf("constructing nodeset, %w", err)
	}

	for _, pod := range pods {
		isScheduled := false
		for _, node := range nodeSet.nodes {
			if err := node.Add(pod); err == nil {
				isScheduled = true
				break
			}
		}
		if !isScheduled {
			n := NewNode(constraints, nodeSet.daemonResources, instanceTypes)
			if err := n.Add(pod); err != nil {
				logging.FromContext(ctx).With("pod", client.ObjectKeyFromObject(pod)).Errorf("Scheduling pod, %s", err)
			} else {
				nodeSet.Add(n)
			}
		}
	}
	logging.FromContext(ctx).Infof("Scheduled %d pods onto %d nodes", len(pods), len(nodeSet.nodes))
	return nodeSet.nodes, nil
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
