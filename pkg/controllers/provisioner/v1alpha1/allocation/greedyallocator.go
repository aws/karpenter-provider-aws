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
)

var _ Allocator = &GreedyAllocator{}

// GreedyAllocator iteratively assigns pods to scheduling groups and then creates capacity for each group.
type GreedyAllocator struct {
	Capacity cloudprovider.Capacity
}

//
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
	Constraints cloudprovider.CapacityConstraints
}

func (a *GreedyAllocator) getSchedulingGroups(pods []*v1.Pod) []SchedulingGroup {
	groups := []SchedulingGroup{}

	for _, pod := range pods {
		for _, group := range groups {
			if a.matchesGroup(pod, group) {
				group.Pods = append(group.Pods, pod)
				break
			}
		}
	}

	return groups
}

// TODO
func (a *GreedyAllocator) matchesGroup(pod *v1.Pod, group SchedulingGroup) bool {
	return true
}
