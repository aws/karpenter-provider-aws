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

	"github.com/awslabs/karpenter/pkg/cloudprovider"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ Allocator = &GreedyAllocator{}

// GreedyAllocator iteratively assigns pods to scheduling groups and then creates capacity for each group.
type GreedyAllocator struct {
	Capacity cloudprovider.Capacity
}

// Allocate will take a list of pending pods and create node based on resources required.
func (a *GreedyAllocator) Allocate(pods []*v1.Pod) error {
	// 1. Separate pods into scheduling groups
	groups := a.getSchedulingGroups(pods)

	// 2. Group pods into equally schedulable constraint group
	for _, group := range groups {
		if err := a.Capacity.Create(context.TODO(), group.Constraints); err != nil {
			return fmt.Errorf("while creating capacity with constraints %v, %w", group.Constraints, err)
		}
	}
	return nil
}

type SchedulingGroup struct {
	Pods        []*v1.Pod
	Constraints *cloudprovider.CapacityConstraints
}

func (a *GreedyAllocator) getSchedulingGroups(pods []*v1.Pod) []*SchedulingGroup {
	groups := []*SchedulingGroup{}
	for _, pod := range pods {
		added := false
		for _, group := range groups {
			if a.matchesGroup(group, pod) {
				a.addToGroup(group, pod)
				added = true
				break
			}
		}
		if added {
			continue
		}
		newGroup := newSchedulingGroup()
		a.addToGroup(newGroup, pod)
		groups = append(groups, newGroup)
	}
	return groups
}

func newSchedulingGroup() *SchedulingGroup {
	return &SchedulingGroup{
		Constraints: &cloudprovider.CapacityConstraints{
			Resources: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("0"),
				v1.ResourceMemory: resource.MustParse("0"),
			},
		},
	}
}

func (a *GreedyAllocator) addToGroup(group *SchedulingGroup, pod *v1.Pod) {
	group.Constraints.Architecture = getSystemArchitecture(pod)
	group.Constraints.Zone = getAvalabiltyZoneForPod(pod)
	calculateResourcesFor(group.Constraints.Resources, pod)
	a.calculateOverheadResources(group.Constraints.Overhead)
	group.Pods = append(group.Pods)
}

// TODO
func (a *GreedyAllocator) matchesGroup(group *SchedulingGroup, pod *v1.Pod) bool {
	return false
}

// TODO
func (a *GreedyAllocator) calculateOverheadResources(resources v1.ResourceList) {
}

func getSystemArchitecture(pod *v1.Pod) cloudprovider.Architecture {
	return cloudprovider.Linux386
}

func getAvalabiltyZoneForPod(pod *v1.Pod) string {
	// TODO parse annotation/label from pod
	return "us-east-2b"
}
func calculateResourcesFor(resources v1.ResourceList, pod *v1.Pod) {
	for _, container := range pod.Spec.Containers {
		resources.Cpu().Add(*container.Resources.Limits.Cpu())
		resources.Memory().Add(*container.Resources.Limits.Memory())
	}
}
