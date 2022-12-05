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
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	"github.com/aws/karpenter-core/pkg/utils/functional"
)

type SecurityGroupProvider struct {
	sync.Mutex
	cache *cache.Cache
}

func NewSecurityGroupProvider(c *cache.Cache) *SecurityGroupProvider {
	return &SecurityGroupProvider{
		cache: c,
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

func (p *SecurityGroupProvider) getSecurityGroups(_ context.Context, filters []*ec2.Filter) ([]*ec2.SecurityGroup, error) {
	hash, err := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}
	securityGroups, _ := p.cache.Get(fmt.Sprint(hash))

	return securityGroups.([]*ec2.SecurityGroup), nil
}
