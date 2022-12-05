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

package nodetemplatestatus

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"
)

type SubnetCollector struct {
	ec2api      ec2iface.EC2API
	subnetCache *cache.Cache
	cm          *pretty.ChangeMonitor
}

func NewSubnetCollector(ec2api ec2iface.EC2API, sc *cache.Cache, changeManger *pretty.ChangeMonitor) *SubnetCollector {
	return &SubnetCollector{
		ec2api:      ec2api,
		subnetCache: sc,
		cm:          changeManger,
	}
}

func (s *SubnetCollector) getListOfSubnets(ctx context.Context, requestName string, nodeTemplate *v1alpha1.AWSNodeTemplate) error {
	subnetFilters := s.getSubnetFilters(&nodeTemplate.Spec.AWS)

	subnetHash, err := hashstructure.Hash(subnetFilters, hashstructure.FormatV2, nil)
	if err != nil {
		return err
	}

	subnetOutput, err := s.getSubnetsFromEC2(ctx, s.ec2api, subnetFilters, nodeTemplate)
	if err != nil {
		return err
	}

	subnetLog := s.prettySubnets(subnetOutput.Subnets)
	s.subnetCache.SetDefault(fmt.Sprint(subnetHash), subnetOutput.Subnets)
	if s.cm.HasChanged(fmt.Sprintf("subnets-ids (%s)", requestName), subnetLog) {
		logging.FromContext(ctx).With("subnets", subnetLog).Debugf("discovered subnets for AWSNodeTemplate (%s)", requestName)
	}

	nodeTemplate.Status.Subnets = nil
	nodeTemplate.Status.Subnets = append(nodeTemplate.Status.Subnets, subnetLog...)

	return nil
}

func (s *SubnetCollector) getSubnetsFromEC2(ctx context.Context, ec2api ec2iface.EC2API, subnetFilters []*ec2.Filter, nodeTemplate *v1alpha1.AWSNodeTemplate) (*ec2.DescribeSubnetsOutput, error) {
	subnetOutput, err := ec2api.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{Filters: subnetFilters})
	if err != nil {
		// Back off and retry to describe the subnets
		return nil, fmt.Errorf("describing subnets %s, %w", pretty.Concise(subnetFilters), err)
	}
	if len(subnetOutput.Subnets) == 0 {
		// Back off and retry to see if there are any new subnets
		return nil, fmt.Errorf("no subnets matched selector %v", nodeTemplate.Spec.AWS.SubnetSelector)
	}

	return subnetOutput, nil
}

func (s *SubnetCollector) getSubnetFilters(provider *v1alpha1.AWS) []*ec2.Filter {
	filters := []*ec2.Filter{}
	// Filter by subnet
	for key, value := range provider.SubnetSelector {
		if key == "aws-ids" {
			filterValues := functional.SplitCommaSeparatedString(value)
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("subnet-id"),
				Values: aws.StringSlice(filterValues),
			})
		} else if value == "*" {
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

func (s *SubnetCollector) prettySubnets(subnets []*ec2.Subnet) []string {
	names := []string{}
	for _, subnet := range subnets {
		names = append(names, fmt.Sprintf("%s (%s)", aws.StringValue(subnet.SubnetId), aws.StringValue(subnet.AvailabilityZone)))
	}
	return names
}
