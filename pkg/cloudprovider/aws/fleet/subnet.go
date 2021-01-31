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

type ZonalSubnets map[string][]*ec2.Subnet

type SubnetProvider struct {
	ec2         ec2iface.EC2API
	subnetCache *cache.Cache
}

func NewSubnetProvider(ec2 ec2iface.EC2API) *SubnetProvider {
	return &SubnetProvider{
		ec2:         ec2,
		subnetCache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (s *SubnetProvider) Get(ctx context.Context, clusterName string) (ZonalSubnets, error) {
	if zonalSubnets, ok := s.subnetCache.Get(clusterName); ok {
		return zonalSubnets.(ZonalSubnets), nil
	}
	return s.getZonalSubnets(ctx, clusterName)
}

func (s *SubnetProvider) getZonalSubnets(ctx context.Context, clusterName string) (ZonalSubnets, error) {
	describeSubnetOutput, err := s.ec2.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("tag-key"),
			Values: []*string{aws.String(fmt.Sprintf(ClusterTagKeyFormat, clusterName))},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("describing subnets, %w", err)
	}

	zonalSubnetMap := ZonalSubnets{}
	for _, subnet := range describeSubnetOutput.Subnets {
		if subnets, ok := zonalSubnetMap[*subnet.AvailabilityZone]; ok {
			zonalSubnetMap[*subnet.AvailabilityZone] = append(subnets, subnet)
		} else {
			zonalSubnetMap[*subnet.AvailabilityZone] = []*ec2.Subnet{subnet}
		}
	}

	s.subnetCache.Set(clusterName, zonalSubnetMap, CacheTTL)
	zap.S().Infof("Successfully discovered subnets in %d zones for cluster %s", len(zonalSubnetMap), clusterName)
	return zonalSubnetMap, nil
}
