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

package fleet

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

type SecurityGroupProvider struct {
	ec2                ec2iface.EC2API
	securityGroupCache *cache.Cache
}

func NewSecurityGroupProvider(ec2 ec2iface.EC2API) *SecurityGroupProvider {
	return &SecurityGroupProvider{
		ec2:                ec2,
		securityGroupCache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (s *SecurityGroupProvider) Get(ctx context.Context, clusterName string) ([]*ec2.SecurityGroup, error) {
	if securityGroups, ok := s.securityGroupCache.Get(clusterName); ok {
		return securityGroups.([]*ec2.SecurityGroup), nil
	}
	return s.getSecurityGroups(ctx, clusterName)
}

func (s *SecurityGroupProvider) getSecurityGroups(ctx context.Context, clusterName string) ([]*ec2.SecurityGroup, error) {
	describeSecurityGroupOutput, err := s.ec2.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("tag-key"),
			Values: []*string{aws.String(fmt.Sprintf(ClusterTagKeyFormat, clusterName))},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("describing security groups with tag key %s, %w", fmt.Sprintf(ClusterTagKeyFormat, clusterName), err)
	}

	securityGroups := describeSecurityGroupOutput.SecurityGroups
	s.securityGroupCache.Set(clusterName, securityGroups, CacheTTL)
	zap.S().Debugf("Successfully discovered %d security groups for cluster %s", len(securityGroups), clusterName)
	return securityGroups, nil
}
