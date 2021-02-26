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
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Terminator struct {
	kubeClient    client.Client
	cloudprovider cloudprovider.Factory
}

// DeleteNodes will use a cloudprovider implementation to delete a set of nodes
func (t *Terminator) DeleteNodes(ctx context.Context, nodes []*v1.Node, spec *v1alpha1.ProvisionerSpec) error {
	// 1. Delete node in cloudprovider's instanceprovider
	if err := t.cloudprovider.CapacityFor(spec).Delete(ctx, nodes); err != nil {
		return fmt.Errorf("terminating %d cloudprovider instances, %w", len(nodes), err)
	}

	// 2. Delete node in APIServer to ensure
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
