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
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Provider interface {
	List(context.Context, *v1.EC2NodeClass) ([]ec2types.SecurityGroup, error)
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
		// Cache is used as a fallback when security groups are not yet available in EC2NodeClass.status
		cache: cache,
	}
}

func (p *DefaultProvider) List(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]ec2types.SecurityGroup, error) {
	p.Lock()
	defer p.Unlock()

	// First check if security groups are available in the EC2NodeClass.status
	if len(nodeClass.Status.SecurityGroups) > 0 {
		// Convert from v1.SecurityGroup to ec2types.SecurityGroup
		securityGroups := lo.Map(nodeClass.Status.SecurityGroups, func(sg v1.SecurityGroup, _ int) ec2types.SecurityGroup {
			return ec2types.SecurityGroup{
				GroupId:   aws.String(sg.ID),
				GroupName: aws.String(sg.Name),
			}
		})
		securityGroupIDs := lo.Map(nodeClass.Status.SecurityGroups, func(sg v1.SecurityGroup, _ int) string { return sg.ID })
		log.FromContext(ctx).
			WithValues("security-groups", securityGroupIDs).
			V(1).Info("using security groups from EC2NodeClass.status")
		return securityGroups, nil
	}

	// Fall back to querying AWS if security groups are not in status
	filterSets := getFilterSets(nodeClass.Spec.SecurityGroupSelectorTerms)
	securityGroups, err := p.getSecurityGroups(ctx, filterSets)
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

func (p *DefaultProvider) getSecurityGroups(ctx context.Context, filterSets [][]ec2types.Filter) ([]ec2types.SecurityGroup, error) {
	hash, err := hashstructure.Hash(filterSets, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, err
	}
	if sg, ok := p.cache.Get(fmt.Sprint(hash)); ok {
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
	p.cache.SetDefault(fmt.Sprint(hash), lo.Values(securityGroups))
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
