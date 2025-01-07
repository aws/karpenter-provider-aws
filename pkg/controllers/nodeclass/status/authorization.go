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

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
)

type Authorization struct {
	cloudProvider    cloudprovider.CloudProvider
	instanceProvider instance.Provider
}

func (a Authorization) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	//nolint:ineffassign, staticcheck
	ctx = context.WithValue(ctx, "DryRun", lo.ToPtr(true))
	if nodeClass.StatusConditions().Get(v1.ConditionTypeSubnetsReady).IsFalse() || nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsFalse() {
		return reconcile.Result{}, nil
	}
	nodeClaim := coretest.NodeClaim()
	nodeClaim.Spec.NodeClassRef.Name = nodeClass.Name
	_, err := a.cloudProvider.Create(ctx, nodeClaim)
	if err == nil {
		err = a.instanceProvider.Delete(ctx, "mock-id")
		if err == nil {
			err = a.instanceProvider.CreateTags(ctx, "mock-id", map[string]string{"mock-tag": "mock-tag-value"})
		}
	}
	//nolint:ineffassign, staticcheck
	ctx = context.WithValue(ctx, "DryRun", lo.ToPtr(false))
	if corecloudprovider.IsNodeClassNotReadyError(err) {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeAuthorization, "NodeClassNotReady", "Unauthorized Operation")
		return reconcile.Result{}, fmt.Errorf("unauthorized operation %w", err)
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeAuthorization)
	return reconcile.Result{}, nil
}
