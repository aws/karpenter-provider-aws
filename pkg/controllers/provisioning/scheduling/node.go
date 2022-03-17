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
}

func NewNode(constraints *v1alpha5.Constraints, instanceTypeOptions []cloudprovider.InstanceType, pods ...*v1.Pod) *Node {
	n := &Node{Constraints: constraints.DeepCopy()}

	instanceTypes := constraints.Requirements.InstanceTypes()
	for _, it := range instanceTypeOptions {
		// provisioner must list the instance type by name (defaults to all instance types)
		if !instanceTypes.Has(it.Name()) {
			continue
		}
		included := false
		// and the instance type must have some valid offering combination per the provisioner constraints
		for _, off := range it.Offerings() {
			if constraints.Requirements.Zones().Has(off.Zone) && constraints.Requirements.CapacityTypes().Has(off.CapacityType) {
				included = true
				break
			}
		}
		if included {
			n.InstanceTypeOptions = append(n.InstanceTypeOptions, it)
		}
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
	podRequirements := v1alpha5.NewPodRequirements(pod)
	if err := n.Constraints.Requirements.Compatible(podRequirements); err != nil {
		return err
	}

	// Ensure that at least one instance type of the instance types that we are already narrowed down to based on the
	// existing pods can support the pod
	for _, it := range n.InstanceTypeOptions {
		if n.isPodCompatible(pod, it) {
			return nil
		}
	}

	return errors.New("no matching instance type found")
}

func (n *Node) Add(pod *v1.Pod) {
	n.Pods = append(n.Pods, pod)
	n.Constraints = n.Constraints.Tighten(pod)
	var instanceTypeOptions []cloudprovider.InstanceType
	for _, it := range n.InstanceTypeOptions {
		if n.isPodCompatible(pod, it) {
			instanceTypeOptions = append(instanceTypeOptions, it)
		}
	}
	n.InstanceTypeOptions = instanceTypeOptions
}

// hasCompatibleNodeSelector returns true if a given set of node selectors match the instance type
//gocyclo:ignore
func (n Node) hasCompatibleNodeSelector(nodeSelector map[string]string, it cloudprovider.InstanceType) bool {
	if ns, ok := nodeSelector[v1.LabelInstanceTypeStable]; ok {
		if ns != it.Name() {
			return false
		}
	}

	provReqs := n.Constraints.Requirements
	hasOffering := func(pred func(off cloudprovider.Offering) bool) bool {
		for _, off := range it.Offerings() {
			// to be valid offering, the offering must be both supported by the pod and the provisioner
			if pred(off) && provReqs.Zones().Has(off.Zone) && provReqs.CapacityTypes().Has(off.CapacityType) {
				return true
			}
		}
		return false
	}

	ct, hasCt := nodeSelector[v1alpha5.LabelCapacityType]
	zone, hasZone := nodeSelector[v1.LabelTopologyZone]
	if hasCt && hasZone {
		if !hasOffering(func(off cloudprovider.Offering) bool { return off.CapacityType == ct && off.Zone == zone }) {
			return false
		}
	} else if hasCt {
		if !hasOffering(func(off cloudprovider.Offering) bool { return off.CapacityType == ct }) {
			return false
		}
	} else if hasZone {
		if !hasOffering(func(off cloudprovider.Offering) bool { return off.Zone == zone }) {
			return false
		}
	}

	if arch, ok := nodeSelector[v1.LabelArchStable]; ok {
		if it.Architecture() != arch {
			return false
		}
	}

	if os, ok := nodeSelector[v1.LabelOSStable]; ok {
		if !it.OperatingSystems().Has(os) {
			return false
		}
	}
	return true
}

func (n Node) isPodCompatible(pod *v1.Pod, it cloudprovider.InstanceType) bool {
	return n.hasCompatibleResources(resources.RequestsForPods(pod), it) &&
		n.hasCompatibleNodeSelector(pod.Spec.NodeSelector, it)
}

// hasCompatibleResources tests if a given node selector and resource request list is compatible with an instance type
func (n Node) hasCompatibleResources(resourceList v1.ResourceList, it cloudprovider.InstanceType) bool {
	for name, quantity := range resourceList {
		switch name {
		case resources.NvidiaGPU:
			if it.NvidiaGPUs().Cmp(quantity) < 0 {
				return false
			}
		case resources.AWSNeuron:
			if it.AWSNeurons().Cmp(quantity) < 0 {
				return false
			}
		case resources.AMDGPU:
			if it.AMDGPUs().Cmp(quantity) < 0 {
				return false
			}
		case resources.AWSPodENI:
			if it.AWSPodENI().Cmp(quantity) < 0 {
				return false
			}
		case resources.AWSPodPrivateIPv4:
			if it.AWSPodPrivateIPv4().Cmp(quantity) < 0 {
				return false
			}
		default:
			continue
		}
	}
	return true
}
