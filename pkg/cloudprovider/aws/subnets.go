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
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

type SubnetProvider struct {
	ec2api ec2iface.EC2API
	cache  *cache.Cache
}

func NewSubnetProvider(ec2api ec2iface.EC2API) *SubnetProvider {
	return &SubnetProvider{
		ec2api: ec2api,
		cache:  cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (s *SubnetProvider) GetZonalSubnets(ctx context.Context, clusterName string) (map[string][]*ec2.Subnet, error) {
	if zonalSubnets, ok := s.cache.Get(clusterName); ok {
		return zonalSubnets.(map[string][]*ec2.Subnet), nil
	}
	zonalSubnets, err := s.getZonalSubnets(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	s.cache.Set(clusterName, zonalSubnets, CacheTTL)
	zap.S().Debugf("Successfully discovered subnets in %d zones for cluster %s", len(zonalSubnets), clusterName)
	return zonalSubnets, nil
}

func (s *SubnetProvider) getZonalSubnets(ctx context.Context, clusterName string) (map[string][]*ec2.Subnet, error) {
	describeSubnetOutput, err := s.ec2api.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("tag-key"),
			Values: []*string{aws.String(fmt.Sprintf(ClusterTagKeyFormat, clusterName))},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("describing subnets, %w", err)
	}
	zonalSubnetMap := map[string][]*ec2.Subnet{}
	for _, subnet := range describeSubnetOutput.Subnets {
		zonalSubnetMap[*subnet.AvailabilityZone] = append(zonalSubnetMap[*subnet.AvailabilityZone], subnet)
	}
	return zonalSubnetMap, nil
}
