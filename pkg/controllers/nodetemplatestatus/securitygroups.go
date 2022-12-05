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

type SecurityGroupsCollector struct {
	ec2api             ec2iface.EC2API
	securityGroupCache *cache.Cache
	cm                 *pretty.ChangeMonitor
}

func NewSecurityGroupsCollector(ec2api ec2iface.EC2API, sgc *cache.Cache, changeMangeer *pretty.ChangeMonitor) *SecurityGroupsCollector {
	return &SecurityGroupsCollector{
		ec2api:             ec2api,
		securityGroupCache: sgc,
		cm:                 changeMangeer,
	}
}

func (s *SecurityGroupsCollector) getListOfSecurityGroups(ctx context.Context, requestName string, nodeTemplate *v1alpha1.AWSNodeTemplate) ([]string, error) {
	filters := s.getSecurityGroupFilters(&nodeTemplate.Spec.AWS)

	securityGroupHash, err := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}

	securityGroupOutput, err := s.getSecurityGroupsFromEC2(ctx, s.ec2api, filters, nodeTemplate)
	if err != nil {
		return nil, err
	}

	securityGroupIdsList := s.securityGroupIds(securityGroupOutput.SecurityGroups)
	s.securityGroupCache.SetDefault(fmt.Sprint(securityGroupHash), securityGroupOutput.SecurityGroups)
	if s.cm.HasChanged("security-groups", securityGroupOutput.SecurityGroups) {
		logging.FromContext(ctx).With("security-groups", securityGroupIdsList).Debugf("discovered security groups for AWSNodeTemplate (%s)", requestName)
	}

	return securityGroupIdsList, nil
}

func (s *SecurityGroupsCollector) getSecurityGroupsFromEC2(ctx context.Context, ec2api ec2iface.EC2API, securityGroupFilters []*ec2.Filter, nodeTemplate *v1alpha1.AWSNodeTemplate) (*ec2.DescribeSecurityGroupsOutput, error) {
	securityGroupOutput, err := s.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{Filters: securityGroupFilters})
	if err != nil {
		// Back off and retry to describe the subnets
		return nil, fmt.Errorf("describing security groups %+v, %w", securityGroupFilters, err)
	}

	return securityGroupOutput, nil
}

func (s *SecurityGroupsCollector) getSecurityGroupFilters(provider *v1alpha1.AWS) []*ec2.Filter {
	filters := []*ec2.Filter{}
	for key, value := range provider.SecurityGroupSelector {
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

func (s *SecurityGroupsCollector) securityGroupIds(securityGroups []*ec2.SecurityGroup) []string {
	names := []string{}
	for _, securityGroup := range securityGroups {
		names = append(names, aws.StringValue(securityGroup.GroupId))
	}
	return names
}
