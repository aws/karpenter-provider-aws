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

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/options"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"
)

type SecurityGroupProvider struct {
	ec2api ec2iface.EC2API
	cache  *cache.Cache
}

func NewSecurityGroupProvider(ec2api ec2iface.EC2API) *SecurityGroupProvider {
	return &SecurityGroupProvider{
		ec2api: ec2api,
		cache:  cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (s *SecurityGroupProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints) ([]string, error) {
	// Get SecurityGroups
	securityGroups, err := s.getSecurityGroups(ctx, s.getFilters(constraints))
	if err != nil {
		return nil, err
	}
	// This hack works around
	// https://github.com/kubernetes-sigs/aws-load-balancer-controller/issues/2367
	// The LoadBalancer Controller expects a single security group with the
	// cluster tag, but provisioning tools like eksctl and kops create multiple.
	securityGroups = filterClusterTaggedGroups(ctx, securityGroups)

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

func (s *SecurityGroupProvider) getFilters(constraints *v1alpha1.Constraints) []*ec2.Filter {
	filters := []*ec2.Filter{}
	for key, value := range constraints.SecurityGroupSelector {
		if value == "*" {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(key)},
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

func (s *SecurityGroupProvider) getSecurityGroups(ctx context.Context, filters []*ec2.Filter) ([]*ec2.SecurityGroup, error) {
	hash, err := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}
	if securityGroups, ok := s.cache.Get(fmt.Sprint(hash)); ok {
		return securityGroups.([]*ec2.SecurityGroup), nil
	}
	output, err := s.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("describing security groups %+v, %w", filters, err)
	}
	s.cache.SetDefault(fmt.Sprint(hash), output.SecurityGroups)
	logging.FromContext(ctx).Debugf("Discovered security groups: %s", securityGroupIds(output.SecurityGroups))
	return output.SecurityGroups, nil
}

func filterClusterTaggedGroups(ctx context.Context, securityGroups []*ec2.SecurityGroup) []*ec2.SecurityGroup {
	filtered := []*ec2.SecurityGroup{}
	foundClusterTag := false
	for _, securityGroup := range securityGroups {
		if hasClusterTag(ctx, securityGroup) {
			if foundClusterTag {
				logging.FromContext(ctx).Debugf("Ignoring security group %s, only one group with tag %s is allowed", aws.StringValue(securityGroup.GroupId),
					fmt.Sprint(v1alpha1.ClusterDiscoveryTagKeyFormat, options.Get(ctx).ClusterName))
				continue
			}
			foundClusterTag = true
		}
		filtered = append(filtered, securityGroup)
	}
	return filtered
}

func hasClusterTag(ctx context.Context, securityGroup *ec2.SecurityGroup) bool {
	for _, tag := range securityGroup.Tags {
		if aws.StringValue(tag.Key) == fmt.Sprintf(v1alpha1.ClusterDiscoveryTagKeyFormat, options.Get(ctx).ClusterName) {
			return true
		}
	}
	return false
}

func securityGroupIds(securityGroups []*ec2.SecurityGroup) []string {
	names := []string{}
	for _, securityGroup := range securityGroups {
		names = append(names, aws.StringValue(securityGroup.GroupId))
	}
	return names
}
