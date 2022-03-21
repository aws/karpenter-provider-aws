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
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/resources"
)

// Node is a set of constraints, compatible pods, and possible instance types that could fulfill these constraints. This
// will be turned into one or more actual node instances within the cluster after bin packing.
type Node struct {
	v1alpha5.Constraints
	InstanceTypeOptions []cloudprovider.InstanceType
	Pods                []*v1.Pod

	podResources    v1.ResourceList
	daemonResources v1.ResourceList
	daemons         []daemon
}

type daemon struct {
	pod          *v1.Pod
	requirements v1alpha5.Requirements
	resources    v1.ResourceList
}

func NewNode(constraints *v1alpha5.Constraints, daemons []*v1.Pod, instanceTypeOptions []cloudprovider.InstanceType, pods ...*v1.Pod) (*Node, error) {
	n := &Node{
		Constraints: *constraints.DeepCopy(),
	}

	for _, d := range daemons {
		// skip any daemons that our provisioner configured taints would cause to not schedule
		if err := n.Taints.Tolerates(d); err != nil {
			continue
		}
		n.daemons = append(n.daemons, daemon{
			pod:          d,
			requirements: v1alpha5.NewPodRequirements(d),
			resources:    resources.RequestsForPods(d),
		})
	}

	// calculate daemon resource consumption so we can filter out instance types based on daemons + instance type overhead
	n.recalculateDaemonResources()

	for _, it := range instanceTypeOptions {
		// If a zero-resource pod can't fit, don't consider this instance type.  This occurs if the node overhead +
		// daemon set requests is already larger than the instance type can support
		if !n.newPodCanFit(nil, it) {
			continue
		}

		// pre-filter our list of all possible instance types by what the provisioner allows
		if !cloudprovider.Compatible(it, constraints.Requirements) {
			continue
		}
		n.InstanceTypeOptions = append(n.InstanceTypeOptions, it)
	}

	for _, p := range pods {
		n.Add(p)
	}
	if len(n.InstanceTypeOptions) == 0 {
		return nil, errors.New("no instance type satisfied requirements")
	}
	return n, nil
}

func (n Node) Compatible(pod *v1.Pod) error {
	podRequirements := v1alpha5.NewPodRequirements(pod)
	if err := n.Requirements.Compatible(podRequirements); err != nil {
		return err
	}

	tightened := n.Requirements.Add(podRequirements.Requirements...)
	// Ensure that at least one instance type of the instance types that we are already narrowed down to based on the
	// existing pods can support the pod resources and combined pod + provider requirements
	podRequests := resources.RequestsForPods(pod)
	for _, it := range n.InstanceTypeOptions {
		newSize := resources.Merge(n.reservedResources(it), podRequests)
		if cloudprovider.Compatible(it, tightened) &&
			n.newPodCanFit(newSize, it) &&
			n.hasCompatibleResources(podRequests, it) {
			return nil
		}
	}
	return errors.New("no matching instance type found")
}

func (n Node) reservedResources(it cloudprovider.InstanceType) v1.ResourceList {
	return resources.Merge(it.Overhead(), n.daemonResources, n.podResources)
}

func (n *Node) newPodCanFit(newSize v1.ResourceList, it cloudprovider.InstanceType) bool {
	for resourceName, totalQuantity := range it.Resources() {
		reservedQuantity := newSize[resourceName]
		if reservedQuantity.Cmp(totalQuantity) > 0 {
			return false
		}
	}

	instancePodMax := it.Resources()[v1.ResourcePods]
	if !instancePodMax.IsZero() && instancePodMax.CmpInt64(int64(len(n.Pods)+1)) < 0 {
		return false
	}
	return true
}

// Add adds a pod to the Node which tightens constraints, possibly reducing the available instance type options for this
// node
func (n *Node) Add(pod *v1.Pod) {
	n.Requirements = n.Requirements.Add(v1alpha5.NewPodRequirements(pod).Requirements...)
	n.recalculateDaemonResources()

	podRequests := resources.RequestsForPods(pod)
	var instanceTypeOptions []cloudprovider.InstanceType
	for _, it := range n.InstanceTypeOptions {
		newSize := resources.Merge(n.reservedResources(it), podRequests)
		if cloudprovider.Compatible(it, n.Requirements) &&
			n.newPodCanFit(newSize, it) &&
			n.hasCompatibleResources(resources.RequestsForPods(pod), it) {
			instanceTypeOptions = append(instanceTypeOptions, it)
		}
	}
	// have to add the pod after filtering instance types as newPodCanFit() checks if a new pod can fit, including the
	// pod count
	n.Pods = append(n.Pods, pod)
	n.InstanceTypeOptions = instanceTypeOptions
	n.podResources = resources.Merge(n.podResources, resources.RequestsForPods(pod))
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

func (n *Node) recalculateDaemonResources() {
	n.daemonResources = nil
	for _, daemon := range n.daemons {
		// we intentionally do not check if the DaemonSet pod will fit on the node here, we want large DS pods to cause
		// us to choose larger instance types
		if err := n.Requirements.Compatible(daemon.requirements); err != nil {
			continue
		}
		n.daemonResources = resources.Merge(n.daemonResources, daemon.resources)
	}
}

func (n Node) String() string {
	var resSb strings.Builder

	var requiredResources v1.ResourceList
	if len(n.InstanceTypeOptions) == 0 {
		requiredResources = resources.Merge(n.daemonResources, n.podResources)
	} else {
		requiredResources = resources.Merge(n.daemonResources, n.InstanceTypeOptions[0].Overhead(), n.podResources)
	}

	for k, v := range requiredResources {
		fmt.Fprintf(&resSb, "%s: %s ", k, v.String())
	}

	var itSb strings.Builder
	for i, it := range n.InstanceTypeOptions {
		// print the first 5 instance types only (indices 0-4)
		if i > 4 {
			fmt.Fprintf(&itSb, " and %d others", len(n.InstanceTypeOptions)-i)
			break
		} else if i > 0 {
			fmt.Fprint(&itSb, ", ")
		}
		fmt.Fprint(&itSb, it.Name())
	}

	return fmt.Sprintf("with %d pods using resources %s from types %s", len(n.Pods), resSb.String(), itSb.String())
}
