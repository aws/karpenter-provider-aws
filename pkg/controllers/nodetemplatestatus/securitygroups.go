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

// Collects the Security Groups information and stores the inforamtion in the cache
// return a list of Security Group ids
func (s *SecurityGroupsCollector) getListOfSecurityGroups(ctx context.Context, requestName string, nodeTemplate *v1alpha1.AWSNodeTemplate) ([]string, error) {
	filters := utils.GetSecurityGroupFilters(nodeTemplate)

	securityGroupHash, err := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}

	securityGroupOutput, err := s.getSecurityGroupsFromEC2(ctx, filters)
	if err != nil {
		return nil, err
	}

	securityGroupIdsList := utils.SecurityGroupIds(securityGroupOutput.SecurityGroups)
	s.securityGroupCache.SetDefault(fmt.Sprint(securityGroupHash), securityGroupOutput.SecurityGroups)
	if s.cm.HasChanged(fmt.Sprintf("security-groups (%s)", requestName), securityGroupOutput.SecurityGroups) {
		logging.FromContext(ctx).With("security-groups", securityGroupIdsList).Debugf("discovered security groups for AWSNodeTemplate (%s)", requestName)
	}

	return securityGroupIdsList, nil
}

// Creates a call to EC2 to request the Security Group information
func (s *SecurityGroupsCollector) getSecurityGroupsFromEC2(ctx context.Context, securityGroupFilters []*ec2.Filter) (*ec2.DescribeSecurityGroupsOutput, error) {
	securityGroupOutput, err := s.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{Filters: securityGroupFilters})
	if err != nil {
		// Back off and retry to describe the subnets
		return nil, fmt.Errorf("describing security groups %+v, %w", securityGroupFilters, err)
	}

	return securityGroupOutput, nil
}
