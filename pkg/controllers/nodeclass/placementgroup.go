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

	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

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
	// If no placement group selector is specified, clear in-memory state and remove any stale
	// PlacementGroupReady condition from a previous reconciliation when the selector was set.
	// Since PlacementGroupReady is no longer in the StatusConditions set when the selector is nil,
	// we need to explicitly clear it.
	if nc.Spec.PlacementGroupSelector == nil {
		p.provider.Clear(nc)
		err := nc.StatusConditions().Clear(v1.ConditionTypePlacementGroupReady)
		return reconcile.Result{}, err
	}

	term := lo.FromPtr(nc.Spec.PlacementGroupSelector)
	selector := term.Name
	if selector == "" {
		selector = term.ID
	}

	pg, err := p.provider.Get(ctx, nc)
	if err != nil {
		nc.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupResolutionFailed", fmt.Sprintf("Failed to resolve placement group %q: %s", selector, err))
		return reconcile.Result{}, fmt.Errorf("resolving placement group %q, %w", selector, err)
	}
	if pg == nil {
		nc.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupNotFound", fmt.Sprintf("Placement group %q not found", selector))
		return reconcile.Result{}, nil
	}

	if p.cm.HasChanged(nc.Name, pg.ID) {
		log.FromContext(ctx).V(1).WithValues("id", pg.ID, "name", pg.Name, "strategy", pg.Strategy).Info("discovered placement group")
	}

	nc.StatusConditions().SetTrue(v1.ConditionTypePlacementGroupReady)
	return reconcile.Result{}, nil
}
