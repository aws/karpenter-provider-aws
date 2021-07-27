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

package allocation

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/utils/pod"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/packing"
	"github.com/mitchellh/hashstructure/v2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Constraints struct {
	KubeClient client.Client
}

// Group separates pods into a set of equivalent scheduling groups. All pods in
// each group can be deployed together on the same node, or separately on
// multiple nodes. These groups map to scheduling properties like taints/labels.
func (c *Constraints) Group(ctx context.Context, provisioner *v1alpha3.Provisioner, pods []*v1.Pod) ([]*packing.Constraints, error) {
	// Groups uniqueness is tracked by hash(Constraints)
	groups := map[uint64]*packing.Constraints{}
	for _, pod := range pods {
		constraints := provisioner.Spec.Constraints.
			WithLabel(v1alpha3.ProvisionerNameLabelKey, provisioner.GetName()).
			WithOverrides(pod)
		key, err := hashstructure.Hash(constraints, hashstructure.FormatV2, nil)
		if err != nil {
			return nil, fmt.Errorf("hashing constraints, %w", err)
		}
		// Create new group if one doesn't exist
		if _, ok := groups[key]; !ok {
			// Uses a theoretical node object to compute schedulablility of daemonset overhead.
			daemons, err := c.getDaemons(ctx, &v1.Node{
				ObjectMeta: metav1.ObjectMeta{Labels: constraints.Labels},
				Spec:       v1.NodeSpec{Taints: provisioner.Spec.Taints},
			})
			if err != nil {
				return nil, fmt.Errorf("computing node overhead, %w", err)
			}
			groups[key] = &packing.Constraints{
				Constraints: constraints,
				Pods:        []*v1.Pod{},
				Daemons:     daemons,
			}
		}
		// Append pod to group, guaranteed to exist
		groups[key].Pods = append(groups[key].Pods, pod)
	}

	result := []*packing.Constraints{}
	for _, group := range groups {
		result = append(result, group)
	}
	return result, nil
}

func (c *Constraints) getDaemons(ctx context.Context, node *v1.Node) ([]*v1.Pod, error) {
	// 1. Get DaemonSets
	daemonSetList := &appsv1.DaemonSetList{}
	if err := c.KubeClient.List(ctx, daemonSetList); err != nil {
		return nil, fmt.Errorf("listing daemonsets, %w", err)
	}

	// 2. filter DaemonSets to include those that will schedule on this node
	pods := []*v1.Pod{}
	for _, daemonSet := range daemonSetList.Items {
		if pod.IsSchedulable(&daemonSet.Spec.Template.Spec, node) {
			pods = append(pods, &v1.Pod{Spec: daemonSet.Spec.Template.Spec})
		}
	}
	return pods, nil
}
