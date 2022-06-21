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

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/aws/karpenter/pkg/utils/sets"
)

type InFlightNode struct {
	Pods               []*v1.Pod
	Node               *v1.Node
	requests           v1.ResourceList
	topology           *Topology
	requirements       scheduling.Requirements
	available          v1.ResourceList
	startupTolerations []v1.Toleration
	hostPortUsage      *scheduling.HostPortUsage
	volumeUsage        *scheduling.VolumeLimits
	volumeLimits       scheduling.VolumeCount
}

func NewInFlightNode(n *state.Node, topology *Topology, startupTaints []v1.Taint, daemonResources v1.ResourceList) *InFlightNode {
	// the remaining daemonResources to schedule are the total daemonResources minus what has already scheduled
	remainingDaemonResources := resources.Subtract(daemonResources, n.DaemonSetRequested)
	node := &InFlightNode{
		Node:          n.Node,
		available:     n.Available,
		topology:      topology,
		requests:      remainingDaemonResources,
		requirements:  scheduling.NewLabelRequirements(n.Node.Labels),
		hostPortUsage: n.HostPortUsage.Copy(),
		volumeUsage:   n.VolumeUsage.Copy(),
		volumeLimits:  n.VolumeLimits,
	}

	if n.Node.Labels[v1alpha5.LabelNodeInitialized] != "true" {
		// add a default toleration for the standard not ready and startup taints if the node hasn't fully
		// launched yet
		node.startupTolerations = append(node.startupTolerations, v1.Toleration{
			Key:      v1.TaintNodeNotReady,
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		})
		node.startupTolerations = append(node.startupTolerations, v1.Toleration{
			Key:      v1.TaintNodeUnreachable,
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		})
	}

	for _, taint := range startupTaints {
		node.startupTolerations = append(node.startupTolerations, scheduling.TaintToToleration(taint))
	}

	// If the in-flight node doesn't have a hostname yet, we treat it's unique name as the hostname.  This allows toppology
	// with hostname keys to schedule correctly.
	hostname := n.Node.Labels[v1.LabelHostname]
	if hostname == "" {
		hostname = n.Node.Name
	}
	node.requirements.Add(scheduling.Requirements{v1.LabelHostname: sets.NewSet(hostname)})
	topology.Register(v1.LabelHostname, hostname)
	return node
}

func (n *InFlightNode) Add(ctx context.Context, pod *v1.Pod) error {
	// Check Taints
	if err := scheduling.Taints(n.Node.Spec.Taints).Tolerates(pod, n.startupTolerations...); err != nil {
		return err
	}

	if err := n.hostPortUsage.Validate(pod); err != nil {
		return err
	}

	// determine the number of volumes that will be mounted if the pod schedules
	mountedVolumeCount, err := n.volumeUsage.Validate(ctx, pod)
	if err != nil {
		return err
	}
	if mountedVolumeCount.Exceeds(n.volumeLimits) {
		return fmt.Errorf("would exceed node volume limits")
	}

	// check resource requests first since that's a pretty likely reason the pod won't schedule on an in-flight
	// node, which at this point can't be increased in size
	requests := resources.Merge(n.requests, resources.RequestsForPods(pod))

	if !resources.Fits(requests, n.available) {
		return fmt.Errorf("exceeds node resources")
	}

	nodeRequirements := scheduling.NewRequirements(n.requirements)
	podRequirements := scheduling.NewPodRequirements(pod)
	// Check Node Affinity Requirements
	if err := nodeRequirements.Compatible(podRequirements); err != nil {
		return err
	}
	nodeRequirements.Add(podRequirements)

	// Check Topology Requirements
	topologyRequirements, err := n.topology.AddRequirements(podRequirements, nodeRequirements, pod)
	if err != nil {
		return err
	}
	if err = nodeRequirements.Compatible(topologyRequirements); err != nil {
		return err
	}
	nodeRequirements.Add(topologyRequirements)

	// Update node
	n.Pods = append(n.Pods, pod)
	n.requests = requests
	n.requirements = nodeRequirements
	n.topology.Record(pod, nodeRequirements)
	n.hostPortUsage.Add(ctx, pod)
	n.volumeUsage.Add(ctx, pod)
	return nil
}
