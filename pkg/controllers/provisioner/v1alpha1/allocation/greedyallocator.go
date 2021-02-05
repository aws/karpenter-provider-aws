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

package allocation

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ Allocator = &GreedyAllocator{}

// GreedyAllocator iteratively assigns pods to scheduling groups and then creates capacity for each group.
type GreedyAllocator struct {
	CloudProvider cloudprovider.Factory
	CoreV1Client  *corev1.CoreV1Client
}

// Allocate takes a list of unschedulable pods and creates nodes based on
// resources required, node selectors and zone balancing.
func (a *GreedyAllocator) Allocate(provisioner *v1alpha1.Provisioner, pods []*v1.Pod) error {
	// 1. Separate pods into scheduling groups
	groups := a.getSchedulingGroups(pods)
	ctx := context.TODO() // TODO wire this in from reconcile loop

	zap.S().Infof("Allocating %d pending pods from %d constraint groups", len(pods), len(groups))
	// 2. Group pods into equally schedulable constraint group
	for _, constraints := range groups {
		packing, err := a.CloudProvider.CapacityFor(&provisioner.Spec).Create(ctx, constraints)
		// TODO accumulate errors if one request fails.
		if err != nil {
			return fmt.Errorf("while creating capacity, %w", err)
		}
		for _, pack := range packing {
			if err := a.bind(ctx, pack.Node, pack.Pods); err != nil {
				// TODO accumulate errors if one request fails.
				return fmt.Errorf("binding pods to node, %w", err)
			}
		}
	}
	return nil
}

func (a *GreedyAllocator) bind(ctx context.Context, node *v1.Node, pods []*v1.Pod) error {
	// 1. Create node object
	if _, err := a.CoreV1Client.Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating node %s, %w", node.Name, err)
	}
	// 2. Bind all pods to node
	for _, pod := range pods {
		if err := a.CoreV1Client.Pods(pod.Namespace).Bind(ctx, &v1.Binding{
			TypeMeta:   pod.TypeMeta,
			ObjectMeta: pod.ObjectMeta,
			Target:     v1.ObjectReference{Name: node.Name},
		}, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("binding pod, %w", err)
		}
		zap.S().Infof("Successfully bound pod %s/%s to node %s", pod.Namespace, pod.Name, node.Name)
	}
	return nil
}

func (a *GreedyAllocator) getSchedulingGroups(pods []*v1.Pod) []*cloudprovider.CapacityConstraints {
	schedulingGroups := []*cloudprovider.CapacityConstraints{}
	for _, pod := range pods {
		added := false
		for _, constraints := range schedulingGroups {
			if a.matchesConstraints(constraints, pod) {
				constraints.Pods = append(constraints.Pods, pod)
				added = true
				break
			}
		}
		if added {
			continue
		}
		schedulingGroups = append(schedulingGroups, constraintsForPod(pod))
	}
	return schedulingGroups
}

// TODO
func (a *GreedyAllocator) matchesConstraints(constraints *cloudprovider.CapacityConstraints, pod *v1.Pod) bool {
	return false
}

func constraintsForPod(pod *v1.Pod) *cloudprovider.CapacityConstraints {
	return &cloudprovider.CapacityConstraints{
		Overhead:     calculateOverheadResources(),
		Architecture: getSystemArchitecture(pod),
		Topology:     map[cloudprovider.TopologyKey]string{},
		Pods:         []*v1.Pod{pod},
	}
}

func calculateOverheadResources() v1.ResourceList {
	//TODO
	return v1.ResourceList{}
}

func getSystemArchitecture(pod *v1.Pod) cloudprovider.Architecture {
	return cloudprovider.ArchitectureLinux386
}
