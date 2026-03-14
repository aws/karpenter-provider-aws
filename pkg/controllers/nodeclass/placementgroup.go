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
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
)

type PlacementGroupReconciler struct {
	provider placementgroup.Provider
}

func NewPlacementGroupReconciler(provider placementgroup.Provider) *PlacementGroupReconciler {
	return &PlacementGroupReconciler{
		provider: provider,
	}
}

func (p *PlacementGroupReconciler) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if nodeClass.Spec.PlacementGroup == nil {
		nodeClass.Status.PlacementGroup = nil
		nodeClass.StatusConditions().SetTrue(v1.ConditionTypePlacementGroupReady)
		return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	pg, err := p.provider.Get(ctx, nodeClass.Spec.PlacementGroup)
	if err != nil {
		return reconcile.Result{}, err
	}
	if pg == nil {
		nodeClass.Status.PlacementGroup = nil
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupNotFound", "placementGroup did not match any PlacementGroup")
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}
	if pg.State != ec2types.PlacementGroupStateAvailable {
		nodeClass.Status.PlacementGroup = v1.PlacementGroupStatusFromEC2(pg)
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupNotAvailable",
			fmt.Sprintf("placementGroup is in state %q, must be %q", pg.State, ec2types.PlacementGroupStateAvailable))
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}
	if nodeClass.Spec.PlacementGroup.Partition != nil && pg.Strategy != ec2types.PlacementStrategyPartition {
		nodeClass.Status.PlacementGroup = v1.PlacementGroupStatusFromEC2(pg)
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupInvalid", "placementGroup.partition may only be set for partition placement groups")
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}
	if nodeClass.Spec.PlacementGroup.Partition != nil && pg.PartitionCount != nil && *nodeClass.Spec.PlacementGroup.Partition > *pg.PartitionCount {
		nodeClass.Status.PlacementGroup = v1.PlacementGroupStatusFromEC2(pg)
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypePlacementGroupReady, "PlacementGroupInvalid",
			fmt.Sprintf("placementGroup.partition %d exceeds placement group partition count %d", *nodeClass.Spec.PlacementGroup.Partition, *pg.PartitionCount))
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	nodeClass.Status.PlacementGroup = v1.PlacementGroupStatusFromEC2(pg)
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypePlacementGroupReady)
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}
