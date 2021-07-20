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

func (s *SubnetProvider) Get(ctx context.Context, provisioner *v1alpha3.Provisioner, constraints *Constraints) ([]*ec2.Subnet, error) {
	// 1. Get all viable subnets for this provisioner
	subnets, err := s.getSubnets(ctx, provisioner)
	if err != nil {
		return nil, err
	}
	// 2. Filter by subnet name if constrained
	if name := constraints.GetSubnetName(); name != nil {
		subnets = filterSubnets(subnets, withSubnetTags(predicates.HasNameTag(*name)))
	}
	// 3. Filter by subnet tag key if constrained
	if tagKey := constraints.GetSubnetTagKey(); tagKey != nil {
		subnets = filterSubnets(subnets, withSubnetTags(predicates.HasTagKey(*tagKey)))
	}
	// 4. Filter by zones if constrained
	if len(constraints.Zones) != 0 {
		subnets = filterSubnets(subnets, withSubnetZone(predicates.WithinStrings(constraints.Zones)))
	}
	// 4. Fail if no subnets found
	if len(subnets) == 0 {
		return nil, fmt.Errorf("no subnets exist given constraints")
	}
	return subnets, nil
}

func (s *SubnetProvider) getSubnets(ctx context.Context, provisioner *v1alpha3.Provisioner) ([]*ec2.Subnet, error) {
	clusterName := ptr.StringValue(provisioner.Spec.Cluster.Name)
	if subnets, ok := s.cache.Get(clusterName); ok {
		return subnets.([]*ec2.Subnet), nil
	}
	output, err := s.ec2api.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{Filters: []*ec2.Filter{{
		Name:   aws.String("tag-key"), // Subnets must be tagged for the cluster
		Values: []*string{aws.String(fmt.Sprintf(ClusterTagKeyFormat, clusterName))},
	}}})
	if err != nil {
		return nil, fmt.Errorf("describing subnets, %w", err)
	}
	s.cache.Set(clusterName, output.Subnets, CacheTTL)
	logging.FromContext(ctx).Debugf("Discovered %d subnets for cluster %s", len(output.Subnets), clusterName)
	return output.Subnets, nil
}

func filterSubnets(subnets []*ec2.Subnet, predicate func(subnet *ec2.Subnet) bool) (result []*ec2.Subnet) {
	for _, subnet := range subnets {
		if predicate(subnet) {
			result = append(result, subnet)
		}
	}
	return result
}

func withSubnetTags(predicate func([]*ec2.Tag) bool) func(subnet *ec2.Subnet) bool {
	return func(subnet *ec2.Subnet) bool { return predicate(subnet.Tags) }
}

func withSubnetZone(predicate func(string) bool) func(subnet *ec2.Subnet) bool {
	return func(subnet *ec2.Subnet) bool { return predicate(aws.StringValue(subnet.AvailabilityZone)) }
}
