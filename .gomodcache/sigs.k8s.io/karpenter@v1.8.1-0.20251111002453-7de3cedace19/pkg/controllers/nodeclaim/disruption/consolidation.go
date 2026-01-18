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

package disruption

import (
	"context"

	"github.com/samber/lo"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// Consolidation is a nodeclaim sub-controller that adds or removes status conditions on empty nodeclaims based on consolidateAfter
type Consolidation struct {
	kubeClient client.Client
	clock      clock.Clock
}

//nolint:gocyclo
func (c *Consolidation) Reconcile(ctx context.Context, nodePool *v1.NodePool, nodeClaim *v1.NodeClaim) (reconcile.Result, error) {
	hasConsolidatableCondition := nodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable) != nil

	// 1. If Consolidation isn't enabled, remove the consolidatable status condition
	if nodePool.Spec.Disruption.ConsolidateAfter.Duration == nil {
		if hasConsolidatableCondition {
			_ = nodeClaim.StatusConditions().Clear(v1.ConditionTypeConsolidatable)
			log.FromContext(ctx).V(1).Info("removing consolidatable status condition, consolidation is disabled")
		}
		return reconcile.Result{}, nil
	}
	initialized := nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized)
	// 2. If NodeClaim is not initialized, remove the consolidatable status condition
	if !initialized.IsTrue() {
		if hasConsolidatableCondition {
			_ = nodeClaim.StatusConditions().Clear(v1.ConditionTypeConsolidatable)
			log.FromContext(ctx).V(1).Info("removing consolidatable status condition, isn't initialized")
		}
		return reconcile.Result{}, nil
	}

	// If the lastPodEvent is zero, use the time that the nodeclaim was initialized, as that's when Karpenter recognizes that pods could have started scheduling
	timeToCheck := lo.Ternary(!nodeClaim.Status.LastPodEventTime.IsZero(), nodeClaim.Status.LastPodEventTime.Time, initialized.LastTransitionTime.Time)

	// Consider a node consolidatable by looking at the lastPodEvent status field on the nodeclaim.
	if c.clock.Since(timeToCheck) < lo.FromPtr(nodePool.Spec.Disruption.ConsolidateAfter.Duration) {
		if hasConsolidatableCondition {
			_ = nodeClaim.StatusConditions().Clear(v1.ConditionTypeConsolidatable)
			log.FromContext(ctx).V(1).Info("removing consolidatable status condition")
		}
		consolidatableTime := timeToCheck.Add(lo.FromPtr(nodePool.Spec.Disruption.ConsolidateAfter.Duration))
		return reconcile.Result{RequeueAfter: consolidatableTime.Sub(c.clock.Now())}, nil
	}

	// 6. Otherwise, add the consolidatable status condition
	nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
	if !hasConsolidatableCondition {
		log.FromContext(ctx).V(1).Info("marking consolidatable")
	}
	return reconcile.Result{}, nil
}
