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
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

type ExistingNode struct {
	*state.StateNode
	cachedAvailable v1.ResourceList // Cache so we don't have to re-subtract resources on the StateNode every time
	cachedTaints    []v1.Taint      // Cache so we don't hae to re-construct the taints each time we attempt to schedule a pod

	Pods         []*v1.Pod
	topology     *Topology
	requests     v1.ResourceList
	requirements scheduling.Requirements
}

func NewExistingNode(n *state.StateNode, topology *Topology, taints []v1.Taint, daemonResources v1.ResourceList) *ExistingNode {
	// The state node passed in here must be a deep copy from cluster state as we modify it
	// the remaining daemonResources to schedule are the total daemonResources minus what has already scheduled
	remainingDaemonResources := resources.Subtract(daemonResources, n.DaemonSetRequests())
	// If unexpected daemonset pods schedule to the node due to labels appearing on the node which cause the
	// DS to be able to schedule, we need to ensure that we don't let our remainingDaemonResources go negative as
	// it will cause us to mis-calculate the amount of remaining resources
	for k, v := range remainingDaemonResources {
		if v.AsApproximateFloat64() < 0 {
			v.Set(0)
			remainingDaemonResources[k] = v
		}
	}
	node := &ExistingNode{
		StateNode:       n,
		cachedAvailable: n.Available(),
		cachedTaints:    taints,
		topology:        topology,
		requests:        remainingDaemonResources,
		requirements:    scheduling.NewLabelRequirements(n.Labels()),
	}
	node.requirements.Add(scheduling.NewRequirement(v1.LabelHostname, v1.NodeSelectorOpIn, n.HostName()))
	topology.Register(v1.LabelHostname, n.HostName())
	return node
}

func (n *ExistingNode) Add(ctx context.Context, kubeClient client.Client, pod *v1.Pod, podData *PodData) error {
	// Check Taints
	if err := scheduling.Taints(n.cachedTaints).ToleratesPod(pod); err != nil {
		return err
	}
	// determine the volumes that will be mounted if the pod schedules
	volumes, err := scheduling.GetVolumes(ctx, kubeClient, pod)
	if err != nil {
		return err
	}
	// determine the host ports that will be used if the pod schedules
	hostPorts := scheduling.GetHostPorts(pod)
	if err = n.VolumeUsage().ExceedsLimits(volumes); err != nil {
		return fmt.Errorf("checking volume usage, %w", err)
	}
	if err = n.HostPortUsage().Conflicts(pod, hostPorts); err != nil {
		return fmt.Errorf("checking host port usage, %w", err)
	}

	// check resource requests first since that's a pretty likely reason the pod won't schedule on an in-flight
	// node, which at this point can't be increased in size
	requests := resources.Merge(n.requests, podData.Requests)

	if !resources.Fits(requests, n.cachedAvailable) {
		return fmt.Errorf("exceeds node resources")
	}

	nodeRequirements := scheduling.NewRequirements(n.requirements.Values()...)
	// Check NodeClaim Affinity Requirements
	if err = nodeRequirements.Compatible(podData.Requirements); err != nil {
		return err
	}
	nodeRequirements.Add(podData.Requirements.Values()...)

	// Check Topology Requirements
	topologyRequirements, err := n.topology.AddRequirements(pod, n.cachedTaints, podData.StrictRequirements, nodeRequirements)
	if err != nil {
		return err
	}
	if err = nodeRequirements.Compatible(topologyRequirements); err != nil {
		return err
	}
	nodeRequirements.Add(topologyRequirements.Values()...)

	// Update node
	n.Pods = append(n.Pods, pod)
	n.requests = requests
	n.requirements = nodeRequirements
	n.topology.Record(pod, n.cachedTaints, nodeRequirements)
	n.HostPortUsage().Add(pod, hostPorts)
	n.VolumeUsage().Add(pod, volumes)
	return nil
}
