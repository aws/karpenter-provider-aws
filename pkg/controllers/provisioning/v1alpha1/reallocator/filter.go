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

package reallocator

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Filter struct {
	kubeClient client.Client
}

// Gets Nodes fitting some underutilization
// TODO: add predicate
func (f *Filter) GetUnderutilizedNodes(ctx context.Context) ([]*v1.Node, error) {
	nodeList := &v1.NodeList{}
	underutilized := []*v1.Node{}

	// 1. Get all nodes
	if err := f.kubeClient.List(ctx, nodeList); err != nil {
		return nil, fmt.Errorf("listing nodes, %w", err)
	}

	// 2. Get nodes and the pods on each node
	for _, node := range nodeList.Items {
		pods, err := f.getPodsOnNode(ctx, node.Name)
		if err != nil {
			return []*v1.Node{}, fmt.Errorf("filtering pods on nodes, %w", err)
		}
		if f.isUnderutilized(&node, pods) {
			underutilized = append(underutilized, &node)
		}
	}
	return underutilized, nil
}

// Get Pods scheduled to a node
func (f *Filter) getPodsOnNode(ctx context.Context, nodeName string) (*v1.PodList, error) {
	pods := &v1.PodList{}
	if err := f.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
		return nil, fmt.Errorf("listing unscheduled pods, %w", err)
	}
	return pods, nil
}

// TODO: implement underutilized function
func (f *Filter) isUnderutilized(*v1.Node, *v1.PodList) bool {
	return false
}
