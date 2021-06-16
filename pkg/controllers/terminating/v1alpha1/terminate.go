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

package v1alpha1

import (
	"context"
	"fmt"

	provisioning "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/pod"
	"github.com/awslabs/karpenter/pkg/utils/ptr"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Terminator struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	coreV1Client  corev1.CoreV1Interface
}

// cordonNodes takes in a list of expired nodes as input and cordons them
func (t *Terminator) cordonNodes(ctx context.Context) error {
	// 1. Get terminable nodes
	nodeList, err := t.getLabeledNodes(ctx, provisioning.ProvisionerTerminablePhase)
	if err != nil {
		return err
	}
	// 2. Cordon nodes
	for _, node := range nodeList {
		persisted := node.DeepCopy()
		node.Spec.Unschedulable = true
		node.Labels = functional.UnionStringMaps(
			node.Labels,
			map[string]string{provisioning.ProvisionerPhaseLabel: provisioning.ProvisionerDrainingPhase},
		)
		if err := t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
			return fmt.Errorf("patching node %s, %w", node.Name, err)
		}
		zap.S().Debugf("Cordoned node %s", node.Name)
	}
	return nil
}

// terminateNodes takes in a list of expired non-drained nodes and calls drain on them
func (t *Terminator) terminateNodes(ctx context.Context) error {
	// 1. Get draining nodes
	draining, err := t.getLabeledNodes(ctx, provisioning.ProvisionerDrainingPhase)
	if err != nil {
		return fmt.Errorf("listing draining nodes, %w", err)
	}
	// 2. Drain nodes
	drained := []*v1.Node{}
	for _, node := range draining {
		// TODO: Check if Node should be drained
		// - Disrupts PDB
		// - Pods owned by controller object
		// - Pod on Node can't be rescheduled elsewhere

		// 2a. Get pods on node
		pods, err := t.getPods(ctx, node)
		if err != nil {
			return fmt.Errorf("listing pods for node %s, %w", node.Name, err)
		}
		// 2b. Evict pods on node
		empty := true
		for _, p := range pods {
			if !pod.IsOwnedByDaemonSet(p) {
				empty = false
				if err := t.coreV1Client.Pods(p.Namespace).Evict(ctx, &v1beta1.Eviction{
					ObjectMeta: metav1.ObjectMeta{
						Name: p.Name,
					},
				}); err != nil {
					zap.S().Debugf("Continuing after failing to evict pods from node %s, %s", node.Name, err.Error())
				}
			}
		}
		// 2c. If node is empty, add to list of nodes to delete
		if empty {
			drained = append(drained, node)
		}
	}
	// 3. Delete empty nodes
	if err := t.deleteNodes(ctx, drained); err != nil {
		return fmt.Errorf("deleting %d nodes, %w", len(drained), err)
	}
	return nil
}

// deleteNode uses a cloudprovider-specific delete to delete a set of nodes
func (t *Terminator) deleteNodes(ctx context.Context, nodes []*v1.Node) error {
	// 1. Delete node in cloudprovider's instanceprovider
	if err := t.cloudProvider.Terminate(ctx, nodes); err != nil {
		return fmt.Errorf("terminating cloudprovider instance, %w", err)
	}
	// 2. Delete node in APIServer
	for _, node := range nodes {
		if err := t.kubeClient.Delete(ctx, node); err != nil {
			zap.S().Debugf("Continuing after failing to delete node %s, %s", node.Name, err.Error())
		}
		zap.S().Infof("Terminated node %s", node.Name)
	}
	return nil
}

// getLabeledNodes returns a list of nodes with the provisioner's labels and given labels
func (t *Terminator) getLabeledNodes(ctx context.Context, phaseLabel string) ([]*v1.Node, error) {
	nodes := &v1.NodeList{}
	if err := t.kubeClient.List(ctx, nodes, client.HasLabels([]string{
		provisioning.ProvisionerNameLabelKey,
		provisioning.ProvisionerNamespaceLabelKey,
	}), client.MatchingLabels(map[string]string{
		provisioning.ProvisionerPhaseLabel: phaseLabel,
	})); err != nil {
		return nil, fmt.Errorf("listing nodes, %w", err)
	}
	return ptr.NodeListToSlice(nodes), nil
}

// getPods returns a list of pods scheduled to a node
func (t *Terminator) getPods(ctx context.Context, node *v1.Node) ([]*v1.Pod, error) {
	pods := &v1.PodList{}
	if err := t.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return nil, fmt.Errorf("listing pods on node %s, %w", node.Name, err)
	}
	return ptr.PodListToSlice(pods), nil
}
