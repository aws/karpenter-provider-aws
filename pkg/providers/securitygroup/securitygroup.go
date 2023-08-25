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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
)

type Provider struct {
	sync.Mutex
	ec2api ec2iface.EC2API
	cache  *cache.Cache
	cm     *pretty.ChangeMonitor
}

const TTL = 5 * time.Minute

func NewProvider(ec2api ec2iface.EC2API, cache *cache.Cache) *Provider {
	return &Provider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache for v1beta1, utilize resolved security groups from the AWSNodeTemplate.status
		cache: cache,
	}
}

func (p *Provider) List(ctx context.Context, nodeClass *v1beta1.NodeClass) ([]*ec2.SecurityGroup, error) {
	p.Lock()
	defer p.Unlock()
	// Get SecurityGroups
	// TODO: When removing custom launchTemplates for v1beta1, security groups will be required.
	// The check will not be necessary
	filterSets := getFilterSets(nodeClass.Spec.SecurityGroupSelectorTerms)
	if len(filterSets) == 0 {
		return []*ec2.SecurityGroup{}, nil
	}
	securityGroups, err := p.getSecurityGroups(ctx, filterSets)
	if err != nil {
		return nil, err
	}
	if p.cm.HasChanged(fmt.Sprintf("security-groups/%t/%s", nodeClass.IsNodeTemplate, nodeClass.Name), securityGroups) {
		logging.FromContext(ctx).
			With("security-groups", lo.Map(securityGroups, func(s *ec2.SecurityGroup, _ int) string {
				return aws.StringValue(s.GroupId)
			})).
			Debugf("discovered security groups")
	}
	return securityGroups, nil
}

func (p *Provider) getSecurityGroups(ctx context.Context, filterSets [][]*ec2.Filter) ([]*ec2.SecurityGroup, error) {
	hash, err := hashstructure.Hash(filterSets, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, err
	}
	if sg, ok := p.cache.Get(fmt.Sprint(hash)); ok {
		return sg.([]*ec2.SecurityGroup), nil
	}
	securityGroups := map[string]*ec2.SecurityGroup{}
	for _, filters := range filterSets {
		output, err := p.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{Filters: filters})
		if err != nil {
			return nil, fmt.Errorf("describing security groups %+v, %w", filterSets, err)
		}
		for _, sg := range output.SecurityGroups {
			securityGroups[lo.FromPtr(sg.GroupId)] = sg
		}
	}
	p.cache.SetDefault(fmt.Sprint(hash), lo.Values(securityGroups))
	return lo.Values(securityGroups), nil
}

// TODO @joinnis: It's possible that we could make this filtering logic more efficient by combining selectors
// that only use the term "id" into a single filtered term or that only use the term "name" into a single filtered term
func getFilterSets(terms []v1beta1.SecurityGroupSelectorTerm) [][]*ec2.Filter {
	return lo.Map(terms, func(t v1beta1.SecurityGroupSelectorTerm, _ int) []*ec2.Filter {
		return getFilters(t)
	})
}

func getFilters(term v1beta1.SecurityGroupSelectorTerm) []*ec2.Filter {
	var filters []*ec2.Filter
	if term.ID != "" {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String("group-id"),
			Values: aws.StringSlice([]string{term.ID}),
		})
	}
	if term.Name != "" {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String("group-name"),
			Values: aws.StringSlice([]string{term.Name}),
		})
	}
	for k, v := range term.Tags {
		if v == "*" {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(k)},
			})
		} else {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(fmt.Sprintf("tag:%s", k)),
				Values: aws.StringSlice(functional.SplitCommaSeparatedString(v)),
			})
		}
	}
	return filters
}
