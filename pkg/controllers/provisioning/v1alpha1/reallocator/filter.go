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
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1"
	podUtil "github.com/awslabs/karpenter/pkg/utils/pod"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Filter struct {
	kubeClient client.Client
}

// Gets Nodes fitting some underutilization predicate
func (f *Filter) GetUnderutilizedNodes(ctx context.Context, provisionerName string, provisionerNamespace string) ([]*v1.Node, error) {
	underutilized := []*v1.Node{}
	nodeList := &v1.NodeList{}

	// 1. Get all provisioner labeled nodes
	if err := f.kubeClient.List(ctx, nodeList, client.MatchingLabels(map[string]string{
		v1alpha1.ProvisionerNameLabelKey:      provisionerName,
		v1alpha1.ProvisionerNamespaceLabelKey: provisionerNamespace,
	})); err != nil {
		return nil, fmt.Errorf("filtering nodes on provisioner labels, %w", err)
	}

	// 2. Get nodes and the pods on each node
	for _, node := range nodeList.Items {
		pods, err := f.getPodsOnNode(ctx, node.Name)
		if err != nil {
			return []*v1.Node{}, fmt.Errorf("filtering pods on nodes, %w", err)
		}
		// Only checks if it has 0 non-daemon pods right now
		if f.isUnderutilized(pods) {
			underutilized = append(underutilized, &node)
		}
	}
	return underutilized, nil
}

// Get Pods scheduled to a node
func (f *Filter) getPodsOnNode(ctx context.Context, nodeName string) (*v1.PodList, error) {
	pods := &v1.PodList{}
	if err := f.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
		return nil, fmt.Errorf("listing pods on nodes, %w", err)
	}
	return pods, nil
}

// TODO: implement underutilized function (some generalized predicate)
func (f *Filter) isUnderutilized(pods *v1.PodList) bool {
	counter := 0
	for _, pod := range pods.Items {
		if podUtil.IsNotIgnored(&pod) {
			counter += 1
		}
	}
	return counter == 0
}

func (f *Filter) getNodesWithLabels(ctx context.Context, labelKeys []string) (*v1.NodeList, error) {
	nodeList := &v1.NodeList{}
	if err := f.kubeClient.List(ctx, nodeList, client.HasLabels(labelKeys)); err != nil {
		return nil, fmt.Errorf("listing provisioner labeled nodes, %w", err)
	}
	return nodeList, nil
}
