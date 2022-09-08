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

package node

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
)

type Tags struct {
	cloudProvider cloudprovider.CloudProvider
}

// Reconcile reconciles the tags on the node
func (r *Tags) Reconcile(ctx context.Context, provisioner *v1alpha5.Provisioner, n *v1.Node) (reconcile.Result, error) {
	hash, err := r.cloudProvider.ReconcileTags(ctx, provisioner, n)
	if err != nil {
		// TODO: Wrap error
		return reconcile.Result{}, err
	}

	if hash != n.Labels[v1alpha5.TagsVersionKey] {
		n.Labels[v1alpha5.TagsVersionKey] = hash
	}

	return reconcile.Result{}, nil
}
