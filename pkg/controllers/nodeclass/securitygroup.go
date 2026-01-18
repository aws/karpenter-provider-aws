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
	"sort"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
)

type SecurityGroup struct {
	securityGroupProvider securitygroup.Provider
}

func NewSecurityGroupReconciler(securityGroupProvider securitygroup.Provider) *SecurityGroup {
	return &SecurityGroup{
		securityGroupProvider: securityGroupProvider,
	}
}

func (sg *SecurityGroup) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	securityGroups, err := sg.securityGroupProvider.List(ctx, nodeClass)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting security groups, %w", err)
	}
	if len(securityGroups) == 0 && len(nodeClass.Spec.SecurityGroupSelectorTerms) > 0 {
		nodeClass.Status.SecurityGroups = nil
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeSecurityGroupsReady, "SecurityGroupsNotFound", "SecurityGroupSelector did not match any SecurityGroups")
		// If users have omitted the necessary tags from their SecurityGroups and later add them, we need to reprocess the information.
		// Returning 'ok' in this case means that the nodeclass will remain in an unready state until the component is restarted.
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	// Filter security groups to only include those in the same VPCs as the subnets.
	// This handles two scenarios:
	// 1. Standard case: Security group's primary VPC matches subnet's VPC
	// 2. Security Group Association: A security group is associated with multiple VPCs
	//    via https://docs.aws.amazon.com/vpc/latest/userguide/security-group-assoc.html
	//
	// Note: DescribeSecurityGroups returns only the primary VPC ID. For complete
	// Security Group Association support, additional API calls would be needed.
	// The current implementation covers the majority of use cases where security groups
	// and subnets are in the same VPC.
	vpcIDs := lo.Uniq(lo.Map(nodeClass.Status.Subnets, func(subnet v1.Subnet, _ int) string {
		return subnet.VpcID
	}))
	if len(vpcIDs) > 0 {
		securityGroups = lo.Filter(securityGroups, func(sg ec2types.SecurityGroup, _ int) bool {
			return lo.Contains(vpcIDs, *sg.VpcId)
		})
	}

	if len(securityGroups) == 0 && len(nodeClass.Spec.SecurityGroupSelectorTerms) > 0 {
		nodeClass.Status.SecurityGroups = nil
		// Provide detailed error message to help users diagnose VPC mismatch issues
		vpcIDList := lo.Map(vpcIDs, func(id string, _ int) string { return id })
		msg := fmt.Sprintf("SecurityGroupSelector did not match any SecurityGroups in the same VPC as the subnets (VPCs: %v). "+
			"Ensure your security groups are in the same VPC as your subnets. "+
			"If using Security Group Association (https://docs.aws.amazon.com/vpc/latest/userguide/security-group-assoc.html), "+
			"note that only the primary VPC is currently checked.", vpcIDList)
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeSecurityGroupsReady, "SecurityGroupsNotFound", msg)
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	sort.Slice(securityGroups, func(i, j int) bool {
		return *securityGroups[i].GroupId < *securityGroups[j].GroupId
	})
	nodeClass.Status.SecurityGroups = lo.Map(securityGroups, func(securityGroup ec2types.SecurityGroup, _ int) v1.SecurityGroup {
		return v1.SecurityGroup{
			ID:    *securityGroup.GroupId,
			Name:  *securityGroup.GroupName,
			VpcID: *securityGroup.VpcId,
		}
	})
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeSecurityGroupsReady)
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}
