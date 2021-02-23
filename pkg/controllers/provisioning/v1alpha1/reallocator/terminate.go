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
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Terminator struct {
	kubeClient    client.Client
	cloudprovider cloudprovider.Factory
}

func (t *Terminator) CordonNodes(ctx context.Context, nodes []*v1.Node) error {
	for _, node := range nodes {
		if node.Spec.Unschedulable {
			continue
		}
		persisted := node.DeepCopyObject()
		node.Spec.Unschedulable = true
		if err := t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
			return fmt.Errorf("patching node %s, %w", node.Name, err)
		}
	}
	return nil
}

func (t *Terminator) AddTTLs(ctx context.Context, nodes []*v1.Node) error {
	for _, node := range nodes {
		if _, ok := node.Annotations[v1alpha1.ProvisionerTTLKey]; !ok {
			persisted := node.DeepCopy()
			node.Annotations[v1alpha1.ProvisionerTTLKey] = time.Now().Add(300 * time.Second).Format(time.RFC3339)
			if err := t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
				return fmt.Errorf("patching node, %w", err)
			}
		}
	}
	return nil
}

func (t *Terminator) DeleteNodes(ctx context.Context, nodes []*v1.Node, spec *v1alpha1.ProvisionerSpec) error {
	err := t.cloudprovider.CapacityFor(spec).Delete(ctx, nodes)
	if err != nil {
		return fmt.Errorf("terminating %d cloudprovider instances, %w", len(nodes), err)
	}
	zap.S().Infof("Succesfully terminated %d instances", len(nodes))

	return nil
}
