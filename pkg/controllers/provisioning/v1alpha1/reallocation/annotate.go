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
	"time"
)

type Annotator struct {
	kubeClient client.Client
}

// MarkUnderutilized takes in a list of underutilized nodes and adds TTL to them
func (a *Annotator) MarkUnderutilized(ctx context.Context, nodes []*v1.Node) error {
	for _, node := range nodes {
		persisted := node.DeepCopy()
		node.Labels[v1alpha1.ProvisionerUnderutilizedKey] = "true"
		node.Annotations[v1alpha1.ProvisionerTTLKey] = time.Now().Add(300 * time.Second).Format(time.RFC3339)
		if err := a.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
			return fmt.Errorf("patching node %s, %w", node.Name, err)
		}
		zap.S().Debugf("Added TTL and label to underutilized node %s", node.Name)
	}
	return nil
}

// ClearUnderutilized takes in a list of nodes labeled underutilized and removes TTL if there is sufficient resource usage
func (a *Annotator) ClearUnderutilized(ctx context.Context, nodes []*v1.Node) error {
	for _, node := range nodes {
		pods := &v1.PodList{}
		if err := a.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
			return fmt.Errorf("listing pods on node %s, %w", node.Name, err)
		}

		if !utilsnode.IsUnderutilized(ptr.PodListToSlice(pods)) {
			persisted := node.DeepCopy()
			delete(node.Labels, v1alpha1.ProvisionerUnderutilizedKey)
			delete(node.Annotations, v1alpha1.ProvisionerTTLKey)
			if err := a.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
				return fmt.Errorf("patching node %s, %w", node.Name, err)
			} else {
				zap.S().Debugf("Removed TTL from node %s", node.Name)
			}
		}
	}
	return nil
}

// CordonNodes takes in a list of expired nodes as input and cordons them
func (a *Annotator) CordonNodes(ctx context.Context, nodes []*v1.Node) error {
	for _, node := range nodes {
		persisted := node.DeepCopy()
		node.Spec.Unschedulable = true
		if err := a.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
			return fmt.Errorf("patching node %s, %w", node.Name, err)
		}
		zap.S().Debugf("Cordoned node %s", node.Name)
	}
	return nil
}
