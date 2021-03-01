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

package reallocation

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	utilsnode "github.com/awslabs/karpenter/pkg/utils/node"
	"github.com/awslabs/karpenter/pkg/utils/ptr"
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
		if utilsnode.IsUnderutilized(pods) {
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

func (f *Filter) GetTTLableNodes(nodes []*v1.Node) []*v1.Node {
	ttlable := []*v1.Node{}
	for _, node := range nodes {
		if _, ok := node.Annotations[v1alpha1.ProvisionerTTLKey]; !ok {
			ttlable = append(ttlable, node)
		}
	}
	return ttlable
}

func (f *Filter) GetCordonableNodes(nodes []*v1.Node) []*v1.Node {
	nonCordonedNodes := []*v1.Node{}
	for _, node := range nodes {
		if !node.Spec.Unschedulable {
			nonCordonedNodes = append(nonCordonedNodes, node)
		}
	}
	return nonCordonedNodes
}

// GetLabeledUnderutilizedNodes gets the nodes that have been labled underutilized for resource usage reevaluation
func (f *Filter) GetLabeledUnderutilizedNodes(ctx context.Context, provisioner *v1alpha1.Provisioner) ([]*v1.Node, error) {
	nodes := &v1.NodeList{}
	labelMap := map[string]string{
		v1alpha1.ProvisionerUnderutilizedKey:  "true",
		v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
		v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
	}
	for k, v := range provisioner.Spec.Labels {
		labelMap[k] = v
	}
	if err := f.kubeClient.List(ctx, nodes, client.MatchingLabels(labelMap)); err != nil {
		return nil, fmt.Errorf("listing labeled underutilized nodes, %w", err)
	}
	return ptr.NodeListToSlice(nodes), nil
}

func (f *Filter) getNodes(ctx context.Context, provisioner *v1alpha1.Provisioner) ([]*v1.Node, error) {
	nodes := &v1.NodeList{}
	labelMap := map[string]string{
		v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
		v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
	}
	for k, v := range provisioner.Spec.Labels {
		labelMap[k] = v
	}
	if err := f.kubeClient.List(ctx, nodes, client.MatchingLabels(labelMap)); err != nil {
		return nil, fmt.Errorf("listing nodes, %w", err)
	}
	return ptr.NodeListToSlice(nodes), nil
}

// Get Pods scheduled to a node
func (f *Filter) getPodsOnNode(ctx context.Context, nodeName string) ([]*v1.Pod, error) {
	pods := &v1.PodList{}
	if err := f.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
		return nil, fmt.Errorf("listing pods on node %s, %w", nodeName, err)
	}
	return ptr.PodListToSlice(pods), nil
}
