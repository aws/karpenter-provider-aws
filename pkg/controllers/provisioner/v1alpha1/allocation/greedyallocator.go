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
	"k8s.io/apimachinery/pkg/api/resource"
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

	zap.S().Infof("Allocating %d pending pods from %d constraint groups", len(pods), len(groups))
	// 2. Group pods into equally schedulable constraint group
	for _, group := range groups {
		nodes, err := a.CloudProvider.CapacityFor(&provisioner.Spec).Create(context.TODO(), group.Constraints)
		// TODO accumulate errors if one request fails.
		if err != nil {
			return fmt.Errorf("while creating capacity, %w", err)
		}
		if err := a.createNodesAndAssignPods(nodes, group.Pods); err != nil {
			return fmt.Errorf("assigning pods to nodes err: %w", err)
		}
	}
	return nil
}

func (a *GreedyAllocator) createNodesAndAssignPods(nodes []*v1.Node, pods []*v1.Pod) error {

	// Currently we are assigning each pod per node.
	// Create node object in the cluster
	// score nodes and pods from bigger to lower in the list
	remainingPods := make([]*v1.Pod, len(pods))
	copy(remainingPods, pods)
	for _, node := range nodes {
		err := a.createNodeObject(node)
		if err != nil {
			return fmt.Errorf("creating node object %w", err)
		}
		remainingPods, err = a.assignPodsToNodes(node, remainingPods)
		if err != nil {
			return fmt.Errorf("update pod spec err: %w", err)
		}
	}
	if len(remainingPods) > 0 {
		// this should not happen
		return fmt.Errorf("unable to assign %d pods to %d nodes", len(remainingPods), len(nodes))
	}
	zap.S().Infof("Successfully assigned %d pods to %d node ", len(pods), len(nodes))
	return nil
}

func (a *GreedyAllocator) assignPodsToNodes(node *v1.Node, pods []*v1.Pod) ([]*v1.Pod, error) {

	remainingPods := make([]*v1.Pod, 0)
	for _, pod := range pods {
		if !canFitPodOnNode(node, pod) {
			remainingPods = append(remainingPods, pod)
			continue
		}
		if err := a.bindPodToNode(node, pod); err != nil {
			return nil, fmt.Errorf("binding pod to node failed err %w", err)
		}
		zap.S().Infof("Pod %s in bind to node %s", pod.Namespace+"/"+pod.Name, node.Name)
	}
	return remainingPods, nil
}

func (a *GreedyAllocator) bindPodToNode(node *v1.Node, pod *v1.Pod) error {
	return a.CoreV1Client.Pods(pod.Namespace).Bind(context.TODO(), &v1.Binding{
		TypeMeta:   pod.TypeMeta,
		ObjectMeta: pod.ObjectMeta,
		Target: v1.ObjectReference{
			Name: node.Name,
		},
	}, metav1.CreateOptions{})
}

func canFitPodOnNode(node *v1.Node, pod *v1.Pod) bool {
	// TODO  podResources := calculateResourcesForPod(pod)
	return true
}

func (a *GreedyAllocator) createNodeObject(node *v1.Node) error {
	_, err := a.CoreV1Client.
		Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
	return err
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
			Topology: map[cloudprovider.TopologyKey]string{
				cloudprovider.TopologyKeyZone: getAvalabiltyZoneForPod(pod),
			},
		},
		Pods: []*v1.Pod{pod},
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
	return "us-west-2b"
}
