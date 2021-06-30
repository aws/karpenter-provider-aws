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

package termination

import (
	"context"
	"fmt"

	provisioning "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
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

// cordon cordons a node
func (t *Terminator) cordon(ctx context.Context, node *v1.Node) error {
	if node.Spec.Unschedulable {
		return nil
	}
	persisted := node.DeepCopy()
	node.Spec.Unschedulable = true
	if err := t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
		return fmt.Errorf("patching node %s, %w", node.Name, err)
	}
	zap.S().Debugf("Cordoned node %s", node.Name)
	return nil
}

// drain evicts pods from the node and returns true when fully drained
func (t *Terminator) drain(ctx context.Context, node *v1.Node) (bool, error) {
	// 1. Get pods on node
	pods, err := t.getPods(ctx, node)
	if err != nil {
		return false, fmt.Errorf("listing pods for node %s, %w", node.Name, err)
	}
	// 2. Evict pods on node
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
	return empty, nil
}

// terminate terminates the node then removes the finalizer to delete the node
func (t *Terminator) terminate(ctx context.Context, node *v1.Node) error {
	// 1. Terminate instance associated with node
	if err := t.cloudProvider.Terminate(ctx, node); err != nil {
		return fmt.Errorf("terminating cloudprovider instance, %w", err)
	}
	zap.S().Infof("Terminated instance %s", node.Name)
	// 2. Remove finalizer from node in APIServer
	persisted := node.DeepCopy()
	node.Finalizers = functional.StringSliceWithout(node.Finalizers, provisioning.KarpenterFinalizer)
	if err := t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
		return fmt.Errorf("removing finalizer from node %s, %w", node.Name, err)
	}
	return nil
}

// getPods returns a list of pods scheduled to a node
func (t *Terminator) getPods(ctx context.Context, node *v1.Node) ([]*v1.Pod, error) {
	pods := &v1.PodList{}
	if err := t.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return nil, fmt.Errorf("listing pods on node %s, %w", node.Name, err)
	}
	return ptr.PodListToSlice(pods), nil
}
