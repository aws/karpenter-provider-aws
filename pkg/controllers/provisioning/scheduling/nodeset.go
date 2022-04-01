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
	"sort"

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/resources"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

type NodeSet struct {
	kubeClient      client.Client
	daemonResources v1.ResourceList
	nodes           []*Node
	constraints     *v1alpha5.Constraints
	instanceTypes   []cloudprovider.InstanceType
	topology        *Topology
}

func NewNodeSet(ctx context.Context, topology *Topology, constraints *v1alpha5.Constraints, instanceTypes []cloudprovider.InstanceType, client client.Client) (*NodeSet, error) {
	ns := &NodeSet{
		kubeClient:    client,
		constraints:   constraints,
		instanceTypes: instanceTypes,
		topology:      topology,
	}

	daemons, err := ns.getDaemons(ctx, constraints)
	if err != nil {
		return nil, err
	}

	for _, d := range daemons {
		// skip any daemons that our provisioner configured taints would cause to not schedule
		if err := constraints.Taints.Tolerates(d); err != nil {
			continue
		}
		// or that aren't compatible with provisioner requirements
		if err := constraints.Requirements.Compatible(v1alpha5.NewPodRequirements(d)); err != nil {
			continue
		}
		ns.daemonResources = resources.Merge(ns.daemonResources, resources.RequestsForPods(d))
	}
	return ns, nil
}

func (s *NodeSet) getDaemons(ctx context.Context, constraints *v1alpha5.Constraints) ([]*v1.Pod, error) {
	daemonSetList := &appsv1.DaemonSetList{}
	if err := s.kubeClient.List(ctx, daemonSetList); err != nil {
		return nil, fmt.Errorf("listing daemonsets, %w", err)
	}
	// Include DaemonSets that will schedule on this node
	var pods []*v1.Pod
	for _, daemonSet := range daemonSetList.Items {
		p := &v1.Pod{Spec: daemonSet.Spec.Template.Spec}
		if err := constraints.ValidatePod(p); err == nil {
			pods = append(pods, p)
		}
	}
	return pods, nil
}

func (s *NodeSet) Add(node *Node) {
	s.nodes = append(s.nodes, node)
}

func (s *NodeSet) Schedule(ctx context.Context, p *v1.Pod) error {
	// try nodes in ascending order of the number of pods on the node to more evenly distribute nodes
	// This sorts upon every pod schedule, but it is likely fast enough.  I benchmarked sorting 2000 nodes, 2000 times
	// which should be an upper bound on our worst case and it took 0.1 seconds.
	sort.Slice(s.nodes, func(a, b int) bool {
		return len(s.nodes[a].Pods) < len(s.nodes[b].Pods)
	})
	var scheduledNode *Node
	for _, node := range s.nodes {
		if err := node.Add(s.topology, p); err == nil {
			scheduledNode = node
			break
		}
	}

	if scheduledNode == nil {
		node := NewNode(s.constraints, s.daemonResources, s.instanceTypes)
		if err := node.Add(s.topology, p); err != nil {
			return err
		}
		s.Add(node)
		scheduledNode = node
	}

	if scheduledNode != nil {
		if err := s.topology.Record(p, scheduledNode.Constraints.Requirements); err != nil {
			logging.FromContext(ctx).Errorf("Recording topology decision, %s", err)
			return err
		}
	}
	return nil
}

type matchingTopology struct {
	topology              *TopologyGroup
	controlsPodScheduling bool
	selectsPod            bool
}
