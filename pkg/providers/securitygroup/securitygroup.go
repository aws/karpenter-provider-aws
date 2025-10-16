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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Provider interface {
	List(context.Context, *v1beta1.EC2NodeClass) ([]*ec2.SecurityGroup, error)
}

type DefaultProvider struct {
	sync.Mutex
	ec2api ec2iface.EC2API
	cache  *cache.Cache
	cm     *pretty.ChangeMonitor
}

func NewDefaultProvider(ec2api ec2iface.EC2API, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache cache when we utilize the security groups from the EC2NodeClass.status
		cache: cache,
	}
}

func (p *DefaultProvider) List(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) ([]*ec2.SecurityGroup, error) {
	p.Lock()
	defer p.Unlock()

	securityGroups, err := p.getSecurityGroups(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	if p.cm.HasChanged(fmt.Sprintf("security-groups/%s", nodeClass.Name), securityGroups) {
		log.FromContext(ctx).
			WithValues("security-groups", lo.Map(securityGroups, func(s *ec2.SecurityGroup, _ int) string {
				return aws.StringValue(s.GroupId)
			})).
			V(1).Info("discovered security groups")
	}
	return securityGroups, nil
}

func (p *DefaultProvider) getSecurityGroups(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) ([]*ec2.SecurityGroup, error) {
	filterSets := getFilterSets(nodeClass.Spec.SecurityGroupSelectorTerms)
	hash := utils.GetNodeClassHash(nodeClass)
	if sg, ok := p.cache.Get(hash); ok {
		// Ensure what's returned from this function is a shallow-copy of the slice (not a deep-copy of the data itself)
		// so that modifications to the ordering of the data don't affect the original
		return append([]*ec2.SecurityGroup{}, sg.([]*ec2.SecurityGroup)...), nil
	}
	securityGroups := map[string]*ec2.SecurityGroup{}
	for _, filters := range filterSets {
		output, err := p.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{Filters: filters})
		if err != nil {
			return nil, fmt.Errorf("describing security groups %+v, %w", filterSets, err)
		}
		for i := range output.SecurityGroups {
			securityGroups[lo.FromPtr(output.SecurityGroups[i].GroupId)] = output.SecurityGroups[i]
		}
	}
	p.cache.SetDefault(hash, lo.Values(securityGroups))
	return lo.Values(securityGroups), nil
}

func getFilterSets(terms []v1beta1.SecurityGroupSelectorTerm) (res [][]*ec2.Filter) {
	idFilter := &ec2.Filter{Name: aws.String("group-id")}
	nameFilter := &ec2.Filter{Name: aws.String("group-name")}
	for _, term := range terms {
		switch {
		case term.ID != "":
			idFilter.Values = append(idFilter.Values, aws.String(term.ID))
		case term.Name != "":
			nameFilter.Values = append(nameFilter.Values, aws.String(term.Name))
		default:
			var filters []*ec2.Filter
			for k, v := range term.Tags {
				if v == "*" {
					filters = append(filters, &ec2.Filter{
						Name:   aws.String("tag-key"),
						Values: []*string{aws.String(k)},
					})
				} else {
					filters = append(filters, &ec2.Filter{
						Name:   aws.String(fmt.Sprintf("tag:%s", k)),
						Values: []*string{aws.String(v)},
					})
				}
			}
			res = append(res, filters)
		}
	}
	if len(idFilter.Values) > 0 {
		res = append(res, []*ec2.Filter{idFilter})
	}
	if len(nameFilter.Values) > 0 {
		res = append(res, []*ec2.Filter{nameFilter})
	}
	return res
}
