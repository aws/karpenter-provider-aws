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

	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
)

type Readiness struct {
	launchTemplateProvider launchtemplate.Provider
}

func (n Readiness) Reconcile(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (reconcile.Result, error) {
	if len(nodeClass.Status.AMIs) == 0 {
		nodeClass.StatusConditions().SetFalse(status.ConditionReady, "NodeClassNotReady", "Failed to resolve AMIs")
		return reconcile.Result{}, nil
	}
	if len(nodeClass.Status.Subnets) == 0 {
		nodeClass.StatusConditions().SetFalse(status.ConditionReady, "NodeClassNotReady", "Failed to resolve subnets")
		return reconcile.Result{}, nil
	}
	if len(nodeClass.Status.SecurityGroups) == 0 {
		nodeClass.StatusConditions().SetFalse(status.ConditionReady, "NodeClassNotReady", "Failed to resolve security groups")
		return reconcile.Result{}, nil
	}
	if len(nodeClass.Status.InstanceProfile) == 0 {
		nodeClass.StatusConditions().SetFalse(status.ConditionReady, "NodeClassNotReady", "Failed to resolve instance profile")
		return reconcile.Result{}, nil
	}
	// A NodeClass that uses AL2023 requires the cluster CIDR for launching nodes.
	// To allow Karpenter to be used for Non-EKS clusters, resolving the Cluster CIDR
	// will not be done at startup but instead in a reconcile loop.
	if lo.FromPtr(nodeClass.Spec.AMIFamily) == v1beta1.AMIFamilyAL2023 {
		if err := n.launchTemplateProvider.ResolveClusterCIDR(ctx); err != nil {
			nodeClass.StatusConditions().SetFalse(status.ConditionReady, "NodeClassNotReady", "Failed to detect the cluster CIDR")
			return reconcile.Result{}, fmt.Errorf("failed to detect the cluster CIDR, %w", err)
		}
	}
	nodeClass.StatusConditions().SetTrue(status.ConditionReady)
	return reconcile.Result{}, nil
}
