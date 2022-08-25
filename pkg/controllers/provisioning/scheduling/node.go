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
	"strings"
	"sync/atomic"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/utils/resources"
)

// Node is a set of constraints, compatible pods, and possible instance types that could fulfill these constraints. This
// will be turned into one or more actual node instances within the cluster after bin packing.
type Node struct {
	scheduling.NodeTemplate
	InstanceTypeOptions []cloudprovider.InstanceType
	Pods                []*v1.Pod

	topology      *Topology
	requests      v1.ResourceList
	hostPortUsage *scheduling.HostPortUsage
}

var nodeID int64

func NewNode(nodeTemplate *scheduling.NodeTemplate, topology *Topology, daemonResources v1.ResourceList, instanceTypes []cloudprovider.InstanceType) *Node {
	// Copy the template, and add hostname
	hostname := fmt.Sprintf("hostname-placeholder-%04d", atomic.AddInt64(&nodeID, 1))
	topology.Register(v1.LabelHostname, hostname)
	template := *nodeTemplate
	template.Requirements = scheduling.NewRequirements()
	template.Requirements.Add(nodeTemplate.Requirements.Values()...)
	template.Requirements.Add(scheduling.NewRequirement(v1.LabelHostname, v1.NodeSelectorOpIn, hostname))

	return &Node{
		NodeTemplate:        template,
		InstanceTypeOptions: instanceTypes,
		hostPortUsage:       scheduling.NewHostPortUsage(),
		topology:            topology,
		requests:            daemonResources,
	}
}

func (n *Node) Add(ctx context.Context, pod *v1.Pod) error {
	// Check Taints
	if err := n.Taints.Tolerates(pod); err != nil {
		return err
	}
	n.constrainExistingTaints(pod)
	n.constrainDynamicTaints(pod)

	// exposed host ports on the node
	if err := n.hostPortUsage.Validate(pod); err != nil {
		return err
	}

	nodeRequirements := scheduling.NewRequirements(n.Requirements.Values()...)
	podRequirements := scheduling.NewPodRequirements(pod)

	// Check Node Affinity Requirements
	if err := nodeRequirements.Compatible(podRequirements); err != nil {
		return fmt.Errorf("incompatible requirements, %w", err)
	}
	nodeRequirements.Add(podRequirements.Values()...)

	// Check Topology Requirements
	topologyRequirements, err := n.topology.AddRequirements(podRequirements, nodeRequirements, pod)
	if err != nil {
		return err
	}
	if err = nodeRequirements.Compatible(topologyRequirements); err != nil {
		return err
	}
	nodeRequirements.Add(topologyRequirements.Values()...)

	// Check instance type combinations
	requests := resources.Merge(n.requests, resources.RequestsForPods(pod))
	instanceTypes := filterInstanceTypesByRequirements(n.InstanceTypeOptions, nodeRequirements, requests)
	if len(instanceTypes) == 0 {
		return fmt.Errorf("no instance type satisfied resources %s and requirements %s", resources.String(resources.RequestsForPods(pod)), nodeRequirements)
	}

	// Update node
	n.Pods = append(n.Pods, pod)
	n.InstanceTypeOptions = instanceTypes
	n.requests = requests
	n.Requirements = nodeRequirements
	n.topology.Record(pod, nodeRequirements)
	n.hostPortUsage.Add(ctx, pod)
	return nil
}

// FinalizeScheduling is called once all scheduling has completed and allows the node to perform any cleanup
// necessary before its requirements are used for instance launching
func (n *Node) FinalizeScheduling() {
	// We need nodes to have hostnames for topology purposes, but we don't want to pass that node name on to consumers
	// of the node as it will be displayed in error messages
	delete(n.Requirements, v1.LabelHostname)

	// If there was no constraint that was put on a taint when it was added to the node after
	// a full round of scheduling then we need to set it to a valid taint value
	for i := range n.Taints {
		if n.Taints[i].Value == v1alpha5.TaintWildcardValue {
			n.Taints[i].Value = ""
		}
	}
}

func (n *Node) String() string {
	return fmt.Sprintf("node with %d pods requesting %s from types %s", len(n.Pods), resources.String(n.requests),
		InstanceTypeList(n.InstanceTypeOptions))
}

// constrainExistingTaints constrains any wildcard taints that were added to
// the node taints. If this new pod was found to be able to tolerate the existing
// taints, it is possible that one of the taints that it tolerates doesn't have a
// fixed value yet. In this case, we need to fix the value if there isn't one.
func (n *Node) constrainExistingTaints(pod *v1.Pod) {
	for i := range n.Taints {
		taint := n.Taints[i]
		if taint.Value == v1alpha5.TaintWildcardValue {
			for _, t := range pod.Spec.Tolerations {
				if t.Key == taint.Key && (t.Operator == "" || t.Operator == v1.TolerationOpEqual) && (t.Effect == "" || t.Effect == taint.Effect) {
					n.Taints[i].Value = t.Value
					break
				}
			}
		}
	}
}

// constrainDynamicTaints add required taints to the node template when we find a toleration on
// a pod that has the matching key and effect for the wildcard taint. In this case, we do one of two things:
// 1. If the pod has any toleration with operator=Equal, we match the taint to that value
// 2. If the pod has no tolerations with operator=Equal but has a toleration with operator=Exists, we add the
// taint with an unconstrained value
func (n *Node) constrainDynamicTaints(pod *v1.Pod) {
	var updatedDynamicTaints []v1.Taint
	for _, taint := range n.DynamicTaints {
		relevantTolerations := lo.Filter(pod.Spec.Tolerations, func(t v1.Toleration, _ int) bool {
			return t.Key == taint.Key && (t.Effect == taint.Effect || t.Effect == "")
		})
		if len(relevantTolerations) > 0 {
			equalTolerations := lo.Filter(relevantTolerations, func(t v1.Toleration, _ int) bool {
				return t.Operator == v1.TolerationOpEqual || t.Operator == ""
			})
			// If we have a toleration that matches the flexible taint, then we constrain it with the value
			// We pick the first value that we see that matches the taint key and operator
			if len(equalTolerations) > 0 {
				n.Taints = append(n.Taints, v1.Taint{
					Key:    taint.Key,
					Effect: taint.Effect,
					Value:  equalTolerations[0].Value,
				})
			} else {
				n.Taints = append(n.Taints, v1.Taint{
					Key:    taint.Key,
					Effect: taint.Effect,
					Value:  v1alpha5.TaintWildcardValue,
				})
			}
		} else if scheduling.Taint(taint).Tolerates(pod) {
			// Basically, if there is something like an empty key toleration that allows the pod to tolerate this taint
			// but it doesn't directly match the key, we can leave it in the dynamic taints list
			updatedDynamicTaints = append(updatedDynamicTaints, taint)
		}
	}
	n.DynamicTaints = updatedDynamicTaints
}

func InstanceTypeList(instanceTypeOptions []cloudprovider.InstanceType) string {
	var itSb strings.Builder
	for i, it := range instanceTypeOptions {
		// print the first 5 instance types only (indices 0-4)
		if i > 4 {
			fmt.Fprintf(&itSb, " and %d other(s)", len(instanceTypeOptions)-i)
			break
		} else if i > 0 {
			fmt.Fprint(&itSb, ", ")
		}
		fmt.Fprint(&itSb, it.Name())
	}
	return itSb.String()
}

func filterInstanceTypesByRequirements(instanceTypes []cloudprovider.InstanceType, requirements scheduling.Requirements, requests v1.ResourceList) []cloudprovider.InstanceType {
	return lo.Filter(instanceTypes, func(instanceType cloudprovider.InstanceType, _ int) bool {
		return compatible(instanceType, requirements) && fits(instanceType, requests) && hasOffering(instanceType, requirements)
	})
}

func compatible(instanceType cloudprovider.InstanceType, requirements scheduling.Requirements) bool {
	return instanceType.Requirements().Intersects(requirements) == nil
}

func fits(instanceType cloudprovider.InstanceType, requests v1.ResourceList) bool {
	return resources.Fits(resources.Merge(requests, instanceType.Overhead()), instanceType.Resources())
}

func hasOffering(instanceType cloudprovider.InstanceType, requirements scheduling.Requirements) bool {
	for _, offering := range cloudprovider.AvailableOfferings(instanceType) {
		if (!requirements.Has(v1.LabelTopologyZone) || requirements.Get(v1.LabelTopologyZone).Has(offering.Zone)) &&
			(!requirements.Has(v1alpha5.LabelCapacityType) || requirements.Get(v1alpha5.LabelCapacityType).Has(offering.CapacityType)) {
			return true
		}
	}
	return false
}
