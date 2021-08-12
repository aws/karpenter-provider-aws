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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/pod"
	"github.com/mitchellh/hashstructure/v2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewScheduler(cloudProvider cloudprovider.CloudProvider, kubeClient client.Client) *Scheduler {
	return &Scheduler{
		KubeClient: kubeClient,
		Topology: &Topology{
			cloudProvider: cloudProvider,
			kubeClient:    kubeClient,
		},
	}
}

type Scheduler struct {
	KubeClient client.Client
	Topology   *Topology
}

type Schedule struct {
	*v1alpha3.Constraints
	// Pods is a set of pods that may schedule to the node; used for binpacking.
	Pods []*v1.Pod
	// Daemons are a set of daemons that will schedule to the node; used for overhead.
	Daemons []*v1.Pod
}

func (s *Scheduler) Solve(ctx context.Context, provisioner *v1alpha3.Provisioner, pods []*v1.Pod) ([]*Schedule, error) {
	// 1. Inject temporarily adds specific NodeSelectors to pods, which are then
	// used by scheduling logic. This isn't strictly necessary, but is a useful
	// trick to avoid passing topology decisions through the scheduling code. It
	// lets us to treat TopologySpreadConstraints as just-in-time NodeSelectors.
	if err := s.Topology.Inject(ctx, provisioner, pods); err != nil {
		return nil, fmt.Errorf("injecting topology, %w", err)
	}

	// 2. Separate pods into schedules of compatible scheduling constraints.
	schedules, err := s.getSchedules(ctx, provisioner, pods)
	if err != nil {
		return nil, fmt.Errorf("getting schedules, %w", err)
	}

	// 3. Remove labels injected by TopologySpreadConstraints.
	for _, schedule := range schedules {
		delete(schedule.Labels, v1.LabelHostname)
	}
	return schedules, nil
}

// getSchedules separates pods into a set of schedules. All pods in each group
// contain compatible scheduling constarints and can be deployed together on the
// same node, or multiple similar nodes if the pods exceed one node's capacity.
func (s *Scheduler) getSchedules(ctx context.Context, provisioner *v1alpha3.Provisioner, pods []*v1.Pod) ([]*Schedule, error) {
	// schedule uniqueness is tracked by hash(Constraints)
	schedules := map[uint64]*Schedule{}
	for _, pod := range pods {
		constraints := provisioner.Spec.Constraints.WithOverrides(pod)
		key, err := hashstructure.Hash(constraints, hashstructure.FormatV2, nil)
		if err != nil {
			return nil, fmt.Errorf("hashing constraints, %w", err)
		}
		// Create new schedule if one doesn't exist
		if _, ok := schedules[key]; !ok {
			// Uses a theoretical node object to compute schedulablility of daemonset overhead.
			daemons, err := s.getDaemons(ctx, &v1.Node{
				ObjectMeta: metav1.ObjectMeta{Labels: constraints.Labels},
				Spec:       v1.NodeSpec{Taints: provisioner.Spec.Taints},
			})
			if err != nil {
				return nil, fmt.Errorf("computing node overhead, %w", err)
			}
			schedules[key] = &Schedule{
				Constraints: constraints,
				Pods:        []*v1.Pod{},
				Daemons:     daemons,
			}
		}
		// Append pod to schedule, guaranteed to exist
		schedules[key].Pods = append(schedules[key].Pods, pod)
	}

	result := []*Schedule{}
	for _, schedule := range schedules {
		result = append(result, schedule)
	}
	return result, nil
}

func (s *Scheduler) getDaemons(ctx context.Context, node *v1.Node) ([]*v1.Pod, error) {
	// 1. Get DaemonSets
	daemonSetList := &appsv1.DaemonSetList{}
	if err := s.KubeClient.List(ctx, daemonSetList); err != nil {
		return nil, fmt.Errorf("listing daemonsets, %w", err)
	}

	// 2. filter DaemonSets to include those that will schedule on this node
	pods := []*v1.Pod{}
	for _, daemonSet := range daemonSetList.Items {
		if pod.IsSchedulable(&daemonSet.Spec.Template.Spec, node) {
			pods = append(pods, &v1.Pod{Spec: daemonSet.Spec.Template.Spec})
		}
	}
	return pods, nil
}
