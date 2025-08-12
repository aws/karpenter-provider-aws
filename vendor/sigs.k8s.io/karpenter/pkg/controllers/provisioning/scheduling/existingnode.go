/*
Copyright The Kubernetes Authors.

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
	"fmt"

	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

type ExistingNode struct {
	*state.StateNode
	cachedAvailable v1.ResourceList // Cache so we don't have to re-subtract resources on the StateNode every time
	cachedTaints    []v1.Taint      // Cache so we don't hae to re-construct the taints each time we attempt to schedule a pod

	Pods               []*v1.Pod
	topology           *Topology
	remainingResources v1.ResourceList
	requirements       scheduling.Requirements
}

func NewExistingNode(n *state.StateNode, topology *Topology, taints []v1.Taint, daemonResources v1.ResourceList) *ExistingNode {
	// The state node passed in here must be a deep copy from cluster state as we modify it
	// the remaining daemonResources to schedule are the total daemonResources minus what has already scheduled
	resources.SubtractFrom(daemonResources, n.DaemonSetRequests())
	// If unexpected daemonset pods schedule to the node due to labels appearing on the node which cause the
	// DS to be able to schedule, we need to ensure that we don't let our remainingDaemonResources go negative as
	// it will cause us to mis-calculate the amount of remaining resources
	for k, v := range daemonResources {
		if v.AsApproximateFloat64() < 0 {
			v.Set(0)
			daemonResources[k] = v
		}
	}
	available := n.Available()
	node := &ExistingNode{
		StateNode:          n,
		cachedAvailable:    available,
		cachedTaints:       taints,
		topology:           topology,
		remainingResources: resources.Subtract(available, daemonResources),
		requirements:       scheduling.NewLabelRequirements(n.Labels()),
	}
	node.requirements.Add(scheduling.NewRequirement(v1.LabelHostname, v1.NodeSelectorOpIn, n.HostName()))
	topology.Register(v1.LabelHostname, n.HostName())
	return node
}

// CanAdd returns whether the pod can be added to the ExistingNode
// based on the taints/tolerations, volume requirements, host port compatibility,
// requirements, resources, and topology requirements
func (n *ExistingNode) CanAdd(pod *v1.Pod, podData *PodData, volumes scheduling.Volumes) (updatedRequirements scheduling.Requirements, err error) {
	// Check Taints
	if err := scheduling.Taints(n.cachedTaints).ToleratesPod(pod); err != nil {
		return nil, err
	}
	// determine the host ports that will be used if the pod schedules
	hostPorts := scheduling.GetHostPorts(pod)
	if err = n.VolumeUsage().ExceedsLimits(volumes); err != nil {
		return nil, fmt.Errorf("checking volume usage, %w", err)
	}
	if err = n.HostPortUsage().Conflicts(pod, hostPorts); err != nil {
		return nil, fmt.Errorf("checking host port usage, %w", err)
	}
	// check resource requests first since that's a pretty likely reason the pod won't schedule on an in-flight
	// node, which at this point can't be increased in size
	if !resources.Fits(podData.Requests, n.remainingResources) {
		return nil, fmt.Errorf("exceeds node resources")
	}
	// Check NodeClaim Affinity Requirements
	if err = n.requirements.Compatible(podData.Requirements); err != nil {
		return nil, err
	}
	// avoid creating our temp set of requirements until after we've ensured that at least
	// the pod is compatible
	nodeRequirements := scheduling.NewRequirements(n.requirements.Values()...)
	nodeRequirements.Add(podData.Requirements.Values()...)

	// Check Topology Requirements
	topologyRequirements, err := n.topology.AddRequirements(pod, n.cachedTaints, podData.StrictRequirements, nodeRequirements)
	if err != nil {
		return nil, err
	}
	if err = nodeRequirements.Compatible(topologyRequirements); err != nil {
		return nil, err
	}
	nodeRequirements.Add(topologyRequirements.Values()...)
	return nodeRequirements, nil
}

// Add updates the ExistingNode to schedule the pod to this ExistingNode, updating
// the ExistingNode with new requirements and volumes based on the pod scheduling
func (n *ExistingNode) Add(pod *v1.Pod, podData *PodData, nodeRequirements scheduling.Requirements, volumes scheduling.Volumes) {
	// Update node
	n.Pods = append(n.Pods, pod)
	resources.SubtractFrom(n.remainingResources, podData.Requests)
	n.requirements = nodeRequirements
	n.topology.Record(pod, n.cachedTaints, nodeRequirements)
	n.HostPortUsage().Add(pod, scheduling.GetHostPorts(pod))
	n.VolumeUsage().Add(pod, volumes)
}
