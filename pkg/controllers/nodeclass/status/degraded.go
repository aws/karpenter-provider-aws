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

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/samber/lo"
)

type Degraded struct {
	instanceTypeProvider instancetype.Provider
	instanceProvider     instance.Provider
	kubeClient           client.Client
}

func (d Degraded) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if nodeClass.StatusConditions().Get(v1.ConditionTypeSubnetsReady).IsFalse() || nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsFalse() {
		return reconcile.Result{}, nil
	}
	nodeClaims := &karpv1.NodeClaimList{}
	if err := d.kubeClient.List(ctx, nodeClaims, nodeclaimutils.ForNodeClass(nodeClass)); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing nodeclaims that are using nodeclass, %w", err)
	}
	_, err := d.instanceProvider.Create(ctx, nodeClass, &nodeClaims.Items[0], nodeClass.Spec.Tags, lo.Must(d.resolveInstanceTypes(ctx, &nodeClaims.Items[0], nodeClass)))
	if corecloudprovider.IsNodeClassNotReadyError(err) {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeNotDegraded, "NodeClassDegraded", "Unauthorized Operation")
		return reconcile.Result{}, fmt.Errorf("Unauthorized Operation %w", err)
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeNotDegraded)
	return reconcile.Result{}, nil
}

func (d *Degraded) resolveInstanceTypes(ctx context.Context, nodeClaim *karpv1.NodeClaim, nodeClass *v1.EC2NodeClass) ([]*corecloudprovider.InstanceType, error) {
	instanceTypes, err := d.instanceTypeProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, fmt.Errorf("getting instance types, %w", err)
	}
	reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...)
	return lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool {
		return reqs.Compatible(i.Requirements, scheduling.AllowUndefinedWellKnownLabels) == nil &&
			len(i.Offerings.Compatible(reqs).Available()) > 0 &&
			resources.Fits(nodeClaim.Spec.Resources.Requests, i.Allocatable())
	}), nil
}
