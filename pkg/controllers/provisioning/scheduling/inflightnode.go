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
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/utils/resources"
)

type InFlightNode struct {
	Pods               []*v1.Pod
	Node               *v1.Node
	requests           v1.ResourceList
	topology           *Topology
	requirements       v1alpha5.Requirements
	available          v1.ResourceList
	startupTolerations []v1.Toleration
}

func NewInFlightNode(n *state.Node, topology *Topology, startupTaints []v1.Taint, daemonResources v1.ResourceList) *InFlightNode {
	// the remaining daemonResources to schedule are the total daemonResources minus what has already scheduled
	remainingDaemonResources := resources.Subtract(daemonResources, n.DaemonSetRequested)

	node := &InFlightNode{
		Node:         n.Node,
		available:    n.Available,
		topology:     topology,
		requests:     remainingDaemonResources,
		requirements: v1alpha5.NewLabelRequirements(n.Node.Labels),
	}

	if _, notReady := n.Node.Annotations[v1alpha5.NotReadyAnnotationKey]; notReady {
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
		node.startupTolerations = append(node.startupTolerations, v1alpha5.TaintToToleration(taint))
	}

	// If the in-flight node doesn't have a hostname yet, we treat it's unique name as the hostname.  This allows toppology
	// with hostname keys to schedule correctly.
	hostname := n.Node.Labels[v1.LabelHostname]
	if hostname == "" {
		hostname = n.Node.Name
	}
	node.requirements = node.requirements.Add(
		v1.NodeSelectorRequirement{
			Key:      v1.LabelHostname,
			Operator: v1.NodeSelectorOpIn,
			Values:   []string{hostname},
		})
	topology.Register(v1.LabelHostname, hostname)
	return node
}

func (n *InFlightNode) Add(pod *v1.Pod) error {
	taints := v1alpha5.Taints(n.Node.Spec.Taints)
	if err := taints.Tolerates(pod, n.startupTolerations...); err != nil {
		return err
	}

	// check resource requests first since that's a pretty likely reason the pod won't schedule on an in-flight
	// node, which at this point can't be increased in size
	requests := resources.Merge(n.requests, resources.RequestsForPods(pod))

	if !resources.Fits(requests, n.available) {
		return fmt.Errorf("exceeds node resources")
	}

	podRequirements := v1alpha5.NewPodRequirements(pod)
	// Check initial compatibility
	if err := n.requirements.Compatible(podRequirements); err != nil {
		return err
	}
	nodeRequirements := n.requirements.Add(podRequirements.Requirements...)

	// Include topology requirements
	requirements, err := n.topology.AddRequirements(podRequirements, nodeRequirements, pod)
	if err != nil {
		return err
	}
	// Check node compatibility
	if err = n.requirements.Compatible(requirements); err != nil {
		return err
	}
	// Tighten requirements
	requirements = n.requirements.Add(requirements.Requirements...)

	// Update node
	n.requests = requests
	n.requirements = requirements
	n.Pods = append(n.Pods, pod)
	n.topology.Record(pod, requirements)
	return nil
}
