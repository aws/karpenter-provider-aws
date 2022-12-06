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

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils"
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

// Collects the Subnet information and stores the information in the cache
// return a list of Subnet ids
func (s *SubnetCollector) getListOfSubnets(ctx context.Context, requestName string, nodeTemplate *v1alpha1.AWSNodeTemplate) ([]string, error) {
	filters := utils.GetSubnetFilters(nodeTemplate)

	subnetHash, err := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}

	subnetOutput, err := s.getSubnetsFromEC2(ctx, filters, nodeTemplate)
	if err != nil {
		return nil, err
	}

	subnetLog := utils.SubnetIds(subnetOutput.Subnets)
	s.subnetCache.SetDefault(fmt.Sprint(subnetHash), subnetOutput.Subnets)
	if s.cm.HasChanged(fmt.Sprintf("subnets-ids (%s)", requestName), subnetLog) {
		logging.FromContext(ctx).With("subnets", subnetLog).Debugf("discovered subnets for AWSNodeTemplate (%s)", requestName)
	}

	return subnetLog, nil
}

// Creates a call to EC2 to request the Security Group information
func (s *SubnetCollector) getSubnetsFromEC2(ctx context.Context, subnetFilters []*ec2.Filter, nodeTemplate *v1alpha1.AWSNodeTemplate) (*ec2.DescribeSubnetsOutput, error) {
	subnetOutput, err := s.ec2api.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{Filters: subnetFilters})
	if err != nil {
		return nil, fmt.Errorf("describing subnets %s, %w", pretty.Concise(subnetFilters), err)
	}
	if len(subnetOutput.Subnets) == 0 {
		return nil, fmt.Errorf("no subnets matched selector %v", nodeTemplate.Spec.AWS.SubnetSelector)
	}

	return subnetOutput, nil
}
