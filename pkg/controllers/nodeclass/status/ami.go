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

package status

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
)

type AMI struct {
	amiProvider amifamily.Provider
}

func (a *AMI) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	amis, err := a.amiProvider.List(ctx, nodeClass)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting amis, %w", err)
	}
	if len(amis) == 0 {
		nodeClass.Status.AMIs = nil
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeAMIsReady, "AMINotFound", "AMISelector did not match any AMIs")
		return reconcile.Result{}, nil
	}
	nodeClass.Status.AMIs = lo.Map(amis, func(ami amifamily.AMI, _ int) v1.AMI {
		reqs := lo.Map(ami.Requirements.NodeSelectorRequirements(), func(item karpv1.NodeSelectorRequirementWithMinValues, _ int) corev1.NodeSelectorRequirement {
			return item.NodeSelectorRequirement
		})

		sort.Slice(reqs, func(i, j int) bool {
			if len(reqs[i].Key) != len(reqs[j].Key) {
				return len(reqs[i].Key) < len(reqs[j].Key)
			}
			return reqs[i].Key < reqs[j].Key
		})
		return v1.AMI{
			Name:         ami.Name,
			ID:           ami.AmiID,
			Deprecated:   ami.Deprecated,
			Requirements: reqs,
		}
	})

	// If deprecated AMIs are discovered set the AMIsDeprecated status condition
	// If no deprecated AMIs are present, and previous status condition for AMIsDeprecated exists, remove the condition
	hasDeprecatedAMIs := lo.Filter(nodeClass.Status.AMIs, func(ami v1.AMI, _ int) bool {
		return ami.Deprecated
	})
	hasDeprecatedCondition := nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsDeprecated) != nil
	if len(hasDeprecatedAMIs) > 0 {
		nodeClass.StatusConditions().SetTrue(v1.ConditionTypeAMIsDeprecated)
	} else if hasDeprecatedCondition {
		_ = nodeClass.StatusConditions().Clear(v1.ConditionTypeAMIsDeprecated)
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeAMIsReady)
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}
