/*
Copyright The Kubernetes Authors.

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

package lifecycle

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/metrics"
)

type Liveness struct {
	clock      clock.Clock
	kubeClient client.Client
}

// registrationTTL is a heuristic time that we expect the node to register within
// If we don't see the node within this time, then we should delete the NodeClaim and try again
const registrationTTL = time.Minute * 15

func (l *Liveness) Reconcile(ctx context.Context, nodeClaim *v1.NodeClaim) (reconcile.Result, error) {
	registered := nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered)
	if registered.IsTrue() {
		return reconcile.Result{}, nil
	}
	if registered == nil {
		return reconcile.Result{Requeue: true}, nil
	}
	// If the Registered statusCondition hasn't gone True during the TTL since we first updated it, we should terminate the NodeClaim
	// NOTE: ttl has to be stored and checked in the same place since l.clock can advance after the check causing a race
	if ttl := registrationTTL - l.clock.Since(registered.LastTransitionTime.Time); ttl > 0 {
		return reconcile.Result{RequeueAfter: ttl}, nil
	}
	if err := l.updateNodePoolRegistrationHealth(ctx, nodeClaim); client.IgnoreNotFound(err) != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, err
	}
	// Delete the NodeClaim if we believe the NodeClaim won't register since we haven't seen the node
	if err := l.kubeClient.Delete(ctx, nodeClaim); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	log.FromContext(ctx).V(1).WithValues("ttl", registrationTTL).Info("terminating due to registration ttl")
	metrics.NodeClaimsDisruptedTotal.Inc(map[string]string{
		metrics.ReasonLabel:       "liveness",
		metrics.NodePoolLabel:     nodeClaim.Labels[v1.NodePoolLabelKey],
		metrics.CapacityTypeLabel: nodeClaim.Labels[v1.CapacityTypeLabelKey],
	})
	return reconcile.Result{}, nil
}

// updateNodePoolRegistrationHealth sets the NodeRegistrationHealthy=False
// on the NodePool if the nodeClaim fails to launch/register
func (l *Liveness) updateNodePoolRegistrationHealth(ctx context.Context, nodeClaim *v1.NodeClaim) error {
	nodePoolName := nodeClaim.Labels[v1.NodePoolLabelKey]
	if nodePoolName != "" {
		nodePool := &v1.NodePool{}
		if err := l.kubeClient.Get(ctx, types.NamespacedName{Name: nodePoolName}, nodePool); err != nil {
			return err
		}
		if nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy).IsUnknown() {
			stored := nodePool.DeepCopy()
			// If the nodeClaim failed to register during the TTL set NodeRegistrationHealthy status condition on
			// NodePool to False. If the launch failed get the launch failure reason and message from nodeClaim.
			if launchCondition := nodeClaim.StatusConditions().Get(v1.ConditionTypeLaunched); launchCondition.IsTrue() {
				nodePool.StatusConditions().SetFalse(v1.ConditionTypeNodeRegistrationHealthy, "RegistrationFailed", "Failed to register node")
			} else {
				nodePool.StatusConditions().SetFalse(v1.ConditionTypeNodeRegistrationHealthy, launchCondition.Reason, launchCondition.Message)
			}
			// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
			// can cause races due to the fact that it fully replaces the list on a change
			// Here, we are updating the status condition list
			if err := l.kubeClient.Status().Patch(ctx, nodePool, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}
	return nil
}
