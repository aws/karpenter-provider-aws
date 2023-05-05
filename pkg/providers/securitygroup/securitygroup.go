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
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
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

func (p *Provider) List(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) ([]string, error) {
	p.Lock()
	defer p.Unlock()
	// Get SecurityGroups
	// TODO: When removing custom launchTemplates for v1beta1, security groups will be required.
	// The check will not be necessary
	filters := p.getFilters(nodeTemplate)
	if len(filters) == 0 {
		return []string{}, nil
	}
	securityGroups, err := p.getSecurityGroups(ctx, filters)
	if err != nil {
		return nil, err
	}
	if p.cm.HasChanged(fmt.Sprintf("security-groups/%s", nodeTemplate.Name), securityGroups) {
		logging.FromContext(ctx).
			With("security-groups", lo.Map(securityGroups, func(s *ec2.SecurityGroup, _ int) string {
				return aws.StringValue(s.GroupId)
			})).
			Debugf("discovered security groups")
	}
	return lo.Map(securityGroups, func(s *ec2.SecurityGroup, _ int) string { return aws.StringValue(s.GroupId) }), nil
}

func (p *Provider) getFilters(nodeTemplate *v1alpha1.AWSNodeTemplate) []*ec2.Filter {
	var filters []*ec2.Filter
	for key, value := range nodeTemplate.Spec.SecurityGroupSelector {
		if key == "aws-ids" {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("group-id"),
				Values: aws.StringSlice(functional.SplitCommaSeparatedString(value)),
			})
		} else if value == "*" {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(key)},
			})
		} else {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(fmt.Sprintf("tag:%s", key)),
				Values: aws.StringSlice(functional.SplitCommaSeparatedString(value)),
			})
		}
	}
	return filters
}

func (p *Provider) getSecurityGroups(ctx context.Context, filters []*ec2.Filter) ([]*ec2.SecurityGroup, error) {
	hash, err := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}
	if securityGroups, ok := p.cache.Get(fmt.Sprint(hash)); ok {
		return securityGroups.([]*ec2.SecurityGroup), nil
	}
	output, err := p.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("describing security groups %+v, %w", filters, err)
	}
	p.cache.SetDefault(fmt.Sprint(hash), output.SecurityGroups)
	return output.SecurityGroups, nil
}
