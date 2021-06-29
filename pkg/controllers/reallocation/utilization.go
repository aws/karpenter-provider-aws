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
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	utilsnode "github.com/awslabs/karpenter/pkg/utils/node"
	"github.com/awslabs/karpenter/pkg/utils/ptr"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Utilization struct {
	kubeClient client.Client
}

// markUnderutilized adds a TTL to underutilized nodes
func (u *Utilization) markUnderutilized(ctx context.Context, provisioner *v1alpha2.Provisioner) error {
	ttlable := []*v1.Node{}
	// 1. Get all provisioner nodes
	nodes, err := u.getNodes(ctx, provisioner, map[string]string{})
	if err != nil {
		return err
	}

	// 2. Get underutilized nodes
	for _, node := range nodes {
		pods, err := u.getPods(ctx, node)
		if err != nil {
			return fmt.Errorf("getting pods for node %s, %w", node.Name, err)
		}
		if utilsnode.IsUnderutilized(node, pods) {
			if _, ok := node.Annotations[v1alpha2.ProvisionerTTLKey]; !ok {
				ttlable = append(ttlable, node)
			}
		}
	}

	// 3. Set TTL for each underutilized node
	for _, node := range ttlable {
		persisted := node.DeepCopy()
		node.Labels = functional.UnionStringMaps(
			node.Labels,
			map[string]string{v1alpha2.ProvisionerUnderutilizedLabelKey: "true"},
		)
		node.Annotations = functional.UnionStringMaps(
			node.Annotations,
			map[string]string{v1alpha2.ProvisionerTTLKey: time.Now().Add(time.Duration(ptr.Int64Value(provisioner.Spec.TTLSecondsAfterEmpty)) * time.Second).Format(time.RFC3339)},
		)
		if err := u.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
			return fmt.Errorf("patching node %s, %w", node.Name, err)
		}
		zap.S().Debugf("Added TTL and label to underutilized node %s", node.Name)
	}
	return nil
}

// clearUnderutilized removes the TTL on underutilized nodes if there is sufficient resource usage
func (u *Utilization) clearUnderutilized(ctx context.Context, provisioner *v1alpha2.Provisioner) error {
	// 1. Get underutilized nodes
	nodes, err := u.getNodes(ctx, provisioner, map[string]string{v1alpha2.ProvisionerUnderutilizedLabelKey: "true"})
	if err != nil {
		return fmt.Errorf("listing labeled underutilized nodes, %w", err)
	}

	// 2. Clear underutilized label if node is utilized
	for _, node := range nodes {
		pods, err := u.getPods(ctx, node)
		if err != nil {
			return fmt.Errorf("listing pods on node %s, %w", node.Name, err)
		}

		if !utilsnode.IsUnderutilized(node, pods) {
			persisted := node.DeepCopy()
			delete(node.Labels, v1alpha2.ProvisionerUnderutilizedLabelKey)
			delete(node.Annotations, v1alpha2.ProvisionerTTLKey)
			if err := u.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
				zap.S().Debugf("Could not remove underutilized labels on node %s, %w", node.Name, err)
			} else {
				zap.S().Debugf("Removed TTL from node %s", node.Name)
			}
		}
	}
	return nil
}

// terminateExpired checks if a node is past its ttl and marks it
func (u *Utilization) terminateExpired(ctx context.Context, provisioner *v1alpha2.Provisioner) error {
	// 1. Get underutilized nodes
	nodes, err := u.getNodes(ctx, provisioner, map[string]string{v1alpha2.ProvisionerUnderutilizedLabelKey: "true"})
	if err != nil {
		return fmt.Errorf("listing underutilized nodes, %w", err)
	}

	// 2. Delete node if past TTL
	// This will kick off work for the termination controller to gracefully shut down the node.
	for _, node := range nodes {
		if utilsnode.IsPastTTL(node) {
			if err := u.kubeClient.Delete(ctx, node); err != nil {
				return fmt.Errorf("sending delete for node %s, %w", node.Name, err)
			}
		}
	}
	return nil
}

// getNodes returns a list of nodes with the provisioner's labels and given labels
func (u *Utilization) getNodes(ctx context.Context, provisioner *v1alpha2.Provisioner, additionalLabels map[string]string) ([]*v1.Node, error) {
	nodes := &v1.NodeList{}
	if err := u.kubeClient.List(ctx, nodes, client.MatchingLabels(functional.UnionStringMaps(map[string]string{
		v1alpha2.ProvisionerNameLabelKey:      provisioner.Name,
		v1alpha2.ProvisionerNamespaceLabelKey: provisioner.Namespace,
	}, additionalLabels))); err != nil {
		return nil, fmt.Errorf("listing nodes, %w", err)
	}
	return ptr.NodeListToSlice(nodes), nil
}

// getPods returns a list of pods scheduled to a node
func (u *Utilization) getPods(ctx context.Context, node *v1.Node) ([]*v1.Pod, error) {
	pods := &v1.PodList{}
	if err := u.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return nil, fmt.Errorf("listing pods on node %s, %w", node.Name, err)
	}
	return ptr.PodListToSlice(pods), nil
}
