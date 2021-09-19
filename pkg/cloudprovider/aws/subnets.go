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
	v1alpha1 "github.com/awslabs/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"
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

func (s *SubnetProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints) ([]*ec2.Subnet, error) {
	// Get subnets
	subnets, err := s.getSubnets(ctx, s.getFilters(ctx, constraints))
	if err != nil {
		return nil, err
	}
	// Fail if no subnets found
	if len(subnets) == 0 {
		return nil, fmt.Errorf("no subnets exist given constraints")
	}
	// Return subnets
	return subnets, nil
}

func (s *SubnetProvider) getFilters(ctx context.Context, constraints *v1alpha1.Constraints) []*ec2.Filter {
	filters := []*ec2.Filter{}
	// Filter by zone
	if constraints.Zones != nil {
		filters = append(filters, &ec2.Filter{
			Name: aws.String("availability-zone"),
			Values: aws.StringSlice(constraints.Zones),
		})
	}
	// Filter by selector
	for key, value := range constraints.SubnetSelector {
		if value == "" {
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

func (s *SubnetProvider) getSubnets(ctx context.Context, filters []*ec2.Filter) ([]*ec2.Subnet, error) {
	hash, err := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}
	if subnets, ok := s.cache.Get(fmt.Sprint(hash)); ok {
		return subnets.([]*ec2.Subnet), nil
	}
	output, err := s.ec2api.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("describing subnets %+v, %w", filters, err)
	}
	s.cache.Set(fmt.Sprint(hash), output.Subnets, CacheTTL)
	logging.FromContext(ctx).Debugf("Discovered subnets: %s", s.subnetIds(output.Subnets))
	return output.Subnets, nil
}

func (s *SubnetProvider) subnetIds(subnets []*ec2.Subnet) []string {
	names := []string{}
	for _, subnet := range subnets {
		names = append(names, aws.StringValue(subnet.SubnetId))
	}
	return names
}
