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

package cloudprovider

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awscontext "github.com/aws/karpenter/pkg/context"

	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

type SecurityGroupProvider struct {
	sync.Mutex
	ec2api ec2iface.EC2API
	cache  *cache.Cache
	cm     *pretty.ChangeMonitor
}

func NewSecurityGroupProvider(ec2api ec2iface.EC2API) *SecurityGroupProvider {
	return &SecurityGroupProvider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		cache:  cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval),
	}
}

func (p *SecurityGroupProvider) Get(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) ([]string, error) {
	p.Lock()
	defer p.Unlock()
	// Get SecurityGroups
	securityGroups, err := p.getSecurityGroups(ctx, p.getFilters(nodeTemplate))
	if err != nil {
		return nil, err
	}
	// Fail if no security groups found
	if len(securityGroups) == 0 {
		return nil, fmt.Errorf("no security groups exist given constraints")
	}
	// Convert to IDs
	securityGroupIds := []string{}
	for _, securityGroup := range securityGroups {
		securityGroupIds = append(securityGroupIds, aws.StringValue(securityGroup.GroupId))
	}
	return securityGroupIds, nil
}

func (p *SecurityGroupProvider) getFilters(nodeTemplate *v1alpha1.AWSNodeTemplate) []*ec2.Filter {
	filters := []*ec2.Filter{}
	for key, value := range nodeTemplate.Spec.SecurityGroupSelector {
		if key == "aws-ids" {
			filterValues := functional.SplitCommaSeparatedString(value)
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("group-id"),
				Values: aws.StringSlice(filterValues),
			})
		} else {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(fmt.Sprintf("tag:%s", key)),
				Values: []*string{aws.String(value)},
			})
		}
	}
	return filters
}

func (p *SecurityGroupProvider) getSecurityGroups(ctx context.Context, filters []*ec2.Filter) ([]*ec2.SecurityGroup, error) {
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
	if p.cm.HasChanged("security-groups", output.SecurityGroups) {
		logging.FromContext(ctx).With("security-groups", p.securityGroupIds(output.SecurityGroups)).Debugf("discovered security groups")
	}
	return output.SecurityGroups, nil
}

func (p *SecurityGroupProvider) securityGroupIds(securityGroups []*ec2.SecurityGroup) []string {
	names := []string{}
	for _, securityGroup := range securityGroups {
		names = append(names, aws.StringValue(securityGroup.GroupId))
	}
	return names
}
