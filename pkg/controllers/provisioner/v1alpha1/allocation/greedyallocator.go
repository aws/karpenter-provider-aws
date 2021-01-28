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
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ Allocator = &GreedyAllocator{}

// GreedyAllocator iteratively assigns pods to scheduling groups and then creates capacity for each group.
type GreedyAllocator struct {
	Capacity cloudprovider.Capacity
}

// Allocate takes a list of unschedulable pods and creates nodes based on resources required.
func (a *GreedyAllocator) Allocate(pods []*v1.Pod) error {
	// 1. Separate pods into scheduling groups
	groups := a.getSchedulingGroups(pods)

	zap.S().Infof("Allocating pending pods count %d grouped into count %d \n", len(pods), len(groups))
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
				addPodResourcesToList(group.Constraints.Resources, pod)
				group.Pods = append(group.Pods, pod)
				added = true
				break
			}
		}
		if added {
			continue
		}
		groups = append(groups, schedulingGroupForPod(pod))
	}
	return groups
}

// TODO
func (a *GreedyAllocator) matchesGroup(group *SchedulingGroup, pod *v1.Pod) bool {
	return false
}

func schedulingGroupForPod(pod *v1.Pod) *SchedulingGroup {
	group := &SchedulingGroup{
		Constraints: &cloudprovider.CapacityConstraints{
			Resources:    calculateResourcesForPod(pod),
			Overhead:     calculateOverheadResources(),
			Architecture: getSystemArchitecture(pod),
			Zone:         getAvalabiltyZoneForPod(pod),
		},
	}
	return group
}

func calculateResourcesForPod(pod *v1.Pod) v1.ResourceList {
	resourceList := v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("0"),
		v1.ResourceMemory: resource.MustParse("0"),
	}
	addPodResourcesToList(resourceList, pod)
	return resourceList
}

func addPodResourcesToList(resources v1.ResourceList, pod *v1.Pod) {
	for _, container := range pod.Spec.Containers {
		resources.Cpu().Add(*container.Resources.Limits.Cpu())
		resources.Memory().Add(*container.Resources.Limits.Memory())
	}
}

func calculateOverheadResources() v1.ResourceList {
	//TODO
	return v1.ResourceList{}
}

func getSystemArchitecture(pod *v1.Pod) cloudprovider.Architecture {
	return cloudprovider.Linux386
}

func getAvalabiltyZoneForPod(pod *v1.Pod) string {
	// TODO parse annotation/label from pod
	return "us-east-2b"
}
