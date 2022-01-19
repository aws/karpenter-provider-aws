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
	"fmt"
	"time"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/injectabletime"
	"github.com/aws/karpenter/pkg/utils/node"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const InitializationTimeout = 15 * time.Minute

// Initialization is a subreconciler that
// 1. Removes the NotReady taint when the node is ready. This taint is originally applied on node creation.
// 2. Terminates nodes that don't transition to ready within InitializationTimeout
type Initialization struct {
	kubeClient client.Client
}

// Reconcile reconciles the node
func (r *Initialization) Reconcile(ctx context.Context, _ *v1alpha5.Provisioner, n *v1.Node) (reconcile.Result, error) {
	if !v1alpha5.Taints(n.Spec.Taints).HasKey(v1alpha5.NotReadyTaintKey) {
		// At this point, the startup of the node is complete and no more evaluation is necessary.
		return reconcile.Result{}, nil
	}

	if !node.IsReady(n) {
		if age := injectabletime.Now().Sub(n.GetCreationTimestamp().Time); age < InitializationTimeout {
			return reconcile.Result{RequeueAfter: InitializationTimeout - age}, nil
		}
		logging.FromContext(ctx).Infof("Triggering termination for node that failed to become ready")
		if err := r.kubeClient.Delete(ctx, n); err != nil {
			return reconcile.Result{}, fmt.Errorf("deleting node, %w", err)
		}
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
