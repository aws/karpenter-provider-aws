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

package nodeclass

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
)

type PlacementGroupReconciler struct {
	provider placementgroup.Provider
	cm       *pretty.ChangeMonitor
}

func NewPlacementGroupReconciler(provider placementgroup.Provider) *PlacementGroupReconciler {
	return &PlacementGroupReconciler{
		provider: provider,
		cm:       pretty.NewChangeMonitor(),
	}
}

func (p *PlacementGroupReconciler) Reconcile(ctx context.Context, nc *v1.EC2NodeClass) (reconcile.Result, error) {
	// If no placement group selector is specified, clear status and remove any stale
	// PlacementGroupReady condition from a previous reconciliation when the selector was set.
	// Since PlacementGroupReady is no longer in the StatusConditions set when the selector is nil,
	// we need to explicitly clear it.
	if nc.Spec.PlacementGroupSelector == nil {
		nc.Status.PlacementGroups = nil
		err := nc.StatusConditions().Clear(v1.ConditionTypePlacementGroupReady)
		return reconcile.Result{}, err
	}

	term := lo.FromPtr(nc.Spec.PlacementGroupSelector)
	selector := term.Name
	if selector == "" {
		selector = term.ID
	}
	pg, err := p.provider.Get(ctx, term)
	if err != nil {
		nc.Status.PlacementGroups = nil
		nc.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupResolutionFailed", fmt.Sprintf("Failed to resolve placement group %q: %s", selector, err))
		return reconcile.Result{}, fmt.Errorf("resolving placement group %q, %w", selector, err)
	}
	if pg == nil {
		nc.Status.PlacementGroups = nil
		nc.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupNotFound", fmt.Sprintf("Placement group %q not found", selector))
		return reconcile.Result{}, nil
	}

	resolved, err := v1.PlacementGroupFromEC2(pg)
	if err != nil {
		nc.Status.PlacementGroups = nil
		nc.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupResolutionFailed", fmt.Sprintf("Failed to parse placement group %q: %s", selector, err))
		return reconcile.Result{}, fmt.Errorf("parsing placement group %q, %w", selector, err)
	}

	if resolved.State != v1.PlacementGroupStateAvailable {
		nc.Status.PlacementGroups = nil
		nc.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupNotAvailable", fmt.Sprintf("Placement group %q is in state %q, expected %q", selector, resolved.State, v1.PlacementGroupStateAvailable))
		return reconcile.Result{}, nil
	}

	if p.cm.HasChanged(nc.Name, resolved.ID) {
		log.FromContext(ctx).V(1).WithValues("id", resolved.ID, "name", resolved.Name, "strategy", resolved.Strategy).Info("discovered placement group")
	}

	nc.Status.PlacementGroups = []v1.PlacementGroup{resolved}
	nc.StatusConditions().SetTrue(v1.ConditionTypePlacementGroupReady)
	return reconcile.Result{}, nil
}
