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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/utils/node"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const LivenessTimeout = 5 * time.Minute

// Liveness is a subreconciler that deletes nodes if its determined to be unrecoverable
type Liveness struct {
	kubeClient client.Client
}

// Reconcile reconciles the node
func (r *Liveness) Reconcile(ctx context.Context, provisioner *v1alpha3.Provisioner, n *v1.Node) (reconcile.Result, error) {
	if Now().Sub(n.GetCreationTimestamp().Time) < LivenessTimeout {
		return reconcile.Result{}, nil
	}
	condition := node.GetCondition(n.Status.Conditions, v1.NodeReady);
	if condition.Reason != "" && condition.Reason != "NodeStatusNeverUpdated" {
		return reconcile.Result{}, nil
	}
	logging.FromContext(ctx).Infof("Triggering termination for node that failed to join %s", n.Name)
	if err := r.kubeClient.Delete(ctx, n); err != nil {
		return reconcile.Result{}, fmt.Errorf("deleting node %s, %w", n.Name, err)
	}
	return reconcile.Result{}, nil
}
