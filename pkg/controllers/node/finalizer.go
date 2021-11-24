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

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/functional"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Finalizer is a subreconciler that ensures nodes have the termination
// finalizer. This protects against instances that launch when Karpenter fails
// to create the node object. In this case, the node will come online without
// the termination finalizer. This controller will update the node accordingly.
type Finalizer struct{}

// Reconcile reconciles the node
func (r *Finalizer) Reconcile(_ context.Context, _ *v1alpha5.Provisioner, n *v1.Node) (reconcile.Result, error) {
	if !n.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}
	if !functional.ContainsString(n.Finalizers, v1alpha5.TerminationFinalizer) {
		n.Finalizers = append(n.Finalizers, v1alpha5.TerminationFinalizer)
	}
	return reconcile.Result{}, nil
}
