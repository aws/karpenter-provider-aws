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

	"k8s.io/apimachinery/pkg/util/clock"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

// Expiration is a subreconciler that terminates nodes after a period of time.
type Expiration struct {
	clock      clock.Clock
	kubeClient client.Client
}

func NewExpiration(clk clock.Clock, kubeClient client.Client) *Expiration {
	return &Expiration{
		clock:      clk,
		kubeClient: kubeClient,
	}
}

// Reconcile reconciles the node
func (r *Expiration) Reconcile(ctx context.Context, provisioner *v1alpha5.Provisioner, node *v1.Node) (reconcile.Result, error) {
	// 1. Ignore node if not applicable
	if provisioner.Spec.TTLSecondsUntilExpired == nil {
		return reconcile.Result{}, nil
	}
	// 2. Trigger termination workflow if expired
	expirationTTL := time.Duration(ptr.Int64Value(provisioner.Spec.TTLSecondsUntilExpired)) * time.Second
	expirationTime := node.CreationTimestamp.Add(expirationTTL)
	if r.clock.Now().After(expirationTime) {
		logging.FromContext(ctx).Infof("Triggering termination for expired node after %s (+%s)", expirationTTL, time.Since(expirationTime))
		if err := r.kubeClient.Delete(ctx, node); err != nil {
			return reconcile.Result{}, fmt.Errorf("deleting node, %w", err)
		}
	}
	// 3. Backoff until expired
	return reconcile.Result{RequeueAfter: time.Until(expirationTime)}, nil
}
