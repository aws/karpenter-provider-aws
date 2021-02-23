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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	utilsnode "github.com/awslabs/karpenter/pkg/utils/node"
	"github.com/awslabs/karpenter/pkg/utils/ptr"
	"github.com/awslabs/karpenter/pkg/utils/scheduling"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Filter struct {
	kubeClient client.Client
}

// Gets Nodes fitting some underutilization predicate
func (f *Filter) GetUnderutilizedNodes(ctx context.Context, provisioner *v1alpha1.Provisioner) ([]*v1.Node, error) {
	underutilized := []*v1.Node{}

	// 1. Get all provisioner labeled nodes
	nodes, err := f.getNodes(ctx, provisioner)
	if err != nil {
		return nil, err
	}

	// 2. Get nodes and the pods on each node
	for _, node := range nodes {
		pods, err := f.getPodsOnNode(ctx, node.Name)
		if err != nil {
			zap.S().Debugf("Unable to get pods for node %s, %s", node.Name, err.Error())
			continue
		}

		// Only checks if it has 0 non-daemon pods right now
		if f.isUnderutilized(pods) {
			underutilized = append(underutilized, node)
		}
	}
	return underutilized, nil
}

func (f *Filter) GetExpiredNodes(ctx context.Context, provisioner *v1alpha1.Provisioner) ([]*v1.Node, error) {
	nodeList, err := f.getNodes(ctx, provisioner)
	if err != nil {
		return nil, err
	}

	nodes := []*v1.Node{}
	for _, node := range nodeList {
		if utilsnode.IsPastTTL(node) {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (f *Filter) getNodes(ctx context.Context, provisioner *v1alpha1.Provisioner) ([]*v1.Node, error) {
	nodes := &v1.NodeList{}
	if err := f.kubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{
		v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
		v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
	})); err != nil {
		return nil, fmt.Errorf("listing nodes, %w", err)
	}

	// Convert each node to a pointer
	nodePointers := []*v1.Node{}
	for _, node := range nodes.Items {
		nodePointers = append(nodePointers, ptr.Node(node))
	}
	return nodePointers, nil
}

// Get Pods scheduled to a node
func (f *Filter) getPodsOnNode(ctx context.Context, nodeName string) (*v1.PodList, error) {
	pods := &v1.PodList{}
	if err := f.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
		return nil, fmt.Errorf("listing pods on node %s, %w", nodeName, err)
	}
	return pods, nil
}

// TODO: implement underutilized function (some generalized predicate)
func (f *Filter) isUnderutilized(pods *v1.PodList) bool {
	counter := 0
	for _, pod := range pods.Items {
		if scheduling.IsNotIgnored(&pod) {
			counter += 1
		}
	}
	return counter == 0
}
