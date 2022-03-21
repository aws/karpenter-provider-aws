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
	"errors"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/resources"
)

// Node is a set of constraints, compatible pods, and possible instance types that could fulfill these constraints. This
// will be turned into one or more actual node instances within the cluster after bin packing.
type Node struct {
	Constraints         *v1alpha5.Constraints
	InstanceTypeOptions []cloudprovider.InstanceType
	Pods                []*v1.Pod
	unschedulable       bool // if true, the node will accept no more pods
}

func NewNode(constraints *v1alpha5.Constraints, instanceTypeOptions []cloudprovider.InstanceType, pods ...*v1.Pod) *Node {
	n := &Node{Constraints: constraints.DeepCopy()}

	for _, it := range instanceTypeOptions {
		// pre-filter our list of all possible instance types by what the provisioner allows
		if !compatibleInstanceType(constraints.Requirements, it) {
			continue
		}
		n.InstanceTypeOptions = append(n.InstanceTypeOptions, it)
	}
	// technically we might create some invalid construct here if the pod can't be supported by any instance type
	// but this is ok as we just won't have any valid instance types, this pod won't be schedulable and nothing else
	// will be compatible with this node
	for _, p := range pods {
		n.Add(p)
	}
	return n
}

func (n Node) Compatible(pod *v1.Pod) error {
	if n.unschedulable {
		return errors.New("node is unschedulable")
	}

	podRequirements := v1alpha5.NewPodRequirements(pod)
	if err := n.Constraints.Requirements.Compatible(podRequirements); err != nil {
		return err
	}

	tightened := n.Constraints.Requirements.Add(podRequirements.Requirements...)
	// Ensure that at least one instance type of the instance types that we are already narrowed down to based on the
	// existing pods can support the pod resources and combined pod + provider requirements
	for _, it := range n.InstanceTypeOptions {
		if compatibleInstanceType(tightened, it) && n.hasCompatibleResources(resources.RequestsForPods(pod), it) {
			return nil
		}
	}
	return errors.New("no matching instance type found")
}

// Add adds a pod to the Node which tightens constraints, possibly reducing the available instance type options for this
// node
func (n *Node) Add(pod *v1.Pod) {
	n.Pods = append(n.Pods, pod)
	n.Constraints = n.Constraints.Tighten(pod)
	var instanceTypeOptions []cloudprovider.InstanceType
	for _, it := range n.InstanceTypeOptions {
		if compatibleInstanceType(n.Constraints.Requirements, it) &&
			n.hasCompatibleResources(resources.RequestsForPods(pod), it) {
			instanceTypeOptions = append(instanceTypeOptions, it)
		}
	}
	n.InstanceTypeOptions = instanceTypeOptions
}

// hasCompatibleResources tests if a given node selector and resource request list is compatible with an instance type
func (n Node) hasCompatibleResources(resourceList v1.ResourceList, it cloudprovider.InstanceType) bool {
	for name, quantity := range resourceList {
		// we don't care if the pod is requesting zero quantity of some resource
		if quantity.IsZero() {
			continue
		}
		// instance must have a non-zero quantity
		if resources.IsZero(it.Resources()[name]) {
			return false
		}
	}
	return true
}

func compatibleInstanceType(requirements v1alpha5.Requirements, it cloudprovider.InstanceType) bool {
	if !requirements.Get(v1.LabelInstanceTypeStable).Has(it.Name()) {
		return false
	}
	if !requirements.Get(v1.LabelArchStable).Has(it.Architecture()) {
		return false
	}
	if !requirements.Get(v1.LabelOSStable).HasAny(it.OperatingSystems().List()...) {
		return false
	}
	// acceptable if we have any offering that is valid
	for _, offering := range it.Offerings() {
		if requirements.Get(v1.LabelTopologyZone).Has(offering.Zone) && requirements.Get(v1alpha5.LabelCapacityType).Has(offering.CapacityType) {
			return true
		}
	}
	return false
}
