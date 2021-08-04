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
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/node"
	"github.com/awslabs/karpenter/pkg/utils/pod"
	"github.com/awslabs/karpenter/pkg/utils/ptr"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Emptiness is a subreconciler that terminates nodes that are empty after a ttl
type Emptiness struct {
	kubeClient client.Client
}

// Reconcile reconciles the node
func (r *Emptiness) Reconcile(ctx context.Context, provisioner *v1alpha3.Provisioner, n *v1.Node) (reconcile.Result, error) {
	// 1. Ignore node if not applicable
	if provisioner.Spec.TTLSecondsAfterEmpty == nil {
		return reconcile.Result{}, nil
	}
	if !node.IsReady(n) {
		return reconcile.Result{}, nil
	}
	// 2. Remove ttl if not empty
	empty, err := r.isEmpty(ctx, n)
	if err != nil {
		return reconcile.Result{}, err
	}

	emptinessTimestamp, hasEmptinessTimestamp := n.Annotations[v1alpha3.EmptinessTimestampAnnotationKey]
	if !empty {
		if hasEmptinessTimestamp {
			delete(n.Annotations, v1alpha3.EmptinessTimestampAnnotationKey)
			logging.FromContext(ctx).Infof("Removed emptiness TTL from node %s", n.Name)
		}
		return reconcile.Result{}, nil
	}
	// 3. Set TTL if not set
	n.Annotations = functional.UnionStringMaps(n.Annotations)
	ttl := time.Duration(ptr.Int64Value(provisioner.Spec.TTLSecondsAfterEmpty)) * time.Second
	if !hasEmptinessTimestamp {
		n.Annotations[v1alpha3.EmptinessTimestampAnnotationKey] = time.Now().Format(time.RFC3339)
		logging.FromContext(ctx).Infof("Added TTL to empty node %s", n.Name)
		return reconcile.Result{RequeueAfter: ttl}, nil
	}
	// 4. Delete node if beyond TTL
	emptinessTime, err := time.Parse(time.RFC3339, emptinessTimestamp)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("parsing emptiness timestamp, %s", emptinessTimestamp)
	}
	if time.Now().After(emptinessTime.Add(ttl)) {
		logging.FromContext(ctx).Infof("Triggering termination after %s for empty node %s", ttl, n.Name)
		if err := r.kubeClient.Delete(ctx, n); err != nil {
			return reconcile.Result{}, fmt.Errorf("deleting node %s, %w", n.Name, err)
		}
	}
	return reconcile.Result{}, nil
}

func (r *Emptiness) isEmpty(ctx context.Context, n *v1.Node) (bool, error) {
	pods := &v1.PodList{}
	if err := r.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": n.Name}); err != nil {
		return false, fmt.Errorf("listing pods for node %s, %w", n.Name, err)
	}
	for _, p := range pods.Items {
		if pod.HasFailed(&p) {
			continue
		}
		if !pod.IsOwnedByDaemonSet(&p) {
			return false, nil
		}
	}
	return true, nil
}
