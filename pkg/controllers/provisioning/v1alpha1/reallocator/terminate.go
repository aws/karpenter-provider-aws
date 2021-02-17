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
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Terminator struct {
	kubeClient client.Client
}

func (t *Terminator) CordonNode(ctx context.Context, node *v1.Node) error {
	if !node.Spec.Unschedulable {
		persisted := node.DeepCopyObject()
		node.Spec.Unschedulable = true
		return t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted))
	}
	return nil
}

func (t *Terminator) AddTTL(ctx context.Context, node *v1.Node) error {
	if _, ok := node.Annotations[v1alpha1.ProvisionerTTLKey]; !ok {
		persisted := node.DeepCopy()
		node.Annotations[v1alpha1.ProvisionerTTLKey] = time.Now().Add(300 * time.Second).Format(time.RFC3339)
		return t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted))
	}
	return nil
}

func (t *Terminator) DeleteNode(ctx context.Context, node *v1.Node) error {
	if err := t.kubeClient.Delete(ctx, node); err != nil {
		return fmt.Errorf("deleting node, %w", err)
	}
	zap.S().Infof("Successfully deleted a node %s", node.Name)
	return nil
}
