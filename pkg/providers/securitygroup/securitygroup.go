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

package securitygroup

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Provider interface {
	List(context.Context, *v1.EC2NodeClass) ([]ec2types.SecurityGroup, error)
}

type DefaultProvider struct {
	sync.Mutex
	ec2api sdk.EC2API
	eksapi sdk.EKSAPI
	cache  *cache.Cache
	cm     *pretty.ChangeMonitor
	vpcID  atomic.Pointer[string]
}

func NewDefaultProvider(ec2api sdk.EC2API, eksapi sdk.EKSAPI, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api: ec2api,
		eksapi: eksapi,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache cache when we utilize the security groups from the EC2NodeClass.status
		cache: cache,
	}
}

// ResolveVpcID resolves the VPC of the EKS cluster, caching the result for the lifetime of the process. The VPC ID
// scopes tag-based and name-based discovery to security groups created in or associated with the cluster VPC.
func (p *DefaultProvider) ResolveVpcID(ctx context.Context) (string, error) {
	if vpcID := p.vpcID.Load(); vpcID != nil {
		return *vpcID, nil
	}
	out, err := p.eksapi.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(options.FromContext(ctx).ClusterName),
	})
	if err != nil {
		return "", err
	}
	if out == nil || out.Cluster == nil || out.Cluster.ResourcesVpcConfig == nil || lo.FromPtr(out.Cluster.ResourcesVpcConfig.VpcId) == "" {
		return "", fmt.Errorf("no vpc id found in DescribeCluster response")
	}
	vpcID := out.Cluster.ResourcesVpcConfig.VpcId
	p.vpcID.Store(vpcID)
	log.FromContext(ctx).WithValues("vpc-id", *vpcID).V(1).Info("discovered cluster vpc id")
	return *vpcID, nil
}

func (p *DefaultProvider) List(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]ec2types.SecurityGroup, error) {
	p.Lock()
	defer p.Unlock()

	securityGroups, err := p.getSecurityGroups(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	securityGroupIDs := lo.Map(securityGroups, func(s ec2types.SecurityGroup, _ int) string { return aws.ToString(s.GroupId) })
	if p.cm.HasChanged(fmt.Sprintf("security-groups/%s", nodeClass.Name), securityGroupIDs) {
		log.FromContext(ctx).
			WithValues("security-groups", securityGroupIDs).
			V(1).Info("discovered security groups")
	}
	return securityGroups, nil
}

func (p *DefaultProvider) getSecurityGroups(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]ec2types.SecurityGroup, error) {
	var vpcID string
	// Selecting security groups by ID is an explicit user intent, so only tag-based and name-based discovery is scoped
	// to the cluster VPC.
	if scopeToClusterVPC(ctx) && lo.ContainsBy(nodeClass.Spec.SecurityGroupSelectorTerms, func(term v1.SecurityGroupSelectorTerm) bool {
		return term.ID == ""
	}) {
		var err error
		if vpcID, err = p.ResolveVpcID(ctx); err != nil {
			return nil, fmt.Errorf("resolving cluster vpc id, %w", err)
		}
	}
	hash := utils.GetNodeClassHash(nodeClass)
	if sg, ok := p.cache.Get(hash); ok {
		// Return a shallow copy so callers can reorder the slice without changing the cached ordering.
		return append([]ec2types.SecurityGroup{}, sg.([]ec2types.SecurityGroup)...), nil
	}
	securityGroups, err := p.discoverSecurityGroups(ctx, getFilterSets(nodeClass.Spec.SecurityGroupSelectorTerms), vpcID)
	if err != nil {
		return nil, err
	}
	p.cache.SetDefault(hash, securityGroups)
	return append([]ec2types.SecurityGroup{}, securityGroups...), nil
}

func (p *DefaultProvider) discoverSecurityGroups(ctx context.Context, filterSets [][]ec2types.Filter, vpcID string) ([]ec2types.SecurityGroup, error) {
	securityGroups := map[string]ec2types.SecurityGroup{}
	associationCandidates := map[string]ec2types.SecurityGroup{}
	for _, filters := range filterSets {
		byID := lo.ContainsBy(filters, func(filter ec2types.Filter) bool { return aws.ToString(filter.Name) == "group-id" })
		paginator := ec2.NewDescribeSecurityGroupsPaginator(p.ec2api, &ec2.DescribeSecurityGroupsInput{
			MaxResults: aws.Int32(500),
			Filters:    filters,
		})
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("describing security groups %+v, %w", filterSets, err)
			}
			for _, group := range output.SecurityGroups {
				id := aws.ToString(group.GroupId)
				if vpcID != "" && !byID && aws.ToString(group.VpcId) != vpcID {
					associationCandidates[id] = group
					continue
				}
				securityGroups[id] = group
			}
		}
	}
	// Explicit IDs already satisfy the union, regardless of whether they also matched a tag or name.
	for id := range securityGroups {
		delete(associationCandidates, id)
	}
	associatedGroups, err := p.getAssociatedSecurityGroups(ctx, vpcID, associationCandidates)
	if err != nil {
		return nil, err
	}
	for _, group := range associatedGroups {
		securityGroups[aws.ToString(group.GroupId)] = group
	}
	return lo.Values(securityGroups), nil
}

// DescribeSecurityGroups' vpc-id filter only matches the VPC where a group was created. Groups created in another
// VPC may also be usable through a VPC association. Refresh these associations with the security group results.
func (p *DefaultProvider) getAssociatedSecurityGroups(ctx context.Context, vpcID string, candidates map[string]ec2types.SecurityGroup) ([]ec2types.SecurityGroup, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	paginator := ec2.NewDescribeSecurityGroupVpcAssociationsPaginator(p.ec2api, &ec2.DescribeSecurityGroupVpcAssociationsInput{
		MaxResults: aws.Int32(500),
		Filters: []ec2types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			{Name: aws.String("state"), Values: []string{string(ec2types.SecurityGroupVpcAssociationStateAssociated)}},
		},
	})
	var groups []ec2types.SecurityGroup
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing security group VPC associations, %w", err)
		}
		for _, association := range output.SecurityGroupVpcAssociations {
			if group, ok := candidates[aws.ToString(association.GroupId)]; ok {
				groups = append(groups, group)
			}
		}
	}
	return groups, nil
}

// scopeToClusterVPC reports whether the cluster VPC can be resolved without new requirements on the deployment: both
// an empty cluster endpoint and an EKS control plane already require DescribeCluster to succeed at startup.
func scopeToClusterVPC(ctx context.Context) bool {
	opts := options.FromContext(ctx)
	return opts.EKSControlPlane || opts.ClusterEndpoint == ""
}

func getFilterSets(terms []v1.SecurityGroupSelectorTerm) (res [][]ec2types.Filter) {
	idFilter := ec2types.Filter{Name: aws.String("group-id")}
	nameFilter := ec2types.Filter{Name: aws.String("group-name")}
	for _, term := range terms {
		switch {
		case term.ID != "":
			idFilter.Values = append(idFilter.Values, term.ID)
		case term.Name != "":
			nameFilter.Values = append(nameFilter.Values, term.Name)
		default:
			var filters []ec2types.Filter
			for k, v := range term.Tags {
				if v == "*" {
					filters = append(filters, ec2types.Filter{
						Name:   aws.String("tag-key"),
						Values: []string{k},
					})
				} else {
					filters = append(filters, ec2types.Filter{
						Name:   aws.String(fmt.Sprintf("tag:%s", k)),
						Values: []string{v},
					})
				}
			}
			res = append(res, filters)
		}
	}
	if len(idFilter.Values) > 0 {
		res = append(res, []ec2types.Filter{idFilter})
	}
	if len(nameFilter.Values) > 0 {
		// Security group names are only unique within a VPC, so the same name can match groups in other VPCs
		res = append(res, []ec2types.Filter{nameFilter})
	}
	return res
}
