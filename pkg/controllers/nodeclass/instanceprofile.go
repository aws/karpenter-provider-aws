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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
)

type InstanceProfile struct {
	instanceProfileProvider instanceprofile.Provider
}

func NewInstanceProfileReconciler(instanceProfileProvider instanceprofile.Provider) *InstanceProfile {
	return &InstanceProfile{
		instanceProfileProvider: instanceProfileProvider,
	}
}

func (ip *InstanceProfile) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if nodeClass.Spec.Role != "" {
		name, err := ip.instanceProfileProvider.Create(ctx, nodeClass)
		if err != nil {
			//Create filters out instanceprofile not found errors so any is not found error will be referencing the role
			if awserrors.IsNotFound(err) || awserrors.IsUnauthorizedOperationError(err) {
				nodeClass.StatusConditions().SetFalse(v1.ConditionTypeInstanceProfileReady, "NodeRoleNotFound", "Failed to detect the NodeRole")
			}
			return reconcile.Result{}, fmt.Errorf("creating instance profile, %w", err)
		}
		nodeClass.Status.InstanceProfile = name
	} else {
		_, _, err := ip.instanceProfileProvider.Get(ctx, nodeClass, lo.FromPtr(nodeClass.Spec.InstanceProfile))
		if err != nil {
			if awserrors.IsNotFound(err) || awserrors.IsUnauthorizedOperationError(err) {
				nodeClass.StatusConditions().SetFalse(v1.ConditionTypeInstanceProfileReady, "InstanceProfileNotFound", "Failed to detect the Instance Profile")
			}
			return reconcile.Result{}, fmt.Errorf("getting instance profile, %w", err)
		}
		nodeClass.Status.InstanceProfile = lo.FromPtr(nodeClass.Spec.InstanceProfile)
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeInstanceProfileReady)
	return reconcile.Result{}, nil
}
