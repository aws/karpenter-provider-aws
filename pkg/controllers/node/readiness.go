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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/awslabs/karpenter/pkg/utils/node"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Readiness is a subreconciler that removes the NotReady taint when the node is ready
type Readiness struct{}

// Reconcile reconciles the node
func (r *Readiness) Reconcile(_ context.Context, _ *v1alpha5.Provisioner, n *v1.Node) (reconcile.Result, error) {
	if !node.IsReady(n) {
		return reconcile.Result{}, nil
	}
	taints := []v1.Taint{}
	for _, taint := range n.Spec.Taints {
		if taint.Key != v1alpha5.NotReadyTaintKey {
			taints = append(taints, taint)
		}
	}
	n.Spec.Taints = taints
	return reconcile.Result{}, nil
}
