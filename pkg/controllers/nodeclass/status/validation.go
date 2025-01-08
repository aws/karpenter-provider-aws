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
	"errors"
	"fmt"

	"github.com/samber/lo"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
)

type Validation struct {
	cloudProvider    cloudprovider.CloudProvider
	instanceProvider instance.Provider
}

func (n Validation) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	//Tag Validation
	if offendingTag, found := lo.FindKeyBy(nodeClass.Spec.Tags, func(k string, v string) bool {
		for _, exp := range v1.RestrictedTagPatterns {
			if exp.MatchString(k) {
				return true
			}
		}
		return false
	}); found {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "TagValidationFailed",
			fmt.Sprintf("%q tag does not pass tag validation requirements", offendingTag))
		return reconcile.Result{}, reconcile.TerminalError(fmt.Errorf("%q tag does not pass tag validation requirements", offendingTag))
	}
	//Auth Validation
	//validates createfleet, describelaunchtemplate, createtags, and terminateinstances
	//nolint:ineffassign, staticcheck
	ctx = context.WithValue(ctx, "DryRun", lo.ToPtr(true))
	if nodeClass.StatusConditions().Get(v1.ConditionTypeSubnetsReady).IsFalse() || nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsFalse() {
		return reconcile.Result{}, nil
	}
	nodeClaim := coretest.NodeClaim()
	nodeClaim.Spec.NodeClassRef.Name = nodeClass.Name
	var errs []error
	//create checks createfleet, and describelaunchtemplate
	if _, err := n.cloudProvider.Create(ctx, nodeClaim); err != nil {
		errs = append(errs, fmt.Errorf("create: %w", err))
	}

	if err := n.instanceProvider.Delete(ctx, "mock-id"); err != nil {
		errs = append(errs, fmt.Errorf("delete: %w", err))
	}

	if err := n.instanceProvider.CreateTags(ctx, "mock-id", map[string]string{"mock-tag": "mock-tag-value"}); err != nil {
		errs = append(errs, fmt.Errorf("create tags: %w", err))
	}
	if corecloudprovider.IsNodeClassNotReadyError(errors.Join(errs...)) {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "NodeClassNotReady", fmt.Sprintf("unauthorized operation %v", errors.Join(errs...)))
		return reconcile.Result{}, fmt.Errorf("unauthorized operation %w", errors.Join(errs...))
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
	return reconcile.Result{}, nil
}
