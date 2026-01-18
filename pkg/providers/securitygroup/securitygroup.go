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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Provider interface {
	List(context.Context, *v1.EC2NodeClass) ([]ec2types.SecurityGroup, error)
	// ValidateSecurityGroupVPCCompatibility checks if security groups can be used with the given VPC IDs.
	// This includes checking for Security Group Association scenarios where a security group
	// may be associated with multiple VPCs.
	ValidateSecurityGroupVPCCompatibility(ctx context.Context, securityGroupIDs []string, vpcIDs []string) (map[string]bool, error)
}

type DefaultProvider struct {
	sync.Mutex
	ec2api sdk.EC2API
	cache  *cache.Cache
	cm     *pretty.ChangeMonitor
}

func NewDefaultProvider(ec2api sdk.EC2API, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache cache when we utilize the security groups from the EC2NodeClass.status
		cache: cache,
	}
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
	filterSets := getFilterSets(nodeClass.Spec.SecurityGroupSelectorTerms)
	hash := utils.GetNodeClassHash(nodeClass)
	if sg, ok := p.cache.Get(hash); ok {
		// Ensure what's returned from this function is a shallow-copy of the slice (not a deep-copy of the data itself)
		// so that modifications to the ordering of the data don't affect the original
		return append([]ec2types.SecurityGroup{}, sg.([]ec2types.SecurityGroup)...), nil
	}
	securityGroups := map[string]ec2types.SecurityGroup{}
	for _, filters := range filterSets {
		paginator := ec2.NewDescribeSecurityGroupsPaginator(p.ec2api, &ec2.DescribeSecurityGroupsInput{
			MaxResults: aws.Int32(500),
			Filters:    filters,
		})
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("describing security groups %+v, %w", filterSets, err)
			}
			for i := range output.SecurityGroups {
				securityGroups[lo.FromPtr(output.SecurityGroups[i].GroupId)] = output.SecurityGroups[i]
			}
		}
	}
	p.cache.SetDefault(hash, lo.Values(securityGroups))
	return lo.Values(securityGroups), nil
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
		res = append(res, []ec2types.Filter{nameFilter})
	}
	return res
}

// ValidateSecurityGroupVPCCompatibility checks if security groups are compatible with the given VPC IDs.
// This method checks both the primary VPC and any additional VPCs the security group may be
// associated with via Security Group Association.
//
// Returns a map of security group ID -> compatible (true/false)
func (p *DefaultProvider) ValidateSecurityGroupVPCCompatibility(ctx context.Context, securityGroupIDs []string, vpcIDs []string) (map[string]bool, error) {
	if len(securityGroupIDs) == 0 || len(vpcIDs) == 0 {
		return map[string]bool{}, nil
	}

	result := make(map[string]bool, len(securityGroupIDs))

	// First, get the basic security group information (includes primary VPC)
	describeOutput, err := p.ec2api.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: securityGroupIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("describing security groups for VPC validation, %w", err)
	}

	// Check primary VPC compatibility
	for _, sg := range describeOutput.SecurityGroups {
		sgID := aws.ToString(sg.GroupId)
		primaryVPCID := aws.ToString(sg.VpcId)
		result[sgID] = lo.Contains(vpcIDs, primaryVPCID)
	}

	// Check for Security Group Association using the dedicated API
	// This API returns all VPCs that a security group is associated with
	assocOutput, err := p.ec2api.DescribeSecurityGroupVpcAssociations(ctx, &ec2.DescribeSecurityGroupVpcAssociationsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("group-id"),
				Values: securityGroupIDs,
			},
			{
				Name:   aws.String("state"),
				Values: []string{string(ec2types.SecurityGroupVpcAssociationStateAssociated)},
			},
		},
	})
	if err != nil {
		// If DescribeSecurityGroupVpcAssociations fails, log but continue with primary VPC check
		// This ensures backward compatibility if the API is not available
		log.FromContext(ctx).V(1).Info("unable to check security group VPC associations, using primary VPC only", "error", err)
		return result, nil
	}

	// Check associations - if a security group is associated with any of our target VPCs, mark it as compatible
	for _, assoc := range assocOutput.SecurityGroupVpcAssociations {
		sgID := aws.ToString(assoc.GroupId)
		associatedVPCID := aws.ToString(assoc.VpcId)
		if lo.Contains(vpcIDs, associatedVPCID) {
			result[sgID] = true
		}
	}

	return result, nil
}
