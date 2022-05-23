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
)

type ProvisionerVersion struct {
	TerminationQueue *TerminationQueue
}

func (r *ProvisionerVersion) Reconcile(ctx context.Context, provisioner *v1alpha5.Provisioner, node *v1.Node) (reconcile.Result, error) {
	nodeProvisionerHash := node.Labels[v1alpha5.ProvisionerVersionKey]
	currentProvisionerHash := v1alpha5.GetProvisionerHash(provisioner)

	if /*nodeProvisionerHash != ""&&*/ currentProvisionerHash != nodeProvisionerHash {
		//Use the number of nodes Provisioned uptil now from Status of provisioner
		r.TerminationQueue.Add(node, provisioner.Status.TotalNodesProvisioned)
	}
	return reconcile.Result{}, nil
}
