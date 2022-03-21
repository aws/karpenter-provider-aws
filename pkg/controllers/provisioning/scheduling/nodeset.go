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

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

type NodeSet struct {
	client  client.Client
	daemons []*v1.Pod
	nodes   []*Node
}

func NewNodeSet(ctx context.Context, constraints *v1alpha5.Constraints, client client.Client) (*NodeSet, error) {
	ns := &NodeSet{
		client: client,
	}

	daemons, err := ns.getDaemons(ctx, constraints)
	if err != nil {
		return nil, err
	}
	ns.daemons = daemons
	return ns, nil
}

func (s *NodeSet) getDaemons(ctx context.Context, constraints *v1alpha5.Constraints) ([]*v1.Pod, error) {
	daemonSetList := &appsv1.DaemonSetList{}
	if err := s.client.List(ctx, daemonSetList); err != nil {
		return nil, fmt.Errorf("listing daemonsets, %w", err)
	}
	// Include DaemonSets that will schedule on this node
	var pods []*v1.Pod
	for _, daemonSet := range daemonSetList.Items {
		pod := &v1.Pod{Spec: daemonSet.Spec.Template.Spec}
		if err := constraints.ValidatePod(pod); err == nil {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (s *NodeSet) Add(node *Node) {
	s.nodes = append(s.nodes, node)
}
