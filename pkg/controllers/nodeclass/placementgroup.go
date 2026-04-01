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

	"github.com/awslabs/operatorpkg/serrors"
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
	pg, err := p.provider.Get(ctx, nc)
	// Determine selector string for error messages
	var selector string
	if nc.Spec.PlacementGroupSelector != nil {
		term := lo.FromPtr(nc.Spec.PlacementGroupSelector)
		selector = term.Name
		if selector == "" {
			selector = term.ID
		}
	}
	if err != nil {
		nc.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupResolutionFailed", "Failed to resolve placement group")
		return reconcile.Result{}, serrors.Wrap(fmt.Errorf("resolving placement group, %w", err), "placement-group", selector)
	}
	if pg == nil {
		if nc.Spec.PlacementGroupSelector != nil {
			nc.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupNotFound", fmt.Sprintf("Placement group %q not found", selector))
		} else {
			nc.StatusConditions().SetTrue(v1.ConditionTypePlacementGroupReady)
		}
		return reconcile.Result{}, nil
	}

	if p.cm.HasChanged(nc.Name, pg.ID) {
		log.FromContext(ctx).V(1).WithValues("id", pg.ID, "name", pg.Name, "strategy", pg.Strategy).Info("discovered placement group")
	}

	nc.StatusConditions().SetTrue(v1.ConditionTypePlacementGroupReady)
	return reconcile.Result{}, nil
}
