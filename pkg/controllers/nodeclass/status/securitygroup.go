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

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	providerv1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
)

type SecurityGroup struct {
	securityGroupProvider securitygroup.Provider
}

func (sg *SecurityGroup) Reconcile(ctx context.Context, nodeClass *providerv1.EC2NodeClass) (reconcile.Result, error) {
	securityGroups, err := sg.securityGroupProvider.List(ctx, nodeClass)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting security groups, %w", err)
	}
	if len(securityGroups) == 0 && len(nodeClass.Spec.SecurityGroupSelectorTerms) > 0 {
		nodeClass.Status.SecurityGroups = nil
		return reconcile.Result{}, nil
	}
	sort.Slice(securityGroups, func(i, j int) bool {
		return *securityGroups[i].GroupId < *securityGroups[j].GroupId
	})
	nodeClass.Status.SecurityGroups = lo.Map(securityGroups, func(securityGroup *ec2.SecurityGroup, _ int) providerv1.SecurityGroup {
		return providerv1.SecurityGroup{
			ID:   *securityGroup.GroupId,
			Name: *securityGroup.GroupName,
		}
	})
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}
