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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

// Reasons for the NetworkCompatible condition
const (
	NetworkCompatibleReasonCompatible               = "Compatible"
	NetworkCompatibleReasonPartiallyIncompatible    = "PartiallyIncompatible"
	NetworkCompatibleReasonNoCompatibleSubnet       = "NoCompatibleSubnet"
	NetworkCompatibleReasonCompatibilityCheckFailed = "CompatibilityCheckFailed"
	NetworkCompatibleReasonDependenciesNotReady     = "DependenciesNotReady"
)

// maxReportedSubnets bounds the number of incompatible subnets listed in condition and event messages
const maxReportedSubnets = 5

// NetworkCompatibility reports, through the NetworkCompatible status condition and events, whether every subnet resolved by
// the subnetSelectorTerms can be used together with the full set of security groups resolved by the
// securityGroupSelectorTerms. EC2 rejects a launch that pairs a subnet with a security group that neither belongs to nor
// is associated with the subnet's VPC.
//
// The check is diagnostic only: selector terms remain plain filters, the resolved subnets and security groups are not
// changed, and the condition is not a dependency of the Ready condition.
type NetworkCompatibility struct {
	clk                   clock.Clock
	recorder              events.Recorder
	subnetProvider        subnet.Provider
	securityGroupProvider securitygroup.Provider
	ec2api                sdk.EC2API
	// associationCache caches the VPCs that security groups are associated with, keyed by the sorted security group IDs
	associationCache *cache.Expiring
	// Retain the last mismatch through transient Unknown conditions so recovery still emits an event.
	incompatibilities sync.Map
	eventLimiters     *cache.Expiring
	cm                *pretty.ChangeMonitor
}

func NewNetworkCompatibilityReconciler(clk clock.Clock, recorder events.Recorder, subnetProvider subnet.Provider,
	securityGroupProvider securitygroup.Provider, ec2api sdk.EC2API) *NetworkCompatibility {
	return &NetworkCompatibility{
		clk:                   clk,
		recorder:              recorder,
		subnetProvider:        subnetProvider,
		securityGroupProvider: securityGroupProvider,
		ec2api:                ec2api,
		associationCache:      cache.NewExpiringWithClock(clk),
		eventLimiters:         cache.NewExpiringWithClock(clk),
		cm:                    pretty.NewChangeMonitor(),
	}
}

// incompatibleSubnet is a resolved subnet that cannot be used with at least one resolved security group
type incompatibleSubnet struct {
	SubnetID string
	VPCID    string
	// SecurityGroups are the resolved security groups that the subnet's VPC can't use, formatted as "<sg-id> (<vpc-id>)"
	SecurityGroups []string
}

func (s incompatibleSubnet) String() string {
	return fmt.Sprintf("%s (%s) cannot use %s", s.SubnetID, s.VPCID, utils.PrettySlice(s.SecurityGroups, 5))
}

type compatibilityResult struct {
	TotalSubnets        int
	IncompatibleSubnets []incompatibleSubnet
}

func (r compatibilityResult) reason() string {
	switch {
	case len(r.IncompatibleSubnets) == 0:
		return NetworkCompatibleReasonCompatible
	case len(r.IncompatibleSubnets) == r.TotalSubnets:
		return NetworkCompatibleReasonNoCompatibleSubnet
	default:
		return NetworkCompatibleReasonPartiallyIncompatible
	}
}

func (r compatibilityResult) message() string {
	details := utils.PrettySlice(r.IncompatibleSubnets, maxReportedSubnets)
	const remediation = "Narrow subnetSelectorTerms or securityGroupSelectorTerms so that every resolved security group belongs to, or is associated with, the VPC of every resolved subnet"
	if r.reason() == NetworkCompatibleReasonNoCompatibleSubnet {
		return fmt.Sprintf("None of the %d resolved subnet(s) is compatible with all resolved security groups, launches will fail: %s. %s", r.TotalSubnets, details, remediation)
	}
	return fmt.Sprintf("%d of %d resolved subnet(s) are incompatible with the resolved security groups, launches into them will fail: %s. %s", len(r.IncompatibleSubnets), r.TotalSubnets, details, remediation)
}

func (n *NetworkCompatibility) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	conditions := nodeClass.StatusConditions(status.WithClock(n.clk))
	// Copy the previous state before it is overwritten, it drives which events are emitted
	previous := lo.FromPtr(conditions.Get(v1.ConditionTypeNetworkCompatible))

	if !networkDependenciesReady(nodeClass) {
		// The discovery conditions already explain why the subnets or security groups aren't available
		conditions.SetUnknownWithReason(v1.ConditionTypeNetworkCompatible, NetworkCompatibleReasonDependenciesNotReady, "Waiting for subnet and security group discovery")
		return reconcile.Result{RequeueAfter: awscache.DefaultTTL}, nil
	}

	result, err := n.check(ctx, nodeClass)
	if err != nil {
		message := fmt.Sprintf("Failed to determine subnet and security group compatibility, %.1024s", err)
		if awserrors.IsUnauthorizedOperationError(err) {
			message = fmt.Sprintf("Failed to determine subnet and security group compatibility, ensure the controller has the ec2:DescribeSecurityGroupVpcAssociations permission, %.1024s", err)
		}
		conditions.SetUnknownWithReason(v1.ConditionTypeNetworkCompatible, NetworkCompatibleReasonCompatibilityCheckFailed, message)
		n.recorder.Publish(NetworkCompatibilityCheckFailedEvent(nodeClass, message))
		if n.cm.HasChanged(fmt.Sprintf("check-failed/%s", nodeClass.Name), err.Error()) {
			log.FromContext(ctx).Error(err, "failed checking subnet and security group compatibility")
		}
		// This is diagnostic only, so don't return the error and put the rest of the nodeclass reconciliation into backoff
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	reason := result.reason()
	key := fmt.Sprintf("%s/%s", nodeClass.Name, nodeClass.UID)
	if reason == NetworkCompatibleReasonCompatible {
		conditions.Set(status.Condition{Type: v1.ConditionTypeNetworkCompatible, Status: metav1.ConditionTrue, Reason: reason})
		if _, wasIncompatible := n.incompatibilities.LoadAndDelete(key); wasIncompatible || previous.IsFalse() {
			n.recorder.Publish(NetworkCompatibilityRestoredEvent(nodeClass))
		}
		return reconcile.Result{RequeueAfter: awscache.DefaultTTL}, nil
	}

	message := result.message()
	conditions.SetFalse(v1.ConditionTypeNetworkCompatible, reason, message)
	// Hash the complete result, including resources omitted from the bounded message.
	data, _ := json.Marshal(result)
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(data))
	last, loaded := n.incompatibilities.Swap(key, fingerprint)
	if !loaded || last != fingerprint {
		event := IncompatibleSubnetSecurityGroupsEvent(nodeClass, message, fingerprint)
		// Allow a small burst of affected-resource changes, then at most one warning per minute per NodeClass.
		limiter, ok := n.eventLimiters.Get(key)
		if !ok {
			limiter = flowcontrol.NewTokenBucketRateLimiterWithClock(1.0/60, 5, n.clk)
			n.eventLimiters.Set(key, limiter, 10*time.Minute)
		}
		event.RateLimiter = limiter.(flowcontrol.RateLimiter)
		n.recorder.Publish(event)
	}
	return reconcile.Result{RequeueAfter: awscache.DefaultTTL}, nil
}

// clearState releases per-NodeClass event history as soon as finalization begins.
func (n *NetworkCompatibility) clearState(nodeClass *v1.EC2NodeClass) {
	key := fmt.Sprintf("%s/%s", nodeClass.Name, nodeClass.UID)
	n.incompatibilities.Delete(key)
	n.eventLimiters.Delete(key)
}

func networkDependenciesReady(nodeClass *v1.EC2NodeClass) bool {
	return nodeClass.StatusConditions().Get(v1.ConditionTypeSubnetsReady).IsTrue() &&
		nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsTrue() &&
		len(nodeClass.Status.Subnets) > 0 && len(nodeClass.Status.SecurityGroups) > 0
}

// resolvedResources gets VPC metadata for exactly the resources recorded in status.
func (n *NetworkCompatibility) resolvedResources(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]ec2types.Subnet, []ec2types.SecurityGroup, error) {
	subnets, err := n.subnetProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, nil, fmt.Errorf("getting subnets, %w", err)
	}
	securityGroups, err := n.securityGroupProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, nil, fmt.Errorf("getting security groups, %w", err)
	}
	// Only evaluate the resources recorded in the status, those are the resources used for launches
	subnetIDs := sets.New(lo.Map(nodeClass.Status.Subnets, func(s v1.Subnet, _ int) string { return s.ID })...)
	securityGroupIDs := sets.New(lo.Map(nodeClass.Status.SecurityGroups, func(s v1.SecurityGroup, _ int) string { return s.ID })...)
	subnets = lo.Filter(subnets, func(s ec2types.Subnet, _ int) bool { return subnetIDs.Has(aws.ToString(s.SubnetId)) })
	securityGroups = lo.Filter(securityGroups, func(sg ec2types.SecurityGroup, _ int) bool {
		return securityGroupIDs.Has(aws.ToString(sg.GroupId))
	})
	sort.Slice(subnets, func(i, j int) bool { return aws.ToString(subnets[i].SubnetId) < aws.ToString(subnets[j].SubnetId) })
	sort.Slice(securityGroups, func(i, j int) bool {
		return aws.ToString(securityGroups[i].GroupId) < aws.ToString(securityGroups[j].GroupId)
	})
	// Discovery can change between the status reconcilers and this check. Never treat missing resources or VPC
	// metadata as compatible, even if the remaining resources happen to match.
	foundSubnets, foundSecurityGroups := sets.New[string](), sets.New[string]()
	for _, s := range subnets {
		id := aws.ToString(s.SubnetId)
		if id == "" || aws.ToString(s.VpcId) == "" {
			return nil, nil, fmt.Errorf("subnet %q is missing its ID or VPC ID", id)
		}
		foundSubnets.Insert(id)
	}
	for _, sg := range securityGroups {
		id := aws.ToString(sg.GroupId)
		if id == "" || aws.ToString(sg.VpcId) == "" {
			return nil, nil, fmt.Errorf("security group %q is missing its ID or VPC ID", id)
		}
		foundSecurityGroups.Insert(id)
	}
	if missing := subnetIDs.Difference(foundSubnets); missing.Len() > 0 {
		return nil, nil, fmt.Errorf("resolved subnets are missing from discovery: %s", utils.PrettySlice(sets.List(missing), 5))
	}
	if missing := securityGroupIDs.Difference(foundSecurityGroups); missing.Len() > 0 {
		return nil, nil, fmt.Errorf("resolved security groups are missing from discovery: %s", utils.PrettySlice(sets.List(missing), 5))
	}
	return subnets, securityGroups, nil
}

// check evaluates each resolved subnet against the full set of resolved security groups.
func (n *NetworkCompatibility) check(ctx context.Context, nodeClass *v1.EC2NodeClass) (compatibilityResult, error) {
	subnets, securityGroups, err := n.resolvedResources(ctx, nodeClass)
	if err != nil {
		return compatibilityResult{}, err
	}
	// A security group can always be used in its own VPC. Associations are only looked up for the security groups that
	// would otherwise be incompatible with at least one subnet, so that the common single-VPC case makes no API call.
	subnetVPCs := sets.New(lo.Map(subnets, func(s ec2types.Subnet, _ int) string {
		return aws.ToString(s.VpcId)
	})...)
	candidates := lo.FilterMap(securityGroups, func(sg ec2types.SecurityGroup, _ int) (string, bool) {
		return aws.ToString(sg.GroupId), subnetVPCs.Len() > 1 || !subnetVPCs.Has(aws.ToString(sg.VpcId))
	})
	associations, err := n.associatedVPCs(ctx, candidates)
	if err != nil {
		return compatibilityResult{}, err
	}

	result := compatibilityResult{TotalSubnets: len(subnets)}
	for _, s := range subnets {
		subnetVPC := aws.ToString(s.VpcId)
		incompatible := lo.FilterMap(securityGroups, func(sg ec2types.SecurityGroup, _ int) (string, bool) {
			sgVPC := aws.ToString(sg.VpcId)
			if sgVPC == subnetVPC || associations[aws.ToString(sg.GroupId)].Has(subnetVPC) {
				return "", false
			}
			return fmt.Sprintf("%s (%s)", aws.ToString(sg.GroupId), sgVPC), true
		})
		if len(incompatible) > 0 {
			result.IncompatibleSubnets = append(result.IncompatibleSubnets, incompatibleSubnet{
				SubnetID:       aws.ToString(s.SubnetId),
				VPCID:          subnetVPC,
				SecurityGroups: incompatible,
			})
		}
	}
	return result, nil
}

// associatedVPCs returns, for each of the passed security groups, the VPCs it is actively associated with through
// Security Group VPC Associations
func (n *NetworkCompatibility) associatedVPCs(ctx context.Context, securityGroupIDs []string) (map[string]sets.Set[string], error) {
	if len(securityGroupIDs) == 0 {
		return map[string]sets.Set[string]{}, nil
	}
	key := strings.Join(securityGroupIDs, ",")
	if cached, ok := n.associationCache.Get(key); ok {
		return cached.(map[string]sets.Set[string]), nil
	}
	requested := sets.New(securityGroupIDs...)
	associations := map[string]sets.Set[string]{}
	paginator := ec2.NewDescribeSecurityGroupVpcAssociationsPaginator(n.ec2api, &ec2.DescribeSecurityGroupVpcAssociationsInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("group-id"), Values: securityGroupIDs},
			{Name: aws.String("state"), Values: []string{string(ec2types.SecurityGroupVpcAssociationStateAssociated)}},
		},
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing security group vpc associations, %w", err)
		}
		for _, association := range out.SecurityGroupVpcAssociations {
			groupID, vpcID := aws.ToString(association.GroupId), aws.ToString(association.VpcId)
			if association.State != ec2types.SecurityGroupVpcAssociationStateAssociated || !requested.Has(groupID) || vpcID == "" {
				continue
			}
			if _, ok := associations[groupID]; !ok {
				associations[groupID] = sets.New[string]()
			}
			associations[groupID].Insert(vpcID)
		}
	}
	n.associationCache.Set(key, associations, awscache.DefaultTTL)
	return associations, nil
}
