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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/utils/predicates"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
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

func (s *SecurityGroupProvider) Get(ctx context.Context, provisioner *v1alpha3.Provisioner, constraints *Constraints) ([]*ec2.SecurityGroup, error) {
	// 1. Get Security Groups
	securityGroups, err := s.getSecurityGroups(ctx, ptr.StringValue(provisioner.Spec.Cluster.Name))
	if err != nil {
		return nil, err
	}
	// 2. Filter by subnet name if constrained
	if name := constraints.GetSecurityGroupName(); name != nil {
		securityGroups = filterSecurityGroups(securityGroups, withSecurityGroupTags(predicates.HasNameTag(*name)))
	}
	// 3. Filter by security group tag key if constrained
	if tagKey := constraints.GetSecurityGroupTagKey(); tagKey != nil {
		securityGroups = filterSecurityGroups(securityGroups, withSecurityGroupTags(predicates.HasTagKey(*tagKey)))
	}
	// 4. Fail if no security groups found, since the constraints may be
	// violated and node cannot connect to the API Server.
	if len(securityGroups) == 0 {
		return nil, fmt.Errorf("no security groups exist given constraints")
	}
	return securityGroups, nil
}

func (s *SecurityGroupProvider) getSecurityGroups(ctx context.Context, clusterName string) ([]*ec2.SecurityGroup, error) {
	if securityGroups, ok := s.cache.Get(clusterName); ok {
		return securityGroups.([]*ec2.SecurityGroup), nil
	}
	output, err := s.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("tag-key"), // Security Groups must be tagged for the cluster
			Values: []*string{aws.String(fmt.Sprintf(ClusterTagKeyFormat, clusterName))},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("describing security groups with tag key %s, %w", fmt.Sprintf(ClusterTagKeyFormat, clusterName), err)
	}
	s.cache.Set(clusterName, output.SecurityGroups, CacheTTL)
	logging.FromContext(ctx).Debugf("Discovered %d security groups for cluster %s", len(output.SecurityGroups), clusterName)
	return output.SecurityGroups, nil
}

func filterSecurityGroups(securityGroups []*ec2.SecurityGroup, predicate func(securityGroup *ec2.SecurityGroup) bool) (result []*ec2.SecurityGroup) {
	for _, securityGroup := range securityGroups {
		if predicate(securityGroup) {
			result = append(result, securityGroup)
		}
	}
	return result
}

func withSecurityGroupTags(predicate func([]*ec2.Tag) bool) func(securityGroup *ec2.SecurityGroup) bool {
	return func(securityGroup *ec2.SecurityGroup) bool { return predicate(securityGroup.Tags) }
}
