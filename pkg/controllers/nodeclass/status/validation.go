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
	"regexp"

	"github.com/samber/lo"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type Validation struct{}

func (n Validation) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if _, found := lo.Find(v1.RestrictedTagPatterns, func(exp *regexp.Regexp) bool {
		for key := range nodeClass.Spec.Tags {
			if exp.MatchString(key) {
				return true
			}
		}
		return false
	}); found {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "NodeClassNotReady", "tag validation bypassed")
		return reconcile.Result{}, fmt.Errorf("tag validation bypassed")
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
	return reconcile.Result{}, nil
}
