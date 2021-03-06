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
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/ptr"
	"github.com/awslabs/karpenter/pkg/utils/scheduling"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Terminator struct {
	kubeClient    client.Client
	cloudprovider cloudprovider.Factory
	coreV1Client  corev1.CoreV1Interface
}

func (t *Terminator) Reconcile(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	// 1. Cordon terminable nodes
	if err := t.cordonNodes(ctx, provisioner); err != nil {
		return fmt.Errorf("cordoning terminable nodes, %w", err)
	}

	// 2. Drain and delete nodes
	if err := t.terminateNodes(ctx, provisioner); err != nil {
		return fmt.Errorf("terminating nodes, %w", err)
	}
	return nil
}

// cordonNodes takes in a list of expired nodes as input and cordons them
func (t *Terminator) cordonNodes(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	// 1. Get terminable nodes
	nodeList, err := t.getNodes(ctx, provisioner, map[string]string{
		v1alpha1.ProvisionerPhaseLabel: v1alpha1.ProvisionerTerminablePhase,
	})
	if err != nil {
		return err
	}
	// 2. Cordon nodes
	for _, node := range nodeList {
		if !node.Spec.Unschedulable {
			persisted := node.DeepCopy()
			node.Spec.Unschedulable = true
			node.Labels[v1alpha1.ProvisionerPhaseLabel] = v1alpha1.ProvisionerDrainingPhase
			if err := t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
				return fmt.Errorf("patching node %s, %w", node.Name, err)
			}
			zap.S().Debugf("Cordoned node %s", node.Name)
		}
	}
	return nil
}

// terminateNodes takes in a list of expired non-drained nodes and calls drain on them
func (t *Terminator) terminateNodes(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	// 1. Get draining nodes
	draining, err := t.getNodes(ctx, provisioner, map[string]string{
		v1alpha1.ProvisionerPhaseLabel: v1alpha1.ProvisionerDrainingPhase,
	})
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
		for _, pod := range pods {
			if !scheduling.IsOwnedByDaemonSet(pod) {
				empty = false
				if err := t.coreV1Client.Pods(pod.Namespace).Evict(ctx, &v1beta1.Eviction{
					ObjectMeta: metav1.ObjectMeta{
						Name: pod.Name,
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
	if err := t.deleteNodes(ctx, drained, provisioner); err != nil {
		return fmt.Errorf("deleting %d nodes, %w", len(drained), err)
	}
	return nil
}

// deleteNode uses a cloudprovider-specific delete to delete a set of nodes
func (t *Terminator) deleteNodes(ctx context.Context, nodes []*v1.Node, provisioner *v1alpha1.Provisioner) error {
	// 1. Delete node in cloudprovider's instanceprovider
	if err := t.cloudprovider.CapacityFor(&provisioner.Spec).Delete(ctx, nodes); err != nil {
		return fmt.Errorf("terminating cloudprovider instance, %w", err)
	}
	// 2. Delete node in APIServer
	// TODO: Prevent leaked nodes: ensure a node is not deleted in apiserver if not deleted in cloudprovider
	// Use the returned ids from the cloudprovider's Delete() function, and then only delete those ids in the apiserver
	for _, node := range nodes {
		if err := t.kubeClient.Delete(ctx, node); err != nil {
			zap.S().Debugf("Continuing after failing to delete node %s, %s", node.Name, err.Error())
		}
		zap.S().Infof("Terminated node %s", node.Name)
	}
	return nil
}

// getNodes returns a list of nodes with the provisioner's labels and given labels
func (t *Terminator) getNodes(ctx context.Context, provisioner *v1alpha1.Provisioner, additionalLabels map[string]string) ([]*v1.Node, error) {
	nodes := &v1.NodeList{}
	if err := t.kubeClient.List(ctx, nodes, client.MatchingLabels(functional.UnionStringMaps(map[string]string{
		v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
		v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
	}, provisioner.Spec.Labels, additionalLabels))); err != nil {
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
